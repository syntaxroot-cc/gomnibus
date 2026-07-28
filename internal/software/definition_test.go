package software

import (
	"os"
	"path/filepath"
	"testing"
)

// ── Definition.Resolve ──────────────────────────────────────────────────────

func TestResolve_UsesDefaultVersion(t *testing.T) {
	def := &Definition{Name: "foo", DefaultVersion: "1.0.0"}
	if err := def.Resolve(""); err != nil {
		t.Fatal(err)
	}
	if def.ResolvedVersion != "1.0.0" {
		t.Errorf("got %q, want 1.0.0", def.ResolvedVersion)
	}
}

func TestResolve_OverrideWins(t *testing.T) {
	def := &Definition{Name: "foo", DefaultVersion: "1.0.0"}
	if err := def.Resolve("2.0.0"); err != nil {
		t.Fatal(err)
	}
	if def.ResolvedVersion != "2.0.0" {
		t.Errorf("got %q, want 2.0.0", def.ResolvedVersion)
	}
}

func TestResolve_VersionBlockOverridesSource(t *testing.T) {
	origSrc := &Source{URL: "https://example.com/1.0.tar.gz"}
	v2Src := &Source{URL: "https://example.com/2.0.tar.gz"}
	def := &Definition{
		Name:           "foo",
		DefaultVersion: "1.0.0",
		Source:         origSrc,
		Versions:       []VersionBlock{{Version: "2.0.0", Source: v2Src}},
	}
	if err := def.Resolve("2.0.0"); err != nil {
		t.Fatal(err)
	}
	if def.Source != v2Src {
		t.Error("source not replaced by version block")
	}
}

func TestResolve_VersionBlockOverridesBuild(t *testing.T) {
	def := &Definition{
		Name:           "foo",
		DefaultVersion: "1.0.0",
		Build:          []BuildStep{{Command: "original"}},
		Versions:       []VersionBlock{{Version: "2.0.0", Build: []BuildStep{{Command: "overridden"}}}},
	}
	if err := def.Resolve("2.0.0"); err != nil {
		t.Fatal(err)
	}
	if len(def.Build) != 1 || def.Build[0].Command != "overridden" {
		t.Errorf("build not overridden: %+v", def.Build)
	}
}

func TestResolve_VersionBlockNoMatch_KeepsOriginal(t *testing.T) {
	origSrc := &Source{URL: "https://example.com/1.0.tar.gz"}
	def := &Definition{
		Name:           "foo",
		DefaultVersion: "1.0.0",
		Source:         origSrc,
		Versions:       []VersionBlock{{Version: "2.0.0", Source: &Source{URL: "other"}}},
	}
	if err := def.Resolve(""); err != nil {
		t.Fatal(err)
	}
	if def.Source != origSrc {
		t.Error("original source should be unchanged when no version block matches")
	}
}

// ── LoadFromFile ─────────────────────────────────────────────────────────────

func TestLoadFromFile_FullDefinition(t *testing.T) {
	yaml := `
name: zlib
default_version: "1.2.13"
source:
  url: https://zlib.net/zlib-1.2.13.tar.gz
  sha256: abc123
  sha512: def456
dependencies:
  - openssl
build:
  - command: ./configure --prefix=${install_dir}
  - make: [install]
whitelist_files:
  - ".*\\.so.*"
license: Zlib
license_file: LICENSE
skip_healthcheck: true
`
	def, err := LoadFromFile(writeTempYAML(t, "zlib.yaml", yaml))
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "zlib" {
		t.Errorf("Name: %q", def.Name)
	}
	if def.DefaultVersion != "1.2.13" {
		t.Errorf("DefaultVersion: %q", def.DefaultVersion)
	}
	if def.Source == nil {
		t.Fatal("Source is nil")
	}
	if def.Source.URL != "https://zlib.net/zlib-1.2.13.tar.gz" {
		t.Errorf("Source.URL: %q", def.Source.URL)
	}
	if def.Source.SHA256 != "abc123" {
		t.Errorf("Source.SHA256: %q", def.Source.SHA256)
	}
	if len(def.Dependencies) != 1 || def.Dependencies[0] != "openssl" {
		t.Errorf("Dependencies: %v", def.Dependencies)
	}
	if len(def.Build) != 2 {
		t.Errorf("Build steps: got %d", len(def.Build))
	}
	if def.License != "Zlib" {
		t.Errorf("License: %q", def.License)
	}
	if !def.SkipHealthcheck {
		t.Error("SkipHealthcheck: want true")
	}
}

func TestLoadFromFile_InfersNameFromFilename(t *testing.T) {
	def, err := LoadFromFile(writeTempYAML(t, "openssl.yaml", "default_version: \"3.0\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "openssl" {
		t.Errorf("Name inferred from filename: got %q, want openssl", def.Name)
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadFromFile_VersionBlocks(t *testing.T) {
	yaml := `
name: openssl
default_version: "1.1.1"
source:
  url: https://example.com/openssl-1.1.1.tar.gz
versions:
  - version: "3.0.0"
    source:
      url: https://example.com/openssl-3.0.0.tar.gz
      sha256: abc
`
	def, err := LoadFromFile(writeTempYAML(t, "openssl.yaml", yaml))
	if err != nil {
		t.Fatal(err)
	}
	if len(def.Versions) != 1 {
		t.Fatalf("Versions: got %d", len(def.Versions))
	}
	if def.Versions[0].Version != "3.0.0" {
		t.Errorf("Versions[0].Version: %q", def.Versions[0].Version)
	}
}

// ── Registry ─────────────────────────────────────────────────────────────────

func TestRegistry_GetAndAll(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.yaml"), "name: alpha\ndefault_version: \"1.0\"\n")
	writeFile(t, filepath.Join(dir, "beta.yaml"), "name: beta\ndefault_version: \"2.0\"\ndependencies:\n  - alpha\n")

	reg, err := NewRegistry([]string{dir})
	if err != nil {
		t.Fatal(err)
	}

	alpha, err := reg.Get("alpha")
	if err != nil {
		t.Fatalf("Get alpha: %v", err)
	}
	if alpha.DefaultVersion != "1.0" {
		t.Errorf("alpha version: %q", alpha.DefaultVersion)
	}

	if len(reg.All()) != 2 {
		t.Errorf("All: got %d entries, want 2", len(reg.All()))
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	reg, _ := NewRegistry([]string{t.TempDir()})
	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown software")
	}
}

func TestRegistry_MissingDirIgnored(t *testing.T) {
	_, err := NewRegistry([]string{filepath.Join(t.TempDir(), "no-such-dir")})
	if err != nil {
		t.Errorf("missing dir should be silently skipped, got: %v", err)
	}
}

func TestRegistry_IgnoresNonYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.yaml"), "name: alpha\ndefault_version: \"1.0\"\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# readme")
	writeFile(t, filepath.Join(dir, "alpha.bak"), "backup")

	reg, err := NewRegistry([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.All()) != 1 {
		t.Errorf("expected 1 software, got %d (non-.yaml files not filtered)", len(reg.All()))
	}
}

func TestRegistry_FirstDefinitionWins(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	writeFile(t, filepath.Join(dir1, "alpha.yaml"), "name: alpha\ndefault_version: \"1.0\"\n")
	writeFile(t, filepath.Join(dir2, "alpha.yaml"), "name: alpha\ndefault_version: \"2.0\"\n")

	reg, err := NewRegistry([]string{dir1, dir2})
	if err != nil {
		t.Fatal(err)
	}
	alpha, _ := reg.Get("alpha")
	if alpha.DefaultVersion != "1.0" {
		t.Errorf("first dir should win: got version %q", alpha.DefaultVersion)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeTempYAML(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	writeFile(t, path, content)
	return path
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
