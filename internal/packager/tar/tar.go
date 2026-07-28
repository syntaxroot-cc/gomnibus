// Package tar produces compressed tarballs as a universal fallback packager.
package tar

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/syntaxroot-cc/gomnibus/internal/packager"
	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

func init() {
	packager.Register(&TarPackager{})
}

type TarPackager struct{}

func (t *TarPackager) Name() string { return "tar" }

func (t *TarPackager) Pack(_ context.Context, proj *project.Definition, installDir, outputDir string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	outName := fmt.Sprintf("%s-%s-%d.tar.gz", proj.Name, proj.BuildVersion, proj.BuildIteration)
	outPath := filepath.Join(outputDir, outName)

	f, err := os.Create(outPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.Walk(installDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(installDir, path)
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(tw, src)
		return err
	})
	if err != nil {
		return nil, err
	}
	return []string{outPath}, nil
}
