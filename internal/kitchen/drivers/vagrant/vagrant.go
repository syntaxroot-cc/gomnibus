// Package vagrant implements the gomnibus kitchen Vagrant driver.
//
// Requires Vagrant (https://www.vagrantup.com) and a compatible provider
// (VirtualBox, VMware, Hyper-V, libvirt, etc.) to be installed on the host.
package vagrant

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/syntaxroot-cc/gomnibus/internal/kitchen"
)

func init() {
	kitchen.Register("vagrant", &VagrantDriver{})
}

// VagrantDriver manages kitchen instances as Vagrant-managed VMs.
type VagrantDriver struct{}

func (v *VagrantDriver) Create(ctx context.Context, inst kitchen.InstanceConfig, state *kitchen.State) error {
	if inst.Driver.Box == "" {
		return fmt.Errorf("vagrant driver: platform %q has no box configured (set driver.box)", inst.Platform.Name)
	}

	vmDir, err := os.MkdirTemp("", "gomnibus-vagrant-"+inst.Name+"-*")
	if err != nil {
		return err
	}

	vf, err := os.Create(filepath.Join(vmDir, "Vagrantfile"))
	if err != nil {
		return err
	}
	err = vagrantfileTemplate.Execute(vf, vagrantfileData{
		Box:       inst.Driver.Box,
		Memory:    inst.Driver.Memory,
		CPUs:      inst.Driver.CPUs,
		EnvVars:   inst.Driver.EnvVars,
		Volumes:   inst.Driver.Volumes,
	})
	vf.Close()
	if err != nil {
		return fmt.Errorf("rendering Vagrantfile: %w", err)
	}

	cmd := exec.CommandContext(ctx, "vagrant", "up")
	cmd.Dir = vmDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vagrant up: %w", err)
	}

	state.VMDir = vmDir
	return nil
}

func (v *VagrantDriver) Exec(ctx context.Context, state *kitchen.State, cmd string) error {
	c := exec.CommandContext(ctx, "vagrant", "ssh", "-c", cmd)
	c.Dir = state.VMDir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("vagrant ssh exec: %w", err)
	}
	return nil
}

func (v *VagrantDriver) CopyTo(ctx context.Context, state *kitchen.State, localPath, remotePath string) error {
	// Use scp via vagrant's SSH config.
	sshConfig, err := vagrantSSHConfig(ctx, state.VMDir)
	if err != nil {
		return err
	}
	args := append(sshConfig, localPath, "default:"+remotePath)
	out, err := exec.CommandContext(ctx, "scp", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp to vagrant VM: %w\n%s", err, out)
	}
	return nil
}

func (v *VagrantDriver) CopyFrom(ctx context.Context, state *kitchen.State, remotePath, localPath string) error {
	sshConfig, err := vagrantSSHConfig(ctx, state.VMDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		return err
	}
	args := append(sshConfig, "default:"+remotePath, localPath)
	out, err := exec.CommandContext(ctx, "scp", "-r", args[0], args[1], args[2]).CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp from vagrant VM: %w\n%s", err, out)
	}
	_ = out
	return nil
}

func (v *VagrantDriver) Login(ctx context.Context, state *kitchen.State) error {
	c := exec.CommandContext(ctx, "vagrant", "ssh")
	c.Dir = state.VMDir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func (v *VagrantDriver) Destroy(ctx context.Context, state *kitchen.State) error {
	if state.VMDir == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "vagrant", "destroy", "-f")
	cmd.Dir = state.VMDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vagrant destroy: %w", err)
	}
	return os.RemoveAll(state.VMDir)
}

// vagrantSSHConfig returns scp-compatible flags derived from `vagrant ssh-config`.
func vagrantSSHConfig(ctx context.Context, vmDir string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "vagrant", "ssh-config").Output()
	if err != nil {
		return nil, fmt.Errorf("vagrant ssh-config: %w", err)
	}
	// Parse HostName, Port, IdentityFile from ssh-config output.
	var host, port, key string
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "HostName":
			host = parts[1]
		case "Port":
			port = parts[1]
		case "IdentityFile":
			key = parts[1]
		}
	}
	_ = vmDir
	return []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-i", key,
		"-P", port,
		"vagrant@" + host,
	}, nil
}

type vagrantfileData struct {
	Box     string
	Memory  string
	CPUs    int
	EnvVars map[string]string
	Volumes []string
}

var vagrantfileTemplate = template.Must(template.New("vagrantfile").Parse(`
Vagrant.configure("2") do |config|
  config.vm.box = "{{.Box}}"

  config.vm.provider "virtualbox" do |vb|
    {{- if .Memory}}
    vb.memory = "{{.Memory}}"
    {{- end}}
    {{- if .CPUs}}
    vb.cpus = {{.CPUs}}
    {{- end}}
  end

  {{range .Volumes -}}
  config.vm.synced_folder *"{{.}}".split(":"), disabled: false
  {{end}}

  config.vm.provision "shell", inline: <<-SHELL
    {{range $k, $v := .EnvVars -}}
    export {{$k}}={{$v}}
    {{end}}
  SHELL
end
`))
