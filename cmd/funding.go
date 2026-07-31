package cmd

import (
	"github.com/spf13/cobra"
)

// fundingCmd is the parent command for all Drips Network funding operations.
var fundingCmd = &cobra.Command{
	Use:   "funding",
	Short: "Manage Drips Network funding streams and splits",
	Long: `The funding command group provides subcommands for interacting
with the Drips Network — inspecting active drip streams, managing
split configurations, and querying on-chain funding state.

Subcommands:
  inspect   Scan dependencies and propose a Drips funding split
  status    Query current Drips streams and split configurations
  split     Configure or preview Drips split rules`,
}

func init() {
	rootCmd.AddCommand(fundingCmd)
}
