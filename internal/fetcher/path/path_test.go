package path

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

func TestName(t *testing.T) {
	f := &PathFetcher{}
	if f.Name() != "path" {
		t.Errorf("Name: got %q, want path", f.Name())
	}
}

func TestFetch_SingleFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "archive.tar.gz")
	if err := os.WriteFile(srcFile, []byte("binary content"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &PathFetcher{}
	if err := f.Fetch(context.Background(), &software.Source{Path: srcFile}, dstDir); err != nil {
		t.Fatalf("Fetch file: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dstDir, "archive.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary content" {
		t.Errorf("content: got %q", got)
	}
}

func TestFetch_Directory(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	writeFile(t, filepath.Join(srcDir, "a.txt"), "file a")
	writeFile(t, filepath.Join(srcDir, "sub", "b.txt"), "file b")

	f := &PathFetcher{}
	if err := f.Fetch(context.Background(), &software.Source{Path: srcDir}, dstDir); err != nil {
		t.Fatalf("Fetch dir: %v", err)
	}

	checkFile(t, filepath.Join(dstDir, "a.txt"), "file a")
	checkFile(t, filepath.Join(dstDir, "sub", "b.txt"), "file b")
}

func TestFetch_PreservesSubdirStructure(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	writeFile(t, filepath.Join(srcDir, "deep", "path", "file.txt"), "deep file")

	f := &PathFetcher{}
	if err := f.Fetch(context.Background(), &software.Source{Path: srcDir}, dstDir); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	checkFile(t, filepath.Join(dstDir, "deep", "path", "file.txt"), "deep file")
}

func TestFetch_CreatesDestDir(t *testing.T) {
	srcDir := t.TempDir()
	writeFile(t, filepath.Join(srcDir, "f.txt"), "content")

	// dstDir doesn't exist yet
	dstDir := filepath.Join(t.TempDir(), "new", "dest")

	f := &PathFetcher{}
	if err := f.Fetch(context.Background(), &software.Source{Path: srcDir}, dstDir); err != nil {
		t.Fatalf("Fetch to new dir: %v", err)
	}
	if _, err := os.Stat(dstDir); err != nil {
		t.Errorf("dest dir not created: %v", err)
	}
}

func TestFetch_NotFound(t *testing.T) {
	f := &PathFetcher{}
	err := f.Fetch(context.Background(), &software.Source{Path: "/nonexistent/path"}, t.TempDir())
	if err == nil {
		t.Error("expected error for non-existent source path")
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

func checkFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s: got %q, want %q", path, got, want)
	}
}
