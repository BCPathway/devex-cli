package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BCPathway/devex-cli/pkg/parser"
)

// --------------------------------------------------------------------------
// go.mod parsing tests
// --------------------------------------------------------------------------

func TestGoModParser_Parse_BasicModule(t *testing.T) {
	content := `module github.com/example/myproject

go 1.22

require (
	github.com/spf13/cobra v1.8.1
	github.com/spf13/viper v1.19.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
)
`
	path := writeTestFile(t, "go.mod", content)

	p := parser.NewGoModParser()
	result, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	// Verify module name.
	if result.ProjectName != "github.com/example/myproject" {
		t.Errorf("ProjectName = %q, want %q", result.ProjectName, "github.com/example/myproject")
	}

	// Verify manifest type.
	if result.Type != parser.ManifestGoMod {
		t.Errorf("Type = %q, want %q", result.Type, parser.ManifestGoMod)
	}

	// Verify dependency count: 3 direct + 2 indirect = 5 total.
	if got := len(result.Dependencies); got != 5 {
		t.Fatalf("len(Dependencies) = %d, want 5", got)
	}

	// Build a lookup map for assertions.
	deps := depMap(result.Dependencies)

	// Check direct dependencies.
	assertDep(t, deps, "github.com/spf13/cobra", "v1.8.1", true)
	assertDep(t, deps, "github.com/spf13/viper", "v1.19.0", true)
	assertDep(t, deps, "gopkg.in/yaml.v3", "v3.0.1", true)

	// Check indirect dependencies.
	assertDep(t, deps, "github.com/fsnotify/fsnotify", "v1.7.0", false)
	assertDep(t, deps, "github.com/hashicorp/hcl", "v1.0.0", false)
}

func TestGoModParser_Parse_SingleLineRequire(t *testing.T) {
	content := `module github.com/example/single

go 1.21

require github.com/stretchr/testify v1.9.0
`
	path := writeTestFile(t, "go.mod", content)

	p := parser.NewGoModParser()
	result, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if got := len(result.Dependencies); got != 1 {
		t.Fatalf("len(Dependencies) = %d, want 1", got)
	}

	dep := result.Dependencies[0]
	if dep.Name != "github.com/stretchr/testify" {
		t.Errorf("Name = %q, want %q", dep.Name, "github.com/stretchr/testify")
	}
	if dep.Version != "v1.9.0" {
		t.Errorf("Version = %q, want %q", dep.Version, "v1.9.0")
	}
	if !dep.Direct {
		t.Error("Direct = false, want true")
	}
}

func TestGoModParser_Parse_EmptyRequireBlock(t *testing.T) {
	content := `module github.com/example/empty

go 1.22
`
	path := writeTestFile(t, "go.mod", content)

	p := parser.NewGoModParser()
	result, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if got := len(result.Dependencies); got != 0 {
		t.Errorf("len(Dependencies) = %d, want 0", got)
	}
}

func TestGoModParser_Parse_CommentLines(t *testing.T) {
	content := `module github.com/example/comments

go 1.22

require (
	// This is a comment
	github.com/spf13/cobra v1.8.1
	// Another comment
	github.com/spf13/viper v1.19.0 // indirect
)
`
	path := writeTestFile(t, "go.mod", content)

	p := parser.NewGoModParser()
	result, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if got := len(result.Dependencies); got != 2 {
		t.Fatalf("len(Dependencies) = %d, want 2", got)
	}

	deps := depMap(result.Dependencies)
	assertDep(t, deps, "github.com/spf13/cobra", "v1.8.1", true)
	assertDep(t, deps, "github.com/spf13/viper", "v1.19.0", false)
}

func TestGoModParser_Parse_FileNotFound(t *testing.T) {
	p := parser.NewGoModParser()
	_, err := p.Parse("/nonexistent/go.mod")
	if err == nil {
		t.Fatal("Parse() expected error for nonexistent file, got nil")
	}
}

func TestGoModParser_Type(t *testing.T) {
	p := parser.NewGoModParser()
	if got := p.Type(); got != parser.ManifestGoMod {
		t.Errorf("Type() = %q, want %q", got, parser.ManifestGoMod)
	}
}

// --------------------------------------------------------------------------
// package.json parsing tests
// --------------------------------------------------------------------------

func TestPackageJSONParser_Parse_StandardDeps(t *testing.T) {
	content := `{
  "name": "my-web-app",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.2.0",
    "express": "~4.18.0",
    "lodash": "4.17.21"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "jest": "^29.0.0"
  }
}`
	path := writeTestFile(t, "package.json", content)

	p := parser.NewPackageJSONParser()
	result, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if result.ProjectName != "my-web-app" {
		t.Errorf("ProjectName = %q, want %q", result.ProjectName, "my-web-app")
	}
	if result.Type != parser.ManifestPackageJSON {
		t.Errorf("Type = %q, want %q", result.Type, parser.ManifestPackageJSON)
	}

	// 3 prod + 2 dev = 5 total.
	if got := len(result.Dependencies); got != 5 {
		t.Fatalf("len(Dependencies) = %d, want 5", got)
	}

	deps := depMap(result.Dependencies)

	// Production deps should be direct.
	assertDep(t, deps, "react", "^18.2.0", true)
	assertDep(t, deps, "express", "~4.18.0", true)
	assertDep(t, deps, "lodash", "4.17.21", true)

	// Dev deps should be indirect.
	assertDep(t, deps, "typescript", "^5.0.0", false)
	assertDep(t, deps, "jest", "^29.0.0", false)
}

func TestPackageJSONParser_Parse_WithDripsMetadata(t *testing.T) {
	content := `{
  "name": "funded-project",
  "version": "2.0.0",
  "drips": {
    "accountId": "12345",
    "address": "0xabcdef1234567890abcdef1234567890abcdef12"
  },
  "dependencies": {
    "react": "^18.0.0"
  }
}`
	path := writeTestFile(t, "package.json", content)

	p := parser.NewPackageJSONParser()
	result, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	// Self-declared Drips metadata.
	if result.SelfDrips == nil {
		t.Fatal("SelfDrips is nil, expected Drips metadata")
	}
	if result.SelfDrips.AccountID != "12345" {
		t.Errorf("SelfDrips.AccountID = %q, want %q", result.SelfDrips.AccountID, "12345")
	}
	if result.SelfDrips.Address != "0xabcdef1234567890abcdef1234567890abcdef12" {
		t.Errorf("SelfDrips.Address = %q, want %q",
			result.SelfDrips.Address, "0xabcdef1234567890abcdef1234567890abcdef12")
	}
}

func TestPackageJSONParser_Parse_NoDeps(t *testing.T) {
	content := `{
  "name": "bare-project",
  "version": "0.0.1"
}`
	path := writeTestFile(t, "package.json", content)

	p := parser.NewPackageJSONParser()
	result, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if got := len(result.Dependencies); got != 0 {
		t.Errorf("len(Dependencies) = %d, want 0", got)
	}
}

func TestPackageJSONParser_Parse_InvalidJSON(t *testing.T) {
	content := `{ invalid json }`
	path := writeTestFile(t, "package.json", content)

	p := parser.NewPackageJSONParser()
	_, err := p.Parse(path)
	if err == nil {
		t.Fatal("Parse() expected error for invalid JSON, got nil")
	}
}

func TestPackageJSONParser_Type(t *testing.T) {
	p := parser.NewPackageJSONParser()
	if got := p.Type(); got != parser.ManifestPackageJSON {
		t.Errorf("Type() = %q, want %q", got, parser.ManifestPackageJSON)
	}
}

// --------------------------------------------------------------------------
// Manifest detection tests
// --------------------------------------------------------------------------

func TestDetectManifest_GoMod(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "go.mod", "module example\n\ngo 1.22\n")

	p, path, err := parser.DetectManifest(dir, "")
	if err != nil {
		t.Fatalf("DetectManifest() returned error: %v", err)
	}
	if p.Type() != parser.ManifestGoMod {
		t.Errorf("parser type = %q, want %q", p.Type(), parser.ManifestGoMod)
	}
	if filepath.Base(path) != "go.mod" {
		t.Errorf("path = %q, want go.mod", path)
	}
}

func TestDetectManifest_PackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "package.json", `{"name":"test"}`)

	p, path, err := parser.DetectManifest(dir, "")
	if err != nil {
		t.Fatalf("DetectManifest() returned error: %v", err)
	}
	if p.Type() != parser.ManifestPackageJSON {
		t.Errorf("parser type = %q, want %q", p.Type(), parser.ManifestPackageJSON)
	}
	if filepath.Base(path) != "package.json" {
		t.Errorf("path = %q, want package.json", path)
	}
}

func TestDetectManifest_GoModTakesPriority(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, dir, "go.mod", "module example\n\ngo 1.22\n")
	writeFileAt(t, dir, "package.json", `{"name":"test"}`)

	p, _, err := parser.DetectManifest(dir, "")
	if err != nil {
		t.Fatalf("DetectManifest() returned error: %v", err)
	}
	// go.mod should take priority over package.json.
	if p.Type() != parser.ManifestGoMod {
		t.Errorf("parser type = %q, want %q (go.mod should take priority)", p.Type(), parser.ManifestGoMod)
	}
}

func TestDetectManifest_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	pkgPath := writeFileAt(t, dir, "package.json", `{"name":"explicit"}`)

	p, path, err := parser.DetectManifest(dir, pkgPath)
	if err != nil {
		t.Fatalf("DetectManifest() returned error: %v", err)
	}
	if p.Type() != parser.ManifestPackageJSON {
		t.Errorf("parser type = %q, want %q", p.Type(), parser.ManifestPackageJSON)
	}
	if path != pkgPath {
		t.Errorf("path = %q, want %q", path, pkgPath)
	}
}

func TestDetectManifest_NoManifest(t *testing.T) {
	dir := t.TempDir()
	_, _, err := parser.DetectManifest(dir, "")
	if err == nil {
		t.Fatal("DetectManifest() expected error for empty directory, got nil")
	}
}

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

// writeTestFile creates a temporary file with the given name and content,
// returning the full path.
func writeTestFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	return writeFileAt(t, dir, name, content)
}

// writeFileAt writes content to a file in the given directory, returning
// the full path.
func writeFileAt(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
	return path
}

// depMap creates a lookup map from dependency name to Dependency.
func depMap(deps []parser.Dependency) map[string]parser.Dependency {
	m := make(map[string]parser.Dependency, len(deps))
	for _, d := range deps {
		m[d.Name] = d
	}
	return m
}

// assertDep checks that a dependency exists in the map with the expected
// version and direct status.
func assertDep(t *testing.T, deps map[string]parser.Dependency, name, version string, direct bool) {
	t.Helper()
	dep, ok := deps[name]
	if !ok {
		t.Errorf("dependency %q not found", name)
		return
	}
	if dep.Version != version {
		t.Errorf("%s: Version = %q, want %q", name, dep.Version, version)
	}
	if dep.Direct != direct {
		t.Errorf("%s: Direct = %v, want %v", name, dep.Direct, direct)
	}
}
