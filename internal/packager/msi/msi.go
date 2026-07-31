// Package msi produces Windows .msi installer packages via WiX Toolset.
//
// Supports WiX v4 (wix build) and WiX v3 (candle + light).
// Install WiX v4:  dotnet tool install --global wix
// Install WiX v3:  https://github.com/wixtoolset/wix3/releases
//
// Cross-compilation from Linux/macOS requires Wine + WiX v3, or a Windows CI runner.
package msi

import (
	"context"
	"crypto/sha1" //nolint:gosec
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/syntaxroot-cc/gomnibus/internal/packager"
	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

func init() {
	packager.Register(&MSIPackager{})
}

// MSIPackager builds Windows .msi installers using WiX Toolset.
// It detects WiX v4 (wix) or WiX v3 (candle/light) automatically.
type MSIPackager struct{}

func (m *MSIPackager) Name() string { return "msi" }

func (m *MSIPackager) Pack(ctx context.Context, proj *project.Definition, installDir, outputDir string) ([]string, error) {
	tool, err := detectWiX()
	if err != nil {
		return nil, err
	}

	stagingDir, err := os.MkdirTemp("", "gomnibus-msi-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}

	// Enumerate install root for WiX file/component authoring.
	files, err := enumerateFiles(installDir)
	if err != nil {
		return nil, fmt.Errorf("enumerating install root: %w", err)
	}

	wxsPath := filepath.Join(stagingDir, proj.Name+".wxs")
	if err := writeWXS(wxsPath, proj, installDir, files); err != nil {
		return nil, fmt.Errorf("writing WXS: %w", err)
	}

	outName := fmt.Sprintf("%s-%s-%d.msi", proj.Name, proj.BuildVersion, proj.BuildIteration)
	outPath := filepath.Join(outputDir, outName)

	switch tool {
	case "wix4":
		if err := buildWiX4(ctx, wxsPath, outPath); err != nil {
			return nil, err
		}
	case "wix3":
		if err := buildWiX3(ctx, stagingDir, wxsPath, outPath); err != nil {
			return nil, err
		}
	}

	return []string{outPath}, nil
}

// ── WiX tool detection ───────────────────────────────────────────────────────

func detectWiX() (string, error) {
	if _, err := exec.LookPath("wix"); err == nil {
		return "wix4", nil
	}
	if _, err := exec.LookPath("candle"); err == nil {
		return "wix3", nil
	}
	return "", fmt.Errorf(
		"WiX Toolset not found — install WiX v4 with:\n" +
			"  dotnet tool install --global wix\n" +
			"or WiX v3 from https://github.com/wixtoolset/wix3/releases",
	)
}

// ── WiX v4 build ────────────────────────────────────────────────────────────

func buildWiX4(ctx context.Context, wxsPath, outPath string) error {
	cmd := exec.CommandContext(ctx, "wix", "build", "-o", outPath, wxsPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wix build: %w", err)
	}
	return nil
}

// ── WiX v3 build ────────────────────────────────────────────────────────────

func buildWiX3(ctx context.Context, stagingDir, wxsPath, outPath string) error {
	wixobjPath := filepath.Join(stagingDir, filepath.Base(wxsPath[:len(wxsPath)-4])+".wixobj")

	candle := exec.CommandContext(ctx, "candle", "-nologo", "-o", wixobjPath, wxsPath)
	candle.Stdout = os.Stdout
	candle.Stderr = os.Stderr
	if err := candle.Run(); err != nil {
		return fmt.Errorf("candle: %w", err)
	}

	light := exec.CommandContext(ctx, "light", "-nologo", "-ext", "WixUIExtension", "-o", outPath, wixobjPath)
	light.Stdout = os.Stdout
	light.Stderr = os.Stderr
	if err := light.Run(); err != nil {
		return fmt.Errorf("light: %w", err)
	}
	return nil
}

// ── WXS generation ──────────────────────────────────────────────────────────

// fileEntry describes a single file in the install root.
type fileEntry struct {
	ID       string // WiX-safe identifier
	GUID     string // deterministic component GUID
	Source   string // absolute path on disk
	RelDir   string // directory path relative to install root
	FileName string
	DirID    string // WiX Directory Id for this file's containing dir
}

// dirEntry tracks a directory node in the WXS tree.
type dirEntry struct {
	ID       string
	Name     string
	Parent   string
	Children []string
}

func enumerateFiles(installDir string) ([]fileEntry, error) {
	var files []fileEntry
	err := filepath.WalkDir(installDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(installDir, path)
		rel = filepath.ToSlash(rel)
		dir := filepath.ToSlash(filepath.Dir(rel))

		files = append(files, fileEntry{
			ID:       wixID("F_" + rel),
			GUID:     deterministicGUID(rel),
			Source:   path,
			RelDir:   dir,
			FileName: d.Name(),
			DirID:    wixID("D_" + dir),
		})
		return nil
	})
	return files, err
}

// wixID converts an arbitrary string into a WiX-safe identifier (alphanumeric + _).
func wixID(s string) string {
	var sb strings.Builder
	for _, r := range strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, s) {
		sb.WriteRune(r)
	}
	id := sb.String()
	// WiX identifiers must not start with a digit.
	if len(id) > 0 && id[0] >= '0' && id[0] <= '9' {
		id = "_" + id
	}
	return id
}

// deterministicGUID derives a GUID from a file's relative path so that
// re-runs of the same build produce identical MSI component GUIDs, which
// is required for correct Windows Installer upgrade behaviour.
func deterministicGUID(rel string) string {
	h := sha1.New() //nolint:gosec
	_, _ = fmt.Fprint(h, "gomnibus:"+rel)
	b := h.Sum(nil)
	// Format as uppercase GUID: {XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}
	return fmt.Sprintf("{%X-%X-%X-%X-%X}",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ── WXS template ────────────────────────────────────────────────────────────

type wxsData struct {
	ProductName  string
	Manufacturer string
	Version      string
	UpgradeCode  string
	ProductID    string
	InstallDir   string
	InstallDirID string
	Description  string
	Homepage     string
	Files        []fileEntry
	Dirs         []dirEntry
}

func writeWXS(path string, proj *project.Definition, installDir string, files []fileEntry) error {
	// Collect unique directories and build tree.
	dirSet := make(map[string]dirEntry)
	for _, f := range files {
		d := f.RelDir
		if d == "." || d == "" {
			d = "."
		}
		if _, ok := dirSet[d]; !ok {
			parts := strings.Split(d, "/")
			name := parts[len(parts)-1]
			parent := strings.Join(parts[:len(parts)-1], "/")
			if parent == "" {
				parent = "INSTALLDIR"
			}
			dirSet[d] = dirEntry{
				ID:     wixID("D_" + d),
				Name:   name,
				Parent: wixID("D_" + parent),
			}
		}
	}
	var dirs []dirEntry
	for _, d := range dirSet {
		dirs = append(dirs, d)
	}

	data := wxsData{
		ProductName:  friendlyName(proj),
		Manufacturer: maintainerName(proj.Maintainer),
		Version:      proj.BuildVersion,
		UpgradeCode:  deterministicGUID("upgrade:" + proj.Name),
		ProductID:    deterministicGUID("product:" + proj.Name + ":" + proj.BuildVersion),
		InstallDir:   windowsPath(proj.InstallDir),
		InstallDirID: "INSTALLDIR",
		Description:  proj.Description,
		Homepage:     proj.Homepage,
		Files:        files,
		Dirs:         dirs,
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return wxsTemplate.Execute(f, data)
}

var wxsTemplate = template.Must(template.New("wxs").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="{{.ProductID}}"
           Name="{{.ProductName}}"
           Language="1033"
           Version="{{.Version}}"
           Manufacturer="{{.Manufacturer}}"
           UpgradeCode="{{.UpgradeCode}}">

    <Package InstallerVersion="500"
             Compressed="yes"
             InstallScope="perMachine"
             Description="{{.Description}}"
             Comments="{{.Homepage}}"/>

    <MajorUpgrade DowngradeErrorMessage="A newer version of [ProductName] is already installed."/>
    <MediaTemplate EmbedCab="yes"/>

    <Feature Id="ProductFeature" Title="{{.ProductName}}" Level="1">
{{- range .Files}}
      <ComponentRef Id="{{.ID}}_Comp"/>
{{- end}}
    </Feature>

    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFilesFolder">
        <Directory Id="{{.InstallDirID}}" Name="{{.InstallDir}}">
{{- range .Dirs}}
          <Directory Id="{{.ID}}" Name="{{.Name}}"/>
{{- end}}
        </Directory>
      </Directory>
    </Directory>

{{range .Files}}
    <Component Id="{{.ID}}_Comp" Guid="{{.GUID}}" Directory="{{.DirID}}">
      <File Id="{{.ID}}" Source="{{.Source}}" Name="{{.FileName}}" KeyPath="yes"/>
    </Component>
{{end}}

  </Product>
</Wix>
`))

func windowsPath(p string) string {
	// Strip leading slashes; Windows install paths are rooted at drive letter.
	p = strings.TrimLeft(p, "/")
	return strings.ReplaceAll(p, "/", `\`)
}

func friendlyName(proj *project.Definition) string {
	if proj.FriendlyName != "" {
		return proj.FriendlyName
	}
	return proj.Name
}

func maintainerName(maintainer string) string {
	if lt := strings.Index(maintainer, "<"); lt > 0 {
		return strings.TrimSpace(maintainer[:lt])
	}
	return maintainer
}
