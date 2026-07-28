package project

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Override allows a project to pin a specific software version, overriding the
// software definition's default_version.
type Override struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// PackageConfig holds packager-specific options.
type PackageConfig struct {
	Type    string            `yaml:"type"` // deb, rpm, pkg, msi, tar
	Options map[string]string `yaml:"options,omitempty"`
}

// CompressConfig describes post-build compression.
type CompressConfig struct {
	Type   string `yaml:"type"` // xz, gzip, bzip2, zstd
	Level  int    `yaml:"level,omitempty"`
	Output string `yaml:"output,omitempty"`
}

// ExtraPackageFile lets projects inject additional files into the install root.
type ExtraPackageFile struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
	Mode        string `yaml:"mode,omitempty"`
}

// Definition is the Go equivalent of a Ruby Omnibus project DSL file.
type Definition struct {
	Name              string             `yaml:"name"`
	FriendlyName      string             `yaml:"friendly_name,omitempty"`
	Maintainer        string             `yaml:"maintainer"`
	Homepage          string             `yaml:"homepage,omitempty"`
	Description       string             `yaml:"description,omitempty"`
	InstallDir        string             `yaml:"install_dir"`
	BuildVersion      string             `yaml:"build_version"`
	BuildIteration    int                `yaml:"build_iteration"`
	Dependencies      []string           `yaml:"dependencies"`
	Overrides         []Override         `yaml:"overrides,omitempty"`
	Packages          []PackageConfig    `yaml:"packages,omitempty"`
	Compress          *CompressConfig    `yaml:"compress,omitempty"`
	ExtraPackageFiles []ExtraPackageFile `yaml:"extra_package_files,omitempty"`
	RuntimeDeps       []string           `yaml:"runtime_dependencies,omitempty"`
	ConflictsWith     []string           `yaml:"conflicts,omitempty"`
	Replaces          []string           `yaml:"replaces,omitempty"`
	Env               map[string]string  `yaml:"env,omitempty"`
	ExcludeFiles      []string           `yaml:"exclude_files,omitempty"`
	IncludeFiles      []string           `yaml:"include_files,omitempty"`
	License           string             `yaml:"license,omitempty"`
	LicenseFile       string             `yaml:"license_file,omitempty"`
}

// OverrideMap returns overrides keyed by software name for fast lookup.
func (d *Definition) OverrideMap() map[string]string {
	m := make(map[string]string, len(d.Overrides))
	for _, o := range d.Overrides {
		m[o.Name] = o.Version
	}
	return m
}

// LoadFromFile parses a YAML project definition file.
func LoadFromFile(path string) (*Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading project definition %q: %w", path, err)
	}
	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parsing project definition %q: %w", path, err)
	}
	if def.Name == "" {
		def.Name = filepath.Base(path[:len(path)-len(filepath.Ext(path))])
	}
	return &def, nil
}

// LoadFromDir finds a project definition by name inside a directory.
func LoadFromDir(dir, name string) (*Definition, error) {
	path := filepath.Join(dir, name+".yaml")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("project %q not found in %s", name, dir)
	}
	return LoadFromFile(path)
}
