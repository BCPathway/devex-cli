package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/drips"
	"github.com/BCPathway/devex-cli/pkg/stellar"
	"github.com/spf13/cobra"
)

var fundingDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show a Git-style diff comparing local config (.devex.drips.yaml / .devex.stellar.yaml) with live on-chain splits",
	Long: `Reads the local split definitions from .devex.drips.yaml or .devex.stellar.yaml
and compares them against the live on-chain split rules retrieved from the
target network.

Displays added, removed, modified, and unchanged split percentages so developers
can verify changes before executing an on-chain transaction with 'devex funding sync'.

Use --network stellar to diff your Stellar split configuration.

Examples:
  devex funding diff
  devex funding diff --network stellar
  devex funding diff --config-path ./custom.yaml
  devex funding diff --json`,
	RunE: runFundingDiff,
}

var (
	diffConfigPath string
	diffNetwork    string
)

func init() {
	fundingDiffCmd.Flags().StringVar(&diffConfigPath, "config-path", "", "path to local configuration file (default .devex.drips.yaml or .devex.stellar.yaml)")
	fundingDiffCmd.Flags().StringVar(&diffNetwork, "network", "drips", "target network: 'drips' (Ethereum) or 'stellar'")
	fundingCmd.AddCommand(fundingDiffCmd)
}

func runFundingDiff(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
	}

	if strings.ToLower(diffNetwork) == "stellar" {
		return runStellarFundingDiff(cmd, args)
	}

	if diffConfigPath == "" {
		diffConfigPath = ".devex.drips.yaml"
	}

	if _, err := os.Stat(diffConfigPath); err != nil {
		return fmt.Errorf("local config %s not found — run 'devex funding generate' first", diffConfigPath)
	}

	localCfg, err := drips.LoadDripsConfig(diffConfigPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", diffConfigPath, err)
	}

	targetID := localCfg.ProjectID
	if targetID == "" {
		targetID = resolveStatusTargetID()
	}
	if targetID == "" {
		return fmt.Errorf("could not determine target Drips ID or address for diff")
	}

	logger.Info("📡  fetching live on-chain splits for %s on chain %d…", targetID, appConfig.Drips.ChainID)
	subgraph := drips.NewSubgraphClient(appConfig.Drips.ChainID)
	telemetry, err := subgraph.QueryStatusTelemetry(context.Background(), targetID)
	if err != nil {
		return fmt.Errorf("querying on-chain telemetry: %w", err)
	}

	diff := drips.CalculateSplitsDiff(targetID, appConfig.Drips.ChainID, localCfg.Splits, telemetry.ActiveSplits)

	printOutput(diff, func() {
		renderSplitsDiffUI(diffConfigPath, diff)
	})

	return nil
}

// renderSplitsDiffUI displays a clean, Git-style diff in the terminal.
func renderSplitsDiffUI(configPath string, diff *drips.SplitsDiff) {
	fmt.Println()
	fmt.Printf("  🔍  Drips Splits Diff — %s vs On-Chain (Chain %d)\n", configPath, diff.ChainID)
	fmt.Printf("  Account: %s\n", diff.ProjectID)
	fmt.Println()

	if len(diff.Items) == 0 {
		fmt.Println("  (No splits found in local config or on-chain)")
		return
	}

	fmt.Printf("  ┌────────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("  │  STATUS     DEPENDENCY / RECEIVER           OLD %%      NEW %%            │\n")
	fmt.Printf("  ├────────────────────────────────────────────────────────────────────────┤\n")

	for _, item := range diff.Items {
		name := truncate(item.Identifier, 30)
		switch item.Type {
		case drips.DiffAdded:
			fmt.Printf("  │  + ADDED    %-30s   —      ->  %3d%%            │\n", name, item.NewPercentage)
		case drips.DiffRemoved:
			fmt.Printf("  │  - REMOVED  %-30s  %3d%%    ->   —             │\n", name, item.OldPercentage)
		case drips.DiffModified:
			fmt.Printf("  │  ~ MODIFIED %-30s  %3d%%    ->  %3d%%            │\n", name, item.OldPercentage, item.NewPercentage)
		case drips.DiffUnchanged:
			fmt.Printf("  │  = UNCHANGED %-29s  %3d%%    (no change)       │\n", name, item.OldPercentage)
		}
	}
	fmt.Printf("  └────────────────────────────────────────────────────────────────────────┘\n")

	fmt.Println()
	fmt.Printf("  ── Summary ──────────────────────────────────────\n")
	fmt.Printf("  Added:      %d dependencies\n", diff.AddedCount)
	fmt.Printf("  Modified:   %d dependencies\n", diff.ModifiedCount)
	fmt.Printf("  Removed:    %d dependencies\n", diff.RemovedCount)
	fmt.Printf("  Unchanged:  %d dependencies\n", diff.UnchangedCount)
	fmt.Println()

	if diff.HasChanges() {
		fmt.Println("  💡 Run 'devex funding sync' to execute this split change on-chain.")
	} else {
		fmt.Println("  💡 Local configuration is identical to on-chain splits. Nothing to sync.")
	}
	fmt.Println()
}

func runStellarFundingDiff(_ *cobra.Command, _ []string) error {
	cfgPath := diffConfigPath
	if cfgPath == "" {
		cfgPath = ".devex.stellar.yaml"
	}

	if _, err := os.Stat(cfgPath); err != nil {
		return fmt.Errorf("local config %s not found — run 'devex funding generate --network stellar' first", cfgPath)
	}

	localCfg, err := stellar.LoadStellarConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	accountID := localCfg.AccountID
	if accountID == "" {
		accountID = resolveStellarStatusTargetID(cfgPath)
	}
	if accountID == "" {
		return fmt.Errorf("could not determine target Stellar Account ID for diff")
	}

	logger.Info("📡  fetching live Stellar telemetry for %s…", accountID)
	client, err := stellar.NewClient(stellar.ClientConfig{
		HorizonURL:        appConfig.Stellar.HorizonURL,
		NetworkPassphrase: appConfig.Stellar.NetworkPassphrase,
		AccountID:         accountID,
	})
	if err != nil {
		return fmt.Errorf("initialising Stellar client: %w", err)
	}
	defer client.Close()

	telemetry, err := client.QueryStatusTelemetry(accountID)
	if err != nil {
		return fmt.Errorf("querying Stellar telemetry: %w", err)
	}

	netName := localCfg.Network
	if netName == "" {
		netName = "testnet"
	}
	var prevSplits []stellar.StellarSplitConfig
	if len(telemetry.RecentPayments) > 0 {
		prevSplits = make([]stellar.StellarSplitConfig, 0, len(telemetry.RecentPayments))
		for _, p := range telemetry.RecentPayments {
			prevSplits = append(prevSplits, stellar.StellarSplitConfig{
				StellarAddress: p.To,
			})
		}
	}
	diff := stellar.CalculateStellarSplitsDiff(accountID, netName, localCfg.Splits, prevSplits)

	printOutput(diff, func() {
		renderStellarSplitsDiffUI(cfgPath, diff)
	})

	return nil
}

func renderStellarSplitsDiffUI(configPath string, diff *stellar.StellarSplitsDiff) {
	fmt.Println()
	fmt.Printf("  🔍  Stellar Splits Diff — %s vs Recorded Payments\n", configPath)
	fmt.Printf("  Account: %s\n", diff.AccountID)
	fmt.Println()

	if len(diff.Items) == 0 {
		fmt.Println("  (No splits found in local config or payment history)")
		return
	}

	fmt.Printf("  ┌────────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("  │  STATUS     DEPENDENCY / RECEIVER           OLD %%      NEW %%            │\n")
	fmt.Printf("  ├────────────────────────────────────────────────────────────────────────┤\n")

	for _, item := range diff.Items {
		name := truncate(item.Identifier, 30)
		switch item.Type {
		case stellar.DiffAdded:
			fmt.Printf("  │  + ADDED    %-30s   —      ->  %3d%%            │\n", name, item.NewPercentage)
		case stellar.DiffRemoved:
			fmt.Printf("  │  - REMOVED  %-30s  %3d%%    ->   —             │\n", name, item.OldPercentage)
		case stellar.DiffModified:
			fmt.Printf("  │  ~ MODIFIED %-30s  %3d%%    ->  %3d%%            │\n", name, item.OldPercentage, item.NewPercentage)
		case stellar.DiffUnchanged:
			fmt.Printf("  │  = UNCHANGED %-29s  %3d%%    (no change)       │\n", name, item.OldPercentage)
		}
	}
	fmt.Printf("  └────────────────────────────────────────────────────────────────────────┘\n")

	fmt.Println()
	fmt.Printf("  ── Summary ──────────────────────────────────────\n")
	fmt.Printf("  Added:      %d dependencies\n", diff.AddedCount)
	fmt.Printf("  Modified:   %d dependencies\n", diff.ModifiedCount)
	fmt.Printf("  Removed:    %d dependencies\n", diff.RemovedCount)
	fmt.Printf("  Unchanged:  %d dependencies\n", diff.UnchangedCount)
	fmt.Println()

	if diff.HasChanges() {
		fmt.Println("  💡 Run 'devex funding sync --network stellar' to execute multi-payment split transaction.")
	} else {
		fmt.Println("  💡 Local Stellar configuration is identical to recorded splits. Nothing to sync.")
	}
	fmt.Println()
}
