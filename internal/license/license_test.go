package license

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syntaxroot-cc/gomnibus/internal/pipeline"
	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

func TestCheck_AllLicensed(t *testing.T) {
	entries := []Entry{
		{Name: "alpha", LicenseID: "MIT"},
		{Name: "beta", LicenseID: "Apache-2.0"},
	}
	if err := Check(entries); err != nil {
		t.Errorf("unexpected error for fully-licensed entries: %v", err)
	}
}

func TestCheck_Missing(t *testing.T) {
	entries := []Entry{
		{Name: "alpha", LicenseID: "MIT"},
		{Name: "beta"},
		{Name: "gamma"},
	}
	err := Check(entries)
	if err == nil {
		t.Fatal("expected error for missing licenses")
	}
	if !strings.Contains(err.Error(), "beta") {
		t.Errorf("error should mention 'beta': %v", err)
	}
	if !strings.Contains(err.Error(), "gamma") {
		t.Errorf("error should mention 'gamma': %v", err)
	}
}

func TestCheck_Empty(t *testing.T) {
	if err := Check(nil); err != nil {
		t.Errorf("empty entry list should pass: %v", err)
	}
}

func TestCollect_Entries(t *testing.T) {
	softDir := t.TempDir()
	writeFile(t, filepath.Join(softDir, "alpha.yaml"), `
name: alpha
default_version: "1.0.0"
license: MIT
`)
	writeFile(t, filepath.Join(softDir, "beta.yaml"), `
name: beta
default_version: "2.0.0"
license: Apache-2.0
dependencies:
  - alpha
`)

	reg, err := software.NewRegistry([]string{softDir})
	if err != nil {
		t.Fatal(err)
	}
	graph, err := pipeline.Build([]string{"beta"}, reg, nil)
	if err != nil {
		t.Fatal(err)
	}

	entries := Collect(graph, t.TempDir())
	if len(entries) != 2 {
		t.Fatalf("Collect: got %d entries, want 2", len(entries))
	}

	byName := make(map[string]Entry)
	for _, e := range entries {
		byName[e.Name] = e
	}
	if byName["alpha"].LicenseID != "MIT" {
		t.Errorf("alpha license: got %q", byName["alpha"].LicenseID)
	}
	if byName["beta"].LicenseID != "Apache-2.0" {
		t.Errorf("beta license: got %q", byName["beta"].LicenseID)
	}
}

func TestCollect_ReadsLicenseFile(t *testing.T) {
	installDir := t.TempDir()
	softDir := t.TempDir()

	writeFile(t, filepath.Join(softDir, "pkg.yaml"), `
name: pkg
default_version: "1.0.0"
license: MIT
license_file: LICENSE.txt
`)
	writeFile(t, filepath.Join(installDir, "LICENSE.txt"), "MIT License text")

	reg, _ := software.NewRegistry([]string{softDir})
	graph, _ := pipeline.Build([]string{"pkg"}, reg, nil)

	entries := Collect(graph, installDir)
	if len(entries) == 0 {
		t.Fatal("no entries")
	}
	if entries[0].Content != "MIT License text" {
		t.Errorf("Content: got %q", entries[0].Content)
	}
	if entries[0].LicenseFile != "LICENSE.txt" {
		t.Errorf("LicenseFile: got %q", entries[0].LicenseFile)
	}
}

func TestCollect_MissingLicenseFile_NoError(t *testing.T) {
	softDir := t.TempDir()
	writeFile(t, filepath.Join(softDir, "pkg.yaml"), `
name: pkg
default_version: "1.0.0"
license_file: MISSING.txt
`)
	reg, _ := software.NewRegistry([]string{softDir})
	graph, _ := pipeline.Build([]string{"pkg"}, reg, nil)

	// Should not panic or return an error — missing file is silently skipped.
	entries := Collect(graph, t.TempDir())
	if len(entries) == 0 {
		t.Fatal("expected one entry even with missing license file")
	}
	if entries[0].Content != "" {
		t.Error("Content should be empty when license file is absent")
	}
}

func TestWriteJSON_Roundtrip(t *testing.T) {
	entries := []Entry{
		{Name: "alpha", Version: "1.0", LicenseID: "MIT", Content: "MIT text"},
		{Name: "beta", Version: "2.0", LicenseID: "Apache-2.0"},
	}
	path := filepath.Join(t.TempDir(), "licenses.json")
	if err := WriteJSON(entries, path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got []Entry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	if got[0].Content != "MIT text" {
		t.Errorf("Content: got %q", got[0].Content)
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
