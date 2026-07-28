// Package kitchen implements Test Kitchen-compatible multi-platform build
// orchestration for gomnibus — spinning up containers or VMs, running the
// build inside each, verifying the produced package, and tearing down.
package kitchen

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the parsed representation of a .kitchen.yml file.
// The format is intentionally close to Ruby Test Kitchen so teams migrating
// from Chef Omnibus can reuse their existing configurations.
type Config struct {
	Driver      DriverConfig      `yaml:"driver"`
	Provisioner ProvisionerConfig `yaml:"provisioner"`
	Transport   TransportConfig   `yaml:"transport"`
	Verifier    VerifierConfig    `yaml:"verifier"`
	Platforms   []PlatformConfig  `yaml:"platforms"`
	Suites      []SuiteConfig     `yaml:"suites"`
}

// DriverConfig describes how a platform is created and managed.
type DriverConfig struct {
	Name       string            `yaml:"name"`        // docker | vagrant
	Image      string            `yaml:"image"`       // Docker image (docker driver)
	Box        string            `yaml:"box"`         // Vagrant box
	RunCommand string            `yaml:"run_command"` // override container entrypoint
	Memory     string            `yaml:"memory"`      // e.g. "2g"
	CPUs       int               `yaml:"cpus"`
	Volumes    []string          `yaml:"volumes"`
	EnvVars    map[string]string `yaml:"env"`
	Privileged bool              `yaml:"privileged"`
	Network    string            `yaml:"network"`
	Options    map[string]string `yaml:"options"`
}

// ProvisionerConfig describes how the gomnibus build runs inside the platform.
type ProvisionerConfig struct {
	Name            string   `yaml:"name"`             // always "gomnibus"
	Project         string   `yaml:"project"`          // project name to build
	GomnibusBinary  string   `yaml:"gomnibus_binary"`  // path on host; copied in
	InstallPackages []string `yaml:"install_packages"` // pre-build apt/dnf installs
	BuildArgs       []string `yaml:"build_args"`       // extra flags for gomnibus build
	WorkDir         string   `yaml:"work_dir"`         // inside-container workspace
}

// TransportConfig describes how gomnibus connects to the platform.
type TransportConfig struct {
	Name     string `yaml:"name"` // docker | ssh | winrm
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
}

// VerifierConfig describes how the produced package is verified.
type VerifierConfig struct {
	Name    string   `yaml:"name"`    // shell | inspec
	Command string   `yaml:"command"` // shell command run inside platform
	Scripts []string `yaml:"scripts"` // scripts copied in and executed
}

// PlatformConfig is a single target OS/distro, potentially overriding the
// global driver, provisioner, or verifier config.
type PlatformConfig struct {
	Name        string            `yaml:"name"`
	Driver      DriverConfig      `yaml:"driver"`
	Provisioner ProvisionerConfig `yaml:"provisioner"`
	Verifier    VerifierConfig    `yaml:"verifier"`
}

// SuiteConfig groups settings that apply across all platforms for one logical
// test scenario.  Most projects need only a single "default" suite.
type SuiteConfig struct {
	Name        string            `yaml:"name"`
	Provisioner ProvisionerConfig `yaml:"provisioner"`
	Verifier    VerifierConfig    `yaml:"verifier"`
}

// InstanceConfig is the merged, fully-resolved configuration for one
// (suite, platform) pair — the actual unit of work.
type InstanceConfig struct {
	Name        string // "<suite>-<platform>"
	Suite       SuiteConfig
	Platform    PlatformConfig
	Driver      DriverConfig
	Provisioner ProvisionerConfig
	Transport   TransportConfig
	Verifier    VerifierConfig
}

// LoadConfig reads and parses a .kitchen.yml file.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = ".kitchen.yml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading kitchen config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing kitchen config %q: %w", path, err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Driver.Name == "" {
		c.Driver.Name = "docker"
	}
	if c.Provisioner.Name == "" {
		c.Provisioner.Name = "gomnibus"
	}
	if c.Provisioner.GomnibusBinary == "" {
		c.Provisioner.GomnibusBinary = "gomnibus"
	}
	if c.Provisioner.WorkDir == "" {
		c.Provisioner.WorkDir = "/workspace"
	}
	if c.Verifier.Name == "" {
		c.Verifier.Name = "shell"
	}
	if len(c.Suites) == 0 {
		c.Suites = []SuiteConfig{{Name: "default"}}
	}
}

// Instances expands all (suite × platform) combinations and merges their
// configurations. If filter is non-empty, only instances whose name contains
// filter (case-insensitive) are returned.
func (c *Config) Instances(filter string) []InstanceConfig {
	var out []InstanceConfig
	for _, suite := range c.Suites {
		for _, platform := range c.Platforms {
			inst := c.mergeInstance(suite, platform)
			if filter != "" && !strings.Contains(strings.ToLower(inst.Name), strings.ToLower(filter)) {
				continue
			}
			out = append(out, inst)
		}
	}
	return out
}

// mergeInstance produces a fully-resolved InstanceConfig by layering global
// defaults → platform overrides → suite overrides, mirroring Test Kitchen's
// attribute-merging semantics.
func (c *Config) mergeInstance(suite SuiteConfig, platform PlatformConfig) InstanceConfig {
	inst := InstanceConfig{
		Name:      suite.Name + "-" + platform.Name,
		Suite:     suite,
		Platform:  platform,
		Transport: c.Transport,
	}

	// Driver: global → platform override.
	inst.Driver = mergeDriver(c.Driver, platform.Driver)

	// Provisioner: global → suite override → platform override.
	inst.Provisioner = mergeProvisioner(c.Provisioner, suite.Provisioner)
	inst.Provisioner = mergeProvisioner(inst.Provisioner, platform.Provisioner)

	// Verifier: global → suite override → platform override.
	inst.Verifier = mergeVerifier(c.Verifier, suite.Verifier)
	inst.Verifier = mergeVerifier(inst.Verifier, platform.Verifier)

	return inst
}

func mergeDriver(base, override DriverConfig) DriverConfig {
	if override.Name != "" {
		base.Name = override.Name
	}
	if override.Image != "" {
		base.Image = override.Image
	}
	if override.Box != "" {
		base.Box = override.Box
	}
	if override.RunCommand != "" {
		base.RunCommand = override.RunCommand
	}
	if override.Memory != "" {
		base.Memory = override.Memory
	}
	if override.CPUs != 0 {
		base.CPUs = override.CPUs
	}
	if len(override.Volumes) > 0 {
		base.Volumes = override.Volumes
	}
	if override.Privileged {
		base.Privileged = true
	}
	if override.Network != "" {
		base.Network = override.Network
	}
	for k, v := range override.EnvVars {
		if base.EnvVars == nil {
			base.EnvVars = map[string]string{}
		}
		base.EnvVars[k] = v
	}
	return base
}

func mergeProvisioner(base, override ProvisionerConfig) ProvisionerConfig {
	if override.Project != "" {
		base.Project = override.Project
	}
	if override.GomnibusBinary != "" {
		base.GomnibusBinary = override.GomnibusBinary
	}
	if len(override.InstallPackages) > 0 {
		base.InstallPackages = override.InstallPackages
	}
	if len(override.BuildArgs) > 0 {
		base.BuildArgs = override.BuildArgs
	}
	if override.WorkDir != "" {
		base.WorkDir = override.WorkDir
	}
	return base
}

func mergeVerifier(base, override VerifierConfig) VerifierConfig {
	if override.Name != "" {
		base.Name = override.Name
	}
	if override.Command != "" {
		base.Command = override.Command
	}
	if len(override.Scripts) > 0 {
		base.Scripts = override.Scripts
	}
	return base
}

// ContainerName returns a Docker-safe container name for an instance.
func ContainerName(instanceName string) string {
	return "gomnibus-" + strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return '-'
	}, instanceName)
}
