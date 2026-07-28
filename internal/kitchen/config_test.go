package kitchen

import (
	"os"
	"path/filepath"
	"testing"
)

var sampleYAML = `
driver:
  name: docker

provisioner:
  name: gomnibus
  project: myapp
  install_packages:
    - build-essential

verifier:
  name: shell
  command: "echo ok"

platforms:
  - name: ubuntu-22.04
    driver:
      image: ubuntu:22.04
  - name: rockylinux-9
    driver:
      image: rockylinux:9
    provisioner:
      install_packages:
        - gcc
        - make
    verifier:
      command: "rpm -q myapp"

suites:
  - name: default
  - name: minimal
    provisioner:
      install_packages: []
`

func writeTmpKitchen(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".kitchen.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfig_Defaults(t *testing.T) {
	path := writeTmpKitchen(t, `
platforms:
  - name: ubuntu-22.04
    driver:
      image: ubuntu:22.04
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Driver.Name != "docker" {
		t.Errorf("default driver: got %q, want docker", cfg.Driver.Name)
	}
	if cfg.Provisioner.WorkDir != "/workspace" {
		t.Errorf("default work_dir: got %q, want /workspace", cfg.Provisioner.WorkDir)
	}
	if len(cfg.Suites) != 1 || cfg.Suites[0].Name != "default" {
		t.Errorf("expected default suite to be injected, got %v", cfg.Suites)
	}
}

func TestInstances_Count(t *testing.T) {
	path := writeTmpKitchen(t, sampleYAML)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	instances := cfg.Instances("")
	// 2 platforms × 2 suites = 4 instances
	if len(instances) != 4 {
		t.Errorf("expected 4 instances, got %d", len(instances))
	}
}

func TestInstances_Filter(t *testing.T) {
	path := writeTmpKitchen(t, sampleYAML)
	cfg, _ := LoadConfig(path)

	got := cfg.Instances("ubuntu")
	if len(got) != 2 {
		t.Errorf("filter 'ubuntu': expected 2 instances, got %d", len(got))
	}
	for _, inst := range got {
		if inst.Platform.Name != "ubuntu-22.04" {
			t.Errorf("unexpected platform %q in ubuntu filter", inst.Platform.Name)
		}
	}
}

func TestMerge_VerifierOverride(t *testing.T) {
	path := writeTmpKitchen(t, sampleYAML)
	cfg, _ := LoadConfig(path)
	instances := cfg.Instances("")

	// rockylinux instances should have the platform-level verifier command.
	for _, inst := range instances {
		if inst.Platform.Name == "rockylinux-9" {
			if inst.Verifier.Command != "rpm -q myapp" {
				t.Errorf("rockylinux verifier command: got %q, want 'rpm -q myapp'", inst.Verifier.Command)
			}
		}
	}
}

func TestMerge_ProvisionerPackageOverride(t *testing.T) {
	path := writeTmpKitchen(t, sampleYAML)
	cfg, _ := LoadConfig(path)
	instances := cfg.Instances("rockylinux")

	// rockylinux platform overrides install_packages.
	for _, inst := range instances {
		pkgs := inst.Provisioner.InstallPackages
		if len(pkgs) != 2 || pkgs[0] != "gcc" {
			t.Errorf("rockylinux packages: got %v, want [gcc make]", pkgs)
		}
	}
}

func TestContainerName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"default-ubuntu-22.04", "gomnibus-default-ubuntu-22-04"},
		{"minimal-rockylinux-9", "gomnibus-minimal-rockylinux-9"},
		{"suite_name-platform", "gomnibus-suite-name-platform"},
	}
	for _, c := range cases {
		got := ContainerName(c.in)
		if got != c.want {
			t.Errorf("ContainerName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildInstallCmd(t *testing.T) {
	cases := []struct {
		platform string
		contains string
	}{
		{"ubuntu-22.04", "apt-get"},
		{"debian-12", "apt-get"},
		{"rockylinux-9", "dnf"},
		{"centos-stream-9", "dnf"},
		{"amazonlinux-2023", "yum"},
		{"fedora-39", "dnf"},
	}
	pkgs := []string{"curl", "git"}
	for _, c := range cases {
		cmd := buildInstallCmd(c.platform, pkgs)
		if cmd == "" {
			t.Errorf("buildInstallCmd(%q): empty result", c.platform)
		}
		found := false
		for _, word := range []string{c.contains} {
			if contains(cmd, word) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildInstallCmd(%q): expected %q in %q", c.platform, c.contains, cmd)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
