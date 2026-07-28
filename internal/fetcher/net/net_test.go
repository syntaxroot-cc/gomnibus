package net

import (
	"context"
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

func TestName(t *testing.T) {
	if (&NetFetcher{}).Name() != "net" {
		t.Error("Name: want net")
	}
}

func TestFetch_OK_NoChecksum(t *testing.T) {
	content := []byte("archive content here")
	srv := serveContent(t, content)

	dstDir := t.TempDir()
	err := (&NetFetcher{}).Fetch(context.Background(),
		&software.Source{URL: srv.URL + "/file.tar.gz"}, dstDir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	checkFile(t, filepath.Join(dstDir, "file.tar.gz"), content)
}

func TestFetch_SHA256_OK(t *testing.T) {
	content := []byte("checksum test content")
	h := sha256.Sum256(content)
	srv := serveContent(t, content)

	err := (&NetFetcher{}).Fetch(context.Background(), &software.Source{
		URL:    srv.URL + "/archive.tar.gz",
		SHA256: hex.EncodeToString(h[:]),
	}, t.TempDir())
	if err != nil {
		t.Fatalf("Fetch with correct SHA256: %v", err)
	}
}

func TestFetch_SHA256_Mismatch(t *testing.T) {
	srv := serveContent(t, []byte("real content"))

	err := (&NetFetcher{}).Fetch(context.Background(), &software.Source{
		URL:    srv.URL + "/archive.tar.gz",
		SHA256: "0000000000000000000000000000000000000000000000000000000000000000",
	}, t.TempDir())
	if err == nil {
		t.Error("expected checksum mismatch error")
	}
}

func TestFetch_SHA512_OK(t *testing.T) {
	content := []byte("sha512 content")
	h := sha512.Sum512(content)
	srv := serveContent(t, content)

	err := (&NetFetcher{}).Fetch(context.Background(), &software.Source{
		URL:    srv.URL + "/f.tar.gz",
		SHA512: hex.EncodeToString(h[:]),
	}, t.TempDir())
	if err != nil {
		t.Fatalf("Fetch with correct SHA512: %v", err)
	}
}

func TestFetch_SHA1_OK(t *testing.T) {
	content := []byte("sha1 content")
	h := sha1.Sum(content) //nolint:gosec
	srv := serveContent(t, content)

	err := (&NetFetcher{}).Fetch(context.Background(), &software.Source{
		URL:  srv.URL + "/f.tar.gz",
		SHA1: hex.EncodeToString(h[:]),
	}, t.TempDir())
	if err != nil {
		t.Fatalf("Fetch with correct SHA1: %v", err)
	}
}

func TestFetch_MD5_OK(t *testing.T) {
	content := []byte("md5 content")
	h := md5.Sum(content) //nolint:gosec
	srv := serveContent(t, content)

	err := (&NetFetcher{}).Fetch(context.Background(), &software.Source{
		URL: srv.URL + "/f.tar.gz",
		MD5: hex.EncodeToString(h[:]),
	}, t.TempDir())
	if err != nil {
		t.Fatalf("Fetch with correct MD5: %v", err)
	}
}

func TestFetch_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := (&NetFetcher{}).Fetch(context.Background(),
		&software.Source{URL: srv.URL + "/missing.tar.gz"}, t.TempDir())
	if err == nil {
		t.Error("expected error for HTTP 404")
	}
}

func TestFetch_CreatesDestDir(t *testing.T) {
	content := []byte("content")
	srv := serveContent(t, content)
	dstDir := filepath.Join(t.TempDir(), "new", "dest")

	err := (&NetFetcher{}).Fetch(context.Background(),
		&software.Source{URL: srv.URL + "/f.tar.gz"}, dstDir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, err := os.Stat(dstDir); err != nil {
		t.Errorf("dest dir not created: %v", err)
	}
}

func TestFetch_ChecksumMismatch_RemovesFile(t *testing.T) {
	srv := serveContent(t, []byte("some data"))
	dstDir := t.TempDir()

	_ = (&NetFetcher{}).Fetch(context.Background(), &software.Source{
		URL:    srv.URL + "/f.tar.gz",
		SHA256: "badhash" + "0000000000000000000000000000000000000000000000000000",
	}, dstDir)

	// The partially-downloaded file should have been removed on checksum failure.
	if _, err := os.Stat(filepath.Join(dstDir, "f.tar.gz")); !os.IsNotExist(err) {
		t.Error("file should be removed after checksum mismatch")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func serveContent(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

func checkFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s: content mismatch", path)
	}
}
