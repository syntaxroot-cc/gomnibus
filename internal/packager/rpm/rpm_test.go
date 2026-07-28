package rpm

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

// ── specTemplate ─────────────────────────────────────────────────────────────

func TestSpecTemplate_NameVersionRelease(t *testing.T) {
	var buf bytes.Buffer
	err := specTemplate.Execute(&buf, specData{
		Project: &project.Definition{
			Name:           "myapp",
			BuildVersion:   "1.2.3",
			BuildIteration: 4,
			InstallDir:     "/opt/myapp",
		},
		InstallDir: "/tmp/install",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"myapp", "1.2.3", "4"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in spec:\n%s", want, out)
		}
	}
}

func TestSpecTemplate_SummaryAndDescription(t *testing.T) {
	var buf bytes.Buffer
	specTemplate.Execute(&buf, specData{
		Project: &project.Definition{
			Name:        "pkg",
			Description: "Unique description text",
			InstallDir:  "/opt/pkg",
		},
		InstallDir: "/tmp/install",
	})
	out := buf.String()
	if strings.Count(out, "Unique description text") < 2 {
		t.Errorf("Description should appear in Summary: and %%description; got:\n%s", out)
	}
}

func TestSpecTemplate_LicenseAndURL(t *testing.T) {
	var buf bytes.Buffer
	specTemplate.Execute(&buf, specData{
		Project: &project.Definition{
			Name:       "pkg",
			License:    "Apache-2.0",
			Homepage:   "https://example.com/pkg",
			InstallDir: "/opt/pkg",
		},
		InstallDir: "/tmp/install",
	})
	out := buf.String()
	if !strings.Contains(out, "Apache-2.0") {
		t.Errorf("License not in spec:\n%s", out)
	}
	if !strings.Contains(out, "https://example.com/pkg") {
		t.Errorf("URL not in spec:\n%s", out)
	}
}

func TestSpecTemplate_InstallSection_ReferencesInstallDir(t *testing.T) {
	var buf bytes.Buffer
	specTemplate.Execute(&buf, specData{
		Project:    &project.Definition{Name: "pkg", InstallDir: "/opt/pkg"},
		InstallDir: "/custom/install/root",
	})
	out := buf.String()
	installIdx := strings.Index(out, "%install")
	if installIdx == -1 {
		t.Fatal("%%install section not found")
	}
	section := out[installIdx:]
	if !strings.Contains(section, "/custom/install/root") {
		t.Errorf("%%install section should reference InstallDir:\n%s", section)
	}
}

func TestSpecTemplate_FilesSection_ListsProjectInstallDir(t *testing.T) {
	var buf bytes.Buffer
	specTemplate.Execute(&buf, specData{
		Project:    &project.Definition{Name: "pkg", InstallDir: "/opt/specialapp"},
		InstallDir: "/tmp/install",
	})
	out := buf.String()
	filesIdx := strings.Index(out, "%files")
	if filesIdx == -1 {
		t.Fatal("spec output is missing a files section")
	}
	section := out[filesIdx:]
	if !strings.Contains(section, "/opt/specialapp") {
		t.Errorf("%%files section should list InstallDir:\n%s", section)
	}
}

func TestSpecTemplate_ChangelogSection_Present(t *testing.T) {
	var buf bytes.Buffer
	specTemplate.Execute(&buf, specData{
		Project:    &project.Definition{Name: "pkg", InstallDir: "/opt/pkg"},
		InstallDir: "/tmp/install",
	})
	if !strings.Contains(buf.String(), "%changelog") {
		t.Error("spec output is missing a changelog section")
	}
}

func TestSpecTemplate_ZeroIteration_RendersWithoutPanic(t *testing.T) {
	var buf bytes.Buffer
	err := specTemplate.Execute(&buf, specData{
		Project:    &project.Definition{Name: "pkg", BuildVersion: "1.0.0", BuildIteration: 0, InstallDir: "/opt/pkg"},
		InstallDir: "/tmp/install",
	})
	if err != nil {
		t.Fatalf("template should not error on zero iteration: %v", err)
	}
	if !strings.Contains(buf.String(), "Release:") {
		t.Error("Release field missing")
	}
}

func TestSpecTemplate_EmptyOptionals_RendersWithoutPanic(t *testing.T) {
	var buf bytes.Buffer
	err := specTemplate.Execute(&buf, specData{
		Project:    &project.Definition{Name: "minimal", InstallDir: "/opt/minimal"},
		InstallDir: "/tmp/install",
	})
	if err != nil {
		t.Fatalf("template with empty optional fields: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("spec output should not be empty")
	}
}

// ── Name ─────────────────────────────────────────────────────────────────────

func TestName(t *testing.T) {
	if (&RPMPackager{}).Name() != "rpm" {
		t.Error("Name: want rpm")
	}
}

// ── Pack (integration) ────────────────────────────────────────────────────────

func TestPack_Integration(t *testing.T) {
	if _, err := exec.LookPath("rpmbuild"); err != nil {
		t.Skip("rpmbuild not available")
	}

	installDir := t.TempDir()
	// Create directory structure mirroring InstallDir so rpmbuild %files resolves.
	subDir := filepath.Join(installDir, "opt", "myapp")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(subDir, "bin", "myapp"), "#!/bin/sh\necho hello\n")

	outputDir := t.TempDir()
	proj := &project.Definition{
		Name:           "myapp",
		BuildVersion:   "1.0.0",
		BuildIteration: 1,
		Description:    "Integration test package",
		License:        "MIT",
		Homepage:       "https://example.com",
		InstallDir:     "/opt/myapp",
	}

	paths, err := (&RPMPackager{}).Pack(context.Background(), proj, installDir, outputDir)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one .rpm output")
	}
	for _, p := range paths {
		if !strings.HasSuffix(p, ".rpm") {
			t.Errorf("expected .rpm output, got %q", p)
		}
		if _, err := os.Stat(p); err != nil {
			t.Errorf(".rpm file not found: %v", err)
		}
	}
}

func TestPack_Integration_CreatesOutputDir(t *testing.T) {
	if _, err := exec.LookPath("rpmbuild"); err != nil {
		t.Skip("rpmbuild not available")
	}

	installDir := t.TempDir()
	subDir := filepath.Join(installDir, "opt", "tool")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(t.TempDir(), "new", "rpm", "output")
	proj := &project.Definition{
		Name:       "tool",
		BuildVersion: "1.0.0",
		License:    "MIT",
		InstallDir: "/opt/tool",
	}

	if _, err := (&RPMPackager{}).Pack(context.Background(), proj, installDir, outputDir); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if _, err := os.Stat(outputDir); err != nil {
		t.Errorf("output dir not created: %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
