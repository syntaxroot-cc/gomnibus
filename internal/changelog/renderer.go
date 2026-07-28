package changelog

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// typeOrder controls the order of sections in the rendered output.
var typeOrder = []string{
	"feat", "fix", "perf", "refactor", "docs", "test", "style", "ci", "build", "chore",
}

var typeHeadings = map[string]string{
	"feat":     "Features",
	"fix":      "Bug Fixes",
	"perf":     "Performance",
	"refactor": "Refactoring",
	"docs":     "Documentation",
	"test":     "Tests",
	"style":    "Style",
	"ci":       "CI",
	"build":    "Build",
	"chore":    "Chores",
}

// Render writes a Markdown changelog for the given sections to w.
// unreleasedTitle is the heading for any section whose Tag is empty.
func Render(w io.Writer, sections []Section, unreleasedTitle string) error {
	if _, err := fmt.Fprintf(w, "# Changelog\n\n"); err != nil {
		return err
	}
	for _, s := range sections {
		if err := renderSection(w, s, unreleasedTitle); err != nil {
			return err
		}
	}
	return nil
}

func renderSection(w io.Writer, s Section, unreleasedTitle string) error {
	title := s.Tag
	if title == "" {
		title = unreleasedTitle
	}

	dateStr := ""
	if !s.Date.IsZero() {
		dateStr = " — " + s.Date.Format("2006-01-02")
	}

	if _, err := fmt.Fprintf(w, "## %s%s\n\n", title, dateStr); err != nil {
		return err
	}

	// Partition commits: breaking get their own section first.
	var breaking []Commit
	byType := make(map[string][]Commit)
	var other []Commit

	for _, c := range s.Commits {
		if c.Breaking {
			breaking = append(breaking, c)
			continue
		}
		if c.Type == "" {
			other = append(other, c)
		} else {
			byType[c.Type] = append(byType[c.Type], c)
		}
	}

	if len(breaking) > 0 {
		if _, err := fmt.Fprintf(w, "### ⚠ Breaking Changes\n\n"); err != nil {
			return err
		}
		for _, c := range breaking {
			if err := writeEntry(w, c); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	// Known types in prescribed order.
	knownTypes := make(map[string]bool)
	for _, t := range typeOrder {
		commits := byType[t]
		if len(commits) == 0 {
			continue
		}
		knownTypes[t] = true
		if _, err := fmt.Fprintf(w, "### %s\n\n", typeHeadings[t]); err != nil {
			return err
		}
		for _, c := range commits {
			if err := writeEntry(w, c); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	// Types not in typeOrder, sorted alphabetically.
	var unknownTypes []string
	for t := range byType {
		if !knownTypes[t] {
			unknownTypes = append(unknownTypes, t)
		}
	}
	sort.Strings(unknownTypes)
	for _, t := range unknownTypes {
		if _, err := fmt.Fprintf(w, "### %s\n\n", capitalize(t)); err != nil {
			return err
		}
		for _, c := range byType[t] {
			if err := writeEntry(w, c); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if len(other) > 0 {
		if _, err := fmt.Fprintf(w, "### Other\n\n"); err != nil {
			return err
		}
		for _, c := range other {
			if err := writeEntry(w, c); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	return nil
}

func writeEntry(w io.Writer, c Commit) error {
	scope := ""
	if c.Scope != "" {
		scope = "**" + c.Scope + "**: "
	}
	_, err := fmt.Fprintf(w, "- %s%s (`%s`)\n", scope, c.Subject, c.Short)
	return err
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
