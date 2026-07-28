package changelog

import (
	"testing"
)

func TestParseSubject(t *testing.T) {
	cases := []struct {
		subject      string
		wantType     string
		wantScope    string
		wantDesc     string
		wantBreaking bool
	}{
		{"feat: add S3 cache", "feat", "", "add S3 cache", false},
		{"fix(auth): resolve login bug", "fix", "auth", "resolve login bug", false},
		{"feat!: remove deprecated API", "feat", "", "remove deprecated API", true},
		{"fix(ui)!: breaking form reset", "fix", "ui", "breaking form reset", true},
		{"chore(deps): bump go-git to v5.13", "chore", "deps", "bump go-git to v5.13", false},
		{"perf: cache manifest lookups", "perf", "", "cache manifest lookups", false},
		{"Merge pull request #42", "", "", "Merge pull request #42", false},
		{"not a conventional commit", "", "", "not a conventional commit", false},
		{"FEAT: uppercase type", "feat", "", "uppercase type", false},
	}

	for _, c := range cases {
		t.Run(c.subject, func(t *testing.T) {
			typ, scope, desc, breaking := ParseSubject(c.subject)
			if typ != c.wantType {
				t.Errorf("type: got %q, want %q", typ, c.wantType)
			}
			if scope != c.wantScope {
				t.Errorf("scope: got %q, want %q", scope, c.wantScope)
			}
			if desc != c.wantDesc {
				t.Errorf("desc: got %q, want %q", desc, c.wantDesc)
			}
			if breaking != c.wantBreaking {
				t.Errorf("breaking: got %v, want %v", breaking, c.wantBreaking)
			}
		})
	}
}
