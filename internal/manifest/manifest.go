// Package manifest generates version manifests — JSON records of every
// software component and its resolved version used in a build.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/syntaxroot-cc/gomnibus/internal/pipeline"
	"github.com/syntaxroot-cc/gomnibus/internal/project"
)

// Entry represents one resolved software component.
type Entry struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Source   string `json:"source,omitempty"`
	LicenseID string `json:"license,omitempty"`
}

// Manifest is the full version manifest for a project build.
type Manifest struct {
	Project     string    `json:"project"`
	Version     string    `json:"version"`
	Iteration   int       `json:"iteration"`
	GeneratedAt time.Time `json:"generated_at"`
	Components  []Entry   `json:"components"`
}

// Generate builds a Manifest from a resolved dependency graph.
func Generate(proj *project.Definition, graph *pipeline.Graph) *Manifest {
	m := &Manifest{
		Project:     proj.Name,
		Version:     proj.BuildVersion,
		Iteration:   proj.BuildIteration,
		GeneratedAt: time.Now().UTC(),
	}
	for _, node := range graph.Order() {
		e := Entry{
			Name:    node.Def.Name,
			Version: node.Def.ResolvedVersion,
		}
		if node.Def.Source != nil {
			switch {
			case node.Def.Source.Git != "":
				e.Source = node.Def.Source.Git
			case node.Def.Source.URL != "":
				e.Source = node.Def.Source.URL
			}
		}
		e.LicenseID = node.Def.License
		m.Components = append(m.Components, e)
	}
	return m
}

// WriteJSON serialises the manifest to a JSON file at path.
func (m *Manifest) WriteJSON(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Print writes the manifest as formatted JSON to stdout.
func (m *Manifest) Print() error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
