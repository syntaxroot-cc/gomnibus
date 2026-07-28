package path

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/syntaxroot-cc/gomnibus/internal/fetcher"
	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

func init() {
	fetcher.Register(&PathFetcher{})
}

type PathFetcher struct{}

func (p *PathFetcher) Name() string { return "path" }

func (p *PathFetcher) Fetch(_ context.Context, src *software.Source, destDir string) error {
	info, err := os.Stat(src.Path)
	if err != nil {
		return fmt.Errorf("path source %q: %w", src.Path, err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src.Path, destDir)
	}
	return copyFile(src.Path, filepath.Join(destDir, filepath.Base(src.Path)))
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
