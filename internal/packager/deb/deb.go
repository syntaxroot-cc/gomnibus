// Package deb produces Debian .deb packages via dpkg-deb.
package deb

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/syntaxroot-cc/gomnibus/internal/packager"
	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

func init() {
	packager.Register(&DebPackager{})
}

type DebPackager struct{}

func (d *DebPackager) Name() string { return "deb" }

func (d *DebPackager) Pack(ctx context.Context, proj *project.Definition, installDir, outputDir string) ([]string, error) {
	stagingDir, err := os.MkdirTemp("", "gomnibus-deb-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	// Mirror install tree into staging.
	installStage := filepath.Join(stagingDir, filepath.Clean(proj.InstallDir))
	if err := os.MkdirAll(installStage, 0o755); err != nil {
		return nil, err
	}
	if err := exec.CommandContext(ctx, "cp", "-a", installDir+"/.", installStage).Run(); err != nil {
		return nil, fmt.Errorf("staging install dir: %w", err)
	}

	// Write DEBIAN/control.
	debianDir := filepath.Join(stagingDir, "DEBIAN")
	if err := os.MkdirAll(debianDir, 0o755); err != nil {
		return nil, err
	}
	arch := debArch()
	control := buildControl(proj, arch)
	if err := os.WriteFile(filepath.Join(debianDir, "control"), []byte(control), 0o644); err != nil {
		return nil, err
	}

	version := proj.BuildVersion
	if proj.BuildIteration > 0 {
		version = fmt.Sprintf("%s-%d", version, proj.BuildIteration)
	}
	pkgName := fmt.Sprintf("%s_%s_%s.deb", proj.Name, version, arch)
	pkgPath := filepath.Join(outputDir, pkgName)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "dpkg-deb", "--build", "--root-owner-group", stagingDir, pkgPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("dpkg-deb: %w", err)
	}
	return []string{pkgPath}, nil
}

func buildControl(proj *project.Definition, arch string) string {
	var sb strings.Builder
	sb.WriteString("Package: " + proj.Name + "\n")
	sb.WriteString("Version: " + proj.BuildVersion + "\n")
	if proj.BuildIteration > 0 {
		sb.WriteString("Revision: " + strconv.Itoa(proj.BuildIteration) + "\n")
	}
	sb.WriteString("Architecture: " + arch + "\n")
	sb.WriteString("Maintainer: " + proj.Maintainer + "\n")
	if proj.Description != "" {
		sb.WriteString("Description: " + proj.Description + "\n")
	}
	if proj.Homepage != "" {
		sb.WriteString("Homepage: " + proj.Homepage + "\n")
	}
	if len(proj.RuntimeDeps) > 0 {
		sb.WriteString("Depends: " + strings.Join(proj.RuntimeDeps, ", ") + "\n")
	}
	if len(proj.ConflictsWith) > 0 {
		sb.WriteString("Conflicts: " + strings.Join(proj.ConflictsWith, ", ") + "\n")
	}
	if len(proj.Replaces) > 0 {
		sb.WriteString("Replaces: " + strings.Join(proj.Replaces, ", ") + "\n")
	}
	return sb.String()
}

func debArch() string {
	switch os.Getenv("GOARCH") {
	case "arm64":
		return "arm64"
	case "386":
		return "i386"
	default:
		return "amd64"
	}
}
