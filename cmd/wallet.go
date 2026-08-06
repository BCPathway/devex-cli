package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/internal/ui"
	"github.com/BCPathway/devex-cli/pkg/keychain"
	"github.com/spf13/cobra"
)

var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Manage Drips & Stellar private/secret keys securely in the OS keychain",
	Long: `The wallet command group provides secure storage and management of your
private/secret keys using your operating system's native credential manager
(macOS Keychain, Linux Secret Service, or Windows Credential Manager).

Private keys are never saved in plain text on disk. You can also override
keychain storage by setting DRIPS_PRIVATE_KEY or STELLAR_SECRET_KEY env vars.

Use --network stellar on any subcommand to manage your Stellar secret seed.

Subcommands:
  import    Securely import a private key or secret seed into the OS keychain
  remove    Remove the stored private key from the OS keychain
  address   Display the address of the stored private key`,
}

var walletImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Securely import a private key or secret seed into the OS keychain",
	Long: `Prompts securely for your private key (or Stellar secret seed) and stores it
in the OS credential manager under the service name 'devex-cli'.

Input is masked while typing. You may also pass --key for scripted imports.
Use --network stellar to import an Ed25519 Stellar secret seed (S...).`,
	RunE: runWalletImport,
}

var walletRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Delete the stored private key from the OS keychain",
	RunE:  runWalletRemove,
}

var walletAddressCmd = &cobra.Command{
	Use:   "address",
	Short: "Display the address of the stored private key",
	RunE:  runWalletAddress,
}

var (
	importKeyFlag string
	walletNetwork string
)

func init() {
	walletImportCmd.Flags().StringVar(&importKeyFlag, "key", "", "private key hex string (optional; prompt if omitted)")
	walletImportCmd.Flags().StringVar(&walletNetwork, "network", "drips", "target network: 'drips' (Ethereum) or 'stellar'")
	walletRemoveCmd.Flags().StringVar(&walletNetwork, "network", "drips", "target network: 'drips' (Ethereum) or 'stellar'")
	walletAddressCmd.Flags().StringVar(&walletNetwork, "network", "drips", "target network: 'drips' (Ethereum) or 'stellar'")

	walletCmd.AddCommand(walletImportCmd)
	walletCmd.AddCommand(walletRemoveCmd)
	walletCmd.AddCommand(walletAddressCmd)

	rootCmd.AddCommand(walletCmd)
}

func runWalletImport(cmd *cobra.Command, args []string) error {
	key := importKeyFlag
	if key == "" {
		pass, err := ui.PromptPassword("🔑 Enter private key / secret seed (input will be hidden): ")
		if err != nil {
			return fmt.Errorf("reading private key securely: %w", err)
		}
		key = pass
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("private key cannot be empty")
	}

	if strings.ToLower(walletNetwork) == "stellar" {
		logger.Debug("wallet import: validating and storing Stellar key…")
		if err := keychain.StoreStellarKey(key); err != nil {
			return fmt.Errorf("failed to store Stellar secret key: %w", err)
		}

		addr, err := keychain.GetStoredStellarAddress()
		if err != nil {
			return fmt.Errorf("key stored but Stellar address derivation failed: %w", err)
		}

		printOutput(map[string]string{"status": "imported", "network": "stellar", "address": addr}, func() {
			fmt.Println()
			fmt.Printf("  Securely stored Stellar secret key in OS keychain.\n")
			fmt.Printf("  ─────────────────────────────────────────────────────\n")
			fmt.Printf("  Stellar Address: %s\n", addr)
			fmt.Println()
		})
		return nil
	}

	logger.Debug("wallet import: validating and storing key…")
	if err := keychain.StoreKey(key); err != nil {
		return fmt.Errorf("failed to store private key: %w", err)
	}

	addr, err := keychain.GetStoredAddress()
	if err != nil {
		return fmt.Errorf("key stored but address derivation failed: %w", err)
	}

	printOutput(map[string]string{"status": "imported", "network": "drips", "address": addr}, func() {
		fmt.Println()
		fmt.Printf("  Securely stored private key in OS keychain.\n")
		fmt.Printf("  ─────────────────────────────────────────────────────\n")
		fmt.Printf("  Wallet Address: %s\n", addr)
		fmt.Println()
	})

	return nil
}

func runWalletRemove(cmd *cobra.Command, args []string) error {
	if strings.ToLower(walletNetwork) == "stellar" {
		if err := keychain.RemoveStellarKey(); err != nil {
			return fmt.Errorf("removing Stellar secret key: %w", err)
		}

		printOutput(map[string]string{"status": "removed", "network": "stellar"}, func() {
			fmt.Println()
			fmt.Println("  Removed Stellar secret key from OS keychain.")
			fmt.Println()
		})
		return nil
	}

	if err := keychain.RemoveKey(); err != nil {
		return fmt.Errorf("removing private key: %w", err)
	}

	printOutput(map[string]string{"status": "removed", "network": "drips"}, func() {
		fmt.Println()
		fmt.Println("  Removed private key from OS keychain.")
		fmt.Println()
	})

	return nil
}

func runWalletAddress(cmd *cobra.Command, args []string) error {
	if strings.ToLower(walletNetwork) == "stellar" {
		addr, err := keychain.GetStoredStellarAddress()
		if err != nil {
			if errors.Is(err, keychain.ErrNoStellarKeyStored) {
				return errors.New("no Stellar secret key found — run 'devex wallet import --network stellar' or set STELLAR_SECRET_KEY")
			}
			return fmt.Errorf("retrieving stored Stellar address: %w", err)
		}

		printOutput(map[string]string{"address": addr, "network": "stellar"}, func() {
			fmt.Println()
			fmt.Printf("  Stellar Address: %s\n", addr)
			fmt.Println()
		})
		return nil
	}

	addr, err := keychain.GetStoredAddress()
	if err != nil {
		if errors.Is(err, keychain.ErrNoKeyStored) {
			return errors.New("no wallet key found — run 'devex wallet import' or set DRIPS_PRIVATE_KEY")
		}
		return fmt.Errorf("retrieving stored wallet address: %w", err)
	}

	printOutput(map[string]string{"address": addr, "network": "drips"}, func() {
		fmt.Println()
		fmt.Printf("  Wallet Address: %s\n", addr)
		fmt.Println()
	})

	return nil
}
