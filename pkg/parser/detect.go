package parser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BCPathway/devex-cli/internal/logger"
)

// DetectManifest searches for a supported manifest file in the given
// directory and returns the appropriate parser and manifest path.
//
// Detection order (first match wins):
//  1. go.mod
//  2. package.json
//
// If manifestPath is explicitly provided (non-empty), it uses that file
// directly instead of auto-detecting.
func DetectManifest(dir string, manifestPath string) (Parser, string, error) {
	// Explicit manifest path takes priority.
	if manifestPath != "" {
		p, err := parserForFile(manifestPath)
		if err != nil {
			return nil, "", err
		}
		return p, manifestPath, nil
	}

	// Auto-detect in order of preference.
	candidates := []struct {
		filename string
		parser   Parser
	}{
		{"go.mod", NewGoModParser()},
		{"package.json", NewPackageJSONParser()},
	}

	for _, c := range candidates {
		path := filepath.Join(dir, c.filename)
		if _, err := os.Stat(path); err == nil {
			logger.Debug("parser: auto-detected manifest: %s", path)
			return c.parser, path, nil
		}
	}

	return nil, "", fmt.Errorf("parser: no supported manifest found in %s (looked for go.mod, package.json)", dir)
}

// parserForFile returns the appropriate parser based on the filename.
func parserForFile(path string) (Parser, error) {
	base := filepath.Base(path)
	switch base {
	case "go.mod":
		return NewGoModParser(), nil
	case "package.json":
		return NewPackageJSONParser(), nil
	default:
		return nil, fmt.Errorf("parser: unsupported manifest file %q (expected go.mod or package.json)", base)
	}
}
