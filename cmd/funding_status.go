package cmd

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/drips"
	"github.com/spf13/cobra"
)

var fundingStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Query live Drips Network on-chain state and incoming funding telemetry",
	Long: `Connects to the Drips Network Subgraph/API to retrieve the real-time,
on-chain state of the project's funding:
  - Current Balance (splittable and collectable funds)
  - Incoming Streams (active senders and streaming rates)
  - Active On-Chain Splits (current split receivers and percentages)

If neither --drips-id nor --address is provided, the command automatically
reads 'project_id' from the local .devex.drips.yaml file.

Examples:
  devex funding status
  devex funding status --drips-id drips:1:myproject
  devex funding status --address 0x1234567890abcdef1234567890abcdef12345678
  devex funding status --json`,
	RunE: runFundingStatus,
}

var (
	statusDripsID   string
	statusAddress   string
	statusAccountID string // legacy alias
	statusConfig    string
)

func init() {
	fundingStatusCmd.Flags().StringVar(&statusDripsID, "drips-id", "", "Drips Account ID to query")
	fundingStatusCmd.Flags().StringVar(&statusAddress, "address", "", "Ethereum wallet address to query")
	fundingStatusCmd.Flags().StringVar(&statusAccountID, "account-id", "", "alias for --drips-id")
	fundingStatusCmd.Flags().StringVar(&statusConfig, "config-path", ".devex.drips.yaml", "path to local Drips configuration file")

	fundingCmd.AddCommand(fundingStatusCmd)
}

func runFundingStatus(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
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
