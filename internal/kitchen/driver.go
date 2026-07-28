package kitchen

import (
	"context"
	"fmt"
)

// Driver manages the lifecycle of a platform instance (container or VM).
type Driver interface {
	// Create starts the instance and populates state with connection details.
	Create(ctx context.Context, inst InstanceConfig, state *State) error
	// Exec runs cmd inside the instance, streaming output to the caller.
	Exec(ctx context.Context, state *State, cmd string) error
	// CopyTo copies a local path into the instance at remotePath.
	CopyTo(ctx context.Context, state *State, localPath, remotePath string) error
	// CopyFrom copies remotePath out of the instance to a local path.
	CopyFrom(ctx context.Context, state *State, remotePath, localPath string) error
	// Login opens an interactive shell in the instance (requires a real TTY).
	Login(ctx context.Context, state *State) error
	// Destroy stops and permanently removes the instance.
	Destroy(ctx context.Context, state *State) error
}

var registry = map[string]Driver{}

// Register adds a Driver implementation by name.
func Register(name string, d Driver) {
	registry[name] = d
}

// ForConfig returns the Driver matching inst.Driver.Name.
func ForConfig(inst InstanceConfig) (Driver, error) {
	d, ok := registry[inst.Driver.Name]
	if !ok {
		return nil, fmt.Errorf("kitchen driver %q not found (available: %v)", inst.Driver.Name, availableDrivers())
	}
	return d, nil
}

func availableDrivers() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}
