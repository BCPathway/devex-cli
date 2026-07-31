package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/keychain"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Manage Drips Network wallet private keys securely in the OS keychain",
	Long: `The wallet command group provides secure storage and management of your
private key using your operating system's native credential manager
(macOS Keychain, Linux Secret Service, or Windows Credential Manager).

Private keys are never saved in plain text on disk. You can also override
keychain storage by setting the DRIPS_PRIVATE_KEY environment variable.

Subcommands:
  import    Securely import a private key into the OS keychain
  remove    Remove the stored private key from the OS keychain
  address   Display the Ethereum address of the stored private key`,
}

var walletImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Securely import a private key into the OS keychain",
	Long: `Prompts securely for your secp256k1 Ethereum private key and stores it
in the OS credential manager under the service name 'devex-cli'.

Input is masked while typing. You may also pass --key for scripted imports.`,
	RunE: runWalletImport,
}

var walletRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Delete the stored private key from the OS keychain",
	RunE: runWalletRemove,
}

var walletAddressCmd = &cobra.Command{
	Use:   "address",
	Short: "Display the Ethereum address of the stored private key",
	RunE: runWalletAddress,
}

var importKeyFlag string

func init() {
	walletImportCmd.Flags().StringVar(&importKeyFlag, "key", "", "private key hex string (optional; prompt if omitted)")

	walletCmd.AddCommand(walletImportCmd)
	walletCmd.AddCommand(walletRemoveCmd)
	walletCmd.AddCommand(walletAddressCmd)

	rootCmd.AddCommand(walletCmd)
}

func runWalletImport(cmd *cobra.Command, args []string) error {
	key := importKeyFlag
	if key == "" {
		fmt.Print("🔑 Enter private key (input will be hidden): ")
		if term.IsTerminal(int(os.Stdin.Fd())) {
			bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println() // new line after hidden input
			if err != nil {
				return fmt.Errorf("reading private key securely: %w", err)
			}
			key = string(bytePassword)
		} else {
			// Fallback if not attached to a TTY terminal (e.g. piped stdin).
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reading private key from stdin: %w", err)
			}
			key = strings.TrimSpace(line)
		}
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("private key cannot be empty")
	}

	logger.Debug("wallet import: validating and storing key…")
	if err := keychain.StoreKey(key); err != nil {
		return fmt.Errorf("failed to store private key: %w", err)
	}

	addr, err := keychain.GetStoredAddress()
	if err != nil {
		return fmt.Errorf("key stored but address derivation failed: %w", err)
	}

	printOutput(map[string]string{"status": "imported", "address": addr}, func() {
		fmt.Println()
		fmt.Printf("  ✅  Securely stored private key in OS keychain.\n")
		fmt.Printf("  ─────────────────────────────────────────────────────\n")
		fmt.Printf("  Wallet Address: %s\n", addr)
		fmt.Println()
	})

	return nil
}

func runWalletRemove(cmd *cobra.Command, args []string) error {
	if err := keychain.RemoveKey(); err != nil {
		return fmt.Errorf("removing private key: %w", err)
	}

	printOutput(map[string]string{"status": "removed"}, func() {
		fmt.Println()
		fmt.Println("  ✅  Removed private key from OS keychain.")
		fmt.Println()
	})

	return nil
}

func runWalletAddress(cmd *cobra.Command, args []string) error {
	addr, err := keychain.GetStoredAddress()
	if err != nil {
		if errors.Is(err, keychain.ErrNoKeyStored) {
			return errors.New("no wallet key found — run 'devex wallet import' or set DRIPS_PRIVATE_KEY")
		}
		return fmt.Errorf("retrieving stored wallet address: %w", err)
	}

	printOutput(map[string]string{"address": addr}, func() {
		fmt.Println()
		fmt.Printf("  Wallet Address: %s\n", addr)
		fmt.Println()
	})

	return nil
}
