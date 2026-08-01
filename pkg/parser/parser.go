// Package parser provides manifest file parsing for extracting project
// dependencies from various package manager formats.
//
// Architecture:
//   - parser.go: Core types and the Parser interface
//   - gomod.go: go.mod parser implementation
//   - packagejson.go: package.json parser implementation
//   - detect.go: Auto-detection of manifest type from filesystem
package parser

// ManifestType identifies the package manager format.
type ManifestType string

const (
	ManifestGoMod       ManifestType = "go.mod"
	ManifestPackageJSON ManifestType = "package.json"
	ManifestUnknown     ManifestType = "unknown"
)

// Dependency represents a single extracted dependency from a manifest file.
type Dependency struct {
	// Name is the fully-qualified package name (e.g., "github.com/spf13/cobra"
	// or "@types/node").
	Name string `json:"name"`

	// Version is the version constraint as declared in the manifest
	// (e.g., "v1.8.1", "^18.0.0").
	Version string `json:"version"`

	// Source identifies which manifest this dependency was extracted from.
	Source ManifestType `json:"source"`

	// Direct indicates whether this is a direct (true) or transitive (false)
	// dependency.
	Direct bool `json:"direct"`

	// DripsMetadata holds optional Drips Network metadata embedded in the
	// manifest (e.g., a "drips" field in package.json). Nil if absent.
	DripsMetadata *EmbeddedDripsInfo `json:"drips_metadata,omitempty"`
}

// EmbeddedDripsInfo represents Drips Network metadata embedded directly
// in a package manifest. This enables packages to self-declare their
// Drips account for receiving funding.
type EmbeddedDripsInfo struct {
	// AccountID is the Drips Account ID (numeric string).
	AccountID string `json:"account_id,omitempty"`

	// Address is the Ethereum address for receiving drips.
	Address string `json:"address,omitempty"`
}

// ParseResult contains the full output of parsing a manifest file.
type ParseResult struct {
	// ManifestPath is the absolute path to the parsed manifest file.
	ManifestPath string `json:"manifest_path"`

	// Type identifies the manifest format.
	Type ManifestType `json:"type"`

	// ProjectName is the name/module path of the project itself.
	ProjectName string `json:"project_name"`

	// Dependencies is the list of extracted dependencies.
	Dependencies []Dependency `json:"dependencies"`

	// SelfDrips holds Drips metadata declared by the project itself
	// (e.g., a top-level "drips" field in package.json). Nil if absent.
	SelfDrips *EmbeddedDripsInfo `json:"self_drips,omitempty"`
}

// Parser is the interface that manifest parsers must implement.
type Parser interface {
	// Parse reads the manifest at the given path and extracts dependencies.
	Parse(path string) (*ParseResult, error)

	// Type returns the manifest type this parser handles.
	Type() ManifestType
}

// WorkspaceType identifies the monorepo workspace manager format.
type WorkspaceType string

const (
	WorkspacePNPM    WorkspaceType = "pnpm"
	WorkspaceNPMYarn WorkspaceType = "npm/yarn"
	WorkspaceLerna   WorkspaceType = "lerna"
	WorkspaceGoWork  WorkspaceType = "go.work"
	WorkspaceNone    WorkspaceType = "none"
)

// SubPackage represents a single discovered package within a monorepo.
type SubPackage struct {
	// Dir is the absolute path to the sub-package directory.
	Dir string `json:"dir"`

	// Name is the package/module name from its manifest.
	Name string `json:"name"`

	// ManifestType is the type of manifest file found in this sub-package.
	ManifestType ManifestType `json:"manifest_type"`

	// Dependencies are the raw dependencies parsed from this sub-package.
	Dependencies []Dependency `json:"dependencies"`
}

// AggregatedDependency extends Dependency with metadata about which
// sub-packages require it. Used in the unified monorepo dependency list.
type AggregatedDependency struct {
	Dependency

	// RequiredBy lists the names of sub-packages that depend on this package.
	RequiredBy []string `json:"required_by"`
}

// ProjectResult is the top-level result returned by ParseProject. It
// transparently handles both single-package and monorepo layouts.
type ProjectResult struct {
	// IsMonorepo is true when a workspace configuration was detected.
	IsMonorepo bool `json:"is_monorepo"`

	// WorkspaceType identifies the workspace manager (only set for monorepos).
	WorkspaceType WorkspaceType `json:"workspace_type"`

	// RootDir is the absolute path to the project root.
	RootDir string `json:"root_dir"`

	// SubPackages contains the parsed results for each discovered sub-package.
	// Only populated for monorepo projects.
	SubPackages []SubPackage `json:"sub_packages,omitempty"`

	// Dependencies is the unified, deduplicated, external-only dependency
	// list. For single-package repos this comes directly from the manifest;
	// for monorepos it is aggregated across all sub-packages.
	Dependencies []AggregatedDependency `json:"dependencies"`

	// SingleResult is the raw ParseResult when the project is a single-package
	// repository. Nil for monorepos.
	SingleResult *ParseResult `json:"single_result,omitempty"`
}
