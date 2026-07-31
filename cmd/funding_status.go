package cmd

import (
	"fmt"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/drips"
	"github.com/spf13/cobra"
)

var fundingStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Query current Drips streams and split configurations",
	Long: `Connects to the Drips Network via the configured RPC endpoint and
retrieves the current state of drip streams, recurring sponsorships,
and split rules for the active wallet/project.`,
	RunE: runFundingStatus,
}

var statusAccountID string

func init() {
	fundingStatusCmd.Flags().StringVar(&statusAccountID, "account-id", "", "Drips account ID to query (overrides config)")
	fundingCmd.AddCommand(fundingStatusCmd)
}

func runFundingStatus(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
	}

	client, err := drips.NewClient(drips.ClientConfig{
		RPCEndpoint:   appConfig.Drips.RPCEndpoint,
		ChainID:       appConfig.Drips.ChainID,
		WalletAddress: appConfig.Drips.WalletAddress,
	})
	if err != nil {
		return fmt.Errorf("initialising Drips client: %w", err)
	}

	accountID := statusAccountID
	if accountID == "" {
		accountID = appConfig.Drips.WalletAddress
	}
	if accountID == "" {
		return fmt.Errorf("no account ID specified — use --account-id or set drips.wallet_address in config")
	}

	logger.Info("📡  querying Drips status for account %s on chain %d…", accountID, appConfig.Drips.ChainID)

	// Fetch streams.
	streams, err := client.GetStreams(accountID)
	if err != nil {
		return fmt.Errorf("fetching streams: %w", err)
	}

	// Fetch splits.
	splits, err := client.GetSplits(accountID)
	if err != nil {
		return fmt.Errorf("fetching splits: %w", err)
	}

	// Fetch balance.
	balance, err := client.GetBalance(accountID)
	if err != nil {
		return fmt.Errorf("fetching balance: %w", err)
	}

	// Output.
	result := statusResult{
		AccountID: accountID,
		ChainID:   appConfig.Drips.ChainID,
		Balance:   balance,
		Streams:   streams,
		Splits:    splits,
	}

	printOutput(result, func() {
		fmt.Printf("Account:  %s\n", result.AccountID)
		fmt.Printf("Chain:    %d\n", result.ChainID)
		fmt.Printf("Balance:  %s\n", result.Balance)
		fmt.Printf("Streams:  %d active\n", len(result.Streams))
		fmt.Printf("Splits:   %d configured\n", len(result.Splits))

		if len(result.Streams) > 0 {
			fmt.Println("\n── Active Streams ──")
			for _, s := range result.Streams {
				fmt.Printf("  → %s  %s/sec  to %s\n", s.ID, s.AmtPerSec, s.Receiver)
			}
		}

		if len(result.Splits) > 0 {
			fmt.Println("\n── Split Configuration ──")
			for _, sp := range result.Splits {
				fmt.Printf("  → %s  %d%%\n", sp.Receiver, sp.Weight)
			}
		}
	})

	return nil
}

// statusResult is the structured output for funding status.
type statusResult struct {
	AccountID string               `json:"account_id"`
	ChainID   int                  `json:"chain_id"`
	Balance   string               `json:"balance"`
	Streams   []drips.StreamInfo   `json:"streams"`
	Splits    []drips.SplitEntry   `json:"splits"`
}
