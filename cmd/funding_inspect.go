package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/drips"
	"github.com/BCPathway/devex-cli/pkg/parser"
	"github.com/spf13/cobra"
)

var fundingInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Scan dependencies and propose a Drips funding split",
	Long: `Reads the local project's dependency manifest (go.mod or package.json),
resolves each dependency against the Drips Network registry, and outputs
a formatted split proposal showing which dependencies have registered
Drips accounts and what percentage allocation is recommended.

Examples:
  devex funding inspect
  devex funding inspect --manifest ./services/api/go.mod
  devex funding inspect --top-n 10 --json
  devex funding inspect --verbose`,
	RunE: runFundingInspect,
}

var (
	inspectManifest string
	inspectTopN     int
	inspectNetwork  string
)

func init() {
	fundingInspectCmd.Flags().StringVar(&inspectManifest, "manifest", "",
		"path to manifest file (default: auto-detect in current directory)")
	fundingInspectCmd.Flags().IntVar(&inspectTopN, "top-n", 20,
		"limit resolution to top N dependencies")
	fundingInspectCmd.Flags().StringVar(&inspectNetwork, "network", "drips",
		"target network: 'drips' (Ethereum) or 'stellar'")

	fundingCmd.AddCommand(fundingInspectCmd)
}

func runFundingInspect(cmd *cobra.Command, args []string) error {
	if appConfig == nil {
		return fmt.Errorf("no configuration loaded — run 'devex init' first")
	}

	// ── Step 1: Detect and parse manifest ──────────────────────────────
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	p, manifestPath, err := parser.DetectManifest(cwd, inspectManifest)
	if err != nil {
		return fmt.Errorf("manifest detection failed: %w", err)
	}

	logger.Info("📦  parsing %s …", manifestPath)

	result, err := p.Parse(manifestPath)
	if err != nil {
		return fmt.Errorf("manifest parsing failed: %w", err)
	}

	if len(result.Dependencies) == 0 {
		fmt.Println("No dependencies found in manifest.")
		return nil
	}

	if strings.ToLower(inspectNetwork) == "stellar" {
		return runStellarFundingInspect(result)
	}

	// Filter to direct dependencies first, then fill with indirect up to topN.
	deps := prioritiseDeps(result.Dependencies, inspectTopN)
	logger.Info("📡  resolving %d dependencies against Drips Network …", len(deps))

	// ── Step 2: Resolve against Drips registry ────────────────────────
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

	// ── Step 3: Build and render output ───────────────────────────────
	output := buildInspectOutput(result, recipients)

	printOutput(output, func() {
		renderInspectTable(output)
	})

	return nil
}

// --------------------------------------------------------------------------
// Output types
// --------------------------------------------------------------------------

// inspectOutput is the complete structured output for the inspect command.
type inspectOutput struct {
	Project       string                `json:"project"`
	ManifestPath  string                `json:"manifest_path"`
	ManifestType  string                `json:"manifest_type"`
	TotalScanned  int                   `json:"total_scanned"`
	TotalResolved int                   `json:"total_resolved"`
	Recipients    []drips.DripsRecipient `json:"recipients"`
	Summary       inspectSummary        `json:"summary"`
}

// inspectSummary contains aggregate metrics for the scan.
type inspectSummary struct {
	Verified       int `json:"verified"`
	Escrowed       int `json:"escrowed"`
	Unregistered   int `json:"unregistered"`
	TotalSplitPct  int `json:"total_split_pct"`
}

// --------------------------------------------------------------------------
// Output construction
// --------------------------------------------------------------------------

func buildInspectOutput(parseResult *parser.ParseResult, recipients []drips.DripsRecipient) inspectOutput {
	summary := inspectSummary{}
	for _, r := range recipients {
		switch r.Status {
		case drips.StatusVerified:
			summary.Verified++
		case drips.StatusEscrow:
			summary.Escrowed++
		case drips.StatusUnregistered:
			summary.Unregistered++
		}
		summary.TotalSplitPct += r.RecommendedSplitPct
	}

	// Cap total at 100%.
	if summary.TotalSplitPct > 100 {
		recipients = normaliseSplits(recipients, 100)
		summary.TotalSplitPct = 100
	}

	return inspectOutput{
		Project:       parseResult.ProjectName,
		ManifestPath:  parseResult.ManifestPath,
		ManifestType:  string(parseResult.Type),
		TotalScanned:  len(recipients),
		TotalResolved: summary.Verified + summary.Escrowed,
		Recipients:    recipients,
		Summary:       summary,
	}
}

// normaliseSplits proportionally scales down split percentages so the total
// does not exceed maxPct.
func normaliseSplits(recipients []drips.DripsRecipient, maxPct int) []drips.DripsRecipient {
	total := 0
	for _, r := range recipients {
		total += r.RecommendedSplitPct
	}
	if total <= maxPct {
		return recipients
	}

	scale := float64(maxPct) / float64(total)
	adjusted := make([]drips.DripsRecipient, len(recipients))
	copy(adjusted, recipients)

	newTotal := 0
	for i := range adjusted {
		adjusted[i].RecommendedSplitPct = int(float64(adjusted[i].RecommendedSplitPct) * scale)
		newTotal += adjusted[i].RecommendedSplitPct
	}

	// Distribute rounding remainder to the first non-zero recipient.
	remainder := maxPct - newTotal
	for i := range adjusted {
		if remainder <= 0 {
			break
		}
		if adjusted[i].RecommendedSplitPct > 0 {
			adjusted[i].RecommendedSplitPct += remainder
			break
		}
	}

	return adjusted
}

// --------------------------------------------------------------------------
// Dependency prioritisation
// --------------------------------------------------------------------------

// prioritiseDeps returns up to topN dependencies, preferring direct deps
// over indirect ones, and sorting alphabetically within each tier.
func prioritiseDeps(deps []parser.Dependency, topN int) []parser.Dependency {
	// Separate direct and indirect.
	var direct, indirect []parser.Dependency
	for _, d := range deps {
		if d.Direct {
			direct = append(direct, d)
		} else {
			indirect = append(indirect, d)
		}
	}

	// Sort each group alphabetically.
	sortDeps := func(s []parser.Dependency) {
		sort.Slice(s, func(i, j int) bool {
			return s[i].Name < s[j].Name
		})
	}
	sortDeps(direct)
	sortDeps(indirect)

	// Take direct first, then fill with indirect.
	result := make([]parser.Dependency, 0, topN)
	for _, d := range direct {
		if len(result) >= topN {
			break
		}
		result = append(result, d)
	}
	for _, d := range indirect {
		if len(result) >= topN {
			break
		}
		result = append(result, d)
	}

	return result
}

// --------------------------------------------------------------------------
// Table rendering
// --------------------------------------------------------------------------

// renderInspectTable outputs a formatted terminal table with the inspection
// results. Uses box-drawing characters for a clean, modern look.
func renderInspectTable(output inspectOutput) {
	// Header.
	fmt.Println()
	fmt.Printf("  📦  %s\n", output.Project)
	fmt.Printf("  Manifest: %s (%s)\n", output.ManifestPath, output.ManifestType)
	fmt.Println()

	// Column widths.
	nameW := 40
	versionW := 14
	statusW := 14
	splitW := 8
	addressW := 18

	// Calculate actual max name width.
	for _, r := range output.Recipients {
		if len(r.DependencyName) > nameW-2 {
			nameW = len(r.DependencyName) + 2
		}
	}
	if nameW > 56 {
		nameW = 56
	}

	totalW := nameW + versionW + statusW + splitW + addressW + 6 // 6 for separators

	// Top border.
	fmt.Printf("  ┌%s┐\n", strings.Repeat("─", totalW))

	// Column headers.
	fmt.Printf("  │ %-*s│ %-*s│ %-*s│ %-*s│ %-*s│\n",
		nameW, " DEPENDENCY",
		versionW, " VERSION",
		statusW, " STATUS",
		splitW, " SPLIT",
		addressW-1, " ADDRESS")

	// Header separator.
	fmt.Printf("  ├%s┤\n", strings.Repeat("─", totalW))

	// Rows.
	for _, r := range output.Recipients {
		statusIcon := statusIndicator(r.Status)
		name := truncate(r.DependencyName, nameW-2)
		version := truncate(r.Version, versionW-2)
		address := truncateAddress(r.Address, addressW-2)

		splitStr := "  —"
		if r.RecommendedSplitPct > 0 {
			splitStr = fmt.Sprintf(" %d%%", r.RecommendedSplitPct)
		}

		fmt.Printf("  │ %-*s│ %-*s│ %s %-*s│ %-*s│ %-*s│\n",
			nameW, " "+name,
			versionW, " "+version,
			statusIcon,
			statusW-3, string(r.Status),
			splitW, splitStr,
			addressW-1, " "+address)
	}

	// Bottom border.
	fmt.Printf("  └%s┘\n", strings.Repeat("─", totalW))

	// Summary metrics.
	fmt.Println()
	fmt.Printf("  ── Summary ──────────────────────────────────────\n")
	fmt.Printf("  Dependencies scanned:    %d\n", output.TotalScanned)
	fmt.Printf("  Drips recipients found:  %d\n", output.TotalResolved)
	fmt.Printf("    ✅ Verified:           %d\n", output.Summary.Verified)
	fmt.Printf("    🔒 Escrow:             %d\n", output.Summary.Escrowed)
	fmt.Printf("    ⬜ Unregistered:       %d\n", output.Summary.Unregistered)
	fmt.Printf("  Total suggested split:   %d%%\n", output.Summary.TotalSplitPct)
	fmt.Println()

	// Call-to-action.
	if output.TotalResolved > 0 {
		fmt.Println("  💡 Run 'devex funding sync' to apply this split configuration on-chain.")
	} else {
		fmt.Println("  💡 No Drips recipients found. Dependencies can register at https://drips.network")
	}
	fmt.Println()
}

// statusIndicator returns an emoji/icon for the verification status.
func statusIndicator(status drips.VerificationStatus) string {
	switch status {
	case drips.StatusVerified:
		return "✅"
	case drips.StatusEscrow:
		return "🔒"
	case drips.StatusUnregistered:
		return "⬜"
	default:
		return "❓"
	}
}

// truncate shortens a string to maxLen, adding "…" if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}

// truncateAddress shortens an Ethereum address for display.
// "0x1234567890abcdef..." → "0x1234…cdef"
func truncateAddress(addr string, maxLen int) string {
	if addr == "" {
		return "—"
	}
	if len(addr) <= maxLen {
		return addr
	}
	if maxLen < 12 {
		return addr[:maxLen-1] + "…"
	}
	// Show first 6 and last 4 characters.
	return addr[:6] + "…" + addr[len(addr)-4:]
}

func runStellarFundingInspect(result *parser.ParseResult) error {
	deps := prioritiseDeps(result.Dependencies, inspectTopN)
	logger.Info("📡  analysing %d dependencies for Stellar Network split proposal …", len(deps))

	// For Stellar, distribute percentages equally among direct deps, or all if no direct deps.
	count := len(deps)
	pctPerDep := 0
	remainder := 0
	if count > 0 {
		pctPerDep = 100 / count
		remainder = 100 % count
	}

	type stellarRecipientRow struct {
		Name    string
		Version string
		Pct     int
	}

	rows := make([]stellarRecipientRow, 0, len(deps))
	for i, d := range deps {
		pct := pctPerDep
		if i == 0 {
			pct += remainder
		}
		rows = append(rows, stellarRecipientRow{
			Name:    d.Name,
			Version: d.Version,
			Pct:     pct,
		})
	}

	printOutput(rows, func() {
		fmt.Println()
		fmt.Printf("  📦  %s (Stellar Network Proposal)\n", result.ProjectName)
		fmt.Printf("  Manifest: %s (%s)\n", result.ManifestPath, result.Type)
		fmt.Println()

		nameW := 44
		for _, r := range rows {
			if len(r.Name) > nameW-2 {
				nameW = len(r.Name) + 2
			}
		}
		if nameW > 56 {
			nameW = 56
		}

		totalW := nameW + 14 + 16 + 8 + 18 + 6

		fmt.Printf("  ┌%s┐\n", strings.Repeat("─", totalW))
		fmt.Printf("  │ %-*s│ %-*s│ %-*s│ %-*s│ %-*s│\n",
			nameW, " DEPENDENCY",
			14, " VERSION",
			16, " STATUS",
			8, " SPLIT",
			17, " STELLAR ADDRESS")
		fmt.Printf("  ├%s┤\n", strings.Repeat("─", totalW))

		for _, r := range rows {
			name := truncate(r.Name, nameW-2)
			ver := truncate(r.Version, 12)
			fmt.Printf("  │ %-*s│ %-*s│ ⬜ %-*s│ %4d%%   │ %-*s│\n",
				nameW, " "+name,
				14, " "+ver,
				13, "UNREGISTERED",
				r.Pct,
				17, " —")
		}

		fmt.Printf("  └%s┘\n", strings.Repeat("─", totalW))
		fmt.Println()
		fmt.Printf("  ── Summary ──────────────────────────────────────\n")
		fmt.Printf("  Dependencies scanned:    %d\n", len(result.Dependencies))
		fmt.Printf("  Proposed recipients:     %d\n", len(rows))
		fmt.Printf("  Total suggested split:   100%%\n")
		fmt.Println()
		fmt.Println("  💡 Run 'devex funding generate --network stellar' to create .devex.stellar.yaml and assign Stellar recipient addresses.")
		fmt.Println()
	})

	return nil
}
