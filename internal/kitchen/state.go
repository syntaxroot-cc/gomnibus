package kitchen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// InstanceState is one of the lifecycle stages an instance passes through.
type InstanceState string

const (
	StateAbsent    InstanceState = "absent"
	StateCreated   InstanceState = "created"
	StateConverged InstanceState = "converged"
	StateVerified  InstanceState = "verified"
	StateError     InstanceState = "error"
)

// State records runtime information about a (suite, platform) instance that
// must persist across separate `gomnibus kitchen` invocations.
type State struct {
	Name        string        `json:"name"`
	Driver      string        `json:"driver"`
	State       InstanceState `json:"state"`
	ContainerID string        `json:"container_id,omitempty"` // Docker
	VMDir       string        `json:"vm_dir,omitempty"`       // Vagrant
	Hostname    string        `json:"hostname,omitempty"`
	Port        int           `json:"port,omitempty"`
	LastAction  string        `json:"last_action,omitempty"`
	Artifacts   []string      `json:"artifacts,omitempty"` // paths to produced packages
	UpdatedAt   time.Time     `json:"updated_at"`
	Error       string        `json:"error,omitempty"`
}

// StateStore persists instance state as JSON files under a directory.
type StateStore struct {
	Dir string
}

func NewStateStore(dir string) *StateStore {
	if dir == "" {
		dir = ".kitchen-state"
	}
	return &StateStore{Dir: dir}
}

func (s *StateStore) Load(instanceName string) (*State, error) {
	path := s.statePath(instanceName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{Name: instanceName, State: StateAbsent}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading state for %q: %w", instanceName, err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing state for %q: %w", instanceName, err)
	}
	return &st, nil
}

func (s *StateStore) Save(st *State) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	st.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.statePath(st.Name), data, 0o644)
}

func (s *StateStore) Delete(instanceName string) error {
	err := os.Remove(s.statePath(instanceName))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *StateStore) statePath(instanceName string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, instanceName)
	return filepath.Join(s.Dir, safe+".json")
}
