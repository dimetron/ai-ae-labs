// Package inference discovers additional relationships between entities.
package inference

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/dimetron/ai-knowledge-graph-go/internal/config"
	"github.com/dimetron/ai-knowledge-graph-go/internal/domain"
	"github.com/dimetron/ai-knowledge-graph-go/internal/extractor"
)

// LLMInferrer provides LLM-based relationship inference.
type LLMInferrer interface {
	InferRelationships(ctx context.Context, entities1, entities2, triplesText string) ([]domain.Triple, error)
	InferWithinCommunity(ctx context.Context, pairsText, triplesText string) ([]domain.Triple, error)
}

// Infer discovers additional relationships between entities.
func Infer(ctx context.Context, triples []domain.Triple, cfg *config.Config, llm LLMInferrer) []domain.Triple {
	if len(triples) < 2 {
		return triples
	}

	slog.Info("inferring relationships", "triples", len(triples))

	// Validate triples.
	var valid []domain.Triple
	for _, t := range triples {
		if t.Subject != "" && t.Predicate != "" && t.Object != "" {
			valid = append(valid, t)
		}
	}

	// Build adjacency graph.
	graph := make(map[string]map[string]bool)
	allEntities := make(map[string]bool)
	for _, t := range valid {
		if graph[t.Subject] == nil {
			graph[t.Subject] = make(map[string]bool)
		}
		graph[t.Subject][t.Object] = true
		allEntities[t.Subject] = true
		allEntities[t.Object] = true
	}

	// Find disconnected communities.
	communities := identifyCommunities(graph)
	slog.Info("identified communities", "count", len(communities))

	var newTriples []domain.Triple

	// LLM-based inference between and within communities.
	if cfg.Inference.UseLLMForInference && llm != nil {
		cross := inferBetweenCommunities(ctx, valid, communities, llm)
		newTriples = append(newTriples, cross...)

		within := inferWithinCommunities(ctx, valid, communities, llm)
		newTriples = append(newTriples, within...)
	}

	// Transitive inference.
	if cfg.Inference.ApplyTransitive {
		transitive := applyTransitive(valid, graph)
		newTriples = append(newTriples, transitive...)
	}

	// Lexical similarity inference.
	lexical := inferByLexical(allEntities, valid)
	newTriples = append(newTriples, lexical...)

	// Merge and deduplicate.
	all := append(valid, newTriples...)
	unique := deduplicate(all)

	// Enforce predicate length.
	for i := range unique {
		unique[i].Predicate = extractor.LimitPredicateLength(unique[i].Predicate, 3)
	}

	// Remove self-references.
	var filtered []domain.Triple
	for _, t := range unique {
		if t.Subject != t.Object {
			filtered = append(filtered, t)
		}
	}

	slog.Info("inference complete",
		"added", len(filtered)-len(triples),
		"total", len(filtered),
	)

	return filtered
}

// identifyCommunities finds connected components via DFS.
func identifyCommunities(graph map[string]map[string]bool) []map[string]bool {
	// Gather all nodes.
	allNodes := make(map[string]bool)
	for src, targets := range graph {
		allNodes[src] = true
		for t := range targets {
			allNodes[t] = true
		}
	}

	// Build undirected adjacency.
	adj := make(map[string]map[string]bool)
	for src, targets := range graph {
		if adj[src] == nil {
			adj[src] = make(map[string]bool)
		}
		for t := range targets {
			adj[src][t] = true
			if adj[t] == nil {
				adj[t] = make(map[string]bool)
			}
			adj[t][src] = true
		}
	}

	visited := make(map[string]bool)
	var communities []map[string]bool

	for node := range allNodes {
		if visited[node] {
			continue
		}
		community := make(map[string]bool)
		var stack []string
		stack = append(stack, node)

		for len(stack) > 0 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			if visited[n] {
				continue
			}
			visited[n] = true
			community[n] = true

			for neighbor := range adj[n] {
				if !visited[neighbor] {
					stack = append(stack, neighbor)
				}
			}
		}

		communities = append(communities, community)
	}

	return communities
}

// inferBetweenCommunities uses the LLM to find cross-community relationships.
func inferBetweenCommunities(ctx context.Context, triples []domain.Triple, communities []map[string]bool, llm LLMInferrer) []domain.Triple {
	if len(communities) <= 1 {
		return nil
	}

	// Sort communities by size descending, keep top 5.
	sort.Slice(communities, func(i, j int) bool {
		return len(communities[i]) > len(communities[j])
	})
	top := communities
	if len(top) > 5 {
		top = top[:5]
	}

	var result []domain.Triple

	for i, c1 := range top {
		for j, c2 := range top {
			if i >= j {
				continue
			}

			rep1 := topN(c1, 5)
			rep2 := topN(c2, 5)

			// Collect context triples.
			var contextTriples []domain.Triple
			for _, t := range triples {
				if c1[t.Subject] || c1[t.Object] || c2[t.Subject] || c2[t.Object] {
					contextTriples = append(contextTriples, t)
				}
			}
			if len(contextTriples) > 20 {
				contextTriples = contextTriples[:20]
			}

			triplesText := formatTriples(contextTriples)
			inferred, err := llm.InferRelationships(ctx, strings.Join(rep1, ", "), strings.Join(rep2, ", "), triplesText)
			if err != nil {
				slog.Warn("cross-community inference failed", "error", err)
				continue
			}

			result = append(result, inferred...)
		}
	}

	slog.Info("cross-community inference", "new_triples", len(result))
	return result
}

// inferWithinCommunities uses the LLM to find intra-community relationships.
func inferWithinCommunities(ctx context.Context, triples []domain.Triple, communities []map[string]bool, llm LLMInferrer) []domain.Triple {
	var result []domain.Triple

	for _, comm := range communities {
		if len(comm) < 5 {
			continue
		}

		entities := make([]string, 0, len(comm))
		for e := range comm {
			entities = append(entities, e)
		}

		// Find existing connections.
		connected := make(map[[2]string]bool)
		for _, t := range triples {
			if comm[t.Subject] && comm[t.Object] {
				connected[[2]string{t.Subject, t.Object}] = true
			}
		}

		// Find disconnected pairs with lexical similarity.
		var pairs [][2]string
		for i, a := range entities {
			for _, b := range entities[i+1:] {
				if connected[[2]string{a, b}] || connected[[2]string{b, a}] {
					continue
				}
				aWords := strings.Fields(strings.ToLower(a))
				bWords := strings.Fields(strings.ToLower(b))
				if hasShared(aWords, bWords) || strings.Contains(strings.ToLower(a), strings.ToLower(b)) || strings.Contains(strings.ToLower(b), strings.ToLower(a)) {
					pairs = append(pairs, [2]string{a, b})
				}
			}
		}

		if len(pairs) == 0 {
			continue
		}
		if len(pairs) > 10 {
			pairs = pairs[:10]
		}

		// Build context.
		interest := make(map[string]bool)
		for _, p := range pairs {
			interest[p[0]] = true
			interest[p[1]] = true
		}

		var contextTriples []domain.Triple
		for _, t := range triples {
			if interest[t.Subject] || interest[t.Object] {
				contextTriples = append(contextTriples, t)
			}
		}
		if len(contextTriples) > 20 {
			contextTriples = contextTriples[:20]
		}

		var pairLines []string
		for _, p := range pairs {
			pairLines = append(pairLines, p[0]+" and "+p[1])
		}

		inferred, err := llm.InferWithinCommunity(ctx, strings.Join(pairLines, "\n"), formatTriples(contextTriples))
		if err != nil {
			slog.Warn("within-community inference failed", "error", err)
			continue
		}

		result = append(result, inferred...)
	}

	slog.Info("within-community inference", "new_triples", len(result))
	return result
}

// applyTransitive finds A->B->C implies A->C relationships.
func applyTransitive(triples []domain.Triple, graph map[string]map[string]bool) []domain.Triple {
	predicates := make(map[[2]string]string)
	for _, t := range triples {
		predicates[[2]string{t.Subject, t.Object}] = t.Predicate
	}

	var result []domain.Triple

	for subj, targets := range graph {
		for mid := range targets {
			for obj := range graph[mid] {
				if subj == obj {
					continue
				}
				if _, exists := predicates[[2]string{subj, obj}]; exists {
					continue
				}

				pred1 := predicates[[2]string{subj, mid}]
				pred2 := predicates[[2]string{mid, obj}]
				if pred1 == "" {
					pred1 = "relates to"
				}
				if pred2 == "" {
					pred2 = "relates to"
				}

				var newPred string
				if pred1 == pred2 {
					newPred = "indirectly " + pred1
				} else {
					newPred = fmt.Sprintf("%s via %s", pred1, mid)
				}

				result = append(result, domain.Triple{
					Subject:   subj,
					Predicate: extractor.LimitPredicateLength(newPred, 3),
					Object:    obj,
					Inferred:  true,
				})
			}
		}
	}

	slog.Info("transitive inference", "new_triples", len(result))
	return result
}

// inferByLexical finds relationships based on shared words or containment.
func inferByLexical(entities map[string]bool, triples []domain.Triple) []domain.Triple {
	existing := make(map[[2]string]bool)
	for _, t := range triples {
		existing[[2]string{t.Subject, t.Object}] = true
	}

	var result []domain.Triple
	processed := make(map[[2]string]bool)

	entityList := make([]string, 0, len(entities))
	for e := range entities {
		entityList = append(entityList, e)
	}
	sort.Strings(entityList)

	for i, e1 := range entityList {
		for _, e2 := range entityList[i+1:] {
			if e1 == e2 {
				continue
			}
			if existing[[2]string{e1, e2}] || existing[[2]string{e2, e1}] {
				continue
			}
			pair := [2]string{e1, e2}
			if processed[pair] {
				continue
			}
			processed[pair] = true

			e1l := strings.ToLower(e1)
			e2l := strings.ToLower(e2)
			w1 := strings.Fields(e1l)
			w2 := strings.Fields(e2l)

			shared := sharedWords(w1, w2)
			if len(shared) > 0 {
				main := longestWord(shared)
				if len(main) >= 4 {
					result = append(result, domain.Triple{
						Subject:   e1,
						Predicate: "related to",
						Object:    e2,
						Inferred:  true,
					})
				}
			} else if strings.Contains(e1l, e2l) {
				result = append(result, domain.Triple{
					Subject:   e1,
					Predicate: "is type of",
					Object:    e2,
					Inferred:  true,
				})
			} else if strings.Contains(e2l, e1l) {
				result = append(result, domain.Triple{
					Subject:   e2,
					Predicate: "is type of",
					Object:    e1,
					Inferred:  true,
				})
			}
		}
	}

	slog.Info("lexical inference", "new_triples", len(result))
	return result
}

// deduplicate removes duplicate triples, preferring originals over inferred.
func deduplicate(triples []domain.Triple) []domain.Triple {
	unique := make(map[[3]string]domain.Triple)
	for _, t := range triples {
		key := [3]string{t.Subject, t.Predicate, t.Object}
		if existing, ok := unique[key]; ok {
			if t.Inferred && !existing.Inferred {
				continue // Keep the original.
			}
		}
		unique[key] = t
	}

	result := make([]domain.Triple, 0, len(unique))
	for _, t := range unique {
		result = append(result, t)
	}
	return result
}

func topN(community map[string]bool, n int) []string {
	var items []string
	for e := range community {
		items = append(items, e)
	}
	if len(items) > n {
		items = items[:n]
	}
	return items
}

func formatTriples(triples []domain.Triple) string {
	var lines []string
	for _, t := range triples {
		lines = append(lines, fmt.Sprintf("%s %s %s", t.Subject, t.Predicate, t.Object))
	}
	return strings.Join(lines, "\n")
}

func hasShared(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, w := range a {
		set[w] = true
	}
	for _, w := range b {
		if set[w] {
			return true
		}
	}
	return false
}

func sharedWords(a, b []string) []string {
	set := make(map[string]bool, len(a))
	for _, w := range a {
		set[w] = true
	}
	var shared []string
	for _, w := range b {
		if set[w] {
			shared = append(shared, w)
		}
	}
	return shared
}

func longestWord(words []string) string {
	longest := ""
	for _, w := range words {
		if len(w) > len(longest) {
			longest = w
		}
	}
	return longest
}
