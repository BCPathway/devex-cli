package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/drips"
	"github.com/BCPathway/devex-cli/pkg/parser"
	"github.com/BCPathway/devex-cli/pkg/stellar"
	"github.com/spf13/cobra"
)

var fundingGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a local version-controlled split configuration (.devex.drips.yaml / .devex.stellar.yaml)",
	Long: `Reads the local project's dependency manifest and outputs a version-controlled
configuration file (.devex.drips.yaml or .devex.stellar.yaml) so developers can
manually review, adjust, and lock percentages.

Use --network stellar to generate a .devex.stellar.yaml file for Stellar splits.

Examples:
  devex funding generate
  devex funding generate --network stellar
  devex funding generate --manifest ./services/api/go.mod --top-n 15
  devex funding generate --output ./custom-split.yaml --force
  devex funding generate --json`,
	RunE: runFundingGenerate,
}

var (
	generateManifest string
	generateTopN     int
	generateOutput   string
	generateForce    bool
	generateNetwork  string
)

func init() {
	fundingGenerateCmd.Flags().StringVar(&generateManifest, "manifest", "",
		"path to manifest file (default: auto-detect in current directory)")
	fundingGenerateCmd.Flags().IntVar(&generateTopN, "top-n", 20,
		"limit resolution to top N dependencies")
	fundingGenerateCmd.Flags().StringVar(&generateOutput, "output", "",
		"output configuration file path (default: auto-detected by network)")
	fundingGenerateCmd.Flags().BoolVarP(&generateForce, "force", "f", false,
		"overwrite existing configuration without prompting")
	fundingGenerateCmd.Flags().StringVar(&generateNetwork, "network", "drips",
		"target network: 'drips' (Ethereum) or 'stellar'")

	fundingCmd.AddCommand(fundingGenerateCmd)
}

func runFundingGenerate(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
	}

	if strings.ToLower(generateNetwork) == "stellar" {
		return runStellarFundingGenerate(cmd, args)
	}

	if generateOutput == "" {
		generateOutput = ".devex.drips.yaml"
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	p, manifestPath, err := parser.DetectManifest(cwd, generateManifest)
	if err != nil {
		return fmt.Errorf("manifest detection failed: %w", err)
	}

	logger.Info("📦  parsing %s …", manifestPath)

	result, err := p.Parse(manifestPath)
	if err != nil {
		return fmt.Errorf("manifest parsing failed: %w", err)
	}

	if len(result.Dependencies) == 0 {
		return fmt.Errorf("no dependencies found in manifest %s", manifestPath)
	}

	deps := prioritiseDeps(result.Dependencies, generateTopN)
	logger.Info("📡  resolving %d dependencies against Drips Network …", len(deps))

	client, err := drips.NewClient(drips.ClientConfig{
		RPCEndpoint:   appConfig.Drips.RPCEndpoint,
		ChainID:       appConfig.Drips.ChainID,
		WalletAddress: appConfig.Drips.WalletAddress,
	})
	if err != nil {
		return fmt.Errorf("initialising Drips client: %w", err)
	}
	defer client.Close()

	resolver := drips.NewRegistryResolver(client)
	recipients, err := resolver.ResolveDependencies(deps)
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Normalise splits to cap total at 100%.
	recipients = normaliseSplits(recipients, 100)

	newlyGenerated := buildDripsConfigFile(recipients)

	var finalConfig *drips.DripsConfigFile = newlyGenerated
	if _, err := os.Stat(generateOutput); err == nil && !generateForce {
		existing, loadErr := drips.LoadDripsConfig(generateOutput)
		if loadErr != nil {
			logger.Warn("could not load existing %s (%v), prompting for overwrite", generateOutput, loadErr)
		}

		action, promptErr := promptConflictAction(generateOutput, existing != nil && hasLockedSplits(existing))
		if promptErr != nil {
			return fmt.Errorf("reading confirmation: %w", promptErr)
		}

		switch action {
		case "cancel":
			fmt.Println("Aborted.")
			return nil
		case "merge":
			logger.Info("🔀  merging with existing %s (preserving locked entries)…", generateOutput)
			finalConfig = drips.MergeDripsConfig(existing, newlyGenerated)
		case "overwrite":
			logger.Info("⚠️  overwriting existing %s…", generateOutput)
			finalConfig = newlyGenerated
		}
	} else if err == nil && generateForce {
		logger.Info("⚠️  overwriting existing %s due to --force flag…", generateOutput)
	}

	if err := drips.SaveDripsConfig(generateOutput, finalConfig); err != nil {
		return fmt.Errorf("saving configuration to %s: %w", generateOutput, err)
	}

	printOutput(finalConfig, func() {
		renderGenerateSuccess(generateOutput, finalConfig)
	})

	return nil
}

// buildDripsConfigFile creates a DripsConfigFile from resolved recipients.
func buildDripsConfigFile(recipients []drips.DripsRecipient) *drips.DripsConfigFile {
	cfg := &drips.DripsConfigFile{
		ProjectID: resolveProjectID(),
		Splits:    make([]drips.SplitConfig, 0, len(recipients)),
	}

	for _, r := range recipients {
		// Only include recipients that have a recommended split or an address/account ID.
		if r.RecommendedSplitPct > 0 || r.DripsAccountID != "" || r.Address != "" {
			cfg.Splits = append(cfg.Splits, drips.SplitConfig{
				DependencyName: r.DependencyName,
				DripsAccountID: r.DripsAccountID,
				Address:        r.Address,
				Percentage:     r.RecommendedSplitPct,
				Locked:         false,
			})
		}
	}

	return cfg
}

func resolveProjectID() string {
	if appConfig != nil && appConfig.Drips.WalletAddress != "" {
		return appConfig.Drips.WalletAddress
	}
	return "0x0000000000000000000000000000000000000000"
}

func hasLockedSplits(cfg *drips.DripsConfigFile) bool {
	if cfg == nil {
		return false
	}
	for _, sp := range cfg.Splits {
		if sp.Locked {
			return true
		}
	}
	return false
}

func promptConflictAction(path string, canMerge bool) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	if canMerge {
		fmt.Printf("⚠️  %s already exists and has locked splits. [M]erge (preserve locked) / [o]verwrite / [c]ancel [M/o/c]: ", path)
	} else {
		fmt.Printf("⚠️  %s already exists. [O]verwrite / [c]ancel [O/c]: ", path)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(strings.ToLower(line))

	if canMerge {
		switch line {
		case "", "m", "merge":
			return "merge", nil
		case "o", "overwrite":
			return "overwrite", nil
		default:
			return "cancel", nil
		}
	} else {
		switch line {
		case "", "o", "overwrite", "y", "yes":
			return "overwrite", nil
		default:
			return "cancel", nil
		}
	}
}

func renderGenerateSuccess(path string, cfg *drips.DripsConfigFile) {
	fmt.Println()
	fmt.Printf("  ✅  Successfully generated %s\n", path)
	fmt.Printf("  ─────────────────────────────────────────────────────\n")
	fmt.Printf("  Project ID:        %s\n", cfg.ProjectID)
	fmt.Printf("  Configured splits: %d dependencies\n", len(cfg.Splits))

	totalPct := 0
	lockedCount := 0
	for _, sp := range cfg.Splits {
		totalPct += sp.Percentage
		if sp.Locked {
			lockedCount++
		}
	}

	fmt.Printf("  Total percentage:  %d%%\n", totalPct)
	if lockedCount > 0 {
		fmt.Printf("  Locked splits:     %d entries preserved\n", lockedCount)
	}
	fmt.Println()
	fmt.Println("  ── Next Steps ───────────────────────────────────────")
	fmt.Printf("  1. Open %s and adjust percentages as needed.\n", path)
	fmt.Println("  2. Add 'locked: true' to any split you want to prevent from overwriting.")
	fmt.Println("  3. Run 'devex funding status' to inspect live on-chain state.")
	fmt.Println()
}

func runStellarFundingGenerate(cmd *cobra.Command, args []string) error {
	outPath := generateOutput
	if outPath == "" {
		outPath = ".devex.stellar.yaml"
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	p, manifestPath, err := parser.DetectManifest(cwd, generateManifest)
	if err != nil {
		return fmt.Errorf("manifest detection failed: %w", err)
	}

	logger.Info("📦  parsing %s …", manifestPath)
	result, err := p.Parse(manifestPath)
	if err != nil {
		return fmt.Errorf("manifest parsing failed: %w", err)
	}

	if len(result.Dependencies) == 0 {
		return fmt.Errorf("no dependencies found in manifest %s", manifestPath)
	}

	deps := prioritiseDeps(result.Dependencies, generateTopN)
	logger.Info("📡  generating Stellar splits for %d dependencies …", len(deps))

	count := len(deps)
	pctPerDep := 0
	remainder := 0
	if count > 0 {
		pctPerDep = 100 / count
		remainder = 100 % count
	}

	accountID := appConfig.Stellar.AccountID
	if accountID == "" {
		accountID = "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF" // placeholder
	}

	newlyGenerated := &stellar.StellarConfigFile{
		AccountID: accountID,
		Splits:    make([]stellar.StellarSplitConfig, 0, len(deps)),
	}

	for i, d := range deps {
		pct := pctPerDep
		if i == 0 {
			pct += remainder
		}
		newlyGenerated.Splits = append(newlyGenerated.Splits, stellar.StellarSplitConfig{
			DependencyName: d.Name,
			StellarAddress: "G...", // placeholder for user to configure
			Percentage:     pct,
			Locked:         false,
		})
	}

	var finalConfig *stellar.StellarConfigFile = newlyGenerated
	if _, err := os.Stat(outPath); err == nil && !generateForce {
		existing, loadErr := stellar.LoadStellarConfig(outPath)
		if loadErr != nil {
			logger.Warn("could not load existing %s (%v), prompting for overwrite", outPath, loadErr)
		}
		action, promptErr := promptConflictAction(outPath, existing != nil && hasStellarLockedSplits(existing))
		if promptErr != nil {
			return fmt.Errorf("reading confirmation: %w", promptErr)
		}
		switch action {
		case "cancel":
			fmt.Println("Aborted.")
			return nil
		case "merge":
			logger.Info("🔀  merging with existing %s (preserving locked entries)…", outPath)
			finalConfig = stellar.MergeStellarConfig(existing, newlyGenerated)
		case "overwrite":
			logger.Info("⚠️  overwriting existing %s…", outPath)
			finalConfig = newlyGenerated
		}
	} else if err == nil && generateForce {
		logger.Info("⚠️  overwriting existing %s due to --force flag…", outPath)
	}

	if err := stellar.SaveStellarConfig(outPath, finalConfig); err != nil {
		return fmt.Errorf("saving configuration to %s: %w", outPath, err)
	}

	printOutput(finalConfig, func() {
		renderStellarGenerateSuccess(outPath, finalConfig)
	})

	return nil
}

func hasStellarLockedSplits(cfg *stellar.StellarConfigFile) bool {
	if cfg == nil {
		return false
	}
	for _, sp := range cfg.Splits {
		if sp.Locked {
			return true
		}
	}
	return false
}

func renderStellarGenerateSuccess(path string, cfg *stellar.StellarConfigFile) {
	fmt.Println()
	fmt.Printf("  ✅  Successfully generated %s\n", path)
	fmt.Printf("  ─────────────────────────────────────────────────────\n")
	fmt.Printf("  Stellar Account ID: %s\n", cfg.AccountID)
	fmt.Printf("  Configured splits:  %d dependencies\n", len(cfg.Splits))

	totalPct := 0
	lockedCount := 0
	for _, sp := range cfg.Splits {
		totalPct += sp.Percentage
		if sp.Locked {
			lockedCount++
		}
	}

	fmt.Printf("  Total percentage:   %d%%\n", totalPct)
	if lockedCount > 0 {
		fmt.Printf("  Locked splits:      %d entries preserved\n", lockedCount)
	}
	fmt.Println()
	fmt.Println("  ── Next Steps ───────────────────────────────────────")
	fmt.Printf("  1. Open %s and set valid Stellar G... addresses for recipients.\n", path)
	fmt.Println("  2. Adjust percentages as needed (must total <= 100%).")
	fmt.Println("  3. Run 'devex funding diff --network stellar' or 'devex funding sync --network stellar'.")
	fmt.Println()
}
