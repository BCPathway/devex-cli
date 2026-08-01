package parser

import (
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
)

// aggregateDependencies collects dependencies from all sub-packages,
// filters out internal monorepo cross-references and workspace protocol
// specifiers, and produces a deduplicated list of external dependencies
// with RequiredBy metadata.
func aggregateDependencies(subPackages []SubPackage) []AggregatedDependency {
	// Build the set of internal package names for cross-reference filtering.
	internalNames := make(map[string]bool, len(subPackages))
	for _, sp := range subPackages {
		if sp.Name != "" {
			internalNames[sp.Name] = true
		}
	}

	// Aggregate: dep name → AggregatedDependency.
	type depEntry struct {
		dep        AggregatedDependency
		firstSeenV string
	}
	index := make(map[string]*depEntry)
	var order []string // Preserve insertion order.

	for _, sp := range subPackages {
		for _, dep := range sp.Dependencies {
			// Filter 1: Internal monorepo cross-references.
			if internalNames[dep.Name] {
				logger.Debug("parser: filtering internal dep %q (from %s)", dep.Name, sp.Name)
				continue
			}

			// Filter 2: workspace:* / workspace:^ / workspace:~ protocol.
			if isWorkspaceProtocol(dep.Version) {
				logger.Debug("parser: filtering workspace protocol dep %q = %q (from %s)", dep.Name, dep.Version, sp.Name)
				continue
			}

			// Filter 3: file: and link: specifiers (local references).
			if isLocalSpecifier(dep.Version) {
				logger.Debug("parser: filtering local specifier dep %q = %q (from %s)", dep.Name, dep.Version, sp.Name)
				continue
			}

			// Deduplicate: first-seen version wins, warn on conflicts.
			existing, exists := index[dep.Name]
			if exists {
				// Add this sub-package to RequiredBy.
				if sp.Name != "" {
					existing.dep.RequiredBy = appendUnique(existing.dep.RequiredBy, sp.Name)
				}

				// Warn on version mismatch.
				if existing.firstSeenV != dep.Version {
					logger.Warn("parser: version conflict for %q: %q (kept) vs %q (from %s)",
						dep.Name, existing.firstSeenV, dep.Version, sp.Name)
				}
				continue
			}

			// New dependency — record it.
			entry := &depEntry{
				dep: AggregatedDependency{
					Dependency: dep,
					RequiredBy: make([]string, 0, 1),
				},
				firstSeenV: dep.Version,
			}
			if sp.Name != "" {
				entry.dep.RequiredBy = append(entry.dep.RequiredBy, sp.Name)
			}
			index[dep.Name] = entry
			order = append(order, dep.Name)
		}
	}

	// Build result in insertion order.
	result := make([]AggregatedDependency, 0, len(order))
	for _, name := range order {
		result = append(result, index[name].dep)
	}

	logger.Debug("parser: aggregated %d external dependencies from %d sub-packages", len(result), len(subPackages))
	return result
}

// isWorkspaceProtocol checks if a version string uses the workspace: protocol
// (PNPM/Yarn workspace references).
func isWorkspaceProtocol(version string) bool {
	return strings.HasPrefix(version, "workspace:")
}

// isLocalSpecifier checks if a version string is a local file reference.
func isLocalSpecifier(version string) bool {
	return strings.HasPrefix(version, "file:") || strings.HasPrefix(version, "link:")
}

// appendUnique adds a value to a slice only if it's not already present.
func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
