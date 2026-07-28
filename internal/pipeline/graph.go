// Package pipeline resolves software dependency order and executes build stages
// in parallel where the dependency DAG allows it.
package pipeline

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

// Node wraps a software definition with resolved dependency edges.
type Node struct {
	Def   *software.Definition
	Deps  []*Node
	Level int // topological depth, used for parallel scheduling
}

// Graph is an ordered, dependency-aware build graph.
type Graph struct {
	nodes map[string]*Node
	order []*Node
}

// Build constructs a Graph for the given software names, resolving dependencies
// transitively from the registry.
func Build(names []string, reg *software.Registry, overrides map[string]string) (*Graph, error) {
	g := &Graph{nodes: make(map[string]*Node)}
	for _, name := range names {
		if err := g.addNode(name, reg, overrides, nil); err != nil {
			return nil, err
		}
	}
	if err := g.topoSort(); err != nil {
		return nil, err
	}
	g.assignLevels()
	return g, nil
}

func (g *Graph) addNode(name string, reg *software.Registry, overrides map[string]string, stack []string) error {
	for _, s := range stack {
		if s == name {
			return fmt.Errorf("circular dependency detected: %v -> %s", stack, name)
		}
	}
	if _, exists := g.nodes[name]; exists {
		return nil
	}

	def, err := reg.Get(name)
	if err != nil {
		return err
	}
	if err := def.Resolve(overrides[name]); err != nil {
		return err
	}

	node := &Node{Def: def}
	g.nodes[name] = node

	for _, dep := range def.Dependencies {
		if err := g.addNode(dep, reg, overrides, append(stack, name)); err != nil {
			return err
		}
		node.Deps = append(node.Deps, g.nodes[dep])
	}
	return nil
}

func (g *Graph) topoSort() error {
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	var visit func(n *Node) error

	visit = func(n *Node) error {
		if temp[n.Def.Name] {
			return fmt.Errorf("cycle detected at %s", n.Def.Name)
		}
		if visited[n.Def.Name] {
			return nil
		}
		temp[n.Def.Name] = true
		for _, dep := range n.Deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		delete(temp, n.Def.Name)
		visited[n.Def.Name] = true
		g.order = append(g.order, n)
		return nil
	}

	for _, n := range g.nodes {
		if err := visit(n); err != nil {
			return err
		}
	}
	return nil
}

// Order returns nodes in dependency-first order.
func (g *Graph) Order() []*Node {
	return g.order
}

// assignLevels sets Node.Level = max(dep.Level)+1 for every node.
// Because g.order is deps-first, iterating it in order guarantees every dep
// already has its level assigned before we reach the node that depends on it.
func (g *Graph) assignLevels() {
	for _, n := range g.order {
		n.Level = 0
		for _, dep := range n.Deps {
			if dep.Level+1 > n.Level {
				n.Level = dep.Level + 1
			}
		}
	}
}

// BuildFunc is the unit of work Run executes for each node.
type BuildFunc func(ctx context.Context, node *Node) error

// Run executes fn for every node in the graph, respecting dependency order and
// running up to workers nodes concurrently.
//
// A node becomes eligible the instant all of its direct dependencies have
// finished — not merely when its whole level is done — so the scheduler
// extracts the maximum parallelism the DAG allows.
//
// If any node's fn returns an error the context is cancelled, in-flight nodes
// drain, and the first error is returned.
func Run(ctx context.Context, graph *Graph, workers int, fn BuildFunc) error {
	if workers <= 0 {
		workers = 1
	}

	// Each node gets a dedicated channel that is closed when the node is done.
	// Dependent nodes block on these channels before acquiring a worker slot.
	done := make(map[string]chan struct{}, len(graph.nodes))
	for name := range graph.nodes {
		done[name] = make(chan struct{})
	}

	sem := make(chan struct{}, workers)

	g, ctx := errgroup.WithContext(ctx)

	for _, n := range graph.order {
		n := n // loop-variable capture
		g.Go(func() error {
			// Wait for every direct dependency to complete (or context cancel).
			for _, dep := range n.Deps {
				select {
				case <-done[dep.Def.Name]:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			// Acquire a worker slot.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			defer func() { <-sem }()

			err := fn(ctx, n)
			if err == nil {
				close(done[n.Def.Name])
			}
			return err
		})
	}

	return g.Wait()
}
