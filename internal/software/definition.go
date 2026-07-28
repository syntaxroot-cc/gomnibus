package software

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Source describes where and how to fetch a software component.
type Source struct {
	URL         string `yaml:"url"`
	Git         string `yaml:"git"`
	Path        string `yaml:"path"`
	MD5         string `yaml:"md5"`
	SHA1        string `yaml:"sha1"`
	SHA256      string `yaml:"sha256"`
	SHA512      string `yaml:"sha512"`
	Branch      string `yaml:"branch"`
	Tag         string `yaml:"tag"`
	Commit      string `yaml:"commit"`
	Subdir      string `yaml:"subdir"`
	S3Bucket    string `yaml:"s3_bucket"`
	S3Key       string `yaml:"s3_key"`
	S3Region    string `yaml:"s3_region"`
	S3AccessKey string `yaml:"s3_access_key"`
	S3SecretKey string `yaml:"s3_secret_key"`
	S3Profile   string `yaml:"s3_profile"`
}

// BuildStep is a single instruction in the build block.
type BuildStep struct {
	Command   string            `yaml:"command,omitempty"`
	Make      []string          `yaml:"make,omitempty"`
	CMake     []string          `yaml:"cmake,omitempty"`
	Configure []string          `yaml:"configure,omitempty"`
	Gem       []string          `yaml:"gem,omitempty"`
	Go        []string          `yaml:"go,omitempty"`
	Mkdir     string            `yaml:"mkdir,omitempty"`
	Copy      *CopySpec         `yaml:"copy,omitempty"`
	Move      *MoveSpec         `yaml:"move,omitempty"`
	Link      *LinkSpec         `yaml:"link,omitempty"`
	Delete    string            `yaml:"delete,omitempty"`
	Patch     *PatchSpec        `yaml:"patch,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	WorkDir   string            `yaml:"work_dir,omitempty"`
}

type CopySpec struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
}

type MoveSpec struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
}

type LinkSpec struct {
	Target string `yaml:"target"`
	Link   string `yaml:"link"`
}

type PatchSpec struct {
	Source string `yaml:"source"`
	Plevel int    `yaml:"plevel"`
}

// VersionBlock allows version-specific source and build overrides.
type VersionBlock struct {
	Version      string      `yaml:"version"`
	Source       *Source     `yaml:"source,omitempty"`
	Build        []BuildStep `yaml:"build,omitempty"`
	RelativePath string      `yaml:"relative_path,omitempty"`
}

// Definition is the Go equivalent of a Ruby Omnibus software DSL file.
type Definition struct {
	Name            string            `yaml:"name"`
	DefaultVersion  string            `yaml:"default_version"`
	Source          *Source           `yaml:"source,omitempty"`
	RelativePath    string            `yaml:"relative_path,omitempty"`
	Dependencies    []string          `yaml:"dependencies,omitempty"`
	Build           []BuildStep       `yaml:"build,omitempty"`
	Versions        []VersionBlock    `yaml:"versions,omitempty"`
	WhitelistFiles  []string          `yaml:"whitelist_files,omitempty"`
	Env             map[string]string `yaml:"env,omitempty"`
	SkipHealthcheck bool              `yaml:"skip_healthcheck,omitempty"`
	License         string            `yaml:"license,omitempty"`
	LicenseFile     string            `yaml:"license_file,omitempty"`

	// resolved at load time
	ResolvedVersion string `yaml:"-"`
}

// Resolve applies any version-specific overrides and sets ResolvedVersion.
func (d *Definition) Resolve(override string) error {
	version := d.DefaultVersion
	if override != "" {
		version = override
	}
	d.ResolvedVersion = version

	for _, vb := range d.Versions {
		if vb.Version == version {
			if vb.Source != nil {
				d.Source = vb.Source
			}
			if len(vb.Build) > 0 {
				d.Build = vb.Build
			}
			if vb.RelativePath != "" {
				d.RelativePath = vb.RelativePath
			}
			return nil
		}
	}
	return nil
}

// LoadFromFile parses a YAML software definition file.
func LoadFromFile(path string) (*Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading software definition %q: %w", path, err)
	}
	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parsing software definition %q: %w", path, err)
	}
	if def.Name == "" {
		def.Name = filepath.Base(path[:len(path)-len(filepath.Ext(path))])
	}
	return &def, nil
}

// Registry indexes software definitions by name across multiple search dirs.
type Registry struct {
	defs map[string]*Definition
}

func NewRegistry(dirs []string) (*Registry, error) {
	r := &Registry{defs: make(map[string]*Definition)}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
				continue
			}
			def, err := LoadFromFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return nil, err
			}
			if _, exists := r.defs[def.Name]; !exists {
				r.defs[def.Name] = def
			}
		}
	}
	return r, nil
}

func (r *Registry) Get(name string) (*Definition, error) {
	d, ok := r.defs[name]
	if !ok {
		return nil, fmt.Errorf("software %q not found in registry", name)
	}
	return d, nil
}

func (r *Registry) All() map[string]*Definition {
	return r.defs
}
