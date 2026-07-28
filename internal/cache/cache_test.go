package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

// ── Key ──────────────────────────────────────────────────────────────────────

func TestKey_Deterministic(t *testing.T) {
	k1, err := Key("zlib", "1.2.13", "")
	if err != nil {
		t.Fatal(err)
	}
	k2, _ := Key("zlib", "1.2.13", "")
	if k1 != k2 {
		t.Errorf("Key is not deterministic: %q vs %q", k1, k2)
	}
}

func TestKey_DifferentName(t *testing.T) {
	k1, _ := Key("zlib", "1.0", "")
	k2, _ := Key("openssl", "1.0", "")
	if k1 == k2 {
		t.Error("different names should produce different keys")
	}
}

func TestKey_DifferentVersion(t *testing.T) {
	k1, _ := Key("zlib", "1.0", "")
	k2, _ := Key("zlib", "2.0", "")
	if k1 == k2 {
		t.Error("different versions should produce different keys")
	}
}

func TestKey_IncludesFileContent(t *testing.T) {
	f := filepath.Join(t.TempDir(), "def.yaml")
	if err := os.WriteFile(f, []byte("version: v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	k1, _ := Key("zlib", "1.0", f)

	if err := os.WriteFile(f, []byte("version: v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	k2, _ := Key("zlib", "1.0", f)

	if k1 == k2 {
		t.Error("key should change when definition file content changes")
	}
}

func TestKey_MissingFileStillWorks(t *testing.T) {
	// A missing defPath should not return an error — it's treated as empty.
	k, err := Key("zlib", "1.0", "/nonexistent/path.yaml")
	if err != nil {
		t.Fatalf("Key with missing file: %v", err)
	}
	if len(k) != 16 {
		t.Errorf("key length: got %d, want 16", len(k))
	}
}

func TestKey_Length(t *testing.T) {
	k, _ := Key("foo", "1.0", "")
	if len(k) != 16 {
		t.Errorf("key length: got %d, want 16", len(k))
	}
}

// ── LocalCache ───────────────────────────────────────────────────────────────

func TestLocalCache_RoundTrip(t *testing.T) {
	cacheDir := t.TempDir()
	srcDir := t.TempDir()
	restoreDir := t.TempDir()

	writeTestFile(t, filepath.Join(srcDir, "file.txt"), "hello cache")
	writeTestFile(t, filepath.Join(srcDir, "sub", "nested.txt"), "nested")

	lc := &LocalCache{Dir: cacheDir, Log: zapNop()}
	ctx := context.Background()
	const key = "testkey"

	has, err := lc.Has(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("Has: expected false before Store")
	}

	if err := lc.Store(ctx, key, srcDir); err != nil {
		t.Fatalf("Store: %v", err)
	}

	has, err = lc.Has(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("Has: expected true after Store")
	}

	if err := lc.Restore(ctx, key, restoreDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	checkFileContent(t, filepath.Join(restoreDir, "file.txt"), "hello cache")
	checkFileContent(t, filepath.Join(restoreDir, "sub", "nested.txt"), "nested")
}

func TestLocalCache_Has_MissingKey(t *testing.T) {
	lc := &LocalCache{Dir: t.TempDir(), Log: zapNop()}
	has, err := lc.Has(context.Background(), "missing")
	if err != nil || has {
		t.Errorf("Has missing key: got (%v, %v), want (false, nil)", has, err)
	}
}

func TestLocalCache_CreatesDir(t *testing.T) {
	srcDir := t.TempDir()
	writeTestFile(t, filepath.Join(srcDir, "a.txt"), "content")

	cacheDir := filepath.Join(t.TempDir(), "new", "cache", "dir")
	lc := &LocalCache{Dir: cacheDir, Log: zapNop()}
	if err := lc.Store(context.Background(), "k", srcDir); err != nil {
		t.Fatalf("Store without pre-existing dir: %v", err)
	}
	if _, err := os.Stat(cacheDir); err != nil {
		t.Errorf("cache dir not created: %v", err)
	}
}

// ── NopCache ─────────────────────────────────────────────────────────────────

func TestNopCache(t *testing.T) {
	ctx := context.Background()
	nc := &NopCache{}

	has, err := nc.Has(ctx, "key")
	if err != nil || has {
		t.Errorf("Has: got (%v, %v), want (false, nil)", has, err)
	}
	if err := nc.Store(ctx, "key", "."); err != nil {
		t.Errorf("Store: unexpected error: %v", err)
	}
	if err := nc.Restore(ctx, "key", "."); err != nil {
		t.Errorf("Restore: unexpected error: %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func checkFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("content of %s: got %q, want %q", path, got, want)
	}
}

func zapNop() *zap.Logger {
	l, _ := zap.NewDevelopment()
	return l
}
