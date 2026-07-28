package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/syntaxroot-cc/gomnibus/internal/builder"
	"github.com/syntaxroot-cc/gomnibus/internal/cache"
	"github.com/syntaxroot-cc/gomnibus/internal/config"
	"github.com/syntaxroot-cc/gomnibus/internal/fetcher"
	_ "github.com/syntaxroot-cc/gomnibus/internal/fetcher/git"
	_ "github.com/syntaxroot-cc/gomnibus/internal/fetcher/net"
	_ "github.com/syntaxroot-cc/gomnibus/internal/fetcher/path"
	"github.com/syntaxroot-cc/gomnibus/internal/health"
	"github.com/syntaxroot-cc/gomnibus/internal/license"
	"github.com/syntaxroot-cc/gomnibus/internal/manifest"
	"github.com/syntaxroot-cc/gomnibus/internal/packager"
	_ "github.com/syntaxroot-cc/gomnibus/internal/packager/deb"
	_ "github.com/syntaxroot-cc/gomnibus/internal/packager/msi"
	_ "github.com/syntaxroot-cc/gomnibus/internal/packager/pkg"
	_ "github.com/syntaxroot-cc/gomnibus/internal/packager/rpm"
	_ "github.com/syntaxroot-cc/gomnibus/internal/packager/tar"
	"github.com/syntaxroot-cc/gomnibus/internal/pipeline"
	"github.com/syntaxroot-cc/gomnibus/internal/project"
	"github.com/syntaxroot-cc/gomnibus/internal/software"
	"github.com/syntaxroot-cc/gomnibus/pkg/log"
)

var (
	cfgFile    string
	logLevel   string
	outputDir  string
	skipHealth bool
	packType   string
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gomnibus",
	Short: "Build full-stack, self-contained installers in Go",
	Long: `gomnibus builds cross-platform software packages (.deb, .rpm, .pkg, .msi, .tar.gz)
by fetching sources, building them in dependency order, and packaging the
install root — all driven by YAML project and software definitions.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return log.Init(logLevel)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: gomnibus.yaml)")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "log level: debug, info, warn, error")

	rootCmd.AddCommand(buildCmd, manifestCmd, newCmd, validateCmd)
}

// ── gomnibus build PROJECT ───────────────────────────────────────────────────

var buildCmd = &cobra.Command{
	Use:   "build PROJECT",
	Short: "Fetch, build, and package a project",
	Args:  cobra.ExactArgs(1),
	RunE:  runBuild,
}

func init() {
	buildCmd.Flags().StringVarP(&outputDir, "output", "o", "pkg", "directory for output packages")
	buildCmd.Flags().BoolVar(&skipHealth, "skip-healthcheck", false, "skip shared-library health check")
	buildCmd.Flags().StringVarP(&packType, "package-type", "p", "", "override package type (deb, rpm, pkg, msi, tar)")
}

func runBuild(cmd *cobra.Command, args []string) error {
	logger := log.L()
	ctx := cmd.Context()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	projName := args[0]
	logger.Info("building project", zap.String("project", projName))

	proj, err := project.LoadFromDir("config/projects", projName)
	if err != nil {
		return err
	}

	softwareDirs := append(cfg.SoftwareDirs, "config/software")
	reg, err := software.NewRegistry(softwareDirs)
	if err != nil {
		return err
	}

	graph, err := pipeline.Build(proj.Dependencies, reg, proj.OverrideMap())
	if err != nil {
		return err
	}

	var buildCache cache.Cache
	if cfg.UseGitCaching {
		buildCache = &cache.LocalCache{Dir: cfg.CacheDir, Log: logger}
	} else {
		buildCache = &cache.NopCache{}
	}

	for _, node := range graph.Order() {
		def := node.Def
		logger.Info("processing", zap.String("software", def.Name), zap.String("version", def.ResolvedVersion))

		srcDir := fmt.Sprintf("%s/src/%s-%s", cfg.BaseDir, def.Name, def.ResolvedVersion)
		installDir := proj.InstallDir

		cacheKey := ""
		if def.Source != nil {
			cacheKey, _ = cache.Key(def.Name, def.ResolvedVersion, "")
			if hit, _ := buildCache.Has(ctx, cacheKey); hit {
				logger.Info("cache hit", zap.String("key", cacheKey))
				if err := buildCache.Restore(ctx, cacheKey, installDir); err != nil {
					return err
				}
				continue
			}
		}

		if def.Source != nil {
			f, err := fetcher.For(def.Source)
			if err != nil {
				return err
			}
			logger.Info("fetching", zap.String("fetcher", f.Name()), zap.String("software", def.Name))
			if err := f.Fetch(ctx, def.Source, srcDir); err != nil {
				return fmt.Errorf("fetch %s: %w", def.Name, err)
			}
		}

		bc := &builder.Context{
			SrcDir:     srcDir,
			BuildDir:   fmt.Sprintf("%s/build/%s", cfg.BaseDir, def.Name),
			InstallDir: installDir,
			Log:        logger,
		}
		if err := builder.Execute(ctx, def, bc); err != nil {
			return err
		}

		if cacheKey != "" {
			_ = buildCache.Store(ctx, cacheKey, installDir)
		}
	}

	if !skipHealth {
		logger.Info("running health check")
		if err := health.Check(health.CheckOptions{
			InstallDir: proj.InstallDir,
			Log:        logger,
		}); err != nil {
			return err
		}
	}

	// Write license manifest.
	licEntries := license.Collect(graph, proj.InstallDir)
	if err := license.WriteJSON(licEntries, fmt.Sprintf("%s/LICENSE.json", proj.InstallDir)); err != nil {
		logger.Warn("could not write license manifest", zap.Error(err))
	}

	// Write version manifest.
	mfst := manifest.Generate(proj, graph)
	if err := mfst.WriteJSON(fmt.Sprintf("%s/version-manifest.json", proj.InstallDir)); err != nil {
		logger.Warn("could not write version manifest", zap.Error(err))
	}

	// Package.
	for _, pkgCfg := range proj.Packages {
		ptype := pkgCfg.Type
		if packType != "" {
			ptype = packType
		}
		p, err := packager.For(ptype)
		if err != nil {
			p, err = packager.DefaultForPlatform()
			if err != nil {
				return err
			}
		}
		logger.Info("packaging", zap.String("type", p.Name()))
		artifacts, err := p.Pack(ctx, proj, proj.InstallDir, outputDir)
		if err != nil {
			return err
		}
		for _, a := range artifacts {
			logger.Info("artifact", zap.String("path", a))
		}
	}
	if len(proj.Packages) == 0 {
		p, err := packager.DefaultForPlatform()
		if err != nil {
			return err
		}
		logger.Info("packaging", zap.String("type", p.Name()))
		artifacts, err := p.Pack(ctx, proj, proj.InstallDir, outputDir)
		if err != nil {
			return err
		}
		for _, a := range artifacts {
			logger.Info("artifact", zap.String("path", a))
		}
	}

	logger.Info("build complete", zap.String("project", projName))
	return nil
}

// ── gomnibus manifest PROJECT ────────────────────────────────────────────────

var manifestCmd = &cobra.Command{
	Use:   "manifest PROJECT",
	Short: "Print the resolved version manifest for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}
		proj, err := project.LoadFromDir("config/projects", args[0])
		if err != nil {
			return err
		}
		reg, err := software.NewRegistry(cfg.SoftwareDirs)
		if err != nil {
			return err
		}
		graph, err := pipeline.Build(proj.Dependencies, reg, proj.OverrideMap())
		if err != nil {
			return err
		}
		return manifest.Generate(proj, graph).Print()
	},
}

// ── gomnibus new PROJECT ─────────────────────────────────────────────────────

var newCmd = &cobra.Command{
	Use:   "new PROJECT",
	Short: "Generate a new gomnibus project skeleton",
	Args:  cobra.ExactArgs(1),
	RunE:  runNew,
}

func runNew(_ *cobra.Command, args []string) error {
	name := args[0]

	dirs := []string{
		"config/projects",
		"config/software",
		"pkg",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	projFile := fmt.Sprintf("config/projects/%s.yaml", name)
	if _, err := os.Stat(projFile); os.IsNotExist(err) {
		content := fmt.Sprintf(`name: %s
friendly_name: "%s"
maintainer: "Your Name <you@example.com>"
homepage: "https://example.com"
description: "A self-contained installer for %s"
install_dir: /opt/%s
build_version: "1.0.0"
build_iteration: 1
dependencies:
  - %s-software
packages:
  - type: deb
  - type: rpm
  - type: tar
`, name, name, name, name, name)
		if err := os.WriteFile(projFile, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Printf("created %s\n", projFile)
	}

	swFile := fmt.Sprintf("config/software/%s-software.yaml", name)
	if _, err := os.Stat(swFile); os.IsNotExist(err) {
		content := fmt.Sprintf(`name: %s-software
default_version: "1.0.0"
license: "Apache-2.0"
source:
  url: "https://example.com/%s-1.0.0.tar.gz"
  sha256: "REPLACE_WITH_ACTUAL_CHECKSUM"
relative_path: "%s-1.0.0"
build:
  - configure:
      - "--prefix=${install_dir}"
  - make: []
  - make:
      - install
`, name, name, name)
		if err := os.WriteFile(swFile, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Printf("created %s\n", swFile)
	}

	cfgFile := "gomnibus.yaml"
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		content := `use_git_caching: true
append_timestamp: false
workers: 4
log_level: info
software_dirs:
  - config/software
`
		if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Printf("created %s\n", cfgFile)
	}

	fmt.Printf("\nProject skeleton for %q created. Next steps:\n", name)
	fmt.Printf("  1. Edit config/projects/%s.yaml\n", name)
	fmt.Printf("  2. Edit config/software/%s-software.yaml\n", name)
	fmt.Printf("  3. Run: gomnibus build %s\n", name)
	return nil
}

// ── gomnibus validate PROJECT ────────────────────────────────────────────────

var validateCmd = &cobra.Command{
	Use:   "validate PROJECT",
	Short: "Validate project and software definition files without building",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}
		proj, err := project.LoadFromDir("config/projects", args[0])
		if err != nil {
			return fmt.Errorf("project: %w", err)
		}
		reg, err := software.NewRegistry(cfg.SoftwareDirs)
		if err != nil {
			return fmt.Errorf("software registry: %w", err)
		}
		if _, err := pipeline.Build(proj.Dependencies, reg, proj.OverrideMap()); err != nil {
			return fmt.Errorf("dependency graph: %w", err)
		}
		fmt.Printf("✓ project %q is valid\n", args[0])
		return nil
	},
}
