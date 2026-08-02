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
	"github.com/BCPathway/devex-cli/pkg/stellar"
	"github.com/spf13/cobra"
)

var fundingSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Execute an on-chain transaction to sync local split config with target network",
	Long: `Reads the local split configuration (.devex.drips.yaml or .devex.stellar.yaml),
computes a state diff, estimates fees, and executes the transaction on the target
network (Drips Network Ethereum contract or Stellar multi-payment transaction).

Requires an imported private key or secret seed in the OS keychain ('devex wallet import').

Examples:
  devex funding sync
  devex funding sync --network stellar --amount 50.0000000
  devex funding sync --yes
  devex funding sync --config-path ./custom.yaml --rpc-url https://mainnet.optimism.io
  devex funding sync --json`,
	RunE: runFundingSync,
}

var (
	syncConfigPath string
	syncRPCURL     string
	syncYes        bool
	syncNetwork    string
	syncAmount     string
)

func init() {
	fundingSyncCmd.Flags().StringVar(&syncConfigPath, "config-path", "", "path to local configuration file (default .devex.drips.yaml or .devex.stellar.yaml)")
	fundingSyncCmd.Flags().StringVar(&syncRPCURL, "rpc-url", "", "Ethereum JSON-RPC URL (overrides RPC_URL env and config)")
	fundingSyncCmd.Flags().BoolVarP(&syncYes, "yes", "y", false, "skip interactive confirmation prompt")
	fundingSyncCmd.Flags().StringVar(&syncNetwork, "network", "drips", "target network: 'drips' (Ethereum) or 'stellar'")
	fundingSyncCmd.Flags().StringVar(&syncAmount, "amount", "10.0000000", "XLM amount to distribute across splits (Stellar network only)")

	fundingCmd.AddCommand(fundingSyncCmd)
}

func runFundingSync(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
	}

	if strings.ToLower(syncNetwork) == "stellar" {
		return runStellarFundingSync(cmd, args)
	}

	if syncConfigPath == "" {
		syncConfigPath = ".devex.drips.yaml"
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

func runStellarFundingSync(cmd *cobra.Command, args []string) error {
	cfgPath := syncConfigPath
	if cfgPath == "" {
		cfgPath = ".devex.stellar.yaml"
	}

	secretKey, err := keychain.GetStellarKey()
	if err != nil {
		if errors.Is(err, keychain.ErrNoStellarKeyStored) {
			return errors.New("no Stellar secret key found in keychain — run 'devex wallet import --network stellar' or set STELLAR_SECRET_KEY")
		}
		return fmt.Errorf("loading Stellar secret key from keychain: %w", err)
	}

	sourceAddr, err := keychain.DeriveStellarAddress(secretKey)
	if err != nil {
		return fmt.Errorf("deriving Stellar sender address from secret key: %w", err)
	}

	if _, err := os.Stat(cfgPath); err != nil {
		return fmt.Errorf("local config %s not found — run 'devex funding generate --network stellar' first", cfgPath)
	}

	localCfg, err := stellar.LoadStellarConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	netName := localCfg.Network
	if netName == "" {
		netName = "testnet"
	}

	splits := make([]stellar.StellarSplitEntry, 0, len(localCfg.Splits))
	for _, s := range localCfg.Splits {
		splits = append(splits, stellar.StellarSplitEntry{
			Receiver: s.StellarAddress,
			Weight:   s.Percentage,
		})
	}

	logger.Info("⚙️   preparing Stellar split multi-payment transaction (%s XLM total)…", syncAmount)
	plan, err := stellar.CreateSplitTxPlan(sourceAddr, syncAmount, splits, netName)
	if err != nil {
		return fmt.Errorf("preparing Stellar split transaction plan: %w", err)
	}

	if !syncYes {
		renderStellarSyncWarningUI(plan)
		confirm, err := promptConfirmation("Confirm Stellar multi-payment transaction? [y/N]: ")
		if err != nil {
			return fmt.Errorf("reading confirmation: %w", err)
		}
		if !confirm {
			fmt.Println("Aborted.")
			return nil
		}
	}

	logger.Info("🚀  submitting multi-payment transaction to Stellar Horizon (%s)…", appConfig.Stellar.HorizonURL)
	txHash, err := stellar.ExecuteSplitTx(context.Background(), appConfig.Stellar.HorizonURL, appConfig.Stellar.NetworkPassphrase, secretKey, plan)
	if err != nil {
		return fmt.Errorf("executing Stellar split transaction: %w", err)
	}

	result := map[string]any{
		"status":           "confirmed",
		"network":          "stellar",
		"transaction_hash": txHash,
		"source_account":   sourceAddr,
		"total_xlm":        plan.TotalAmount,
		"payments_count":   len(plan.Payments),
	}

	printOutput(result, func() {
		renderStellarSyncSuccessUI(txHash, plan)
	})

	return nil
}

func renderStellarSyncWarningUI(plan *stellar.SplitTxPlan) {
	fmt.Println()
	fmt.Printf("  ⚠️  You are about to execute a Stellar multi-payment distribution\n")
	fmt.Printf("  ────────────────────────────────────────────────────────────\n")
	fmt.Printf("  Source Account: %s\n", plan.SourceAccount)
	fmt.Printf("  Network:        %s\n", strings.ToUpper(plan.NetworkName))
	fmt.Printf("  Total Amount:   %s XLM\n", plan.TotalAmount)
	fmt.Printf("  Payments:       %d destination addresses\n", len(plan.Payments))
	fmt.Printf("  Total Fee:      %s XLM (%d stroops)\n", plan.TotalFeeXLM, plan.BaseFeeStroops*int64(plan.OperationCount))
	if plan.IsSimulation {
		fmt.Printf("  Mode:           Offline Simulation\n")
	}
	fmt.Println()
}

func renderStellarSyncSuccessUI(txHash string, plan *stellar.SplitTxPlan) {
	fmt.Println()
	fmt.Printf("  🚀  Stellar transaction submitted successfully!\n")
	fmt.Printf("  ────────────────────────────────────────────────────────────\n")
	fmt.Printf("  Transaction Hash:  %s\n", txHash)
	fmt.Printf("  Status:            Confirmed\n")
	fmt.Printf("  Payments Sent:     %d recipients (%s XLM total)\n", len(plan.Payments), plan.TotalAmount)
	if plan.IsSimulation {
		fmt.Printf("  Note:              Executed in offline simulation mode\n")
	}
	fmt.Println()
}
