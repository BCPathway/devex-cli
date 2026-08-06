package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Spin up the local development environment",
	Long: `Starts the local development environment as defined in .devex.yaml.

By default this runs the configured dev.start_command (e.g. 'docker compose up').
Use --detach to run containers in the background.`,
	RunE: runDev,
}

var devDetach bool

func init() {
	devCmd.Flags().BoolVarP(&devDetach, "detach", "d", false, "run in detached/background mode")
	rootCmd.AddCommand(devCmd)
}

func runDev(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
	}

	startCmd := appConfig.Dev.StartCommand
	if startCmd == "" {
		return fmt.Errorf("dev.start_command is not set in .devex.yaml")
	}

	// Append --detach flag for docker compose if requested.
	if devDetach && strings.HasPrefix(startCmd, "docker compose") {
		startCmd += " -d"
	}

	logger.Info("starting dev environment: %s", startCmd)

	parts := strings.Fields(startCmd)
	if len(parts) == 0 {
		return fmt.Errorf("dev.start_command is empty")
	}

	// Build and execute the command, streaming stdout/stderr to the terminal.
	proc := exec.Command(parts[0], parts[1:]...)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Stdin = os.Stdin

	if err := proc.Run(); err != nil {
		return fmt.Errorf("dev environment exited with error: %w", err)
	}

	logger.Info("dev environment stopped cleanly")
	return nil
}
