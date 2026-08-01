package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/bmatcuk/doublestar/v4"
)

// ignoredDirs is the set of directory names that are always skipped during
// workspace traversal. These are build outputs, dependency caches, and
// version control directories that should never contain real sub-packages.
var ignoredDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".git":         true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".turbo":       true,
	"out":          true,
	"coverage":     true,
	"__pycache__":  true,
}

// resolveWorkspaceGlobs takes workspace glob patterns and resolves them to
// actual sub-package directories on disk. A "sub-package directory" is one
// that contains a manifest file (package.json or go.mod).
//
// Patterns support standard glob syntax including ** for recursive matching.
// Negation patterns (prefixed with !) are applied as exclusions.
//
// Returns a deduplicated, sorted list of absolute directory paths.
func resolveWorkspaceGlobs(rootDir string, patterns []string) ([]string, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("parser: failed to resolve root dir: %w", err)
	}

	// Separate positive and negation patterns.
	var includePatterns, excludePatterns []string
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			excludePatterns = append(excludePatterns, strings.TrimPrefix(p, "!"))
		} else {
			includePatterns = append(includePatterns, p)
		}
	}

	// Resolve each include pattern to candidate directories.
	seen := make(map[string]bool)
	var dirs []string

	for _, pattern := range includePatterns {
		absPattern := filepath.Join(absRoot, filepath.FromSlash(pattern))
		matches, err := doublestar.FilepathGlob(absPattern)
		if err != nil {
			logger.Warn("parser: invalid glob pattern %q: %v", pattern, err)
			continue
		}

		for _, match := range matches {
			// Must be a directory.
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}

			// Skip ignored directories.
			if isIgnoredPath(absRoot, match) {
				logger.Debug("parser: skipping ignored path: %s", match)
				continue
			}

			// Must contain a manifest file.
			if !hasManifest(match) {
				logger.Debug("parser: skipping directory without manifest: %s", match)
				continue
			}

			// Deduplicate.
			abs, _ := filepath.Abs(match)
			if !seen[abs] {
				seen[abs] = true
				dirs = append(dirs, abs)
			}
		}
	}

	// Apply exclusion patterns.
	if len(excludePatterns) > 0 {
		dirs = applyExclusions(absRoot, dirs, excludePatterns)
	}

	sort.Strings(dirs)
	logger.Debug("parser: resolved %d sub-package directories from %d glob patterns", len(dirs), len(patterns))
	return dirs, nil
}

// resolveExplicitDirs resolves explicit directory paths (as used by go.work)
// relative to a root directory. Unlike resolveWorkspaceGlobs, this does not
// do glob expansion — paths are taken literally.
func resolveExplicitDirs(rootDir string, relativeDirs []string) ([]string, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("parser: failed to resolve root dir: %w", err)
	}

	var dirs []string
	for _, rel := range relativeDirs {
		abs := filepath.Join(absRoot, filepath.FromSlash(rel))
		info, err := os.Stat(abs)
		if err != nil {
			logger.Warn("parser: go.work use directory does not exist: %s", abs)
			continue
		}
		if !info.IsDir() {
			logger.Warn("parser: go.work use path is not a directory: %s", abs)
			continue
		}
		if !hasManifest(abs) {
			logger.Warn("parser: go.work use directory has no manifest: %s", abs)
			continue
		}
		dirs = append(dirs, abs)
	}

	sort.Strings(dirs)
	return dirs, nil
}

// isIgnoredPath checks if any component of the path (relative to root)
// matches an ignored directory name.
func isIgnoredPath(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts {
		if ignoredDirs[part] {
			return true
		}
	}
	return false
}

// hasManifest checks whether a directory contains a supported manifest file.
func hasManifest(dir string) bool {
	manifests := []string{"package.json", "go.mod"}
	for _, m := range manifests {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// applyExclusions filters out directories that match any exclusion pattern.
func applyExclusions(root string, dirs []string, excludePatterns []string) []string {
	var filtered []string
	for _, dir := range dirs {
		excluded := false
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			filtered = append(filtered, dir)
			continue
		}
		relSlash := filepath.ToSlash(rel)

		for _, pattern := range excludePatterns {
			matched, err := doublestar.Match(filepath.ToSlash(pattern), relSlash)
			if err == nil && matched {
				logger.Debug("parser: excluding %s (matched negation pattern %q)", dir, pattern)
				excluded = true
				break
			}
		}

		if !excluded {
			filtered = append(filtered, dir)
		}
	}
	return filtered
}
