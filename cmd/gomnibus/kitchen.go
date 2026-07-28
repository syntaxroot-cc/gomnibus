package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/syntaxroot-cc/gomnibus/internal/kitchen"
	_ "github.com/syntaxroot-cc/gomnibus/internal/kitchen/drivers/docker"
	_ "github.com/syntaxroot-cc/gomnibus/internal/kitchen/drivers/vagrant"
	"github.com/syntaxroot-cc/gomnibus/pkg/log"
)

var (
	kitchenCfgFile  string
	kitchenStateDir string
	kitchenOutDir   string
	kitchenConcur   int
)

// kitchenCmd is the top-level `gomnibus kitchen` command.
var kitchenCmd = &cobra.Command{
	Use:   "kitchen",
	Short: "Multi-platform build orchestration (Test Kitchen-compatible)",
	Long: `Spin up containers or VMs, run the gomnibus build inside each platform,
verify the resulting package, and tear everything down.

Configuration lives in .kitchen.yml (Test Kitchen format).`,
}

func init() {
	kitchenCmd.PersistentFlags().StringVarP(&kitchenCfgFile, "kitchen-config", "K", ".kitchen.yml", "kitchen config file")
	kitchenCmd.PersistentFlags().StringVar(&kitchenStateDir, "state-dir", ".kitchen-state", "directory for instance state files")
	kitchenCmd.PersistentFlags().StringVar(&kitchenOutDir, "output", "pkg/kitchen", "directory for packages copied from instances")
	kitchenCmd.PersistentFlags().IntVar(&kitchenConcur, "concurrency", 1, "number of instances to run in parallel")

	kitchenCmd.AddCommand(
		kitchenListCmd,
		kitchenCreateCmd,
		kitchenConvergeCmd,
		kitchenVerifyCmd,
		kitchenDestroyCmd,
		kitchenTestCmd,
		kitchenLoginCmd,
	)

	rootCmd.AddCommand(kitchenCmd)
}

// ── gomnibus kitchen list ────────────────────────────────────────────────────

var kitchenListCmd = &cobra.Command{
	Use:   "list [REGEX]",
	Short: "List configured instances and their current state",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := kitchen.LoadConfig(kitchenCfgFile)
		if err != nil {
			return err
		}
		filter := ""
		if len(args) > 0 {
			filter = args[0]
		}
		instances := cfg.Instances(filter)
		store := kitchen.NewStateStore(kitchenStateDir)

		fmt.Printf("%-40s %-10s %-12s %s\n", "Instance", "Driver", "State", "Last Action")
		fmt.Println(strings.Repeat("-", 80))
		for _, inst := range instances {
			state, _ := store.Load(inst.Name)
			stateStr := string(state.State)
			if state.Error != "" {
				stateStr = "error"
			}
			fmt.Printf("%-40s %-10s %-12s %s\n",
				inst.Name, inst.Driver.Name, stateStr, state.LastAction)
		}
		return nil
	},
}

// ── gomnibus kitchen create ──────────────────────────────────────────────────

var kitchenCreateCmd = &cobra.Command{
	Use:   "create [REGEX]",
	Short: "Start instance containers/VMs without running the build",
	Args:  cobra.MaximumNArgs(1),
	RunE: kitchenAction("create", func(r *kitchen.Runner, _ kitchen.InstanceConfig) runnerFn {
		return r.Create
	}),
}

// ── gomnibus kitchen converge ────────────────────────────────────────────────

var kitchenConvergeCmd = &cobra.Command{
	Use:   "converge [REGEX]",
	Short: "Run the gomnibus build inside each instance",
	Args:  cobra.MaximumNArgs(1),
	RunE: kitchenAction("converge", func(r *kitchen.Runner, _ kitchen.InstanceConfig) runnerFn {
		return r.Converge
	}),
}

// ── gomnibus kitchen verify ──────────────────────────────────────────────────

var kitchenVerifyCmd = &cobra.Command{
	Use:   "verify [REGEX]",
	Short: "Run verifier commands inside converged instances",
	Args:  cobra.MaximumNArgs(1),
	RunE: kitchenAction("verify", func(r *kitchen.Runner, _ kitchen.InstanceConfig) runnerFn {
		return r.Verify
	}),
}

// ── gomnibus kitchen destroy ─────────────────────────────────────────────────

var kitchenDestroyCmd = &cobra.Command{
	Use:   "destroy [REGEX]",
	Short: "Stop and remove instance containers/VMs",
	Args:  cobra.MaximumNArgs(1),
	RunE: kitchenAction("destroy", func(r *kitchen.Runner, _ kitchen.InstanceConfig) runnerFn {
		return r.Destroy
	}),
}

// ── gomnibus kitchen test ────────────────────────────────────────────────────

var kitchenTestCmd = &cobra.Command{
	Use:   "test [REGEX]",
	Short: "Full create → converge → verify → destroy cycle",
	Args:  cobra.MaximumNArgs(1),
	RunE: kitchenAction("test", func(r *kitchen.Runner, _ kitchen.InstanceConfig) runnerFn {
		return r.Test
	}),
}

// ── gomnibus kitchen login ───────────────────────────────────────────────────

var kitchenLoginCmd = &cobra.Command{
	Use:   "login INSTANCE",
	Short: "Open an interactive shell inside a running instance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := log.L()
		cfg, err := kitchen.LoadConfig(kitchenCfgFile)
		if err != nil {
			return err
		}
		instances := cfg.Instances(args[0])
		if len(instances) == 0 {
			return fmt.Errorf("no instance matching %q", args[0])
		}
		if len(instances) > 1 {
			return fmt.Errorf("pattern %q matches multiple instances — use a more specific name", args[0])
		}
		runner := kitchen.NewRunner(kitchenStateDir, kitchenOutDir, logger)
		return runner.Login(cmd.Context(), instances[0])
	},
}

// ── shared plumbing ──────────────────────────────────────────────────────────

// runnerFn matches the signature of Runner.Create/Converge/Verify/Destroy/Test.
type runnerFn func(ctx context.Context, inst kitchen.InstanceConfig) error

// kitchenAction returns a cobra RunE implementation that:
//  1. Loads the kitchen config and resolves matching instances.
//  2. Runs actionFn for each instance sequentially (concurrency planned).
//  3. Collects all errors and reports them together.
func kitchenAction(
	actionName string,
	actionFn func(*kitchen.Runner, kitchen.InstanceConfig) runnerFn,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		logger := log.L()
		cfg, err := kitchen.LoadConfig(kitchenCfgFile)
		if err != nil {
			return err
		}
		filter := ""
		if len(args) > 0 {
			filter = args[0]
		}
		instances := cfg.Instances(filter)
		if len(instances) == 0 {
			return fmt.Errorf("no instances match filter %q", filter)
		}

		runner := kitchen.NewRunner(kitchenStateDir, kitchenOutDir, logger)
		ctx := cmd.Context()

		var errs []string
		for _, inst := range instances {
			logger.Info("kitchen "+actionName, zap.String("instance", inst.Name))
			fn := actionFn(runner, inst)
			if err := fn(ctx, inst); err != nil {
				logger.Error("kitchen "+actionName+" failed",
					zap.String("instance", inst.Name),
					zap.Error(err),
				)
				errs = append(errs, fmt.Sprintf("%s: %v", inst.Name, err))
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("%d instance(s) failed:\n  %s", len(errs), strings.Join(errs, "\n  "))
		}
		return nil
	}
}
