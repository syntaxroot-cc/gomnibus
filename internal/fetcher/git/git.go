package git

import (
	"context"
	"fmt"
	"os"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/syntaxroot-cc/gomnibus/internal/fetcher"
	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

func init() {
	fetcher.Register(&GitFetcher{})
}

type GitFetcher struct{}

func (g *GitFetcher) Name() string { return "git" }

func (g *GitFetcher) Fetch(ctx context.Context, src *software.Source, destDir string) error {
	if _, err := os.Stat(destDir); err == nil {
		// Already cloned — pull latest.
		repo, err := gogit.PlainOpen(destDir)
		if err != nil {
			return fmt.Errorf("git open %s: %w", destDir, err)
		}
		wt, err := repo.Worktree()
		if err != nil {
			return err
		}
		if err := wt.PullContext(ctx, &gogit.PullOptions{
			RemoteName: "origin",
			Force:      true,
		}); err != nil && err != gogit.NoErrAlreadyUpToDate {
			return fmt.Errorf("git pull: %w", err)
		}
		return g.checkout(repo, src)
	}

	cloneOpts := &gogit.CloneOptions{
		URL:   src.Git,
		Depth: 0,
	}
	if src.Branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(src.Branch)
		cloneOpts.SingleBranch = true
	}
	repo, err := gogit.PlainCloneContext(ctx, destDir, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("git clone %s: %w", src.Git, err)
	}
	return g.checkout(repo, src)
}

func (g *GitFetcher) checkout(repo *gogit.Repository, src *software.Source) error {
	if src.Commit == "" && src.Tag == "" {
		return nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	opts := &gogit.CheckoutOptions{Force: true}
	switch {
	case src.Commit != "":
		opts.Hash = plumbing.NewHash(src.Commit)
	case src.Tag != "":
		opts.Branch = plumbing.NewTagReferenceName(src.Tag)
	}
	return wt.Checkout(opts)
}
