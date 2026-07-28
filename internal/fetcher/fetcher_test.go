// Package fetcher_test verifies the dispatch logic in fetcher.For using
// real fetcher implementations registered via blank imports.
package fetcher_test

import (
	"testing"

	"github.com/syntaxroot-cc/gomnibus/internal/fetcher"
	_ "github.com/syntaxroot-cc/gomnibus/internal/fetcher/git"
	_ "github.com/syntaxroot-cc/gomnibus/internal/fetcher/net"
	_ "github.com/syntaxroot-cc/gomnibus/internal/fetcher/path"
	_ "github.com/syntaxroot-cc/gomnibus/internal/fetcher/s3"
	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

func TestFor_Git(t *testing.T) {
	f, err := fetcher.For(&software.Source{Git: "https://github.com/foo/bar"})
	if err != nil {
		t.Fatalf("For git: %v", err)
	}
	if f.Name() != "git" {
		t.Errorf("Name: got %q, want git", f.Name())
	}
}

func TestFor_Net(t *testing.T) {
	f, err := fetcher.For(&software.Source{URL: "https://example.com/foo.tar.gz"})
	if err != nil {
		t.Fatalf("For net: %v", err)
	}
	if f.Name() != "net" {
		t.Errorf("Name: got %q, want net", f.Name())
	}
}

func TestFor_Path(t *testing.T) {
	f, err := fetcher.For(&software.Source{Path: "/usr/local/src/mylib"})
	if err != nil {
		t.Fatalf("For path: %v", err)
	}
	if f.Name() != "path" {
		t.Errorf("Name: got %q, want path", f.Name())
	}
}

func TestFor_S3(t *testing.T) {
	f, err := fetcher.For(&software.Source{S3Bucket: "my-bucket", S3Key: "prefix/file.tar.gz"})
	if err != nil {
		t.Fatalf("For s3: %v", err)
	}
	if f.Name() != "s3" {
		t.Errorf("Name: got %q, want s3", f.Name())
	}
}

func TestFor_GitTakesPrecedence(t *testing.T) {
	// If both git and url are set, git wins because the switch is ordered.
	f, err := fetcher.For(&software.Source{Git: "https://github.com/foo/bar", URL: "https://example.com/x.tgz"})
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if f.Name() != "git" {
		t.Errorf("Name: got %q, want git (git takes precedence over url)", f.Name())
	}
}

func TestFor_NoSource(t *testing.T) {
	_, err := fetcher.For(&software.Source{})
	if err == nil {
		t.Error("expected error for empty source")
	}
}
