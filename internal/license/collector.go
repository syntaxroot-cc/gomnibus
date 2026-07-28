// Package license collects license information from built software components.
package license

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/syntaxroot-cc/gomnibus/internal/pipeline"
)

// Entry records license information for one component.
type Entry struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	LicenseID string `json:"license_id,omitempty"`
	LicenseFile string `json:"license_file,omitempty"`
	Content   string `json:"content,omitempty"`
}

// Collect gathers license metadata from all nodes in the build graph.
func Collect(graph *pipeline.Graph, installDir string) []Entry {
	var entries []Entry
	for _, node := range graph.Order() {
		e := Entry{
			Name:      node.Def.Name,
			Version:   node.Def.ResolvedVersion,
			LicenseID: node.Def.License,
		}
		if node.Def.LicenseFile != "" {
			lf := filepath.Join(installDir, node.Def.LicenseFile)
			if data, err := os.ReadFile(lf); err == nil {
				e.LicenseFile = node.Def.LicenseFile
				e.Content = string(data)
			}
		}
		entries = append(entries, e)
	}
	return entries
}

// WriteJSON writes the collected license entries to a JSON file.
func WriteJSON(entries []Entry, path string) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Check validates that every component declares a license, returning an error
// listing any that do not.
func Check(entries []Entry) error {
	var missing []string
	for _, e := range entries {
		if e.LicenseID == "" {
			missing = append(missing, e.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("components without license declarations: %v", missing)
	}
	return nil
}
