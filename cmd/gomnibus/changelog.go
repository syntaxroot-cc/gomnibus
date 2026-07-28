package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/syntaxroot-cc/gomnibus/internal/changelog"
	"github.com/syntaxroot-cc/gomnibus/pkg/log"
)

var (
	changelogSince      string
	changelogOutput     string
	changelogUnreleased string
)

var changelogCmd = &cobra.Command{
	Use:   "changelog",
	Short: "Changelog management",
}

var changelogGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate CHANGELOG.md from git history using conventional commits",
	Long: `Generate a CHANGELOG.md from the git history of the current repository.

Commits are expected to follow the Conventional Commits specification:

    <type>[(scope)][!]: <description>

Supported types: feat, fix, perf, refactor, docs, test, style, ci, build, chore.
Commits marked with '!' or containing 'BREAKING CHANGE:' in the footer are placed
in a separate "Breaking Changes" section. Non-conventional commits appear under
"Other". Sections are ordered newest first, grouped by git tag.`,
	RunE: runChangelogGenerate,
}

func init() {
	changelogGenerateCmd.Flags().StringVar(&changelogSince, "since", "", "only include changes since this tag (exclusive)")
	changelogGenerateCmd.Flags().StringVarP(&changelogOutput, "output", "o", "CHANGELOG.md", "output file path (- for stdout)")
	changelogGenerateCmd.Flags().StringVar(&changelogUnreleased, "unreleased-title", "Unreleased", "heading for commits not yet tagged")
	changelogCmd.AddCommand(changelogGenerateCmd)
	rootCmd.AddCommand(changelogCmd)
}

func runChangelogGenerate(cmd *cobra.Command, args []string) error {
	logger := log.L()

	sections, err := changelog.Parse(".", changelogSince)
	if err != nil {
		return fmt.Errorf("parsing git history: %w", err)
	}
	if len(sections) == 0 {
		logger.Info("no commits found in repository")
		return nil
	}

	var w io.Writer
	if changelogOutput == "-" {
		w = os.Stdout
	} else {
		f, err := os.Create(changelogOutput)
		if err != nil {
			return fmt.Errorf("creating %s: %w", changelogOutput, err)
		}
		defer f.Close()
		w = f
	}

	if err := changelog.Render(w, sections, changelogUnreleased); err != nil {
		return err
	}

	if changelogOutput != "-" {
		logger.Info("changelog written", zap.String("path", changelogOutput))
	}
	return nil
}
