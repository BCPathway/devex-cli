package parser

import (
	"encoding/json"
	"fmt"
	"os"
)

// parseNPMWorkspaces reads a root package.json and extracts workspace
// glob patterns from the "workspaces" field.
//
// It handles both formats:
//
// Array format (npm ≥7, Yarn):
//
//	"workspaces": ["packages/*", "apps/*"]
//
// Object format (Yarn):
//
//	"workspaces": {
//	  "packages": ["packages/*", "apps/*"],
//	  "nohoist": ["**/react-native"]
//	}
func parseNPMWorkspaces(packageJSONPath string) ([]string, error) {
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return nil, fmt.Errorf("parser: failed to read package.json: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parser: failed to parse package.json: %w", err)
	}

	wsRaw, exists := raw["workspaces"]
	if !exists {
		return nil, fmt.Errorf("parser: package.json has no \"workspaces\" field")
	}

	// Try array format first: "workspaces": ["packages/*"]
	var patterns []string
	if err := json.Unmarshal(wsRaw, &patterns); err == nil {
		if len(patterns) == 0 {
			return nil, fmt.Errorf("parser: package.json \"workspaces\" array is empty")
		}
		return patterns, nil
	}

	// Try object format: "workspaces": { "packages": ["packages/*"] }
	var wsObj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(wsRaw, &wsObj); err != nil {
		return nil, fmt.Errorf("parser: failed to parse \"workspaces\" field — expected array or object: %w", err)
	}

	if len(wsObj.Packages) == 0 {
		return nil, fmt.Errorf("parser: package.json \"workspaces.packages\" is empty or missing")
	}

	return wsObj.Packages, nil
}
