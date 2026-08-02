package cmd

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/drips"
	"github.com/BCPathway/devex-cli/pkg/stellar"
	"github.com/spf13/cobra"
)

var fundingStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Query live Drips or Stellar Network on-chain state and incoming funding telemetry",
	Long: `Connects to the Drips Network Subgraph or Stellar Horizon API to retrieve the
real-time, on-chain state of the project's funding:
  - Current Balance (XLM or tokens / splittable and collectable funds)
  - Incoming Payments / Streams (active contributors or streaming rates)
  - Active On-Chain Splits (current split receivers and percentages)

Use --network stellar to query a Stellar project account.

Examples:
  devex funding status
  devex funding status --network stellar --account-id G...
  devex funding status --drips-id drips:1:myproject
  devex funding status --json`,
	RunE: runFundingStatus,
}

var (
	statusDripsID   string
	statusAddress   string
	statusAccountID string // legacy alias / Stellar account ID
	statusConfig    string
	statusNetwork   string
)

func init() {
	fundingStatusCmd.Flags().StringVar(&statusDripsID, "drips-id", "", "Drips Account ID to query")
	fundingStatusCmd.Flags().StringVar(&statusAddress, "address", "", "Ethereum wallet address to query")
	fundingStatusCmd.Flags().StringVar(&statusAccountID, "account-id", "", "Stellar or Drips Account ID to query")
	fundingStatusCmd.Flags().StringVar(&statusConfig, "config-path", "", "path to local configuration file (default .devex.drips.yaml or .devex.stellar.yaml)")
	fundingStatusCmd.Flags().StringVar(&statusNetwork, "network", "drips", "target network: 'drips' (Ethereum) or 'stellar'")

	fundingCmd.AddCommand(fundingStatusCmd)
}

func runFundingStatus(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
	}

	if statusNetwork == "stellar" {
		cfgPath := statusConfig
		if cfgPath == "" {
			cfgPath = ".devex.stellar.yaml"
		}
		targetID := resolveStellarStatusTargetID(cfgPath)
		if targetID == "" {
			return fmt.Errorf("no Stellar Account ID specified — pass --account-id, create %s with 'devex funding generate --network stellar', or set stellar.account_id in config", cfgPath)
		}

		logger.Info("📡  querying Stellar Horizon telemetry for %s…", targetID)
		client, err := stellar.NewClient(stellar.ClientConfig{
			HorizonURL:        appConfig.Stellar.HorizonURL,
			NetworkPassphrase: appConfig.Stellar.NetworkPassphrase,
			AccountID:         targetID,
		})
		if err != nil {
			return fmt.Errorf("initialising Stellar client: %w", err)
		}
		defer client.Close()

		telemetry, err := client.QueryStatusTelemetry(targetID)
		if err != nil {
			return fmt.Errorf("querying Stellar telemetry: %w", err)
		}

		printOutput(telemetry, func() {
			renderStellarStatusTelemetryUI(telemetry)
		})
		return nil
	}

	if statusConfig == "" {
		statusConfig = ".devex.drips.yaml"
	}

	targetID := resolveStatusTargetID()
	if targetID == "" {
		return fmt.Errorf("no Drips ID or address specified — pass --drips-id/--address, create %s with 'devex funding generate', or set drips.wallet_address in config", statusConfig)
	}

	logger.Info("📡  querying Drips Subgraph telemetry for %s on chain %d…", targetID, appConfig.Drips.ChainID)

	subgraph := drips.NewSubgraphClient(appConfig.Drips.ChainID)
	telemetry, err := subgraph.QueryStatusTelemetry(context.Background(), targetID)
	if err != nil {
		return fmt.Errorf("querying on-chain telemetry: %w", err)
	}

	printOutput(telemetry, func() {
		renderStatusTelemetryUI(telemetry)
	})

	return nil
}

// resolveStatusTargetID finds the target Drips ID or address by precedence:
// 1. --drips-id / --address / --account-id flag
// 2. project_id in .devex.drips.yaml
// 3. appConfig.Drips.WalletAddress
func resolveStatusTargetID() string {
	if statusDripsID != "" {
		return statusDripsID
	}
	if statusAddress != "" {
		return statusAddress
	}
	if statusAccountID != "" {
		return statusAccountID
	}

	if _, err := os.Stat(statusConfig); err == nil {
		if localCfg, loadErr := drips.LoadDripsConfig(statusConfig); loadErr == nil && localCfg.ProjectID != "" {
			logger.Debug("status: using project_id %q from %s", localCfg.ProjectID, statusConfig)
			return localCfg.ProjectID
		}
	}

	if appConfig != nil && appConfig.Drips.WalletAddress != "" {
		return appConfig.Drips.WalletAddress
	}

	return ""
}

// renderStatusTelemetryUI displays the 3 required sections in a clean terminal UI.
func renderStatusTelemetryUI(t *drips.StatusTelemetry) {
	fmt.Println()
	fmt.Printf("  📡  Drips Network On-Chain Telemetry (Chain %d)\n", t.ChainID)
	fmt.Printf("  Account:  %s\n", t.AccountID)
	if t.Address != "" && t.Address != t.AccountID {
		fmt.Printf("  Address:  %s\n", t.Address)
	}
	if t.Source != "" {
		fmt.Printf("  Source:   %s\n", t.Source)
	}
	fmt.Println()

	// ── Section 1: Current Balance ────────────────────────────────────
	fmt.Printf("  ┌────────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("  │  💰  CURRENT BALANCE                                                   │\n")
	fmt.Printf("  ├────────────────────────────────────────────────────────────────────────┤\n")
	fmt.Printf("  │  Splittable Balance:   %-48s│\n", t.SplittableBalance)
	fmt.Printf("  │  Collectable Balance:  %-48s│\n", t.CollectableBalance)
	fmt.Printf("  └────────────────────────────────────────────────────────────────────────┘\n")
	fmt.Println()

	// ── Section 2: Incoming Streams ───────────────────────────────────
	fmt.Printf("  ┌────────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("  │  🌊  INCOMING STREAMS (%d active)                                       │\n", len(t.IncomingStreams))
	fmt.Printf("  ├────────────────────────────────────────────────────────────────────────┤\n")
	if len(t.IncomingStreams) == 0 {
		fmt.Printf("  │  (No active incoming streams found)                                    │\n")
	} else {
		for i, st := range t.IncomingStreams {
			sender := st.SenderAccountID
			if sender == "" {
				sender = st.SenderAddress
			}
			rateHuman := formatWeiPerSecToMonthly(st.AmtPerSec, st.TokenSymbol)
			fmt.Printf("  │  %d. From: %-28s Rate: %-24s│\n", i+1, truncate(sender, 28), rateHuman)
		}
	}
	fmt.Printf("  └────────────────────────────────────────────────────────────────────────┘\n")
	fmt.Println()

	// ── Section 3: Active On-Chain Splits ─────────────────────────────
	fmt.Printf("  ┌────────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("  │  ⚡  ACTIVE ON-CHAIN SPLITS (%d configured)                             │\n", len(t.ActiveSplits))
	fmt.Printf("  ├────────────────────────────────────────────────────────────────────────┤\n")
	if len(t.ActiveSplits) == 0 {
		fmt.Printf("  │  (No active split rules configured on contract)                        │\n")
	} else {
		for i, sp := range t.ActiveSplits {
			receiver := sp.ReceiverAccountID
			if receiver == "" {
				receiver = sp.ReceiverAddress
			}
			fmt.Printf("  │  %d. Receiver: %-32s Allocation: %3d%%          │\n",
				i+1, truncate(receiver, 32), sp.Percentage)
		}
	}
	fmt.Printf("  └────────────────────────────────────────────────────────────────────────┘\n")
	fmt.Println()
}

// formatWeiPerSecToMonthly converts wei/sec to a monthly human-readable string.
func formatWeiPerSecToMonthly(amtPerSec, symbol string) string {
	rate, ok := new(big.Int).SetString(amtPerSec, 10)
	if !ok || rate.Sign() == 0 {
		return "0 " + symbol + "/sec"
	}

	// 30 days in seconds = 2,592,000
	monthSec := big.NewInt(2592000)
	monthlyWei := new(big.Int).Mul(rate, monthSec)

	// Convert wei to ETH (10^18)
	ethDiv := big.NewFloat(1e18)
	monthlyEth, _ := new(big.Float).SetInt(monthlyWei).Quo(new(big.Float).SetInt(monthlyWei), ethDiv).Float64()

	return fmt.Sprintf("~%.2f %s/month", monthlyEth, symbol)
}

func resolveStellarStatusTargetID(configPath string) string {
	if statusAccountID != "" {
		return statusAccountID
	}
	if statusDripsID != "" {
		return statusDripsID
	}

	if _, err := os.Stat(configPath); err == nil {
		if localCfg, loadErr := stellar.LoadStellarConfig(configPath); loadErr == nil && localCfg.AccountID != "" {
			logger.Debug("status: using account_id %q from %s", localCfg.AccountID, configPath)
			return localCfg.AccountID
		}
	}

	if appConfig != nil && appConfig.Stellar.AccountID != "" {
		return appConfig.Stellar.AccountID
	}

	return ""
}

func renderStellarStatusTelemetryUI(t *stellar.StellarStatusTelemetry) {
	fmt.Println()
	fmt.Printf("  📡  Stellar Network On-Chain Telemetry (%s)\n", strings.ToUpper(t.Network))
	fmt.Printf("  Account:  %s\n", t.AccountID)
	if t.Source != "" {
		fmt.Printf("  Source:   %s\n", t.Source)
	}
	fmt.Println()

	// ── Section 1: Current Balance ────────────────────────────────────
	fmt.Printf("  ┌────────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("  │  💰  CURRENT BALANCE                                                   │\n")
	fmt.Printf("  ├────────────────────────────────────────────────────────────────────────┤\n")
	fmt.Printf("  │  Native XLM Balance:   %-48s│\n", t.XLMBalance)
	for _, ob := range t.OtherBalances {
		fmt.Printf("  │  %-21s %-48s│\n", truncate(ob.Asset, 21)+":", ob.Balance)
	}
	fmt.Printf("  └────────────────────────────────────────────────────────────────────────┘\n")
	fmt.Println()

	// ── Section 2: Incoming Payments / History ────────────────────────
	fmt.Printf("  ┌────────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("  │  🌊  RECENT INCOMING PAYMENTS (%d found)                                │\n", len(t.RecentPayments))
	fmt.Printf("  ├────────────────────────────────────────────────────────────────────────┤\n")
	if len(t.RecentPayments) == 0 {
		fmt.Printf("  │  (No recent incoming payments found)                                   │\n")
	} else {
		for i, p := range t.RecentPayments {
			fmt.Printf("  │  %d. From: %-28s Amount: %-22s│\n", i+1, truncate(p.From, 28), p.Amount+" "+p.Asset)
		}
	}
	fmt.Printf("  └────────────────────────────────────────────────────────────────────────┘\n")
	fmt.Println()
}
