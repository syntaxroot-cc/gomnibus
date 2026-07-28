package tar

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

func TestName(t *testing.T) {
	if (&TarPackager{}).Name() != "tar" {
		t.Error("Name: want tar")
	}
}

func TestPack_OutputFilename(t *testing.T) {
	installDir := populateInstallRoot(t)
	outputDir := t.TempDir()
	proj := &project.Definition{
		Name:           "myapp",
		BuildVersion:   "1.2.3",
		BuildIteration: 2,
	}
	paths, err := (&TarPackager{}).Pack(context.Background(), proj, installDir, outputDir)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	want := "myapp-1.2.3-2.tar.gz"
	if filepath.Base(paths[0]) != want {
		t.Errorf("filename: got %q, want %q", filepath.Base(paths[0]), want)
	}
}

func TestPack_ContainsAllFiles(t *testing.T) {
	installDir := t.TempDir()
	writeFile(t, filepath.Join(installDir, "bin", "myapp"), "binary")
	writeFile(t, filepath.Join(installDir, "lib", "libfoo.so"), "library")
	writeFile(t, filepath.Join(installDir, "share", "doc", "README"), "docs")

	outputDir := t.TempDir()
	proj := &project.Definition{Name: "pkg", BuildVersion: "1.0.0", BuildIteration: 1}

	paths, err := (&TarPackager{}).Pack(context.Background(), proj, installDir, outputDir)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	entries := listTarEntries(t, paths[0])
	wantFiles := map[string]bool{
		"bin/myapp":        false,
		"lib/libfoo.so":    false,
		"share/doc/README": false,
	}
	for _, e := range entries {
		delete(wantFiles, e)
	}
	for missing := range wantFiles {
		t.Errorf("expected entry %q in tarball", missing)
	}
}

func TestPack_ValidGzip(t *testing.T) {
	installDir := populateInstallRoot(t)
	outputDir := t.TempDir()
	proj := &project.Definition{Name: "test", BuildVersion: "0.1.0", BuildIteration: 1}

	paths, err := (&TarPackager{}).Pack(context.Background(), proj, installDir, outputDir)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	// Verify it's a valid gzip stream
	f, err := os.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("not a valid gzip file: %v", err)
	}
	gr.Close()
}

func TestPack_CreatesOutputDir(t *testing.T) {
	installDir := populateInstallRoot(t)
	outputDir := filepath.Join(t.TempDir(), "new", "output", "dir")
	proj := &project.Definition{Name: "test", BuildVersion: "0.1.0", BuildIteration: 1}

	if _, err := (&TarPackager{}).Pack(context.Background(), proj, installDir, outputDir); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if _, err := os.Stat(outputDir); err != nil {
		t.Errorf("output dir not created: %v", err)
	}
}

func TestPack_EmptyInstallRoot(t *testing.T) {
	installDir := t.TempDir()
	proj := &project.Definition{Name: "empty", BuildVersion: "1.0.0", BuildIteration: 1}

	paths, err := (&TarPackager{}).Pack(context.Background(), proj, installDir, t.TempDir())
	if err != nil {
		t.Fatalf("Pack empty root: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	// File should still be created even for an empty install root
	if _, err := os.Stat(paths[0]); err != nil {
		t.Errorf("tarball not created: %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func populateInstallRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bin", "app"), "binary content")
	writeFile(t, filepath.Join(dir, "lib", "libapp.so"), "library content")
	return dir
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

func listTarEntries(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var entries []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			entries = append(entries, hdr.Name)
		}
	}
	return entries
}
