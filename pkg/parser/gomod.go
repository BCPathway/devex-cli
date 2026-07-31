package parser

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
)

// GoModParser parses Go module files (go.mod) to extract dependencies.
type GoModParser struct{}

// NewGoModParser creates a new go.mod parser.
func NewGoModParser() *GoModParser {
	return &GoModParser{}
}

// Type returns ManifestGoMod.
func (p *GoModParser) Type() ManifestType {
	return ManifestGoMod
}

// Parse reads a go.mod file and extracts all require directives.
//
// It handles both single-line requires:
//
//	require github.com/spf13/cobra v1.8.1
//
// And block requires:
//
//	require (
//	    github.com/spf13/cobra v1.8.1
//	    github.com/spf13/viper v1.19.0 // indirect
//	)
func (p *GoModParser) Parse(path string) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("parser: failed to open %s: %w", path, err)
	}
	defer f.Close()

	result := &ParseResult{
		ManifestPath: path,
		Type:         ManifestGoMod,
		Dependencies: make([]Dependency, 0),
	}

	scanner := bufio.NewScanner(f)
	inRequireBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Extract module name.
		if strings.HasPrefix(line, "module ") {
			result.ProjectName = strings.TrimPrefix(line, "module ")
			continue
		}

		// Handle require block open/close.
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		// Single-line require.
		if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			dep := parseGoRequireLine(strings.TrimPrefix(line, "require "))
			if dep != nil {
				result.Dependencies = append(result.Dependencies, *dep)
			}
			continue
		}

		// Line inside a require block.
		if inRequireBlock && line != "" && !strings.HasPrefix(line, "//") {
			dep := parseGoRequireLine(line)
			if dep != nil {
				result.Dependencies = append(result.Dependencies, *dep)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parser: error reading %s: %w", path, err)
	}

	logger.Debug("parser: go.mod parsed — module=%s, deps=%d", result.ProjectName, len(result.Dependencies))
	return result, nil
}

// parseGoRequireLine parses a single dependency line from a go.mod file.
// Format: "module/path v1.2.3" or "module/path v1.2.3 // indirect"
func parseGoRequireLine(line string) *Dependency {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") {
		return nil
	}

	// Check for // indirect comment.
	isIndirect := strings.Contains(line, "// indirect")
	// Strip inline comments for parsing.
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		logger.Debug("parser: skipping malformed go.mod line: %q", line)
		return nil
	}

	return &Dependency{
		Name:    parts[0],
		Version: parts[1],
		Source:  ManifestGoMod,
		Direct:  !isIndirect,
	}
}
