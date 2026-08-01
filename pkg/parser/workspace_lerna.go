package parser

import (
	"encoding/json"
	"fmt"
	"os"
)

// lernaConfig represents the structure of lerna.json.
type lernaConfig struct {
	Packages []string `json:"packages"`
	Version  string   `json:"version"`
}

// parseLernaConfig reads a lerna.json file and returns the list of
// workspace glob patterns. If the "packages" field is omitted, it
// falls back to Lerna's default of ["packages/*"].
//
// Example lerna.json:
//
//	{
//	  "version": "independent",
//	  "packages": ["packages/*", "modules/*"]
//	}
func parseLernaConfig(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("parser: failed to read lerna.json: %w", err)
	}

	var config lernaConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parser: failed to parse lerna.json: %w", err)
	}

	// Lerna defaults to ["packages/*"] if the field is omitted.
	if len(config.Packages) == 0 {
		return []string{"packages/*"}, nil
	}

	return config.Packages, nil
}
