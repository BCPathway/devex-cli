package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/drips"
	"github.com/BCPathway/devex-cli/pkg/stellar"
	"github.com/spf13/cobra"
)

var fundingSplitCmd = &cobra.Command{
	Use:   "split",
	Short: "Configure or preview Drips or Stellar split rules",
	Long: `Manages the split configuration for the current project.

In preview mode (default), displays what the split allocation would
look like without submitting any transactions. Use --apply to write
the split configuration on-chain (or update .devex.stellar.yaml for Stellar).

Use --network stellar to configure rules for Stellar recipients (G...).

Example:
  devex funding split --add 0xabc...123=25 --add 0xdef...456=10
  devex funding split --network stellar --add G...=50 --add G...=50
  devex funding split --preview
  devex funding split --apply`,
	RunE: runFundingSplit,
}

var (
	splitAddEntries []string
	splitPreview    bool
	splitApply      bool
	splitNetwork    string
)

func init() {
	fundingSplitCmd.Flags().StringArrayVar(&splitAddEntries, "add", nil,
		"add a split entry as ADDRESS=PERCENT (can be repeated)")
	fundingSplitCmd.Flags().BoolVar(&splitPreview, "preview", false,
		"preview the split configuration without applying")
	fundingSplitCmd.Flags().BoolVar(&splitApply, "apply", false,
		"apply the split configuration on-chain (requires wallet)")
	fundingSplitCmd.Flags().StringVar(&splitNetwork, "network", "drips",
		"target network: 'drips' (Ethereum) or 'stellar'")

	fundingCmd.AddCommand(fundingSplitCmd)
}

func runFundingSplit(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
	}

	if strings.ToLower(splitNetwork) == "stellar" {
		return runStellarFundingSplit(cmd, args)
	}

	client, err := drips.NewClient(drips.ClientConfig{
		RPCEndpoint:   appConfig.Drips.RPCEndpoint,
		ChainID:       appConfig.Drips.ChainID,
		WalletAddress: appConfig.Drips.WalletAddress,
	})
	if err != nil {
		return fmt.Errorf("initialising Drips client: %w", err)
	}

	// Parse --add entries into SplitEntry slice.
	entries, err := parseSplitEntries(splitAddEntries)
	if err != nil {
		return err
	}

	// If no entries provided, show current configuration.
	if len(entries) == 0 && !splitApply {
		return showCurrentSplits(client)
	}

	// Validate total weight.
	totalWeight := 0
	for _, e := range entries {
		totalWeight += e.Weight
	}
	if totalWeight > 100 {
		return fmt.Errorf("total split weight %d%% exceeds 100%%", totalWeight)
	}

	// Preview mode (default unless --apply is set).
	if !splitApply {
		return previewSplits(entries, totalWeight)
	}

	// Apply mode — confirm before transacting.
	logger.Info("⚡  applying split configuration on chain %d…", appConfig.Drips.ChainID)

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("This will submit an on-chain transaction. Continue? [y/N]: ")
	answer, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(answer)) != "y" {
		fmt.Println("Aborted.")
		return nil
	}

	txHash, err := client.SetSplits(appConfig.Drips.WalletAddress, entries)
	if err != nil {
		return fmt.Errorf("setting splits: %w", err)
	}

	result := splitApplyResult{
		TxHash:  txHash,
		Entries: entries,
	}

	printOutput(result, func() {
		fmt.Printf("Splits configured successfully\n")
		fmt.Printf("    Transaction: %s\n", txHash)
	})

	return nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func parseSplitEntries(raw []string) ([]drips.SplitEntry, error) {
	var entries []drips.SplitEntry
	for _, r := range raw {
		parts := strings.SplitN(r, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid split entry %q — expected ADDRESS=PERCENT", r)
		}
		weight, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || weight < 0 || weight > 100 {
			return nil, fmt.Errorf("invalid weight in %q — must be 0-100", r)
		}
		entries = append(entries, drips.SplitEntry{
			Receiver: strings.TrimSpace(parts[0]),
			Weight:   weight,
		})
	}
	return entries, nil
}

func showCurrentSplits(client *drips.Client) error {
	splits, err := client.GetSplits(appConfig.Drips.WalletAddress)
	if err != nil {
		return fmt.Errorf("fetching current splits: %w", err)
	}

	printOutput(splits, func() {
		if len(splits) == 0 {
			fmt.Println("No splits configured for this account.")
			return
		}
		fmt.Println("── Current Split Configuration ──")
		for _, sp := range splits {
			fmt.Printf("  → %s  %d%%\n", sp.Receiver, sp.Weight)
		}
	})
	return nil
}

func previewSplits(entries []drips.SplitEntry, total int) error {
	fmt.Println("── Split Preview (dry-run) ──")
	for _, e := range entries {
		fmt.Printf("  → %s  %d%%\n", e.Receiver, e.Weight)
	}
	fmt.Printf("  ─────────────────────\n")
	fmt.Printf("  Total allocated: %d%%\n", total)
	fmt.Printf("  Remainder:       %d%%\n", 100-total)
	fmt.Println("\nUse --apply to submit this configuration on-chain.")
	return nil
}

// splitApplyResult is the structured output for a successful split application.
type splitApplyResult struct {
	TxHash  string             `json:"tx_hash"`
	Entries []drips.SplitEntry `json:"entries"`
}

func runStellarFundingSplit(_ *cobra.Command, _ []string) error {
	cfgPath := ".devex.stellar.yaml"
	entries, err := parseStellarSplitEntries(splitAddEntries)
	if err != nil {
		return err
	}

	if len(entries) == 0 && !splitApply {
		if _, err := os.Stat(cfgPath); err != nil {
			fmt.Println("No Stellar splits configured (.devex.stellar.yaml not found).")
			return nil
		}
		cfg, err := stellar.LoadStellarConfig(cfgPath)
		if err != nil {
			return err
		}
		fmt.Println("── Current Stellar Split Configuration ──")
		for _, sp := range cfg.Splits {
			fmt.Printf("  → %s (%s)  %d%%\n", sp.DependencyName, sp.StellarAddress, sp.Percentage)
		}
		return nil
	}

	totalWeight := 0
	for _, e := range entries {
		totalWeight += e.Weight
	}
	if totalWeight > 100 {
		return fmt.Errorf("total Stellar split weight %d%% exceeds 100%%", totalWeight)
	}

	if !splitApply {
		fmt.Println("── Stellar Split Preview (dry-run) ──")
		for _, e := range entries {
			fmt.Printf("  → %s  %d%%\n", e.Receiver, e.Weight)
		}
		fmt.Printf("  ─────────────────────\n")
		fmt.Printf("  Total allocated: %d%%\n", totalWeight)
		fmt.Printf("  Remainder:       %d%%\n", 100-totalWeight)
		fmt.Println("\nUse --apply to update .devex.stellar.yaml with this configuration.")
		return nil
	}

	logger.Info("⚡  applying Stellar split configuration to %s…", cfgPath)
	cfg := &stellar.StellarConfigFile{
		AccountID: appConfig.Stellar.AccountID,
		Network:   "testnet",
		Splits:    make([]stellar.StellarSplitConfig, 0, len(entries)),
	}
	for _, e := range entries {
		cfg.Splits = append(cfg.Splits, stellar.StellarSplitConfig{
			DependencyName: "custom:" + e.Receiver[:8],
			StellarAddress: e.Receiver,
			Percentage:     e.Weight,
		})
	}

	if err := stellar.SaveStellarConfig(cfgPath, cfg); err != nil {
		return fmt.Errorf("saving %s: %w", cfgPath, err)
	}

	printOutput(cfg, func() {
		fmt.Printf("Stellar splits saved to %s successfully\n", cfgPath)
		fmt.Println("  💡 Run 'devex funding sync --network stellar' to execute this distribution on Stellar Network.")
	})

	return nil
}

func parseStellarSplitEntries(raw []string) ([]stellar.StellarSplitEntry, error) {
	var entries []stellar.StellarSplitEntry
	for _, r := range raw {
		parts := strings.SplitN(r, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid split entry %q — expected STELLAR_ADDRESS=PERCENT", r)
		}
		addr := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(addr, "G") || len(addr) != 56 {
			return nil, fmt.Errorf("invalid Stellar recipient address %q — must start with G and be 56 characters", addr)
		}
		weight, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || weight < 0 || weight > 100 {
			return nil, fmt.Errorf("invalid weight in %q — must be 0-100", r)
		}
		entries = append(entries, stellar.StellarSplitEntry{
			Receiver: addr,
			Weight:   weight,
		})
	}
	return entries, nil
}
