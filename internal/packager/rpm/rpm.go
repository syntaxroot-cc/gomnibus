// Package rpm produces RPM packages via rpmbuild.
package rpm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/syntaxroot-cc/gomnibus/internal/packager"
	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

func init() {
	packager.Register(&RPMPackager{})
}

type RPMPackager struct{}

func (r *RPMPackager) Name() string { return "rpm" }

func (r *RPMPackager) Pack(ctx context.Context, proj *project.Definition, installDir, outputDir string) ([]string, error) {
	rpmBase, err := os.MkdirTemp("", "gomnibus-rpm-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(rpmBase)

	for _, d := range []string{"BUILD", "RPMS", "SOURCES", "SPECS", "SRPMS"} {
		if err := os.MkdirAll(filepath.Join(rpmBase, d), 0o755); err != nil {
			return nil, err
		}
	}

	// Write spec file.
	specPath := filepath.Join(rpmBase, "SPECS", proj.Name+".spec")
	specF, err := os.Create(specPath)
	if err != nil {
		return nil, err
	}
	if err := specTemplate.Execute(specF, specData{
		Project:       proj,
		InstallDir:    installDir,
		ChangelogDate: time.Now().Format("Mon Jan 02 2006"),
	}); err != nil {
		specF.Close()
		return nil, err
	}
	specF.Close()

	cmd := exec.CommandContext(ctx, "rpmbuild",
		"--define", "_topdir "+rpmBase,
		"--define", "_builddir "+installDir,
		"-bb", specPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rpmbuild: %w", err)
	}

	// Find produced RPMs.
	var artifacts []string
	_ = filepath.Walk(filepath.Join(rpmBase, "RPMS"), func(p string, _ os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(p, ".rpm") {
			dest := filepath.Join(outputDir, filepath.Base(p))
			if err := os.MkdirAll(outputDir, 0o755); err == nil {
				_ = exec.Command("cp", p, dest).Run()
				artifacts = append(artifacts, dest)
			}
		}
		return nil
	})
	return artifacts, nil
}

type specData struct {
	Project       *project.Definition
	InstallDir    string
	ChangelogDate string
}

var specTemplate = template.Must(template.New("spec").Parse(`
Name:           {{.Project.Name}}
Version:        {{.Project.BuildVersion}}
Release:        {{.Project.BuildIteration}}
Summary:        {{if .Project.Description}}{{.Project.Description}}{{else}}{{.Project.Name}}{{end}}
License:        {{if .Project.License}}{{.Project.License}}{{else}}Proprietary{{end}}
{{if .Project.Homepage}}URL:            {{.Project.Homepage}}
{{end -}}
BuildArch:      x86_64

%description
{{.Project.Description}}

%install
cp -a {{.InstallDir}}/. %{buildroot}/

%files
{{.Project.InstallDir}}

%changelog
* {{.ChangelogDate}} gomnibus <noreply@example.com>
- Packaged by gomnibus
`))
