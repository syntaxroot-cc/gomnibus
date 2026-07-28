package deb

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

// ── buildControl ─────────────────────────────────────────────────────────────

func TestBuildControl_RequiredFields(t *testing.T) {
	proj := &project.Definition{
		Name:         "myapp",
		BuildVersion: "1.0.0",
		Maintainer:   "Test User <test@example.com>",
	}
	ctrl := buildControl(proj, "amd64")

	for _, want := range []string{
		"Package: myapp",
		"Version: 1.0.0",
		"Architecture: amd64",
		"Maintainer: Test User <test@example.com>",
	} {
		if !lineExists(ctrl, want) {
			t.Errorf("expected line %q in control file:\n%s", want, ctrl)
		}
	}
}

func TestBuildControl_Revision_WhenIterationNonZero(t *testing.T) {
	proj := &project.Definition{
		Name:           "myapp",
		BuildVersion:   "1.0.0",
		BuildIteration: 3,
		Maintainer:     "Test",
	}
	ctrl := buildControl(proj, "amd64")
	if !lineExists(ctrl, "Revision: 3") {
		t.Errorf("expected Revision: 3:\n%s", ctrl)
	}
}

func TestBuildControl_NoRevision_WhenIterationZero(t *testing.T) {
	proj := &project.Definition{
		Name: "myapp", BuildVersion: "1.0.0", BuildIteration: 0, Maintainer: "Test",
	}
	ctrl := buildControl(proj, "amd64")
	if strings.Contains(ctrl, "Revision:") {
		t.Errorf("Revision should be absent when BuildIteration == 0:\n%s", ctrl)
	}
}

func TestBuildControl_OptionalFields(t *testing.T) {
	proj := &project.Definition{
		Name:          "myapp",
		BuildVersion:  "1.0.0",
		Maintainer:    "Test",
		Description:   "A test application",
		Homepage:      "https://example.com",
		RuntimeDeps:   []string{"libssl1.1", "libc6"},
		ConflictsWith: []string{"myapp-old"},
		Replaces:      []string{"myapp-legacy"},
	}
	ctrl := buildControl(proj, "amd64")

	for _, want := range []string{
		"Description: A test application",
		"Homepage: https://example.com",
		"Depends: libssl1.1, libc6",
		"Conflicts: myapp-old",
		"Replaces: myapp-legacy",
	} {
		if !lineExists(ctrl, want) {
			t.Errorf("expected line %q in control file:\n%s", want, ctrl)
		}
	}
}

func TestBuildControl_EmptyOptionals_Absent(t *testing.T) {
	proj := &project.Definition{
		Name: "myapp", BuildVersion: "1.0.0", Maintainer: "Test",
	}
	ctrl := buildControl(proj, "amd64")
	for _, field := range []string{"Description:", "Homepage:", "Depends:", "Conflicts:", "Replaces:"} {
		if strings.Contains(ctrl, field) {
			t.Errorf("field %q should be absent when empty:\n%s", field, ctrl)
		}
	}
}

func TestBuildControl_MultipleRuntimeDeps_Joined(t *testing.T) {
	proj := &project.Definition{
		Name:        "myapp",
		BuildVersion: "1.0.0",
		Maintainer:  "Test",
		RuntimeDeps: []string{"dep1", "dep2", "dep3"},
	}
	ctrl := buildControl(proj, "amd64")
	if !lineExists(ctrl, "Depends: dep1, dep2, dep3") {
		t.Errorf("Depends not comma-joined:\n%s", ctrl)
	}
}

func TestBuildControl_ArchPassedThrough(t *testing.T) {
	proj := &project.Definition{Name: "p", BuildVersion: "1.0", Maintainer: "t"}
	for _, arch := range []string{"amd64", "arm64", "i386"} {
		ctrl := buildControl(proj, arch)
		if !lineExists(ctrl, "Architecture: "+arch) {
			t.Errorf("arch %q not in control:\n%s", arch, ctrl)
		}
	}
}

// ── debArch ───────────────────────────────────────────────────────────────────

func TestDebArch_Default_IsAmd64(t *testing.T) {
	t.Setenv("GOARCH", "")
	if got := debArch(); got != "amd64" {
		t.Errorf("empty GOARCH: got %q, want amd64", got)
	}
}

func TestDebArch_ARM64(t *testing.T) {
	t.Setenv("GOARCH", "arm64")
	if got := debArch(); got != "arm64" {
		t.Errorf("GOARCH=arm64: got %q", got)
	}
}

func TestDebArch_386_MapsToI386(t *testing.T) {
	t.Setenv("GOARCH", "386")
	if got := debArch(); got != "i386" {
		t.Errorf("GOARCH=386: got %q, want i386", got)
	}
}

func TestDebArch_AMD64_Explicit(t *testing.T) {
	t.Setenv("GOARCH", "amd64")
	if got := debArch(); got != "amd64" {
		t.Errorf("GOARCH=amd64: got %q", got)
	}
}

func TestDebArch_UnknownGoarch_DefaultsToAmd64(t *testing.T) {
	t.Setenv("GOARCH", "mips64")
	if got := debArch(); got != "amd64" {
		t.Errorf("unknown GOARCH: got %q, want amd64", got)
	}
}

// ── Name ─────────────────────────────────────────────────────────────────────

func TestName(t *testing.T) {
	if (&DebPackager{}).Name() != "deb" {
		t.Error("Name: want deb")
	}
}

// ── Pack (integration) ────────────────────────────────────────────────────────

func TestPack_Integration(t *testing.T) {
	for _, tool := range []string{"dpkg-deb", "cp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}

	installDir := t.TempDir()
	writeFile(t, filepath.Join(installDir, "bin", "myapp"), "#!/bin/sh\necho hello\n")
	writeFile(t, filepath.Join(installDir, "share", "doc", "myapp", "README"), "readme")

	outputDir := t.TempDir()
	proj := &project.Definition{
		Name:           "myapp",
		BuildVersion:   "1.0.0",
		BuildIteration: 1,
		Maintainer:     "Test User <test@example.com>",
		Description:    "Integration test package",
		InstallDir:     "/opt/myapp",
	}

	paths, err := (&DebPackager{}).Pack(context.Background(), proj, installDir, outputDir)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 output, got %d: %v", len(paths), paths)
	}
	if !strings.HasSuffix(paths[0], ".deb") {
		t.Errorf("expected .deb output, got %q", paths[0])
	}
	if _, err := os.Stat(paths[0]); err != nil {
		t.Errorf(".deb file not found: %v", err)
	}
}

func TestPack_Integration_VersionInFilename(t *testing.T) {
	for _, tool := range []string{"dpkg-deb", "cp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}

	installDir := t.TempDir()
	writeFile(t, filepath.Join(installDir, "bin", "tool"), "binary")

	outputDir := t.TempDir()
	proj := &project.Definition{
		Name:           "mytool",
		BuildVersion:   "2.3.4",
		BuildIteration: 2,
		Maintainer:     "Test",
		InstallDir:     "/opt/mytool",
	}

	paths, err := (&DebPackager{}).Pack(context.Background(), proj, installDir, outputDir)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no output files")
	}
	base := filepath.Base(paths[0])
	if !strings.Contains(base, "mytool") || !strings.Contains(base, "2.3.4") {
		t.Errorf("filename should contain name and version: got %q", base)
	}
}

func TestPack_Integration_CreatesOutputDir(t *testing.T) {
	for _, tool := range []string{"dpkg-deb", "cp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}

	installDir := t.TempDir()
	writeFile(t, filepath.Join(installDir, "bin", "app"), "binary")

	outputDir := filepath.Join(t.TempDir(), "new", "output")
	proj := &project.Definition{
		Name:       "app",
		BuildVersion: "1.0.0",
		Maintainer: "Test",
		InstallDir: "/opt/app",
	}

	if _, err := (&DebPackager{}).Pack(context.Background(), proj, installDir, outputDir); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if _, err := os.Stat(outputDir); err != nil {
		t.Errorf("output dir not created: %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// lineExists reports whether content contains a line that exactly matches want.
func lineExists(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		if line == want {
			return true
		}
	}
	return false
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
