package project

import (
	"os"
	"path/filepath"
	"testing"
)

var sampleYAML = `
name: myapp
friendly_name: My Application
maintainer: Test User <test@example.com>
homepage: https://example.com
description: A test application
install_dir: /opt/myapp
build_version: "1.0.0"
build_iteration: 2
dependencies:
  - zlib
  - openssl
overrides:
  - name: zlib
    version: "1.2.13"
  - name: openssl
    version: "3.0.0"
license: Apache-2.0
license_file: LICENSE
`

func TestLoadFromFile_FullDefinition(t *testing.T) {
	def, err := LoadFromFile(writeTempYAML(t, sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	check := func(label string, got, want interface{}) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %v, want %v", label, got, want)
		}
	}
	check("Name", def.Name, "myapp")
	check("FriendlyName", def.FriendlyName, "My Application")
	check("BuildVersion", def.BuildVersion, "1.0.0")
	check("BuildIteration", def.BuildIteration, 2)
	check("InstallDir", def.InstallDir, "/opt/myapp")
	check("License", def.License, "Apache-2.0")
	check("LicenseFile", def.LicenseFile, "LICENSE")

	if len(def.Dependencies) != 2 {
		t.Errorf("Dependencies: got %v", def.Dependencies)
	}
	if len(def.Overrides) != 2 {
		t.Errorf("Overrides: got %v", def.Overrides)
	}
}

func TestLoadFromFile_InfersNameFromFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myproj.yaml")
	if err := os.WriteFile(path, []byte("build_version: \"1.0\"\nmaintainer: tester\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	def, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "myproj" {
		t.Errorf("Name: got %q, want myproj", def.Name)
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestOverrideMap(t *testing.T) {
	def := &Definition{
		Overrides: []Override{
			{Name: "zlib", Version: "1.2.13"},
			{Name: "openssl", Version: "3.0.0"},
		},
	}
	m := def.OverrideMap()
	if m["zlib"] != "1.2.13" {
		t.Errorf("zlib: got %q", m["zlib"])
	}
	if m["openssl"] != "3.0.0" {
		t.Errorf("openssl: got %q", m["openssl"])
	}
	if len(m) != 2 {
		t.Errorf("map size: got %d, want 2", len(m))
	}
}

func TestOverrideMap_Empty(t *testing.T) {
	def := &Definition{}
	if m := def.OverrideMap(); len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestLoadFromDir_Found(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "myapp.yaml"), []byte(sampleYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	def, err := LoadFromDir(dir, "myapp")
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "myapp" {
		t.Errorf("Name: got %q", def.Name)
	}
}

func TestLoadFromDir_NotFound(t *testing.T) {
	_, err := LoadFromDir(t.TempDir(), "nonexistent")
	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestPackageConfig_InDefinition(t *testing.T) {
	yaml := `
name: myapp
build_version: "1.0"
maintainer: test
packages:
  - type: deb
    options:
      priority: optional
  - type: rpm
`
	def, err := LoadFromFile(writeTempYAML(t, yaml))
	if err != nil {
		t.Fatal(err)
	}
	if len(def.Packages) != 2 {
		t.Fatalf("Packages: got %d", len(def.Packages))
	}
	if def.Packages[0].Type != "deb" {
		t.Errorf("Packages[0].Type: %q", def.Packages[0].Type)
	}
	if def.Packages[0].Options["priority"] != "optional" {
		t.Errorf("Packages[0].Options: %v", def.Packages[0].Options)
	}
}

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "myapp.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
