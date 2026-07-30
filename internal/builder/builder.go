// Package builder executes the build steps defined in a software definition.
package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/syntaxroot-cc/gomnibus/internal/software"
	"go.uber.org/zap"
)

// Context carries all runtime paths a build step needs.
type Context struct {
	SrcDir     string // where the fetched source lives
	BuildDir   string // temporary scratch area
	InstallDir string // destination install root
	Env        []string
	Log        *zap.Logger
}

// Execute runs all build steps defined for a software definition.
func Execute(ctx context.Context, def *software.Definition, bc *Context) error {
	for i, step := range def.Build {
		bc.Log.Info("build step", zap.Int("step", i+1), zap.String("software", def.Name))
		if err := runStep(ctx, step, bc); err != nil {
			return fmt.Errorf("software %s step %d: %w", def.Name, i+1, err)
		}
	}
	return nil
}

func runStep(ctx context.Context, step software.BuildStep, bc *Context) error {
	workDir := bc.SrcDir
	if step.WorkDir != "" {
		workDir = expandPath(step.WorkDir, bc)
	}

	env := mergeEnv(bc.Env, step.Env)

	switch {
	case step.Command != "":
		return runShell(ctx, expandVars(step.Command, bc), workDir, env)

	case step.Make != nil:
		return runCmd(ctx, workDir, env, "make", expandSlice(step.Make, bc)...)

	case step.CMake != nil:
		args := append([]string{"-DCMAKE_INSTALL_PREFIX=" + bc.InstallDir}, expandSlice(step.CMake, bc)...)
		return runCmd(ctx, workDir, env, "cmake", args...)

	case step.Configure != nil:
		args := append([]string{"--prefix=" + bc.InstallDir}, expandSlice(step.Configure, bc)...)
		return runCmd(ctx, workDir, env, "./configure", args...)

	case step.Go != nil:
		return runCmd(ctx, workDir, env, "go", expandSlice(step.Go, bc)...)

	case step.Gem != nil:
		return runCmd(ctx, workDir, env, "gem", expandSlice(step.Gem, bc)...)

	case step.Mkdir != "":
		return os.MkdirAll(expandPath(step.Mkdir, bc), 0o755)

	case step.Delete != "":
		return os.RemoveAll(expandPath(step.Delete, bc))

	case step.Copy != nil:
		return copyFile(expandPath(step.Copy.Src, bc), expandPath(step.Copy.Dst, bc))

	case step.Move != nil:
		return os.Rename(expandPath(step.Move.Src, bc), expandPath(step.Move.Dst, bc))

	case step.Link != nil:
		return os.Symlink(expandPath(step.Link.Target, bc), expandPath(step.Link.Link, bc))

	case step.Patch != nil:
		patchFile := expandPath(step.Patch.Source, bc)
		plevel := fmt.Sprintf("-p%d", step.Patch.Plevel)
		return runCmd(ctx, workDir, env, "patch", plevel, "-i", patchFile)

	default:
		return fmt.Errorf("empty or unknown build step")
	}
}

func runShell(ctx context.Context, command, dir string, env []string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCmd(ctx context.Context, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func mergeEnv(base []string, extra map[string]string) []string {
	result := make([]string, len(base))
	copy(result, base)
	for k, v := range extra {
		result = append(result, k+"="+v)
	}
	return result
}

func expandPath(p string, bc *Context) string {
	p = expandVars(p, bc)
	if !path.IsAbs(p) && !filepath.IsAbs(p) {
		p = path.Join(bc.SrcDir, p)
	}
	return p
}

func expandVars(s string, bc *Context) string {
	s = strings.ReplaceAll(s, "${install_dir}", bc.InstallDir)
	s = strings.ReplaceAll(s, "${src_dir}", bc.SrcDir)
	s = strings.ReplaceAll(s, "${build_dir}", bc.BuildDir)
	return s
}

func expandSlice(args []string, bc *Context) []string {
	if len(args) == 0 {
		return args
	}
	expanded := make([]string, len(args))
	for i, a := range args {
		expanded[i] = expandVars(a, bc)
	}
	return expanded
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = fmt.Fprintf(out, "")
	if err != nil {
		return err
	}
	_, err = out.ReadFrom(in)
	return err
}
