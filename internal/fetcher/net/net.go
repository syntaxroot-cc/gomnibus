package net

import (
	"context"
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/syntaxroot-cc/gomnibus/internal/fetcher"
	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

func init() {
	fetcher.Register(&NetFetcher{})
}

type NetFetcher struct{}

func (n *NetFetcher) Name() string { return "net" }

func (n *NetFetcher) Fetch(ctx context.Context, src *software.Source, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	filename := filepath.Base(src.URL)
	dest := filepath.Join(destDir, filename)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", src.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, src.URL)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	var h hash.Hash
	var expected string
	switch {
	case src.SHA256 != "":
		h, expected = sha256.New(), src.SHA256
	case src.SHA512 != "":
		h, expected = sha512.New(), src.SHA512
	case src.SHA1 != "":
		h, expected = sha1.New(), src.SHA1 //nolint:gosec
	case src.MD5 != "":
		h, expected = md5.New(), src.MD5 //nolint:gosec
	}

	var w io.Writer = f
	if h != nil {
		w = io.MultiWriter(f, h)
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return err
	}

	if h != nil {
		got := hex.EncodeToString(h.Sum(nil))
		if got != expected {
			os.Remove(dest)
			return fmt.Errorf("checksum mismatch for %s: got %s, want %s", filename, got, expected)
		}
	}
	return nil
}
