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
