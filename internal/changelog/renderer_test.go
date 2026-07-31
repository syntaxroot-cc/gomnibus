package changelog

import (
	"strings"
	"testing"
	"time"
)

func TestRender_Header(t *testing.T) {
	var buf strings.Builder
	if err := Render(&buf, nil, "Unreleased"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "# Changelog") {
		t.Error("expected changelog header")
	}
}

func TestRender_UnreleasedTitle(t *testing.T) {
	sections := []Section{{Tag: "", Commits: []Commit{{Short: "abc1234", Type: "fix", Subject: "patch"}}}}
	var buf strings.Builder
	if err := Render(&buf, sections, "Unreleased"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "## Unreleased") {
		t.Error("expected Unreleased heading")
	}
}

func TestRender_TagWithDate(t *testing.T) {
	sections := []Section{{
		Tag:     "v1.0.0",
		Date:    time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		Commits: []Commit{{Short: "abc1234", Type: "feat", Subject: "initial release"}},
	}}
	var buf strings.Builder
	if err := Render(&buf, sections, "Unreleased"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "## v1.0.0 — 2024-03-15") {
		t.Errorf("expected version heading with date, got:\n%s", out)
	}
}

func TestRender_TypeSections(t *testing.T) {
	sections := []Section{{
		Tag: "v1.1.0",
		Commits: []Commit{
			{Short: "aaa0001", Type: "feat", Subject: "new feature"},
			{Short: "bbb0002", Type: "fix", Subject: "bug fix"},
			{Short: "ccc0003", Type: "docs", Subject: "update README"},
			{Short: "ddd0004", Subject: "Merge branch main"},
		},
	}}
	var buf strings.Builder
	if err := Render(&buf, sections, "Unreleased"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{"### Features", "### Bug Fixes", "### Documentation", "### Other"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected section %q in output:\n%s", want, out)
		}
	}
}

func TestRender_ScopeFormatting(t *testing.T) {
	sections := []Section{{
		Commits: []Commit{{Short: "abc1234", Type: "feat", Scope: "cache", Subject: "add S3 backend"}},
	}}
	var buf strings.Builder
	if err := Render(&buf, sections, "Unreleased"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "**cache**: add S3 backend") {
		t.Error("expected bold scope prefix")
	}
}

func TestRender_BreakingChange(t *testing.T) {
	sections := []Section{{
		Tag: "v2.0.0",
		Commits: []Commit{
			{Short: "aaa0001", Type: "feat", Subject: "remove old API", Breaking: true},
			{Short: "bbb0002", Type: "feat", Subject: "new API"},
		},
	}}
	var buf strings.Builder
	if err := Render(&buf, sections, "Unreleased"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "⚠ Breaking Changes") {
		t.Error("expected breaking changes section")
	}
	if !strings.Contains(out, "remove old API") {
		t.Error("expected breaking commit description")
	}

	// The breaking commit must not also appear in Features.
	featIdx := strings.Index(out, "### Features")
	if featIdx == -1 {
		t.Fatal("expected Features section for non-breaking feat")
	}
	if strings.Contains(out[featIdx:], "remove old API") {
		t.Error("breaking commit must not appear in Features section")
	}
}

func TestRender_MultipleVersions(t *testing.T) {
	sections := []Section{
		{
			Tag:     "",
			Commits: []Commit{{Short: "ccc0003", Type: "feat", Subject: "upcoming feature"}},
		},
		{
			Tag:     "v1.1.0",
			Date:    time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			Commits: []Commit{{Short: "bbb0002", Type: "fix", Subject: "patch something"}},
		},
		{
			Tag:     "v1.0.0",
			Date:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Commits: []Commit{{Short: "aaa0001", Type: "feat", Subject: "initial release"}},
		},
	}
	var buf strings.Builder
	if err := Render(&buf, sections, "Unreleased"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	unrelIdx := strings.Index(out, "## Unreleased")
	v110Idx := strings.Index(out, "## v1.1.0")
	v100Idx := strings.Index(out, "## v1.0.0")

	if unrelIdx == -1 || v110Idx == -1 || v100Idx == -1 {
		t.Fatalf("missing expected sections in:\n%s", out)
	}
	if unrelIdx >= v110Idx || v110Idx >= v100Idx {
		t.Error("sections out of order: expected Unreleased > v1.1.0 > v1.0.0 (newest first)")
	}
}

func TestCapitalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"feat", "Feat"},
		{"ci", "Ci"},
		{"", ""},
		{"A", "A"},
	}
	for _, c := range cases {
		if got := capitalize(c.in); got != c.want {
			t.Errorf("capitalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
