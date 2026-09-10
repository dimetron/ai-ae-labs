// Package graph builds a graph structure from triples and performs community detection.
package graph

import (
	"log/slog"
	"math"
	"sort"

	"github.com/dimetron/ai-knowledge-graph-go/internal/domain"
)

// Node represents a node in the knowledge graph.
type Node struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Color     string  `json:"color"`
	Size      float64 `json:"size"`
	Community int     `json:"community"`
	Degree    int     `json:"degree"`
}

// Edge represents an edge in the knowledge graph.
type Edge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Label    string `json:"label"`
	Inferred bool   `json:"inferred"`
}

// KnowledgeGraph holds nodes, edges, and graph metrics.
type KnowledgeGraph struct {
	Nodes       []Node         `json:"nodes"`
	Edges       []Edge         `json:"edges"`
	Stats       domain.GraphStats `json:"stats"`
	Communities int            `json:"communities"`
}

// CommunityColors are colorblind-friendly colors for communities.
var CommunityColors = []string{
	"#e41a1c", "#377eb8", "#4daf4a", "#984ea3",
	"#ff7f00", "#ffff33", "#a65628", "#f781bf",
}

// Build constructs a KnowledgeGraph from triples.
func Build(triples []domain.Triple) *KnowledgeGraph {
	if len(triples) == 0 {
		return &KnowledgeGraph{}
	}

	slog.Info("building graph", "triples", len(triples))

	// Collect unique nodes.
	nodeSet := make(map[string]bool)
	inferredEdges := make(map[[2]string]bool)

	for _, t := range triples {
		nodeSet[t.Subject] = true
		nodeSet[t.Object] = true
		if t.Inferred {
			inferredEdges[[2]string{t.Subject, t.Object}] = true
		}
	}

	// Build undirected adjacency for metrics.
	adj := make(map[string]map[string]bool)
	for _, t := range triples {
		if adj[t.Subject] == nil {
			adj[t.Subject] = make(map[string]bool)
		}
		adj[t.Subject][t.Object] = true
		if adj[t.Object] == nil {
			adj[t.Object] = make(map[string]bool)
		}
		adj[t.Object][t.Subject] = true
	}

	// Calculate centrality metrics.
	degree := make(map[string]int)
	for node, neighbors := range adj {
		degree[node] = len(neighbors)
	}

	betweenness := computeBetweenness(adj, nodeSet)

	// Community detection (simple label propagation).
	communities := detectCommunities(adj, nodeSet)
	communityCount := countUnique(communities)

	// Calculate node sizes based on centrality.
	sizes := computeSizes(nodeSet, degree, betweenness)

	// Build nodes.
	var nodes []Node
	for id := range nodeSet {
		comm := communities[id]
		nodes = append(nodes, Node{
			ID:        id,
			Label:     id,
			Color:     CommunityColors[comm%len(CommunityColors)],
			Size:      sizes[id],
			Community: comm,
			Degree:    degree[id],
		})
	}

	// Sort nodes by ID for deterministic output.
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	// Build edges.
	var edges []Edge
	for _, t := range triples {
		edges = append(edges, Edge{
			From:     t.Subject,
			To:       t.Object,
			Label:    t.Predicate,
			Inferred: t.Inferred,
		})
	}

	inferredCount := len(inferredEdges)
	stats := domain.GraphStats{
		Nodes:         len(nodeSet),
		Edges:         len(triples),
		OriginalEdges: len(triples) - inferredCount,
		InferredEdges: inferredCount,
		Communities:   communityCount,
	}

	slog.Info("graph built",
		"nodes", stats.Nodes,
		"edges", stats.Edges,
		"communities", stats.Communities,
	)

	return &KnowledgeGraph{
		Nodes:       nodes,
		Edges:       edges,
		Stats:       stats,
		Communities: communityCount,
	}
}

// detectCommunities uses iterative label propagation.
func detectCommunities(adj map[string]map[string]bool, nodes map[string]bool) map[string]int {
	// Initialize each node with a unique community label.
	labels := make(map[string]int)
	id := 0
	for node := range nodes {
		labels[node] = id
		id++
	}

	// Iterate until convergence (max 50 iterations).
	for iter := 0; iter < 50; iter++ {
		changed := false

		for node := range nodes {
			neighbors := adj[node]
			if len(neighbors) == 0 {
				continue
			}

			// Count neighbor labels.
			freq := make(map[int]int)
			for n := range neighbors {
				freq[labels[n]]++
			}

			// Find the most common label.
			maxCount := 0
			maxLabel := labels[node]
			for label, count := range freq {
				if count > maxCount || (count == maxCount && label < maxLabel) {
					maxCount = count
					maxLabel = label
				}
			}

			if maxLabel != labels[node] {
				labels[node] = maxLabel
				changed = true
			}
		}

		if !changed {
			break
		}
	}

	// Compact labels to 0..n-1.
	labelMap := make(map[int]int)
	nextID := 0
	result := make(map[string]int)
	for node, label := range labels {
		if _, ok := labelMap[label]; !ok {
			labelMap[label] = nextID
			nextID++
		}
		result[node] = labelMap[label]
	}

	return result
}

// computeBetweenness calculates approximate betweenness centrality via BFS.
func computeBetweenness(adj map[string]map[string]bool, nodes map[string]bool) map[string]float64 {
	bc := make(map[string]float64)

	for source := range nodes {
		// BFS from source.
		dist := make(map[string]int)
		pathCount := make(map[string]float64)
		dist[source] = 0
		pathCount[source] = 1.0

		var queue []string
		var stack []string
		queue = append(queue, source)

		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			stack = append(stack, v)

			for w := range adj[v] {
				if _, ok := dist[w]; !ok {
					dist[w] = dist[v] + 1
					queue = append(queue, w)
				}
				if dist[w] == dist[v]+1 {
					pathCount[w] += pathCount[v]
				}
			}
		}

		// Back-propagation.
		delta := make(map[string]float64)
		for i := len(stack) - 1; i >= 0; i-- {
			w := stack[i]
			for v := range adj[w] {
				if dist[v] == dist[w]-1 {
					delta[v] += (pathCount[v] / pathCount[w]) * (1.0 + delta[w])
				}
			}
			if w != source {
				bc[w] += delta[w]
			}
		}
	}

	// Normalize.
	n := float64(len(nodes))
	if n > 2 {
		for k := range bc {
			bc[k] /= (n - 1) * (n - 2)
		}
	}

	return bc
}

// computeSizes determines node sizes from centrality metrics.
func computeSizes(nodes map[string]bool, degree map[string]int, betweenness map[string]float64) map[string]float64 {
	maxDeg := 1.0
	maxBet := 0.001

	for _, d := range degree {
		if float64(d) > maxDeg {
			maxDeg = float64(d)
		}
	}
	for _, b := range betweenness {
		if b > maxBet {
			maxBet = b
		}
	}

	sizes := make(map[string]float64)
	for node := range nodes {
		degNorm := float64(degree[node]) / maxDeg
		betNorm := betweenness[node] / maxBet

		importance := 0.6*degNorm + 0.4*betNorm
		sizes[node] = 10 + 20*math.Min(importance, 1.0)
	}

	return sizes
}

func countUnique(m map[string]int) int {
	seen := make(map[int]bool)
	for _, v := range m {
		seen[v] = true
	}
	return len(seen)
}
