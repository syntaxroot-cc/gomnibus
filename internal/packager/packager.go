// Package packager defines the Packager interface and registry for creating
// installable packages from a built install root.
package packager

import (
	"context"
	"fmt"
	"runtime"

	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

// Packager creates a platform-specific installer from an install root.
type Packager interface {
	// Pack produces one or more package artifacts and returns their paths.
	Pack(ctx context.Context, proj *project.Definition, installDir, outputDir string) ([]string, error)
	// Name returns the packager identifier (deb, rpm, pkg, msi, tar).
	Name() string
}

var registry = map[string]Packager{}

// Register adds a Packager under its name.
func Register(p Packager) {
	registry[p.Name()] = p
}

// For returns the Packager for a given type string.
func For(typ string) (Packager, error) {
	p, ok := registry[typ]
	if !ok {
		return nil, fmt.Errorf("packager %q not found (available: deb, rpm, pkg, msi, tar)", typ)
	}
	return p, nil
}

// DefaultForPlatform returns the appropriate default packager for the current OS.
func DefaultForPlatform() (Packager, error) {
	switch runtime.GOOS {
	case "linux":
		if p, ok := registry["deb"]; ok {
			return p, nil
		}
		return For("rpm")
	case "darwin":
		return For("pkg")
	case "windows":
		return For("msi")
	default:
		return For("tar")
	}
}
