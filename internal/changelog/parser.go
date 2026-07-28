// Package changelog parses git history into conventional-commit sections.
package changelog

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Commit holds one parsed git commit.
type Commit struct {
	Hash     string
	Short    string
	Type     string
	Scope    string
	Subject  string
	Breaking bool
	Author   string
	When     time.Time
}

// Section groups commits that belong to a single release.
// Tag is empty for the unreleased section.
type Section struct {
	Tag     string
	Date    time.Time
	Commits []Commit
}

var conventionalRE = regexp.MustCompile(`^(\w[\w-]*)(?:\(([^)]*)\))?(!)?: (.+)`)

var errStop = errors.New("stop iteration")

// Parse opens the git repository at or above repoPath and returns Sections
// grouped by release tag, newest first. If since is non-empty, iteration stops
// when that tag is reached (the since-tagged section is excluded from output).
func Parse(repoPath, since string) ([]Section, error) {
	repo, err := gogit.PlainOpenWithOptions(repoPath, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open git repo: %w", err)
	}

	tagMap, err := buildTagMap(repo)
	if err != nil {
		return nil, err
	}

	logIter, err := repo.Log(&gogit.LogOptions{
		Order: gogit.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	defer logIter.Close()

	var sections []Section
	current := Section{}

	iterErr := logIter.ForEach(func(c *object.Commit) error {
		tag, isTag := tagMap[c.Hash]
		if isTag {
			if since != "" && tag == since {
				// Save what we have and stop — the since-tagged commits are excluded.
				if len(current.Commits) > 0 || current.Tag != "" {
					sections = append(sections, current)
				}
				return errStop
			}
			// Tag boundary: save the current section and start a new one.
			if len(current.Commits) > 0 || current.Tag != "" {
				sections = append(sections, current)
			}
			current = Section{Tag: tag, Date: c.Author.When}
		}
		current.Commits = append(current.Commits, parseCommit(c))
		return nil
	})
	if iterErr != nil && !errors.Is(iterErr, errStop) {
		return nil, fmt.Errorf("walking commits: %w", iterErr)
	}

	// Flush the last (oldest) section unless we stopped at the since boundary.
	if !errors.Is(iterErr, errStop) && len(current.Commits) > 0 {
		sections = append(sections, current)
	}

	return sections, nil
}

func buildTagMap(repo *gogit.Repository) (map[plumbing.Hash]string, error) {
	tagIter, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer tagIter.Close()

	m := make(map[plumbing.Hash]string)
	if err := tagIter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		// Annotated tags point to a tag object; lightweight tags point directly to the commit.
		if tagObj, err := repo.TagObject(ref.Hash()); err == nil {
			m[tagObj.Target] = name
		} else {
			m[ref.Hash()] = name
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("iterating tags: %w", err)
	}
	return m, nil
}

func parseCommit(c *object.Commit) Commit {
	msg := strings.TrimSpace(c.Message)
	subject, body, _ := strings.Cut(msg, "\n")
	subject = strings.TrimSpace(subject)

	entry := Commit{
		Hash:    c.Hash.String(),
		Short:   c.Hash.String()[:7],
		Subject: subject,
		Author:  c.Author.Name,
		When:    c.Author.When,
	}

	if m := conventionalRE.FindStringSubmatch(subject); m != nil {
		entry.Type = strings.ToLower(m[1])
		entry.Scope = m[2]
		entry.Breaking = m[3] == "!"
		entry.Subject = strings.TrimSpace(m[4])
	}

	// Body may contain a footer indicating a breaking change.
	if strings.Contains(body, "BREAKING CHANGE:") {
		entry.Breaking = true
	}

	return entry
}

// ParseSubject parses a conventional commit subject line and returns its
// components. Exported so tests can exercise the regex without a real repo.
func ParseSubject(subject string) (typ, scope, desc string, breaking bool) {
	m := conventionalRE.FindStringSubmatch(strings.TrimSpace(subject))
	if m == nil {
		return "", "", subject, false
	}
	return strings.ToLower(m[1]), m[2], strings.TrimSpace(m[4]), m[3] == "!"
}
