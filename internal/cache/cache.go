// Package cache provides a git-based build artefact cache, analogous to
// Omnibus's GitCache, plus an optional S3 backend.
package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// Cache stores and restores build artefacts using a content-addressable key.
type Cache interface {
	// Has returns true if an artefact for key exists.
	Has(ctx context.Context, key string) (bool, error)
	// Restore unpacks a cached artefact into destDir.
	Restore(ctx context.Context, key, destDir string) error
	// Store archives srcDir under key.
	Store(ctx context.Context, key, srcDir string) error
}

// Key computes a cache key from a software name, resolved version, and the
// content hash of its definition file.
func Key(name, version, defPath string) (string, error) {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%s:", name, version)
	if f, err := os.Open(defPath); err == nil {
		defer f.Close()
		_, _ = io.Copy(h, f)
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16], nil
}

// LocalCache stores artefacts as tarballs in a local directory.
type LocalCache struct {
	Dir string
	Log *zap.Logger
}

func (c *LocalCache) Has(_ context.Context, key string) (bool, error) {
	_, err := os.Stat(c.tarPath(key))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (c *LocalCache) Restore(_ context.Context, key, destDir string) error {
	c.Log.Info("restoring from cache", zap.String("key", key))
	return untar(c.tarPath(key), destDir)
}

func (c *LocalCache) Store(_ context.Context, key, srcDir string) error {
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return err
	}
	c.Log.Info("storing to cache", zap.String("key", key))
	return archiveDir(srcDir, c.tarPath(key))
}

func (c *LocalCache) tarPath(key string) string {
	return filepath.Join(c.Dir, key+".tar.gz")
}

// NopCache is a no-op cache used when caching is disabled.
type NopCache struct{}

func (n *NopCache) Has(_ context.Context, _ string) (bool, error) { return false, nil }
func (n *NopCache) Restore(_ context.Context, _, _ string) error  { return nil }
func (n *NopCache) Store(_ context.Context, _, _ string) error    { return nil }
