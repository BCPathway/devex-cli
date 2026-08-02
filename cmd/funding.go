package cmd

import (
	"github.com/spf13/cobra"
)

// fundingCmd is the parent command for all funding operations (Drips and Stellar).
var fundingCmd = &cobra.Command{
	Use:   "funding",
	Short: "Manage open-source funding streams and split distributions (Drips & Stellar)",
	Long: `The funding command group provides subcommands for interacting
with decentralized funding networks (Drips/Ethereum and Stellar Network) —
inspecting dependencies, managing split configurations, and querying on-chain
funding state.

Use the --network flag on any subcommand to select between 'drips' (default)
and 'stellar'.

Subcommands:
  inspect   Scan dependencies and propose a funding split configuration
  generate  Generate a local version-controlled split config (.devex.drips.yaml / .devex.stellar.yaml)
  diff      Show a Git-style diff comparing local config with known on-chain state
  sync      Execute an on-chain transaction to sync local config with the target network
  status    Query live on-chain state and incoming funding telemetry
  split     Configure or preview split rules directly`,
}

func init() {
	rootCmd.AddCommand(fundingCmd)
}
