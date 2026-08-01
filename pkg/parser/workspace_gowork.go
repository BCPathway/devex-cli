package parser

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// parseGoWorkFile reads a go.work file and extracts the directory paths
// from use directives.
//
// It handles both block and single-line forms:
//
// Block form:
//
//	use (
//	    ./cmd/api
//	    ./pkg/shared
//	)
//
// Single-line form:
//
//	use ./cmd/api
func parseGoWorkFile(configPath string) ([]string, error) {
	f, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("parser: failed to open go.work: %w", err)
	}
	defer f.Close()

	var dirs []string
	scanner := bufio.NewScanner(f)
	inUseBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Handle use block open.
		if line == "use (" {
			inUseBlock = true
			continue
		}

		// Handle use block close.
		if inUseBlock && line == ")" {
			inUseBlock = false
			continue
		}

		// Lines inside a use block.
		if inUseBlock {
			dir := strings.TrimSpace(line)
			// Strip inline comments.
			if idx := strings.Index(dir, "//"); idx >= 0 {
				dir = strings.TrimSpace(dir[:idx])
			}
			if dir != "" {
				dirs = append(dirs, dir)
			}
			continue
		}

		// Single-line use directive.
		if strings.HasPrefix(line, "use ") && !strings.Contains(line, "(") {
			dir := strings.TrimSpace(strings.TrimPrefix(line, "use "))
			// Strip inline comments.
			if idx := strings.Index(dir, "//"); idx >= 0 {
				dir = strings.TrimSpace(dir[:idx])
			}
			if dir != "" {
				dirs = append(dirs, dir)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parser: error reading go.work: %w", err)
	}

	if len(dirs) == 0 {
		return nil, fmt.Errorf("parser: go.work has no use directives")
	}

	return dirs, nil
}
