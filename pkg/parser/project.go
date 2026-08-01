package parser

import (
	"fmt"
	"path/filepath"

	"github.com/BCPathway/devex-cli/internal/logger"
)

// ParseProject is the primary entry point for analyzing a project root.
// It automatically detects whether the project is a monorepo or a
// single-package repository and returns a unified ProjectResult.
//
// For monorepos, it:
//  1. Detects the workspace type (PNPM, NPM/Yarn, Lerna, Go Work).
//  2. Parses the workspace config to extract sub-package patterns.
//  3. Resolves patterns to real directories on disk.
//  4. Parses each sub-package manifest.
//  5. Aggregates and deduplicates external dependencies.
//
// For single-package repos, it falls back to DetectManifest + Parse
// and wraps the result in a ProjectResult.
func ParseProject(rootDir string) (*ProjectResult, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("parser: failed to resolve root dir: %w", err)
	}

	// Step 1: Detect workspace type.
	wsType, configPath, err := DetectWorkspace(absRoot)
	if err != nil {
		return nil, fmt.Errorf("parser: workspace detection failed: %w", err)
	}

	// No workspace → single-package fallback.
	if wsType == WorkspaceNone {
		return parseSinglePackage(absRoot)
	}

	logger.Info("parser: detected %s workspace at %s", wsType, configPath)

	// Step 2: Parse workspace config for glob patterns / directory paths.
	patterns, err := parseWorkspaceConfig(wsType, configPath)
	if err != nil {
		return nil, err
	}

	logger.Debug("parser: workspace patterns: %v", patterns)

	// Step 3: Resolve patterns to sub-package directories.
	var subDirs []string
	if wsType == WorkspaceGoWork {
		// Go workspaces use explicit directory paths, not globs.
		subDirs, err = resolveExplicitDirs(absRoot, patterns)
	} else {
		subDirs, err = resolveWorkspaceGlobs(absRoot, patterns)
	}
	if err != nil {
		return nil, fmt.Errorf("parser: failed to resolve workspace directories: %w", err)
	}

	if len(subDirs) == 0 {
		logger.Warn("parser: workspace config found but no sub-packages discovered")
		// Fall back to single-package if no sub-packages resolved.
		return parseSinglePackage(absRoot)
	}

	logger.Info("parser: discovered %d sub-packages", len(subDirs))

	// Step 4: Parse each sub-package manifest.
	subPackages := make([]SubPackage, 0, len(subDirs))
	for _, dir := range subDirs {
		sp, err := parseSubPackage(dir)
		if err != nil {
			logger.Warn("parser: skipping sub-package %s: %v", dir, err)
			continue
		}
		subPackages = append(subPackages, *sp)
	}

	if len(subPackages) == 0 {
		return nil, fmt.Errorf("parser: all sub-packages failed to parse")
	}

	// Step 5: Aggregate and deduplicate dependencies.
	aggregated := aggregateDependencies(subPackages)

	return &ProjectResult{
		IsMonorepo:    true,
		WorkspaceType: wsType,
		RootDir:       absRoot,
		SubPackages:   subPackages,
		Dependencies:  aggregated,
	}, nil
}

// parseSinglePackage handles the non-monorepo case by detecting and
// parsing a single manifest file.
func parseSinglePackage(rootDir string) (*ProjectResult, error) {
	p, manifestPath, err := DetectManifest(rootDir, "")
	if err != nil {
		return nil, err
	}

	result, err := p.Parse(manifestPath)
	if err != nil {
		return nil, err
	}

	// Wrap single-package dependencies as AggregatedDependency for a
	// consistent return type.
	aggregated := make([]AggregatedDependency, len(result.Dependencies))
	for i, dep := range result.Dependencies {
		aggregated[i] = AggregatedDependency{
			Dependency: dep,
			RequiredBy: []string{result.ProjectName},
		}
	}

	return &ProjectResult{
		IsMonorepo:    false,
		WorkspaceType: WorkspaceNone,
		RootDir:       rootDir,
		Dependencies:  aggregated,
		SingleResult:  result,
	}, nil
}

// parseWorkspaceConfig dispatches to the appropriate config parser based
// on workspace type and returns the list of glob patterns or directory paths.
func parseWorkspaceConfig(wsType WorkspaceType, configPath string) ([]string, error) {
	switch wsType {
	case WorkspacePNPM:
		return parsePNPMWorkspace(configPath)
	case WorkspaceNPMYarn:
		return parseNPMWorkspaces(configPath)
	case WorkspaceLerna:
		return parseLernaConfig(configPath)
	case WorkspaceGoWork:
		return parseGoWorkFile(configPath)
	default:
		return nil, fmt.Errorf("parser: unsupported workspace type %q", wsType)
	}
}

// parseSubPackage detects and parses the manifest in a single sub-package
// directory, returning a SubPackage.
func parseSubPackage(dir string) (*SubPackage, error) {
	p, manifestPath, err := DetectManifest(dir, "")
	if err != nil {
		return nil, fmt.Errorf("no manifest in %s: %w", dir, err)
	}

	result, err := p.Parse(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", manifestPath, err)
	}

	return &SubPackage{
		Dir:          dir,
		Name:         result.ProjectName,
		ManifestType: result.Type,
		Dependencies: result.Dependencies,
	}, nil
}
