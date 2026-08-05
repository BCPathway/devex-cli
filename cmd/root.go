// Package cmd implements the CLI command tree for devex.
//
// Architecture:
//   - root.go: Root command, persistent flags, config bootstrap
//   - init.go: Interactive project initialization
//   - dev.go: Local development environment orchestration
//   - funding.go: Parent command for Drips Network funding operations
//   - funding_status.go: Query Drips streams and splits
//   - funding_split.go: Configure Drips split rules
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/BCPathway/devex-cli/internal/config"
	"github.com/BCPathway/devex-cli/internal/env"
	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/spf13/cobra"
)

var (
	// Persistent flag values bound at the root level.
	cfgFile        string
	verbose        bool
	jsonOutput     bool
	ciMode         bool
	nonInteractive bool

	// Global application config, hydrated during PersistentPreRun.
	appConfig *config.Config
)

// rootCmd is the base command for the devex CLI.
var rootCmd = &cobra.Command{
	Use:   "devex",
	Short: "DevEx CLI — developer workflow automation with Drips Network integration",
	Long: `devex is a command-line tool that streamlines developer workflows
and integrates with the Drips Network for transparent, on-chain
funding streams and dependency splits.

Use 'devex init' to bootstrap a new project, 'devex dev' to spin up
your local environment, and 'devex funding' to manage Drips streams.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialise structured logger.
		logger.Init(verbose)

		// Initialise CI environment detection.
		env.Init()
		if ciMode || nonInteractive {
			env.SetCI(true)
		}

		// Load configuration (file + env).
		cfg, err := config.Load(cfgFile)
		if err != nil {
			// Config is optional for some commands (e.g. init), so we
			// only warn rather than fatally exit here.
			logger.Debug("config load skipped or partial: %v", err)
			cfg = config.Default()
		}
		appConfig = cfg

		logger.Debug("config loaded: rpc=%s, chain=%d", cfg.Drips.RPCEndpoint, cfg.Drips.ChainID)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./.devex.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose/debug output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output results as JSON")
	rootCmd.PersistentFlags().BoolVar(&ciMode, "ci", false, "run in CI/non-interactive mode (suppresses prompts and colors)")
	rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "alias for --ci")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once.
func Execute() error {
	return rootCmd.Execute()
}

// --------------------------------------------------------------------------
// Shared output helpers
// --------------------------------------------------------------------------

// printOutput renders data either as JSON or as formatted text depending on
// the --json flag. This keeps output logic consistent across subcommands.
func printOutput(data any, textFormatter func()) {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to encode JSON: %v\n", err)
		}
		return
	}
	textFormatter()
}
