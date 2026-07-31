package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
)

// PackageJSONParser parses npm/Node.js package.json files.
type PackageJSONParser struct{}

// NewPackageJSONParser creates a new package.json parser.
func NewPackageJSONParser() *PackageJSONParser {
	return &PackageJSONParser{}
}

// Type returns ManifestPackageJSON.
func (p *PackageJSONParser) Type() ManifestType {
	return ManifestPackageJSON
}

// packageJSON is the internal deserialization target for package.json files.
// We only model the fields we care about — the rest is ignored.
type packageJSON struct {
	Name            string                 `json:"name"`
	Dependencies    map[string]interface{} `json:"dependencies"`
	DevDependencies map[string]interface{} `json:"devDependencies"`

	// Drips is an optional top-level field for self-declaring a Drips
	// account. Format:
	//   "drips": { "accountId": "123", "address": "0x..." }
	Drips *packageJSONDrips `json:"drips,omitempty"`

	// Funding can also be used. npm's "funding" field is overloaded here
	// to optionally carry Drips metadata alongside standard funding info.
	Funding interface{} `json:"funding,omitempty"`
}

// packageJSONDrips represents the "drips" field in package.json.
type packageJSONDrips struct {
	AccountID string `json:"accountId"`
	Address   string `json:"address"`
}

// Parse reads a package.json file and extracts all dependencies.
// It checks both "dependencies" and "devDependencies" (marking the latter
// as indirect), and extracts any embedded Drips metadata.
func (p *PackageJSONParser) Parse(path string) (*ParseResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("parser: failed to read %s: %w", path, err)
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parser: failed to parse %s: %w", path, err)
	}

	result := &ParseResult{
		ManifestPath: path,
		Type:         ManifestPackageJSON,
		ProjectName:  pkg.Name,
		Dependencies: make([]Dependency, 0, len(pkg.Dependencies)+len(pkg.DevDependencies)),
	}

	// Extract self-declared Drips metadata.
	if pkg.Drips != nil && (pkg.Drips.AccountID != "" || pkg.Drips.Address != "") {
		result.SelfDrips = &EmbeddedDripsInfo{
			AccountID: pkg.Drips.AccountID,
			Address:   pkg.Drips.Address,
		}
		logger.Debug("parser: found self-declared drips metadata: %+v", result.SelfDrips)
	}

	// Production dependencies (direct).
	for name, ver := range pkg.Dependencies {
		dep := Dependency{
			Name:    name,
			Version: resolveVersion(ver),
			Source:  ManifestPackageJSON,
			Direct:  true,
		}
		// Check if this dependency's version specifier includes a drips hint.
		// Some packages might embed drips info in a structured dependency object.
		dep.DripsMetadata = extractDripsFromDep(ver)
		result.Dependencies = append(result.Dependencies, dep)
	}

	// Dev dependencies (indirect/dev).
	for name, ver := range pkg.DevDependencies {
		dep := Dependency{
			Name:    name,
			Version: resolveVersion(ver),
			Source:  ManifestPackageJSON,
			Direct:  false,
		}
		dep.DripsMetadata = extractDripsFromDep(ver)
		result.Dependencies = append(result.Dependencies, dep)
	}

	logger.Debug("parser: package.json parsed — name=%s, deps=%d (prod=%d, dev=%d)",
		result.ProjectName, len(result.Dependencies), len(pkg.Dependencies), len(pkg.DevDependencies))

	return result, nil
}

// resolveVersion extracts a version string from the dependency value.
// In standard package.json, values are strings like "^1.0.0".
// In extended formats, they may be objects.
func resolveVersion(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case map[string]interface{}:
		if ver, ok := val["version"]; ok {
			if s, ok := ver.(string); ok {
				return s
			}
		}
		return "*"
	default:
		return "*"
	}
}

// extractDripsFromDep checks if a dependency value contains embedded Drips
// metadata. This supports an extended package.json format where dependencies
// can be objects with a "drips" field:
//
//	"dependencies": {
//	  "some-package": {
//	    "version": "^1.0.0",
//	    "drips": { "accountId": "123", "address": "0xabc..." }
//	  }
//	}
func extractDripsFromDep(v interface{}) *EmbeddedDripsInfo {
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}

	dripsRaw, exists := obj["drips"]
	if !exists {
		return nil
	}

	dripsObj, ok := dripsRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	info := &EmbeddedDripsInfo{}
	if id, ok := dripsObj["accountId"].(string); ok {
		info.AccountID = id
	}
	if addr, ok := dripsObj["address"].(string); ok && strings.HasPrefix(addr, "0x") {
		info.Address = addr
	}

	if info.AccountID == "" && info.Address == "" {
		return nil
	}
	return info
}
