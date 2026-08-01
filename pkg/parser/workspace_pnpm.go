package parser

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// pnpmWorkspaceConfig represents the structure of pnpm-workspace.yaml.
type pnpmWorkspaceConfig struct {
	Packages []string `yaml:"packages"`
}

// parsePNPMWorkspace reads a pnpm-workspace.yaml file and returns the
// list of workspace glob patterns defined in the "packages" field.
//
// Example pnpm-workspace.yaml:
//
//	packages:
//	  - 'packages/*'
//	  - 'apps/**'
//	  - '!**/test/**'
func parsePNPMWorkspace(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("parser: failed to read pnpm-workspace.yaml: %w", err)
	}

	var config pnpmWorkspaceConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parser: failed to parse pnpm-workspace.yaml: %w", err)
	}

	if len(config.Packages) == 0 {
		return nil, fmt.Errorf("parser: pnpm-workspace.yaml has no packages defined")
	}

	return config.Packages, nil
}
