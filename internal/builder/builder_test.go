package builder

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/syntaxroot-cc/gomnibus/internal/software"
	"go.uber.org/zap"
)

// ── expandPath ────────────────────────────────────────────────────────────────

func TestExpandPath_InstallDir(t *testing.T) {
	bc := &Context{SrcDir: "/src", BuildDir: "/build", InstallDir: "/install"}
	got := expandPath("${install_dir}/lib", bc)
	if got != "/install/lib" {
		t.Errorf("got %q, want /install/lib", got)
	}
}

func TestExpandPath_SrcDir(t *testing.T) {
	bc := &Context{SrcDir: "/src", BuildDir: "/build", InstallDir: "/install"}
	got := expandPath("${src_dir}/configure", bc)
	if got != "/src/configure" {
		t.Errorf("got %q, want /src/configure", got)
	}
}

func TestExpandPath_BuildDir(t *testing.T) {
	bc := &Context{SrcDir: "/src", BuildDir: "/build", InstallDir: "/install"}
	got := expandPath("${build_dir}/obj", bc)
	if got != "/build/obj" {
		t.Errorf("got %q, want /build/obj", got)
	}
}

func TestExpandPath_Absolute(t *testing.T) {
	bc := &Context{SrcDir: "/src", BuildDir: "/build", InstallDir: "/install"}
	got := expandPath("/absolute/path", bc)
	if got != "/absolute/path" {
		t.Errorf("absolute path should be unchanged: got %q", got)
	}
}

func TestExpandPath_Relative(t *testing.T) {
	bc := &Context{SrcDir: "/src", BuildDir: "/build", InstallDir: "/install"}
	got := expandPath("relative/file", bc)
	want := filepath.Join("/src", "relative/file")
	if got != want {
		t.Errorf("relative path: got %q, want %q", got, want)
	}
}

func TestExpandPath_MultipleVars(t *testing.T) {
	bc := &Context{SrcDir: "/src", BuildDir: "/build", InstallDir: "/install"}
	// expandPath only expands one variable per call but the path may contain a var
	got := expandPath("${install_dir}/bin", bc)
	if got != "/install/bin" {
		t.Errorf("got %q", got)
	}
}

// ── mergeEnv ─────────────────────────────────────────────────────────────────

func TestMergeEnv_AppendsExtra(t *testing.T) {
	base := []string{"A=1", "B=2"}
	extra := map[string]string{"C": "3", "D": "4"}
	result := mergeEnv(base, extra)

	if result[0] != "A=1" || result[1] != "B=2" {
		t.Errorf("base not preserved: %v", result)
	}
	hasVar := func(key string) bool {
		for _, e := range result {
			if strings.HasPrefix(e, key+"=") {
				return true
			}
		}
		return false
	}
	if !hasVar("C") || !hasVar("D") {
		t.Errorf("extra vars not added: %v", result)
	}
}

func TestMergeEnv_NilExtra(t *testing.T) {
	base := []string{"A=1"}
	result := mergeEnv(base, nil)
	if len(result) != 1 || result[0] != "A=1" {
		t.Errorf("nil extra should return copy of base: %v", result)
	}
}

func TestMergeEnv_PreservesBaseOrder(t *testing.T) {
	base := []string{"X=1", "Y=2", "Z=3"}
	result := mergeEnv(base, nil)
	for i, want := range base {
		if result[i] != want {
			t.Errorf("result[%d] = %q, want %q", i, result[i], want)
		}
	}
}

// ── Execute: filesystem steps ─────────────────────────────────────────────────

func testCtx(t *testing.T) *Context {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return &Context{
		SrcDir:     t.TempDir(),
		BuildDir:   t.TempDir(),
		InstallDir: t.TempDir(),
		Env:        os.Environ(),
		Log:        logger,
	}
}

func TestExecute_Mkdir(t *testing.T) {
	bc := testCtx(t)
	target := filepath.Join(bc.InstallDir, "lib", "pkgconfig")
	def := &software.Definition{
		Name:  "test",
		Build: []software.BuildStep{{Mkdir: "${install_dir}/lib/pkgconfig"}},
	}
	if err := Execute(context.Background(), def, bc); err != nil {
		t.Fatalf("Mkdir step: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

func TestExecute_Delete(t *testing.T) {
	bc := testCtx(t)
	dir := filepath.Join(bc.InstallDir, "toremove")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	def := &software.Definition{
		Name:  "test",
		Build: []software.BuildStep{{Delete: "${install_dir}/toremove"}},
	}
	if err := Execute(context.Background(), def, bc); err != nil {
		t.Fatalf("Delete step: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("directory should have been deleted")
	}
}

func TestExecute_Copy(t *testing.T) {
	bc := testCtx(t)
	if err := os.WriteFile(filepath.Join(bc.SrcDir, "input.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	def := &software.Definition{
		Name: "test",
		Build: []software.BuildStep{{Copy: &software.CopySpec{
			Src: "${src_dir}/input.txt",
			Dst: "${install_dir}/output.txt",
		}}},
	}
	if err := Execute(context.Background(), def, bc); err != nil {
		t.Fatalf("Copy step: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(bc.InstallDir, "output.txt"))
	if err != nil || string(got) != "hello" {
		t.Errorf("Copy: got %q, err %v", got, err)
	}
}

func TestExecute_Move(t *testing.T) {
	bc := testCtx(t)
	src := filepath.Join(bc.SrcDir, "moveme.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	def := &software.Definition{
		Name: "test",
		Build: []software.BuildStep{{Move: &software.MoveSpec{
			Src: "${src_dir}/moveme.txt",
			Dst: "${install_dir}/moved.txt",
		}}},
	}
	if err := Execute(context.Background(), def, bc); err != nil {
		t.Fatalf("Move step: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source should be gone after move")
	}
	got, err := os.ReadFile(filepath.Join(bc.InstallDir, "moved.txt"))
	if err != nil || string(got) != "data" {
		t.Errorf("Move: got %q, err %v", got, err)
	}
}

func TestExecute_Link(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on Windows")
	}
	bc := testCtx(t)
	target := filepath.Join(bc.InstallDir, "target.txt")
	if err := os.WriteFile(target, []byte("linked"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(bc.InstallDir, "link.txt")
	def := &software.Definition{
		Name: "test",
		Build: []software.BuildStep{{Link: &software.LinkSpec{
			Target: "${install_dir}/target.txt",
			Link:   "${install_dir}/link.txt",
		}}},
	}
	if err := Execute(context.Background(), def, bc); err != nil {
		t.Fatalf("Link step: %v", err)
	}
	dest, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if dest != target {
		t.Errorf("symlink target: got %q, want %q", dest, target)
	}
}

func TestExecute_EmptyStep(t *testing.T) {
	bc := testCtx(t)
	def := &software.Definition{
		Name:  "test",
		Build: []software.BuildStep{{}},
	}
	if err := Execute(context.Background(), def, bc); err == nil {
		t.Error("expected error for empty build step")
	}
}

func TestExecute_Env_StepOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell env test requires sh")
	}
	bc := testCtx(t)
	outFile := filepath.Join(bc.InstallDir, "env.txt")
	def := &software.Definition{
		Name: "test",
		Build: []software.BuildStep{{
			Command: "echo $TESTVAR > " + outFile,
			Env:     map[string]string{"TESTVAR": "hello_from_env"},
		}},
	}
	if err := Execute(context.Background(), def, bc); err != nil {
		t.Fatalf("Command with env: %v", err)
	}
}

func TestExecute_Command(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell commands require sh, not available on Windows")
	}
	bc := testCtx(t)
	outFile := filepath.Join(bc.InstallDir, "stamp")
	def := &software.Definition{
		Name: "test",
		Build: []software.BuildStep{{
			Command: "touch " + outFile,
		}},
	}
	if err := Execute(context.Background(), def, bc); err != nil {
		t.Fatalf("Command step: %v", err)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Error("command did not create stamp file")
	}
}

func TestExecute_MultipleSteps(t *testing.T) {
	bc := testCtx(t)
	dir1 := filepath.Join(bc.InstallDir, "step1")
	dir2 := filepath.Join(bc.InstallDir, "step2")
	def := &software.Definition{
		Name: "test",
		Build: []software.BuildStep{
			{Mkdir: "${install_dir}/step1"},
			{Mkdir: "${install_dir}/step2"},
		},
	}
	if err := Execute(context.Background(), def, bc); err != nil {
		t.Fatalf("MultipleSteps: %v", err)
	}
	for _, d := range []string{dir1, dir2} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("dir %s not created: %v", d, err)
		}
	}
}

func TestExecute_NoBuildSteps(t *testing.T) {
	bc := testCtx(t)
	def := &software.Definition{Name: "noop"}
	if err := Execute(context.Background(), def, bc); err != nil {
		t.Errorf("empty build should be a no-op, got: %v", err)
	}
}
