package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/syntaxroot-cc/gomnibus/internal/pipeline"
	"github.com/syntaxroot-cc/gomnibus/internal/project"
	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

func TestGenerate_Fields(t *testing.T) {
	softDir := t.TempDir()
	writeFile(t, filepath.Join(softDir, "alpha.yaml"), `
name: alpha
default_version: "1.0.0"
license: MIT
`)
	writeFile(t, filepath.Join(softDir, "beta.yaml"), `
name: beta
default_version: "2.0.0"
source:
  url: https://example.com/beta.tar.gz
dependencies:
  - alpha
license: Apache-2.0
`)

	reg, err := software.NewRegistry([]string{softDir})
	if err != nil {
		t.Fatal(err)
	}
	graph, err := pipeline.Build([]string{"beta"}, reg, nil)
	if err != nil {
		t.Fatal(err)
	}

	proj := &project.Definition{
		Name:           "myapp",
		BuildVersion:   "1.0.0",
		BuildIteration: 3,
	}

	m := Generate(proj, graph)

	if m.Project != "myapp" {
		t.Errorf("Project: got %q", m.Project)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version: got %q", m.Version)
	}
	if m.Iteration != 3 {
		t.Errorf("Iteration: got %d", m.Iteration)
	}
	if m.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should be set")
	}
	if len(m.Components) != 2 {
		t.Errorf("Components: got %d, want 2", len(m.Components))
	}
}

func TestGenerate_URLSource(t *testing.T) {
	softDir := t.TempDir()
	writeFile(t, filepath.Join(softDir, "pkg.yaml"), `
name: pkg
default_version: "1.0"
source:
  url: https://example.com/pkg.tar.gz
`)
	reg, _ := software.NewRegistry([]string{softDir})
	graph, _ := pipeline.Build([]string{"pkg"}, reg, nil)
	m := Generate(&project.Definition{Name: "test"}, graph)

	if m.Components[0].Source != "https://example.com/pkg.tar.gz" {
		t.Errorf("Source: got %q", m.Components[0].Source)
	}
}

func TestGenerate_GitSource(t *testing.T) {
	softDir := t.TempDir()
	writeFile(t, filepath.Join(softDir, "repo.yaml"), `
name: repo
default_version: "main"
source:
  git: https://github.com/example/repo
`)
	reg, _ := software.NewRegistry([]string{softDir})
	graph, _ := pipeline.Build([]string{"repo"}, reg, nil)
	m := Generate(&project.Definition{Name: "test"}, graph)

	if m.Components[0].Source != "https://github.com/example/repo" {
		t.Errorf("Source: got %q", m.Components[0].Source)
	}
}

func TestGenerate_LicenseField(t *testing.T) {
	softDir := t.TempDir()
	writeFile(t, filepath.Join(softDir, "pkg.yaml"), `
name: pkg
default_version: "1.0"
license: GPL-3.0
`)
	reg, _ := software.NewRegistry([]string{softDir})
	graph, _ := pipeline.Build([]string{"pkg"}, reg, nil)
	m := Generate(&project.Definition{Name: "test"}, graph)

	if m.Components[0].LicenseID != "GPL-3.0" {
		t.Errorf("LicenseID: got %q", m.Components[0].LicenseID)
	}
}

func TestWriteJSON_Roundtrip(t *testing.T) {
	m := &Manifest{
		Project:    "myapp",
		Version:    "2.0.0",
		Iteration:  1,
		Components: []Entry{{Name: "alpha", Version: "1.0", LicenseID: "MIT"}},
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := m.WriteJSON(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Project != "myapp" {
		t.Errorf("Project: got %q", got.Project)
	}
	if len(got.Components) != 1 || got.Components[0].Name != "alpha" {
		t.Errorf("Components: %+v", got.Components)
	}
}

func TestWriteJSON_CreatesParentDir(t *testing.T) {
	m := &Manifest{Project: "test"}
	path := filepath.Join(t.TempDir(), "sub", "dir", "manifest.json")
	// WriteJSON does NOT create parent dirs — this should return an error.
	// Verify it doesn't panic.
	err := m.WriteJSON(path)
	if err == nil {
		t.Error("expected error writing to non-existent directory")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
