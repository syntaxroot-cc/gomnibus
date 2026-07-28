package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

// buildTestGraph constructs a Graph directly without going through the registry,
// for fast unit tests that don't need YAML files on disk.
func buildTestGraph(nodes map[string][]string) *Graph {
	g := &Graph{nodes: make(map[string]*Node)}
	// First pass: create nodes.
	for name := range nodes {
		g.nodes[name] = &Node{Def: &software.Definition{Name: name}}
	}
	// Second pass: wire deps.
	for name, deps := range nodes {
		n := g.nodes[name]
		n.Def.Dependencies = deps
		for _, dep := range deps {
			n.Deps = append(n.Deps, g.nodes[dep])
		}
	}
	// Topo-sort + assign levels.
	if err := g.topoSort(); err != nil {
		panic(err)
	}
	g.assignLevels()
	return g
}

// TestLevels verifies that assignLevels produces correct depths for a diamond DAG.
//
//	  D
//	 / \
//	B   C
//	 \ /
//	  A
func TestLevels(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"A"},
		"D": {"B", "C"},
	})
	want := map[string]int{"A": 0, "B": 1, "C": 1, "D": 2}
	for name, wantLevel := range want {
		got := g.nodes[name].Level
		if got != wantLevel {
			t.Errorf("node %s: Level=%d, want %d", name, got, wantLevel)
		}
	}
}

// TestRunOrder verifies that no node's fn runs before all its deps have finished.
func TestRunOrder(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"A"},
		"D": {"B", "C"},
	})

	var mu sync.Mutex
	finished := map[string]bool{}

	err := Run(context.Background(), g, 4, func(_ context.Context, node *Node) error {
		mu.Lock()
		for _, dep := range node.Deps {
			if !finished[dep.Def.Name] {
				mu.Unlock()
				t.Errorf("node %s ran before dep %s finished", node.Def.Name, dep.Def.Name)
				return nil
			}
		}
		mu.Unlock()

		time.Sleep(5 * time.Millisecond) // simulate work

		mu.Lock()
		finished[node.Def.Name] = true
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(finished) != 4 {
		t.Errorf("only %d of 4 nodes ran", len(finished))
	}
}

// TestRunParallelism verifies that independent nodes (B and C) run concurrently.
func TestRunParallelism(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"A"},
		"D": {"B", "C"},
	})

	var concurrentPeak int64
	var inFlight int64

	err := Run(context.Background(), g, 4, func(ctx context.Context, node *Node) error {
		cur := atomic.AddInt64(&inFlight, 1)
		for {
			old := atomic.LoadInt64(&concurrentPeak)
			if cur <= old || atomic.CompareAndSwapInt64(&concurrentPeak, old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt64(&inFlight, -1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// B and C are independent and should have run concurrently.
	if concurrentPeak < 2 {
		t.Errorf("expected concurrent peak >= 2, got %d (B and C should overlap)", concurrentPeak)
	}
}

// TestRunWorkerLimit verifies that the semaphore caps concurrency at workers.
func TestRunWorkerLimit(t *testing.T) {
	// 5 independent nodes, 2 workers — peak concurrency must not exceed 2.
	g := buildTestGraph(map[string][]string{
		"A": {}, "B": {}, "C": {}, "D": {}, "E": {},
	})

	var inFlight int64
	var peak int64

	err := Run(context.Background(), g, 2, func(_ context.Context, _ *Node) error {
		cur := atomic.AddInt64(&inFlight, 1)
		for {
			old := atomic.LoadInt64(&peak)
			if cur <= old || atomic.CompareAndSwapInt64(&peak, old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt64(&inFlight, -1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if peak > 2 {
		t.Errorf("concurrency peak %d exceeded worker limit of 2", peak)
	}
}

// TestRunCancellation verifies that context cancellation stops remaining nodes.
func TestRunCancellation(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		"A": {}, "B": {}, "C": {}, "D": {}, "E": {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	var ran int64

	err := Run(ctx, g, 1, func(_ context.Context, _ *Node) error {
		if atomic.AddInt64(&ran, 1) == 2 {
			cancel() // cancel after the second node runs
		}
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	if err == nil {
		t.Error("expected a context cancellation error, got nil")
	}
	if atomic.LoadInt64(&ran) >= 5 {
		t.Error("all 5 nodes ran despite cancellation")
	}
}
