// Package pipeline contains a lightweight workflow graph implementation
// that mirrors the ADK v2 workflow.Graph API.
//
// When the real google.golang.org/adk/v2 module is available, replace
// this file with the ADK import and update pipeline.go to use:
//
//	import "google.golang.org/adk/v2/workflow"
//
// The API surface is intentionally identical so the swap is a one-line
// import change.
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
)

// Graph is a typed sequential workflow graph.
// Nodes are registered by name and connected via edges.
type Graph[S any] struct {
	name       string
	nodes      map[string]*FunctionNode[S]
	edges      map[string][]string
	entrypoint string
}

// NewGraph creates a new workflow graph with the given name.
func NewGraph[S any](name string) *Graph[S] {
	return &Graph[S]{
		name:  name,
		nodes: make(map[string]*FunctionNode[S]),
		edges: make(map[string][]string),
	}
}

// FunctionNode wraps a pure function as a graph node.
type FunctionNode[S any] struct {
	name string
	fn   func(context.Context, S) (S, error)
}

// NewFunctionNode creates a named function node.
func NewFunctionNode[S any](name string, fn func(context.Context, S) (S, error)) *FunctionNode[S] {
	return &FunctionNode[S]{name: name, fn: fn}
}

// AddNode registers a node in the graph.
func (g *Graph[S]) AddNode(node *FunctionNode[S]) {
	g.nodes[node.name] = node
}

// AddEdge connects two nodes by name.
func (g *Graph[S]) AddEdge(from, to string) {
	g.edges[from] = append(g.edges[from], to)
}

// SetEntrypoint designates the starting node.
func (g *Graph[S]) SetEntrypoint(name string) {
	g.entrypoint = name
}

// Execute runs the workflow graph from the entrypoint, following edges in order.
func (g *Graph[S]) Execute(ctx context.Context, state S) error {
	if g.entrypoint == "" {
		return fmt.Errorf("workflow %q: no entrypoint set", g.name)
	}

	slog.Info("workflow started", "name", g.name, "entrypoint", g.entrypoint)

	current := g.entrypoint
	for {
		node, ok := g.nodes[current]
		if !ok {
			return fmt.Errorf("workflow %q: node %q not found", g.name, current)
		}

		slog.Info("executing node", "workflow", g.name, "node", current)

		var err error
		state, err = node.fn(ctx, state)
		if err != nil {
			return fmt.Errorf("workflow %q node %q: %w", g.name, current, err)
		}

		// Follow the first edge (sequential pipeline).
		next := g.edges[current]
		if len(next) == 0 {
			break // Terminal node.
		}
		current = next[0]
	}

	slog.Info("workflow complete", "name", g.name)
	return nil
}
