package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BCPathway/devex-cli/internal/logger"
)

// DetectWorkspace probes the given root directory for monorepo workspace
// configuration files. It returns the detected workspace type and the
// absolute path to the configuration file.
//
// Detection order (first match wins):
//  1. pnpm-workspace.yaml → WorkspacePNPM
//  2. package.json with "workspaces" field → WorkspaceNPMYarn
//  3. lerna.json → WorkspaceLerna
//  4. go.work → WorkspaceGoWork
//  5. None found → WorkspaceNone
func DetectWorkspace(rootDir string) (WorkspaceType, string, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return WorkspaceNone, "", fmt.Errorf("parser: failed to resolve absolute path for %q: %w", rootDir, err)
	}

	// 1. PNPM workspaces.
	pnpmPath := filepath.Join(absRoot, "pnpm-workspace.yaml")
	if _, err := os.Stat(pnpmPath); err == nil {
		logger.Debug("parser: detected PNPM workspace: %s", pnpmPath)
		return WorkspacePNPM, pnpmPath, nil
	}

	// 2. NPM/Yarn workspaces (package.json with "workspaces" field).
	pkgJSONPath := filepath.Join(absRoot, "package.json")
	if _, err := os.Stat(pkgJSONPath); err == nil {
		if hasWorkspacesField(pkgJSONPath) {
			logger.Debug("parser: detected NPM/Yarn workspace: %s", pkgJSONPath)
			return WorkspaceNPMYarn, pkgJSONPath, nil
		}
	}

	// 3. Lerna workspaces.
	lernaPath := filepath.Join(absRoot, "lerna.json")
	if _, err := os.Stat(lernaPath); err == nil {
		logger.Debug("parser: detected Lerna workspace: %s", lernaPath)
		return WorkspaceLerna, lernaPath, nil
	}

	// 4. Go workspaces.
	goWorkPath := filepath.Join(absRoot, "go.work")
	if _, err := os.Stat(goWorkPath); err == nil {
		logger.Debug("parser: detected Go workspace: %s", goWorkPath)
		return WorkspaceGoWork, goWorkPath, nil
	}

	// 5. No workspace detected.
	return WorkspaceNone, "", nil
}

// hasWorkspacesField checks whether a package.json file contains a
// top-level "workspaces" field. It does a minimal JSON decode to avoid
// fully parsing the file.
func hasWorkspacesField(pkgJSONPath string) bool {
	data, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return false
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}

	_, exists := raw["workspaces"]
	return exists
}
