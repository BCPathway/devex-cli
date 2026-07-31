package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/drips"
	"github.com/BCPathway/devex-cli/pkg/keychain"
	"github.com/spf13/cobra"
)

var fundingSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Execute an on-chain transaction to sync .devex.drips.yaml with Drips Network",
	Long: `Reads the local split configuration from .devex.drips.yaml, computes a state diff
against live on-chain splits, estimates gas, and executes the smart contract
transaction on Drips Network to update your split rules.

Requires an RPC URL (set via --rpc-url, RPC_URL env var, or .devex.yaml) and an
imported private key in the OS keychain ('devex wallet import').

Examples:
  devex funding sync
  devex funding sync --yes
  devex funding sync --config-path ./custom.yaml --rpc-url https://mainnet.optimism.io
  devex funding sync --json`,
	RunE: runFundingSync,
}

var (
	syncConfigPath string
	syncRPCURL     string
	syncYes        bool
)

func init() {
	fundingSyncCmd.Flags().StringVar(&syncConfigPath, "config-path", ".devex.drips.yaml", "path to local Drips configuration file")
	fundingSyncCmd.Flags().StringVar(&syncRPCURL, "rpc-url", "", "Ethereum JSON-RPC URL (overrides RPC_URL env and config)")
	fundingSyncCmd.Flags().BoolVarP(&syncYes, "yes", "y", false, "skip interactive confirmation prompt")

	fundingCmd.AddCommand(fundingSyncCmd)
}

func runFundingSync(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
	}

	// 1. Resolve RPC URL
	rpcURL := resolveRPCURL()
	if rpcURL == "" {
		return errors.New("no RPC URL specified — set RPC_URL env var, use --rpc-url, or configure drips.rpc_endpoint")
	}

	// 2. Load private key from keychain
	privateKey, err := keychain.GetKey()
	if err != nil {
		if errors.Is(err, keychain.ErrNoKeyStored) {
			return errors.New("no wallet key found in keychain — run 'devex wallet import' or set DRIPS_PRIVATE_KEY")
		}
		return fmt.Errorf("loading private key from keychain: %w", err)
	}

	senderAddr, err := keychain.DeriveAddress(privateKey)
	if err != nil {
		return fmt.Errorf("deriving sender address from key: %w", err)
	}

	// 3. Load local .devex.drips.yaml
	if _, err := os.Stat(syncConfigPath); err != nil {
		return fmt.Errorf("local config %s not found — run 'devex funding generate' first", syncConfigPath)
	}

	localCfg, err := drips.LoadDripsConfig(syncConfigPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", syncConfigPath, err)
	}

	// 4. Check live on-chain state & calculate diff
	logger.Info("📡  checking live on-chain splits for %s on chain %d…", senderAddr, appConfig.Drips.ChainID)
	subgraph := drips.NewSubgraphClient(appConfig.Drips.ChainID)
	telemetry, err := subgraph.QueryStatusTelemetry(context.Background(), senderAddr)
	if err != nil {
		return fmt.Errorf("querying on-chain telemetry: %w", err)
	}

	diff := drips.CalculateSplitsDiff(senderAddr, appConfig.Drips.ChainID, localCfg.Splits, telemetry.ActiveSplits)
	if !diff.HasChanges() {
		printOutput(map[string]any{"status": "in_sync", "changes": false}, func() {
			fmt.Println()
			fmt.Println("  ✅  On-chain splits are already in sync with .devex.drips.yaml. No transaction required.")
			fmt.Println()
		})
		return nil
	}

	// 5. Create transaction plan & estimate gas
	logger.Info("⚙️   preparing setSplits transaction and estimating gas…")
	plan, err := drips.CreateSyncTxPlan(context.Background(), rpcURL, appConfig.Drips.ChainID, senderAddr, localCfg.Splits)
	if err != nil {
		return fmt.Errorf("preparing transaction plan: %w", err)
	}

	// 6. Display safety warning and confirmation prompt
	if !syncYes {
		renderSyncWarningUI(diff, plan)
		confirm, err := promptConfirmation("Confirm on-chain transaction? [y/N]: ")
		if err != nil {
			return fmt.Errorf("reading confirmation: %w", err)
		}
		if !confirm {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// 7. Execute transaction on-chain
	logger.Info("🚀  submitting transaction to DripsHub (%s)…", plan.ContractAddress)
	txHash, err := drips.ExecuteSyncTx(context.Background(), rpcURL, appConfig.Drips.ChainID, privateKey, plan)
	if err != nil {
		return fmt.Errorf("executing setSplits transaction: %w", err)
	}

	result := map[string]any{
		"status":                "confirmed",
		"transaction_hash":      txHash,
		"chain_id":              appConfig.Drips.ChainID,
		"sender":                senderAddr,
		"is_offline_simulation": plan.IsOfflineSimulation,
	}

	printOutput(result, func() {
		renderSyncSuccessUI(txHash, plan)
	})

	return nil
}

func resolveRPCURL() string {
	if syncRPCURL != "" {
		return syncRPCURL
	}
	if env := os.Getenv("RPC_URL"); env != "" {
		return env
	}
	if env := os.Getenv("DRIPS_NETWORK_RPC"); env != "" {
		return env
	}
	if appConfig != nil && appConfig.Drips.RPCEndpoint != "" {
		return appConfig.Drips.RPCEndpoint
	}
	return ""
}

func promptConfirmation(promptText string) (bool, error) {
	fmt.Print(promptText)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

func renderSyncWarningUI(diff *drips.SplitsDiff, plan *drips.SyncTxPlan) {
	fmt.Println()
	fmt.Printf("  ⚠️  You are about to update on-chain splits on Drips Network\n")
	fmt.Printf("  ────────────────────────────────────────────────────────────\n")
	fmt.Printf("  Account:        %s\n", plan.SenderAddress)
	fmt.Printf("  Contract:       %s (Chain %d)\n", plan.ContractAddress, plan.ChainID)
	fmt.Printf("  Receivers:      %d split entries configured\n", len(plan.Receivers))
	fmt.Printf("  State Changes:  +%d added, ~%d modified, -%d removed\n",
		diff.AddedCount, diff.ModifiedCount, diff.RemovedCount)
	fmt.Printf("  Estimated Gas:  %s (Limit: %d | Gas Price: %.2f gwei)\n",
		plan.EstimatedCostETH, plan.EstimatedGasLimit, plan.GasPriceGwei)
	if plan.IsOfflineSimulation {
		fmt.Printf("  Mode:           Offline Simulation (RPC fallback)\n")
	}
	fmt.Println()
}

func renderSyncSuccessUI(txHash string, plan *drips.SyncTxPlan) {
	fmt.Println()
	fmt.Printf("  🚀  Transaction submitted successfully!\n")
	fmt.Printf("  ────────────────────────────────────────────────────────────\n")
	fmt.Printf("  Transaction Hash:  %s\n", txHash)
	fmt.Printf("  Status:            Confirmed (Block confirmation received)\n")
	fmt.Printf("  Receivers Updated: %d split rules on contract %s\n", len(plan.Receivers), plan.ContractAddress)
	if plan.IsOfflineSimulation {
		fmt.Printf("  Note:              Executed in offline simulation mode\n")
	}
	fmt.Println()
}
