package kitchen

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// Runner orchestrates the full lifecycle of a kitchen instance:
// Create → Converge → Verify → Destroy.
type Runner struct {
	store  *StateStore
	log    *zap.Logger
	outDir string // local directory where packages are copied after converge
}

func NewRunner(stateDir, outDir string, log *zap.Logger) *Runner {
	if outDir == "" {
		outDir = filepath.Join("pkg", "kitchen")
	}
	return &Runner{
		store:  NewStateStore(stateDir),
		log:    log,
		outDir: outDir,
	}
}

// ── Lifecycle actions ────────────────────────────────────────────────────────

// Create starts the platform instance (idempotent if already created).
func (r *Runner) Create(ctx context.Context, inst InstanceConfig) error {
	state, err := r.store.Load(inst.Name)
	if err != nil {
		return err
	}
	if state.State != StateAbsent {
		r.log.Info("instance already created", zap.String("instance", inst.Name), zap.String("state", string(state.State)))
		return nil
	}

	driver, err := ForConfig(inst)
	if err != nil {
		return err
	}

	r.log.Info("creating instance", zap.String("instance", inst.Name), zap.String("driver", inst.Driver.Name))
	state.Name = inst.Name
	state.Driver = inst.Driver.Name
	if err := driver.Create(ctx, inst, state); err != nil {
		state.State = StateError
		state.Error = err.Error()
		_ = r.store.Save(state)
		return fmt.Errorf("create %s: %w", inst.Name, err)
	}
	state.State = StateCreated
	state.LastAction = "create"
	return r.store.Save(state)
}

// Converge runs the gomnibus build inside an already-created instance.
func (r *Runner) Converge(ctx context.Context, inst InstanceConfig) error {
	state, err := r.store.Load(inst.Name)
	if err != nil {
		return err
	}
	if state.State == StateAbsent {
		if err := r.Create(ctx, inst); err != nil {
			return err
		}
		state, _ = r.store.Load(inst.Name)
	}

	driver, err := ForConfig(inst)
	if err != nil {
		return err
	}

	r.log.Info("converging instance", zap.String("instance", inst.Name))
	if err := r.runProvisioner(ctx, inst, driver, state); err != nil {
		state.State = StateError
		state.Error = err.Error()
		_ = r.store.Save(state)
		return fmt.Errorf("converge %s: %w", inst.Name, err)
	}
	state.State = StateConverged
	state.LastAction = "converge"
	state.Error = ""
	return r.store.Save(state)
}

// Verify runs the verifier inside an already-converged instance.
func (r *Runner) Verify(ctx context.Context, inst InstanceConfig) error {
	state, err := r.store.Load(inst.Name)
	if err != nil {
		return err
	}
	if state.State == StateAbsent || state.State == StateCreated {
		return fmt.Errorf("instance %s has not been converged yet (state: %s)", inst.Name, state.State)
	}

	driver, err := ForConfig(inst)
	if err != nil {
		return err
	}

	r.log.Info("verifying instance", zap.String("instance", inst.Name))
	if err := r.runVerifier(ctx, inst, driver, state); err != nil {
		state.State = StateError
		state.Error = err.Error()
		_ = r.store.Save(state)
		return fmt.Errorf("verify %s: %w", inst.Name, err)
	}
	state.State = StateVerified
	state.LastAction = "verify"
	state.Error = ""
	return r.store.Save(state)
}

// Destroy stops and removes the instance, cleaning up all state.
func (r *Runner) Destroy(ctx context.Context, inst InstanceConfig) error {
	state, err := r.store.Load(inst.Name)
	if err != nil {
		return err
	}
	if state.State == StateAbsent {
		r.log.Info("instance already absent", zap.String("instance", inst.Name))
		return nil
	}

	driver, err := ForConfig(inst)
	if err != nil {
		return err
	}

	r.log.Info("destroying instance", zap.String("instance", inst.Name))
	if err := driver.Destroy(ctx, state); err != nil {
		return fmt.Errorf("destroy %s: %w", inst.Name, err)
	}
	return r.store.Delete(inst.Name)
}

// Test runs the full create → converge → verify → destroy cycle.
// On failure, the instance is left running so it can be inspected.
func (r *Runner) Test(ctx context.Context, inst InstanceConfig) error {
	if err := r.Create(ctx, inst); err != nil {
		return err
	}
	if err := r.Converge(ctx, inst); err != nil {
		return err
	}
	if err := r.Verify(ctx, inst); err != nil {
		return err
	}
	return r.Destroy(ctx, inst)
}

// Login opens an interactive shell inside the instance.
func (r *Runner) Login(ctx context.Context, inst InstanceConfig) error {
	state, err := r.store.Load(inst.Name)
	if err != nil {
		return err
	}
	if state.State == StateAbsent {
		return fmt.Errorf("instance %s does not exist — run 'gomnibus kitchen create %s' first", inst.Name, inst.Name)
	}
	driver, err := ForConfig(inst)
	if err != nil {
		return err
	}
	return driver.Login(ctx, state)
}

// ── Provisioner ──────────────────────────────────────────────────────────────

func (r *Runner) runProvisioner(ctx context.Context, inst InstanceConfig, drv Driver, state *State) error {
	prov := inst.Provisioner
	workDir := prov.WorkDir

	// 1. Install build prerequisites.
	if len(prov.InstallPackages) > 0 {
		r.log.Info("installing packages", zap.Strings("packages", prov.InstallPackages))
		installCmd := buildInstallCmd(inst.Platform.Name, prov.InstallPackages)
		if err := drv.Exec(ctx, state, installCmd); err != nil {
			return fmt.Errorf("installing packages: %w", err)
		}
	}

	// 2. Create workspace directory.
	if err := drv.Exec(ctx, state, "mkdir -p "+workDir); err != nil {
		return err
	}

	// 3. Copy gomnibus binary into the container.
	gomnibusBin, err := resolveBinary(prov.GomnibusBinary)
	if err != nil {
		return fmt.Errorf("resolving gomnibus binary: %w", err)
	}
	if err := drv.CopyTo(ctx, state, gomnibusBin, workDir+"/gomnibus"); err != nil {
		return fmt.Errorf("copying gomnibus binary: %w", err)
	}
	if err := drv.Exec(ctx, state, "chmod +x "+workDir+"/gomnibus"); err != nil {
		return err
	}

	// 4. Copy project configuration into the container.
	for _, dir := range []string{"config", "gomnibus.yaml"} {
		if _, err := os.Stat(dir); err == nil {
			if err := drv.CopyTo(ctx, state, dir, workDir+"/"+dir); err != nil {
				return fmt.Errorf("copying %s: %w", dir, err)
			}
		}
	}

	// 5. Run the build.
	buildArgs := strings.Join(prov.BuildArgs, " ")
	buildCmd := fmt.Sprintf("cd %s && ./gomnibus build %s %s", workDir, prov.Project, buildArgs)
	r.log.Info("running build", zap.String("instance", inst.Name), zap.String("project", prov.Project))
	if err := drv.Exec(ctx, state, buildCmd); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// 6. Copy produced packages back to the host.
	pkgDir := filepath.Join(r.outDir, inst.Platform.Name)
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return err
	}
	remotePkgDir := workDir + "/pkg"
	if err := drv.CopyFrom(ctx, state, remotePkgDir, pkgDir); err != nil {
		// Non-fatal: packages may have been placed differently.
		r.log.Warn("could not copy packages from instance", zap.Error(err))
	} else {
		r.log.Info("packages copied", zap.String("local_dir", pkgDir))
	}

	return nil
}

// ── Verifier ─────────────────────────────────────────────────────────────────

func (r *Runner) runVerifier(ctx context.Context, inst InstanceConfig, drv Driver, state *State) error {
	verifier := inst.Verifier
	if verifier.Command == "" && len(verifier.Scripts) == 0 {
		r.log.Info("no verifier configured, skipping", zap.String("instance", inst.Name))
		return nil
	}

	// Copy and run any verifier scripts.
	for _, script := range verifier.Scripts {
		remotePath := inst.Provisioner.WorkDir + "/verify/" + filepath.Base(script)
		if err := drv.CopyTo(ctx, state, script, remotePath); err != nil {
			return fmt.Errorf("copying verifier script %s: %w", script, err)
		}
		if err := drv.Exec(ctx, state, "chmod +x "+remotePath+" && "+remotePath); err != nil {
			return fmt.Errorf("verifier script %s failed: %w", script, err)
		}
	}

	// Run inline command.
	if verifier.Command != "" {
		r.log.Info("running verifier command", zap.String("instance", inst.Name))
		if err := drv.Exec(ctx, state, verifier.Command); err != nil {
			return fmt.Errorf("verifier command failed: %w", err)
		}
	}
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// buildInstallCmd produces the correct package manager invocation based on
// clues in the platform name.
func buildInstallCmd(platformName string, packages []string) string {
	pkgList := strings.Join(packages, " ")
	name := strings.ToLower(platformName)
	switch {
	case strings.Contains(name, "ubuntu") || strings.Contains(name, "debian"):
		return "DEBIAN_FRONTEND=noninteractive apt-get update -q && apt-get install -y --no-install-recommends " + pkgList
	case strings.Contains(name, "centos") || strings.Contains(name, "rocky") ||
		strings.Contains(name, "alma") || strings.Contains(name, "rhel"):
		return "dnf install -y " + pkgList
	case strings.Contains(name, "amazon") || strings.Contains(name, "amazonlinux"):
		return "yum install -y " + pkgList
	case strings.Contains(name, "fedora"):
		return "dnf install -y " + pkgList
	case strings.Contains(name, "opensuse") || strings.Contains(name, "sles"):
		return "zypper install -y " + pkgList
	default:
		return "apt-get update -q && apt-get install -y " + pkgList
	}
}

// resolveBinary finds the gomnibus binary: checks the PATH entry, then looks
// for a freshly built binary next to the current executable.
func resolveBinary(name string) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}
	// Prefer a built binary in the current working directory.
	if _, err := os.Stat(name); err == nil {
		abs, _ := filepath.Abs(name)
		return abs, nil
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%q not found in PATH — build it first with: go build ./cmd/gomnibus", name)
	}
	return path, nil
}
