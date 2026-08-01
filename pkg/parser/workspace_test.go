package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BCPathway/devex-cli/pkg/parser"
)

// ==========================================================================
// Workspace Detection Tests
// ==========================================================================

func TestDetectWorkspace_PNPM(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "pnpm-workspace.yaml", "packages:\n  - 'packages/*'\n")

	wsType, configPath, err := parser.DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace() returned error: %v", err)
	}
	if wsType != parser.WorkspacePNPM {
		t.Errorf("WorkspaceType = %q, want %q", wsType, parser.WorkspacePNPM)
	}
	if filepath.Base(configPath) != "pnpm-workspace.yaml" {
		t.Errorf("config path = %q, want pnpm-workspace.yaml", configPath)
	}
}

func TestDetectWorkspace_NPMYarn(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "package.json", `{"name":"root","workspaces":["packages/*"]}`)

	wsType, _, err := parser.DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace() returned error: %v", err)
	}
	if wsType != parser.WorkspaceNPMYarn {
		t.Errorf("WorkspaceType = %q, want %q", wsType, parser.WorkspaceNPMYarn)
	}
}

func TestDetectWorkspace_NPMYarn_NoWorkspacesField(t *testing.T) {
	dir := t.TempDir()
	// package.json without workspaces field should NOT be detected as a workspace.
	writeFileAt(t, dir, "package.json", `{"name":"plain-project"}`)

	wsType, _, err := parser.DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace() returned error: %v", err)
	}
	if wsType != parser.WorkspaceNone {
		t.Errorf("WorkspaceType = %q, want %q (package.json without workspaces)", wsType, parser.WorkspaceNone)
	}
}

func TestDetectWorkspace_Lerna(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "lerna.json", `{"version":"independent","packages":["packages/*"]}`)

	wsType, configPath, err := parser.DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace() returned error: %v", err)
	}
	if wsType != parser.WorkspaceLerna {
		t.Errorf("WorkspaceType = %q, want %q", wsType, parser.WorkspaceLerna)
	}
	if filepath.Base(configPath) != "lerna.json" {
		t.Errorf("config path base = %q, want lerna.json", filepath.Base(configPath))
	}
}

func TestDetectWorkspace_GoWork(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "go.work", "go 1.22\n\nuse (\n\t./cmd/api\n)\n")

	wsType, configPath, err := parser.DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace() returned error: %v", err)
	}
	if wsType != parser.WorkspaceGoWork {
		t.Errorf("WorkspaceType = %q, want %q", wsType, parser.WorkspaceGoWork)
	}
	if filepath.Base(configPath) != "go.work" {
		t.Errorf("config path base = %q, want go.work", filepath.Base(configPath))
	}
}

func TestDetectWorkspace_None(t *testing.T) {
	dir := t.TempDir()

	wsType, configPath, err := parser.DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace() returned error: %v", err)
	}
	if wsType != parser.WorkspaceNone {
		t.Errorf("WorkspaceType = %q, want %q", wsType, parser.WorkspaceNone)
	}
	if configPath != "" {
		t.Errorf("config path = %q, want empty string", configPath)
	}
}

func TestDetectWorkspace_PNPMTakesPriority(t *testing.T) {
	dir := t.TempDir()
	// Create both pnpm-workspace.yaml and package.json with workspaces.
	writeFileAt(t, dir, "pnpm-workspace.yaml", "packages:\n  - 'packages/*'\n")
	writeFileAt(t, dir, "package.json", `{"name":"root","workspaces":["packages/*"]}`)

	wsType, _, err := parser.DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace() returned error: %v", err)
	}
	if wsType != parser.WorkspacePNPM {
		t.Errorf("WorkspaceType = %q, want %q (PNPM should take priority)", wsType, parser.WorkspacePNPM)
	}
}

// ==========================================================================
// ParseProject — Single Package Fallback
// ==========================================================================

func TestParseProject_SinglePackage(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "package.json", `{
		"name": "solo-app",
		"dependencies": {
			"express": "^4.18.0",
			"lodash": "^4.17.21"
		}
	}`)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	if result.IsMonorepo {
		t.Error("IsMonorepo = true, want false")
	}
	if result.WorkspaceType != parser.WorkspaceNone {
		t.Errorf("WorkspaceType = %q, want %q", result.WorkspaceType, parser.WorkspaceNone)
	}
	if result.SingleResult == nil {
		t.Fatal("SingleResult is nil for single-package project")
	}
	if result.SingleResult.ProjectName != "solo-app" {
		t.Errorf("ProjectName = %q, want %q", result.SingleResult.ProjectName, "solo-app")
	}
	if got := len(result.Dependencies); got != 2 {
		t.Fatalf("len(Dependencies) = %d, want 2", got)
	}

	// Verify RequiredBy is populated for single-package.
	for _, dep := range result.Dependencies {
		if len(dep.RequiredBy) != 1 || dep.RequiredBy[0] != "solo-app" {
			t.Errorf("dep %q RequiredBy = %v, want [solo-app]", dep.Name, dep.RequiredBy)
		}
	}
}

func TestParseProject_SinglePackage_GoMod(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "go.mod", `module github.com/example/solo

go 1.22

require github.com/spf13/cobra v1.8.1
`)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	if result.IsMonorepo {
		t.Error("IsMonorepo = true, want false")
	}
	if got := len(result.Dependencies); got != 1 {
		t.Fatalf("len(Dependencies) = %d, want 1", got)
	}
	if result.Dependencies[0].Name != "github.com/spf13/cobra" {
		t.Errorf("dep name = %q, want %q", result.Dependencies[0].Name, "github.com/spf13/cobra")
	}
}

// ==========================================================================
// ParseProject — PNPM Monorepo (end-to-end)
// ==========================================================================

func TestParseProject_PNPMMonorepo(t *testing.T) {
	dir := buildPNPMFixture(t)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	if !result.IsMonorepo {
		t.Fatal("IsMonorepo = false, want true")
	}
	if result.WorkspaceType != parser.WorkspacePNPM {
		t.Errorf("WorkspaceType = %q, want %q", result.WorkspaceType, parser.WorkspacePNPM)
	}

	// Should discover 2 sub-packages: pkg-a and pkg-b.
	if got := len(result.SubPackages); got != 2 {
		t.Fatalf("len(SubPackages) = %d, want 2", got)
	}

	// Build dep map for assertions.
	aggDeps := aggDepMap(result.Dependencies)

	// "react" is used by both pkg-a and pkg-b → should appear once with RequiredBy=[both].
	assertAggDep(t, aggDeps, "react", "^18.2.0", 2)

	// "lodash" is only in pkg-a.
	assertAggDep(t, aggDeps, "lodash", "^4.17.21", 1)

	// "axios" is only in pkg-b.
	assertAggDep(t, aggDeps, "axios", "^1.6.0", 1)

	// "typescript" (devDep from pkg-a) should be present.
	assertAggDep(t, aggDeps, "typescript", "^5.0.0", 1)

	// Internal cross-reference: pkg-a depends on "@myorg/pkg-b" — should be filtered out.
	if _, found := aggDeps["@myorg/pkg-b"]; found {
		t.Error("internal dep @myorg/pkg-b should have been filtered out")
	}

	// workspace:* reference should be filtered.
	if _, found := aggDeps["@myorg/shared"]; found {
		t.Error("workspace:* dep @myorg/shared should have been filtered out")
	}
}

func TestParseProject_NPMMonorepo_ArrayFormat(t *testing.T) {
	dir := buildNPMWorkspaceFixture(t, "array")

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	if !result.IsMonorepo {
		t.Fatal("IsMonorepo = false, want true")
	}
	if result.WorkspaceType != parser.WorkspaceNPMYarn {
		t.Errorf("WorkspaceType = %q, want %q", result.WorkspaceType, parser.WorkspaceNPMYarn)
	}
	if got := len(result.SubPackages); got != 2 {
		t.Fatalf("len(SubPackages) = %d, want 2", got)
	}
}

func TestParseProject_NPMMonorepo_ObjectFormat(t *testing.T) {
	dir := buildNPMWorkspaceFixture(t, "object")

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	if !result.IsMonorepo {
		t.Fatal("IsMonorepo = false, want true")
	}
	if got := len(result.SubPackages); got != 2 {
		t.Fatalf("len(SubPackages) = %d, want 2", got)
	}
}

// ==========================================================================
// ParseProject — Go Work Monorepo (end-to-end)
// ==========================================================================

func TestParseProject_GoWorkMonorepo(t *testing.T) {
	dir := buildGoWorkFixture(t)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	if !result.IsMonorepo {
		t.Fatal("IsMonorepo = false, want true")
	}
	if result.WorkspaceType != parser.WorkspaceGoWork {
		t.Errorf("WorkspaceType = %q, want %q", result.WorkspaceType, parser.WorkspaceGoWork)
	}

	// Should discover 2 sub-modules.
	if got := len(result.SubPackages); got != 2 {
		t.Fatalf("len(SubPackages) = %d, want 2", got)
	}

	aggDeps := aggDepMap(result.Dependencies)

	// "github.com/spf13/cobra" used by both → RequiredBy count = 2.
	assertAggDep(t, aggDeps, "github.com/spf13/cobra", "v1.8.1", 2)

	// "github.com/gorilla/mux" only in cmd/api.
	assertAggDep(t, aggDeps, "github.com/gorilla/mux", "v1.8.0", 1)

	// Internal cross-references should be filtered.
	if _, found := aggDeps["github.com/example/monorepo/pkg/shared"]; found {
		t.Error("internal dep github.com/example/monorepo/pkg/shared should be filtered out")
	}
}

// ==========================================================================
// ParseProject — Lerna Monorepo
// ==========================================================================

func TestParseProject_LernaMonorepo(t *testing.T) {
	dir := buildLernaFixture(t)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	if !result.IsMonorepo {
		t.Fatal("IsMonorepo = false, want true")
	}
	if result.WorkspaceType != parser.WorkspaceLerna {
		t.Errorf("WorkspaceType = %q, want %q", result.WorkspaceType, parser.WorkspaceLerna)
	}
	if got := len(result.SubPackages); got != 2 {
		t.Fatalf("len(SubPackages) = %d, want 2", got)
	}
}

func TestParseProject_LernaDefaultPackages(t *testing.T) {
	// Lerna with no "packages" field should default to ["packages/*"].
	dir := t.TempDir()
	writeFileAt(t, dir, "lerna.json", `{"version":"independent"}`)
	mkdirAll(t, dir, "packages", "core")
	writeFileAt(t, filepath.Join(dir, "packages", "core"), "package.json",
		`{"name":"@myorg/core","dependencies":{"react":"^18.0.0"}}`)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	if !result.IsMonorepo {
		t.Fatal("IsMonorepo = false, want true")
	}
	if got := len(result.SubPackages); got != 1 {
		t.Fatalf("len(SubPackages) = %d, want 1", got)
	}
}

// ==========================================================================
// Aggregation & Deduplication Tests
// ==========================================================================

func TestParseProject_DeduplicatesSameDep(t *testing.T) {
	dir := buildPNPMFixture(t)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	// Count how many times "react" appears — should be exactly 1.
	count := 0
	for _, dep := range result.Dependencies {
		if dep.Name == "react" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("react appears %d times, want 1 (should be deduplicated)", count)
	}
}

func TestParseProject_FiltersInternalDeps(t *testing.T) {
	dir := buildPNPMFixture(t)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	for _, dep := range result.Dependencies {
		if dep.Name == "@myorg/pkg-b" {
			t.Error("internal dep @myorg/pkg-b should have been filtered out")
		}
	}
}

func TestParseProject_FiltersWorkspaceProtocol(t *testing.T) {
	dir := buildPNPMFixture(t)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	for _, dep := range result.Dependencies {
		if dep.Name == "@myorg/shared" {
			t.Error("workspace:* dep @myorg/shared should have been filtered out")
		}
	}
}

func TestParseProject_FiltersFileProtocol(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "pnpm-workspace.yaml", "packages:\n  - 'packages/*'\n")
	mkdirAll(t, dir, "packages", "app")
	writeFileAt(t, filepath.Join(dir, "packages", "app"), "package.json", `{
		"name": "@myorg/app",
		"dependencies": {
			"react": "^18.0.0",
			"local-lib": "file:../lib"
		}
	}`)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	for _, dep := range result.Dependencies {
		if dep.Name == "local-lib" {
			t.Error("file: protocol dep local-lib should have been filtered out")
		}
	}
}

// ==========================================================================
// Ignored Directories Tests
// ==========================================================================

func TestParseProject_IgnoresNodeModules(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "pnpm-workspace.yaml", "packages:\n  - '**'\n")

	// Real package.
	mkdirAll(t, dir, "packages", "app")
	writeFileAt(t, filepath.Join(dir, "packages", "app"), "package.json",
		`{"name":"@myorg/app","dependencies":{"express":"^4.0.0"}}`)

	// Fake package inside node_modules — should be ignored.
	mkdirAll(t, dir, "node_modules", "some-pkg")
	writeFileAt(t, filepath.Join(dir, "node_modules", "some-pkg"), "package.json",
		`{"name":"some-pkg","dependencies":{"internal-thing":"^1.0.0"}}`)

	result, err := parser.ParseProject(dir)
	if err != nil {
		t.Fatalf("ParseProject() returned error: %v", err)
	}

	// Should only find the real package, not the node_modules one.
	if got := len(result.SubPackages); got != 1 {
		t.Fatalf("len(SubPackages) = %d, want 1 (node_modules should be ignored)", got)
	}
	if result.SubPackages[0].Name != "@myorg/app" {
		t.Errorf("SubPackages[0].Name = %q, want @myorg/app", result.SubPackages[0].Name)
	}
}

// ==========================================================================
// Error Handling Tests
// ==========================================================================

func TestParseProject_MalformedPNPMYAML(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "pnpm-workspace.yaml", ":::invalid yaml{{{}}")

	_, err := parser.ParseProject(dir)
	if err == nil {
		t.Fatal("ParseProject() expected error for malformed YAML, got nil")
	}
}

func TestParseProject_MalformedLernaJSON(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "lerna.json", "{invalid json!!}")

	_, err := parser.ParseProject(dir)
	if err == nil {
		t.Fatal("ParseProject() expected error for malformed JSON, got nil")
	}
}

func TestParseProject_NoManifestAnywhere(t *testing.T) {
	dir := t.TempDir()

	_, err := parser.ParseProject(dir)
	if err == nil {
		t.Fatal("ParseProject() expected error for empty directory, got nil")
	}
}

func TestParseProject_EmptyGoWork(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "go.work", "go 1.22\n")

	_, err := parser.ParseProject(dir)
	if err == nil {
		t.Fatal("ParseProject() expected error for go.work with no use directives, got nil")
	}
}

// ==========================================================================
// Fixture Builders
// ==========================================================================

// buildPNPMFixture creates a simulated PNPM monorepo with 2 sub-packages:
//
//	root/
//	├── pnpm-workspace.yaml   (packages: ['packages/*'])
//	├── packages/
//	│   ├── pkg-a/
//	│   │   └── package.json  (deps: react, lodash; devDeps: typescript;
//	│   │                      deps on @myorg/pkg-b, @myorg/shared via workspace:*)
//	│   └── pkg-b/
//	│       └── package.json  (deps: react, axios)
func buildPNPMFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFileAt(t, dir, "pnpm-workspace.yaml", "packages:\n  - 'packages/*'\n")

	// Sub-package A.
	mkdirAll(t, dir, "packages", "pkg-a")
	writeFileAt(t, filepath.Join(dir, "packages", "pkg-a"), "package.json", `{
		"name": "@myorg/pkg-a",
		"dependencies": {
			"react": "^18.2.0",
			"lodash": "^4.17.21",
			"@myorg/pkg-b": "^1.0.0",
			"@myorg/shared": "workspace:*"
		},
		"devDependencies": {
			"typescript": "^5.0.0"
		}
	}`)

	// Sub-package B.
	mkdirAll(t, dir, "packages", "pkg-b")
	writeFileAt(t, filepath.Join(dir, "packages", "pkg-b"), "package.json", `{
		"name": "@myorg/pkg-b",
		"dependencies": {
			"react": "^18.2.0",
			"axios": "^1.6.0"
		}
	}`)

	return dir
}

// buildNPMWorkspaceFixture creates a simulated NPM/Yarn workspace.
// format is either "array" or "object".
func buildNPMWorkspaceFixture(t *testing.T, format string) string {
	t.Helper()
	dir := t.TempDir()

	var rootPkg string
	switch format {
	case "array":
		rootPkg = `{"name":"root","workspaces":["packages/*"]}`
	case "object":
		rootPkg = `{"name":"root","workspaces":{"packages":["packages/*"]}}`
	default:
		t.Fatalf("unknown format %q", format)
	}
	writeFileAt(t, dir, "package.json", rootPkg)

	mkdirAll(t, dir, "packages", "core")
	writeFileAt(t, filepath.Join(dir, "packages", "core"), "package.json",
		`{"name":"@ws/core","dependencies":{"react":"^18.0.0"}}`)

	mkdirAll(t, dir, "packages", "utils")
	writeFileAt(t, filepath.Join(dir, "packages", "utils"), "package.json",
		`{"name":"@ws/utils","dependencies":{"lodash":"^4.17.21"}}`)

	return dir
}

// buildGoWorkFixture creates a simulated Go workspace with 2 modules:
//
//	root/
//	├── go.work        (use ./cmd/api, ./pkg/shared)
//	├── cmd/api/
//	│   └── go.mod     (deps: cobra, gorilla/mux, internal shared)
//	└── pkg/shared/
//	    └── go.mod     (deps: cobra)
func buildGoWorkFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFileAt(t, dir, "go.work", `go 1.22

use (
	./cmd/api
	./pkg/shared
)
`)

	// Module: cmd/api.
	mkdirAll(t, dir, "cmd", "api")
	writeFileAt(t, filepath.Join(dir, "cmd", "api"), "go.mod", `module github.com/example/monorepo/cmd/api

go 1.22

require (
	github.com/spf13/cobra v1.8.1
	github.com/gorilla/mux v1.8.0
	github.com/example/monorepo/pkg/shared v0.0.0
)
`)

	// Module: pkg/shared.
	mkdirAll(t, dir, "pkg", "shared")
	writeFileAt(t, filepath.Join(dir, "pkg", "shared"), "go.mod", `module github.com/example/monorepo/pkg/shared

go 1.22

require github.com/spf13/cobra v1.8.1
`)

	return dir
}

// buildLernaFixture creates a simulated Lerna monorepo.
func buildLernaFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFileAt(t, dir, "lerna.json", `{
		"version": "independent",
		"packages": ["packages/*"]
	}`)

	mkdirAll(t, dir, "packages", "ui")
	writeFileAt(t, filepath.Join(dir, "packages", "ui"), "package.json",
		`{"name":"@lerna/ui","dependencies":{"react":"^18.0.0"}}`)

	mkdirAll(t, dir, "packages", "api")
	writeFileAt(t, filepath.Join(dir, "packages", "api"), "package.json",
		`{"name":"@lerna/api","dependencies":{"express":"^4.18.0"}}`)

	return dir
}

// ==========================================================================
// Test Helpers
// ==========================================================================

// mkdirAll creates nested directories under root.
func mkdirAll(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}

// aggDepMap creates a lookup map from dependency name to AggregatedDependency.
func aggDepMap(deps []parser.AggregatedDependency) map[string]parser.AggregatedDependency {
	m := make(map[string]parser.AggregatedDependency, len(deps))
	for _, d := range deps {
		m[d.Name] = d
	}
	return m
}

// assertAggDep checks that an aggregated dependency exists with the expected
// version and RequiredBy count.
func assertAggDep(t *testing.T, deps map[string]parser.AggregatedDependency, name, version string, requiredByCount int) {
	t.Helper()
	dep, ok := deps[name]
	if !ok {
		t.Errorf("aggregated dependency %q not found", name)
		return
	}
	if dep.Version != version {
		t.Errorf("%s: Version = %q, want %q", name, dep.Version, version)
	}
	if got := len(dep.RequiredBy); got != requiredByCount {
		t.Errorf("%s: RequiredBy count = %d, want %d (RequiredBy = %v)",
			name, got, requiredByCount, dep.RequiredBy)
	}
}
