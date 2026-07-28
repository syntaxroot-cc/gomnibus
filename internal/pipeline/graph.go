// Package pipeline resolves software dependency order and executes build stages
// in parallel where the dependency DAG allows it.
package pipeline

import (
	"fmt"

	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

// Node wraps a software definition with resolved dependency edges.
type Node struct {
	Def    *software.Definition
	Deps   []*Node
	Level  int // topological depth, used for parallel scheduling
}

// Graph is an ordered, dependency-aware build graph.
type Graph struct {
	nodes  map[string]*Node
	order  []*Node
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
