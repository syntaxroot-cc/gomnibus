// Package pkg produces macOS .pkg installer packages via pkgbuild + productbuild.
//
// Required tools (bundled with Xcode Command Line Tools):
//
//	xcode-select --install
package pkg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/syntaxroot-cc/gomnibus/internal/packager"
	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

func init() {
	packager.Register(&PkgPackager{})
}

// PkgPackager builds macOS flat packages (.pkg) using the standard Apple toolchain.
// It runs pkgbuild to assemble a component package, then productbuild to produce the
// final distributable installer — the same approach used by most macOS installers.
type PkgPackager struct{}

func (p *PkgPackager) Name() string { return "pkg" }

func (p *PkgPackager) Pack(ctx context.Context, proj *project.Definition, installDir, outputDir string) ([]string, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("pkg packager requires macOS (darwin); current OS: %s", runtime.GOOS)
	}
	if err := requireTool("pkgbuild"); err != nil {
		return nil, err
	}
	if err := requireTool("productbuild"); err != nil {
		return nil, err
	}

	stagingDir, err := os.MkdirTemp("", "gomnibus-pkg-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stagingDir)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}

	identifier := bundleID(proj)
	version := pkgVersion(proj)
	componentPkg := filepath.Join(stagingDir, proj.Name+"-component.pkg")
	distributionXML := filepath.Join(stagingDir, "Distribution.xml")

	// Step 1: pkgbuild — assemble the component package from the install root.
	pkgbuild := exec.CommandContext(ctx, "pkgbuild",
		"--root", installDir,
		"--identifier", identifier,
		"--version", version,
		"--install-location", proj.InstallDir,
		componentPkg,
	)
	pkgbuild.Stdout = os.Stdout
	pkgbuild.Stderr = os.Stderr
	if err := pkgbuild.Run(); err != nil {
		return nil, fmt.Errorf("pkgbuild: %w", err)
	}

	// Step 2: generate a Distribution.xml for productbuild.
	if err := writeDistribution(distributionXML, proj, identifier, version, componentPkg); err != nil {
		return nil, err
	}

	// Step 3: productbuild — produce the final distributable .pkg.
	outName := fmt.Sprintf("%s-%s-%d.pkg", proj.Name, proj.BuildVersion, proj.BuildIteration)
	outPath := filepath.Join(outputDir, outName)

	productbuild := exec.CommandContext(ctx, "productbuild",
		"--distribution", distributionXML,
		"--package-path", stagingDir,
		outPath,
	)
	productbuild.Stdout = os.Stdout
	productbuild.Stderr = os.Stderr
	if err := productbuild.Run(); err != nil {
		return nil, fmt.Errorf("productbuild: %w", err)
	}

	return []string{outPath}, nil
}

// writeDistribution writes a Distribution.xml that controls the macOS installer UI.
func writeDistribution(path string, proj *project.Definition, identifier, version, componentPkg string) error {
	data := distributionData{
		Title:        friendlyName(proj),
		Identifier:   identifier,
		Version:      version,
		InstallDir:   proj.InstallDir,
		ComponentPkg: filepath.Base(componentPkg),
		Homepage:     proj.Homepage,
		License:      proj.LicenseFile,
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return distributionTemplate.Execute(f, data)
}

type distributionData struct {
	Title        string
	Identifier   string
	Version      string
	InstallDir   string
	ComponentPkg string
	Homepage     string
	License      string
}

var distributionTemplate = template.Must(template.New("dist").Parse(`<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="2">
    <title>{{.Title}}</title>
    <organization>{{.Identifier}}</organization>
    <domains enable_localSystem="true"/>
    <options customize="never" require-scripts="false" rootVolumeOnly="true"/>
    <volume-check>
        <allowed-os-versions>
            <os-version min="10.13"/>
        </allowed-os-versions>
    </volume-check>
    <choices-outline>
        <line choice="default">
            <line choice="{{.Identifier}}"/>
        </line>
    </choices-outline>
    <choice id="default"/>
    <choice id="{{.Identifier}}" visible="false">
        <pkg-ref id="{{.Identifier}}"/>
    </choice>
    <pkg-ref id="{{.Identifier}}" version="{{.Version}}" onConclusion="none">{{.ComponentPkg}}</pkg-ref>
</installer-gui-script>
`))

// bundleID derives a reverse-DNS bundle identifier from the project.
// Uses the maintainer email domain if parseable, otherwise falls back to
// a sanitised form of the project name.
func bundleID(proj *project.Definition) string {
	domain := "com.gomnibus"
	if proj.Maintainer != "" {
		// Extract domain from "Name <email@domain.com>" or "email@domain.com"
		s := proj.Maintainer
		if lt := strings.Index(s, "<"); lt >= 0 {
			s = s[lt+1:]
		}
		if gt := strings.Index(s, ">"); gt >= 0 {
			s = s[:gt]
		}
		if at := strings.Index(s, "@"); at >= 0 {
			d := s[at+1:]
			parts := strings.Split(d, ".")
			// Reverse: com.example → example.com — we want com.example.<name>
			if len(parts) >= 2 {
				domain = d
			}
		}
	}
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, proj.Name)
	return domain + "." + safe
}

func pkgVersion(proj *project.Definition) string {
	if proj.BuildIteration > 0 {
		return fmt.Sprintf("%s.%d", proj.BuildVersion, proj.BuildIteration)
	}
	return proj.BuildVersion
}

func friendlyName(proj *project.Definition) string {
	if proj.FriendlyName != "" {
		return proj.FriendlyName
	}
	return proj.Name
}

func requireTool(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("required tool %q not found — install Xcode Command Line Tools: xcode-select --install", name)
	}
	return nil
}
