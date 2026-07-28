// Package fetcher defines the Fetcher interface and a dispatch registry for
// downloading or copying software sources.
package fetcher

import (
	"context"
	"fmt"

	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

// Fetcher downloads or copies a software source to a destination directory.
type Fetcher interface {
	// Fetch retrieves the source described by src into destDir.
	Fetch(ctx context.Context, src *software.Source, destDir string) error
	// Name returns the fetcher identifier for logging.
	Name() string
}

var registry = map[string]Fetcher{}

// Register adds a Fetcher implementation under its name.
func Register(f Fetcher) {
	registry[f.Name()] = f
}

// For selects the appropriate Fetcher for a given Source.
func For(src *software.Source) (Fetcher, error) {
	switch {
	case src.Git != "":
		return registry["git"], nil
	case src.URL != "":
		return registry["net"], nil
	case src.Path != "":
		return registry["path"], nil
	case src.S3Bucket != "":
		return registry["s3"], nil
	default:
		return nil, fmt.Errorf("no source specified — set url, git, path, or s3_bucket")
	}
}
