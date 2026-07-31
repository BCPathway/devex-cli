package drips

import (
	"fmt"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/parser"
)

// VerificationStatus indicates the Drips registration state of a dependency.
type VerificationStatus string

const (
	// StatusVerified means the dependency has a confirmed Drips account
	// with on-chain verification linking the package identity to the account.
	StatusVerified VerificationStatus = "Verified"

	// StatusUnregistered means no Drips account was found for this dependency.
	StatusUnregistered VerificationStatus = "Unregistered"

	// StatusEscrow means the dependency has been claimed via escrow —
	// funds can be streamed to it, but the owner hasn't completed
	// on-chain identity verification yet.
	StatusEscrow VerificationStatus = "Escrow"
)

// DripsRecipient is the resolved Drips Network identity for a single
// dependency, combining package metadata with on-chain lookup results.
type DripsRecipient struct {
	// DependencyName is the fully-qualified package name.
	DependencyName string `json:"dependency_name"`

	// Version is the declared version constraint from the manifest.
	Version string `json:"version"`

	// DripsAccountID is the resolved Drips Account ID (if found).
	DripsAccountID string `json:"drips_account_id,omitempty"`

	// Address is the Ethereum address associated with the Drips account.
	Address string `json:"address,omitempty"`

	// Status indicates the verification state of the Drips account.
	Status VerificationStatus `json:"status"`

	// RecommendedSplitPct is the suggested split percentage for this
	// dependency, based on whether it's a direct dependency and its
	// verification status. Range: 0–100.
	RecommendedSplitPct int `json:"recommended_split_pct"`

	// Source indicates how the Drips info was resolved.
	Source string `json:"source,omitempty"`
}

// RegistryResolver handles looking up dependencies against the Drips
// Network registry to determine which ones have registered accounts.
type RegistryResolver struct {
	client *Client

	// subgraphURL is the Drips Network subgraph endpoint for GraphQL queries.
	subgraphURL string
}

// NewRegistryResolver creates a new resolver backed by the given Drips client.
func NewRegistryResolver(client *Client) *RegistryResolver {
	subgraphURL := resolveSubgraphURL(client.config.ChainID)
	logger.Debug("drips: registry resolver using subgraph: %s", subgraphURL)

	return &RegistryResolver{
		client:      client,
		subgraphURL: subgraphURL,
	}
}

// resolveSubgraphURL returns the appropriate Drips subgraph endpoint for
// the given chain ID.
func resolveSubgraphURL(chainID int) string {
	switch chainID {
	case 1:
		return "https://api.thegraph.com/subgraphs/name/drips/drips-on-ethereum"
	case 10:
		return "https://api.thegraph.com/subgraphs/name/drips/drips-on-optimism"
	case 11155420:
		return "https://api.thegraph.com/subgraphs/name/drips/drips-on-optimism-sepolia"
	default:
		return fmt.Sprintf("https://api.thegraph.com/subgraphs/name/drips/drips-on-chain-%d", chainID)
	}
}

// ResolveDependencies takes a list of parsed dependencies and resolves each
// one against the Drips Network registry. It returns a DripsRecipient for
// every input dependency, whether or not a Drips account was found.
//
// Resolution strategy (in priority order):
//  1. Embedded metadata — if the dependency's manifest includes Drips info
//  2. Subgraph lookup — query the Drips Network subgraph by project URL
//  3. Well-known registry — check a curated mapping of popular packages
//  4. Unregistered — no Drips account found
func (r *RegistryResolver) ResolveDependencies(deps []parser.Dependency) ([]DripsRecipient, error) {
	if err := r.client.connect(); err != nil {
		return nil, fmt.Errorf("drips: resolver connect failed: %w", err)
	}

	recipients := make([]DripsRecipient, 0, len(deps))

	for _, dep := range deps {
		recipient := r.resolveSingle(dep)
		recipients = append(recipients, recipient)
	}

	// Log resolution summary.
	var verified, escrowed, unregistered int
	for _, rec := range recipients {
		switch rec.Status {
		case StatusVerified:
			verified++
		case StatusEscrow:
			escrowed++
		case StatusUnregistered:
			unregistered++
		}
	}
	logger.Debug("drips: resolved %d deps — verified=%d, escrow=%d, unregistered=%d",
		len(recipients), verified, escrowed, unregistered)

	return recipients, nil
}

// resolveSingle resolves a single dependency through the resolution cascade.
func (r *RegistryResolver) resolveSingle(dep parser.Dependency) DripsRecipient {
	base := DripsRecipient{
		DependencyName: dep.Name,
		Version:        dep.Version,
	}

	// Strategy 1: Embedded metadata from manifest.
	if dep.DripsMetadata != nil {
		if dep.DripsMetadata.AccountID != "" || dep.DripsMetadata.Address != "" {
			base.DripsAccountID = dep.DripsMetadata.AccountID
			base.Address = dep.DripsMetadata.Address
			base.Status = StatusVerified
			base.Source = "manifest"
			base.RecommendedSplitPct = calculateSplitPct(dep, StatusVerified)
			logger.Debug("drips: %s resolved via manifest metadata", dep.Name)
			return base
		}
	}

	// Strategy 2: Subgraph/API lookup.
	if result, err := r.querySubgraph(dep); err == nil && result != nil {
		base.DripsAccountID = result.DripsAccountID
		base.Address = result.Address
		base.Status = result.Status
		base.Source = "subgraph"
		base.RecommendedSplitPct = calculateSplitPct(dep, result.Status)
		logger.Debug("drips: %s resolved via subgraph — status=%s", dep.Name, result.Status)
		return base
	}

	// Strategy 3: Well-known registry for popular packages.
	if result := r.checkWellKnown(dep); result != nil {
		base.DripsAccountID = result.DripsAccountID
		base.Address = result.Address
		base.Status = result.Status
		base.Source = "well-known"
		base.RecommendedSplitPct = calculateSplitPct(dep, result.Status)
		logger.Debug("drips: %s resolved via well-known registry — status=%s", dep.Name, result.Status)
		return base
	}

	// Strategy 4: Unregistered.
	base.Status = StatusUnregistered
	base.RecommendedSplitPct = 0
	base.Source = "none"
	return base
}

// querySubgraph queries the Drips Network subgraph for a project matching
// the given dependency.
//
// TODO: Replace with actual GraphQL query. The query would look like:
//
//	query {
//	  repoDriverAccounts(where: { url_contains: "<dep.Name>" }) {
//	    id
//	    driver { address }
//	    status
//	  }
//	}
func (r *RegistryResolver) querySubgraph(dep parser.Dependency) (*subgraphResult, error) {
	// Construct the project URL to search for.
	projectURL := inferProjectURL(dep)
	if projectURL == "" {
		return nil, nil
	}

	logger.Debug("drips: querying subgraph for %s (url=%s)", dep.Name, projectURL)

	// TODO: Implement actual HTTP POST to r.subgraphURL with GraphQL query.
	//
	//   query := fmt.Sprintf(`{
	//     repoDriverAccounts(where: { url_contains: "%s" }) {
	//       id
	//       driver { address }
	//       status
	//     }
	//   }`, projectURL)
	//
	//   resp, err := http.Post(r.subgraphURL, "application/json",
	//       bytes.NewBufferString(fmt.Sprintf(`{"query": %q}`, query)))
	//   ...

	// Placeholder: return nil to indicate no result found via subgraph.
	return nil, nil
}

// subgraphResult is the internal representation of a subgraph query match.
type subgraphResult struct {
	DripsAccountID string
	Address        string
	Status         VerificationStatus
}

// checkWellKnown checks a curated registry of popular packages that have
// known Drips accounts. This serves as a fallback when subgraph lookup
// fails or for packages whose Drips accounts are not discoverable via
// their repository URL alone.
//
// In production, this would be loaded from a periodically-updated JSON
// file or API endpoint maintained by the DevEx community.
func (r *RegistryResolver) checkWellKnown(dep parser.Dependency) *subgraphResult {
	// Well-known Drips accounts for popular Go and npm packages.
	// These are illustrative examples — replace with real data.
	wellKnown := map[string]*subgraphResult{
		// Go ecosystem
		"github.com/spf13/cobra": {
			DripsAccountID: "drips:1:cobra",
			Address:        "0x1234567890abcdef1234567890abcdef12345678",
			Status:         StatusVerified,
		},
		"github.com/spf13/viper": {
			DripsAccountID: "drips:1:viper",
			Address:        "0x234567890abcdef1234567890abcdef123456789",
			Status:         StatusEscrow,
		},
		// npm ecosystem
		"react": {
			DripsAccountID: "drips:1:react",
			Address:        "0xfacebookreactdripsaddress00000000000000",
			Status:         StatusVerified,
		},
		"express": {
			DripsAccountID: "drips:1:express",
			Address:        "0xexpressjsdripsaddress000000000000000000",
			Status:         StatusVerified,
		},
		"typescript": {
			DripsAccountID: "drips:1:typescript",
			Address:        "0xtypescriptdripsaddress00000000000000000",
			Status:         StatusEscrow,
		},
	}

	result, found := wellKnown[dep.Name]
	if !found {
		return nil
	}
	return result
}

// inferProjectURL attempts to derive a project URL from a dependency name.
// For Go packages, the module path often IS the repository URL.
// For npm packages, we construct a likely GitHub URL.
func inferProjectURL(dep parser.Dependency) string {
	switch dep.Source {
	case parser.ManifestGoMod:
		// Go module paths are typically repo URLs already.
		if strings.Contains(dep.Name, "github.com") ||
			strings.Contains(dep.Name, "gitlab.com") ||
			strings.Contains(dep.Name, "bitbucket.org") {
			return "https://" + dep.Name
		}
		return ""

	case parser.ManifestPackageJSON:
		// For npm packages, construct a GitHub search URL.
		// In production, use the npm registry API to get the actual repo URL.
		return fmt.Sprintf("https://www.npmjs.com/package/%s", dep.Name)

	default:
		return ""
	}
}

// calculateSplitPct computes a recommended split percentage based on
// dependency characteristics and verification status.
//
// Heuristic:
//   - Verified direct deps: 5%
//   - Verified indirect deps: 2%
//   - Escrow direct deps: 3%
//   - Escrow indirect deps: 1%
//   - Unregistered: 0%
//
// These are starting suggestions — the total is normalised later in the
// CLI output to ensure it doesn't exceed 100%.
func calculateSplitPct(dep parser.Dependency, status VerificationStatus) int {
	switch status {
	case StatusVerified:
		if dep.Direct {
			return 5
		}
		return 2
	case StatusEscrow:
		if dep.Direct {
			return 3
		}
		return 1
	default:
		return 0
	}
}
