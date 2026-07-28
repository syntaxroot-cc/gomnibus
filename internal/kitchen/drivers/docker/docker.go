// Package docker implements the gomnibus kitchen Docker driver.
//
// Requires the docker CLI to be installed and the Docker daemon to be running.
// No Go Docker SDK dependency — shelling out keeps this simple and compatible
// with Docker Desktop, Podman-compatible daemons, and remote sockets alike.
package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/syntaxroot-cc/gomnibus/internal/kitchen"
)

func init() {
	kitchen.Register("docker", &DockerDriver{})
}

// DockerDriver runs each kitchen instance as a long-lived Docker container.
type DockerDriver struct{}

// Create starts a detached container and stores its ID in state.
func (d *DockerDriver) Create(ctx context.Context, inst kitchen.InstanceConfig, state *kitchen.State) error {
	if inst.Driver.Image == "" {
		return fmt.Errorf("docker driver: platform %q has no image configured", inst.Platform.Name)
	}

	name := kitchen.ContainerName(inst.Name)

	// Remove any stale container with the same name.
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()

	args := []string{"run", "-d", "--name", name}

	// Resource limits.
	if inst.Driver.Memory != "" {
		args = append(args, "--memory", inst.Driver.Memory)
	}
	if inst.Driver.CPUs != 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", inst.Driver.CPUs))
	}

	// Privileged (needed for some build tools / systemd init).
	if inst.Driver.Privileged {
		args = append(args, "--privileged")
	}

	// Network.
	if inst.Driver.Network != "" {
		args = append(args, "--network", inst.Driver.Network)
	}

	// Extra volumes.
	for _, v := range inst.Driver.Volumes {
		args = append(args, "-v", v)
	}

	// Environment variables.
	for k, v := range inst.Driver.EnvVars {
		args = append(args, "-e", k+"="+v)
	}

	args = append(args, inst.Driver.Image)

	// Entrypoint: default keeps the container alive.
	if inst.Driver.RunCommand != "" {
		args = append(args, strings.Fields(inst.Driver.RunCommand)...)
	} else {
		args = append(args, "sleep", "infinity")
	}

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run: %w\n%s", err, out)
	}

	state.ContainerID = name
	state.Hostname = "localhost"
	return nil
}

// Exec runs cmd inside the container via `docker exec`.
func (d *DockerDriver) Exec(ctx context.Context, state *kitchen.State, cmd string) error {
	c := exec.CommandContext(ctx, "docker", "exec", state.ContainerID, "sh", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("exec in %s: %w", state.ContainerID, err)
	}
	return nil
}

// CopyTo copies localPath into the container at remotePath using `docker cp`.
func (d *DockerDriver) CopyTo(ctx context.Context, state *kitchen.State, localPath, remotePath string) error {
	// Ensure parent directory exists inside the container.
	parent := remotePath
	if idx := strings.LastIndex(remotePath, "/"); idx > 0 {
		parent = remotePath[:idx]
	}
	_ = exec.CommandContext(ctx, "docker", "exec", state.ContainerID, "mkdir", "-p", parent).Run()

	out, err := exec.CommandContext(ctx, "docker", "cp", localPath, state.ContainerID+":"+remotePath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp to %s: %w\n%s", state.ContainerID, err, out)
	}
	return nil
}

// CopyFrom copies remotePath out of the container to localPath.
func (d *DockerDriver) CopyFrom(ctx context.Context, state *kitchen.State, remotePath, localPath string) error {
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, "docker", "cp", state.ContainerID+":"+remotePath, localPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp from %s: %w\n%s", state.ContainerID, err, out)
	}
	return nil
}

// Login opens an interactive shell inside the container.
// This requires the caller's stdin to be a real TTY.
func (d *DockerDriver) Login(ctx context.Context, state *kitchen.State) error {
	shell := detectShell(ctx, state.ContainerID)
	c := exec.CommandContext(ctx, "docker", "exec", "-it", state.ContainerID, shell)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// Destroy stops and removes the container.
func (d *DockerDriver) Destroy(ctx context.Context, state *kitchen.State) error {
	out, err := exec.CommandContext(ctx, "docker", "rm", "-f", state.ContainerID).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "No such container") {
		return fmt.Errorf("docker rm %s: %w\n%s", state.ContainerID, err, out)
	}
	return nil
}

// detectShell probes the container for bash, falling back to sh.
func detectShell(ctx context.Context, containerID string) string {
	err := exec.CommandContext(ctx, "docker", "exec", containerID, "which", "bash").Run()
	if err == nil {
		return "/bin/bash"
	}
	return "/bin/sh"
}
