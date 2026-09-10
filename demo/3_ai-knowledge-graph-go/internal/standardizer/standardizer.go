// Package standardizer normalizes entity names across triples.
package standardizer

import (
	"context"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/dimetron/ai-knowledge-graph-go/internal/config"
	"github.com/dimetron/ai-knowledge-graph-go/internal/domain"
	"github.com/dimetron/ai-knowledge-graph-go/internal/extractor"
)

// LLMResolver resolves entities using an LLM.
type LLMResolver interface {
	ResolveEntities(ctx context.Context, entityList string) (map[string][]string, error)
}

// Standardize normalizes entity names across all triples.
func Standardize(ctx context.Context, triples []domain.Triple, cfg *config.Config, llm LLMResolver) []domain.Triple {
	if len(triples) == 0 {
		return triples
	}

	slog.Info("standardizing entity names", "triples", len(triples))

	// Validate triples.
	var valid []domain.Triple
	for _, t := range triples {
		if t.Subject != "" && t.Predicate != "" && t.Object != "" {
			valid = append(valid, t)
		}
	}

	if len(valid) == 0 {
		slog.Warn("no valid triples for standardization")
		return nil
	}

	// Extract unique entities.
	allEntities := make(map[string]bool)
	for _, t := range valid {
		allEntities[strings.ToLower(t.Subject)] = true
		allEntities[strings.ToLower(t.Object)] = true
	}

	// Group similar entities by normalized form.
	entityGroups := make(map[string][]string)
	for entity := range allEntities {
		norm := normalize(entity)
		if norm != "" {
			entityGroups[norm] = append(entityGroups[norm], entity)
		}
	}

	// Choose a standard form for each group.
	stdMap := make(map[string]string)
	for _, variants := range entityGroups {
		if len(variants) == 1 {
			stdMap[variants[0]] = variants[0]
			continue
		}

		// Count usage frequency.
		freq := make(map[string]int)
		for _, t := range valid {
			for _, v := range variants {
				if strings.ToLower(t.Subject) == v {
					freq[v]++
				}
				if strings.ToLower(t.Object) == v {
					freq[v]++
				}
			}
		}

		// Pick the most frequent, then shortest.
		sort.Slice(variants, func(i, j int) bool {
			if freq[variants[i]] != freq[variants[j]] {
				return freq[variants[i]] > freq[variants[j]]
			}
			return len(variants[i]) < len(variants[j])
		})

		standard := variants[0]
		for _, v := range variants {
			stdMap[v] = standard
		}
	}

	// Second pass: root-word relationships.
	stdForms := uniqueValues(stdMap)
	sort.Slice(stdForms, func(i, j int) bool { return len(stdForms[i]) < len(stdForms[j]) })

	for i, e1 := range stdForms {
		w1 := strings.Fields(e1)
		ws1 := toSet(w1)

		for _, e2 := range stdForms[i+1:] {
			if e1 == e2 {
				continue
			}
			w2 := strings.Fields(e2)
			ws2 := toSet(w2)

			if isSubset(ws1, ws2) && len(ws1) > 0 {
				stdMap[e2] = e1
			} else if isSubset(ws2, ws1) && len(ws2) > 0 {
				stdMap[e1] = e2
			} else {
				stems1 := stemSet(ws1)
				stems2 := stemSet(ws2)
				shared := intersection(stems1, stems2)
				maxLen := max(len(stems1), len(stems2))
				if maxLen > 0 && len(shared) > 0 && float64(len(shared))/float64(maxLen) > 0.5 {
					if len(e1) <= len(e2) {
						stdMap[e2] = e1
					} else {
						stdMap[e1] = e2
					}
				}
			}
		}
	}

	// Apply standardization.
	var result []domain.Triple
	for _, t := range valid {
		subj := stdMap[strings.ToLower(t.Subject)]
		if subj == "" {
			subj = t.Subject
		}
		obj := stdMap[strings.ToLower(t.Object)]
		if obj == "" {
			obj = t.Object
		}

		result = append(result, domain.Triple{
			Subject:   subj,
			Predicate: extractor.LimitPredicateLength(t.Predicate, 3),
			Object:    obj,
			Chunk:     t.Chunk,
			Inferred:  t.Inferred,
		})
	}

	// Optional LLM-based entity resolution.
	if cfg.Standardization.UseLLMForEntities && llm != nil {
		result = resolvWithLLM(ctx, result, llm)
	}

	// Remove self-referencing triples.
	var filtered []domain.Triple
	for _, t := range result {
		if t.Subject != t.Object {
			filtered = append(filtered, t)
		}
	}

	slog.Info("standardization complete",
		"original", len(allEntities),
		"standard_forms", len(uniqueValues(stdMap)),
		"self_refs_removed", len(result)-len(filtered),
	)

	return filtered
}

// resolvWithLLM uses an LLM to further standardize entities.
func resolvWithLLM(ctx context.Context, triples []domain.Triple, llm LLMResolver) []domain.Triple {
	entities := make(map[string]bool)
	for _, t := range triples {
		entities[t.Subject] = true
		entities[t.Object] = true
	}

	// Limit to top 100.
	sorted := make([]string, 0, len(entities))
	for e := range entities {
		sorted = append(sorted, e)
	}
	sort.Strings(sorted)
	if len(sorted) > 100 {
		sorted = sorted[:100]
	}

	entityList := strings.Join(sorted, "\n")
	mapping, err := llm.ResolveEntities(ctx, entityList)
	if err != nil {
		slog.Warn("LLM entity resolution failed", "error", err)
		return triples
	}

	if mapping == nil {
		return triples
	}

	// Build reverse mapping.
	toStd := make(map[string]string)
	for standard, variants := range mapping {
		toStd[standard] = standard
		for _, v := range variants {
			toStd[v] = standard
		}
	}

	for i := range triples {
		if s, ok := toStd[triples[i].Subject]; ok {
			triples[i].Subject = s
		}
		if o, ok := toStd[triples[i].Object]; ok {
			triples[i].Object = o
		}
	}

	slog.Info("LLM entity resolution applied", "groups", len(mapping))
	return triples
}

// normalize strips stopwords and lowercases.
func normalize(text string) string {
	text = strings.ToLower(text)
	stop := map[string]bool{
		"the": true, "a": true, "an": true, "of": true, "and": true,
		"or": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "with": true, "by": true, "as": true,
	}
	re := regexp.MustCompile(`\b\w+\b`)
	words := re.FindAllString(text, -1)
	var kept []string
	for _, w := range words {
		if !stop[w] {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " ")
}

func uniqueValues(m map[string]string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range m {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func toSet(words []string) map[string]bool {
	s := make(map[string]bool, len(words))
	for _, w := range words {
		s[w] = true
	}
	return s
}

func isSubset(a, b map[string]bool) bool {
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func stemSet(words map[string]bool) map[string]bool {
	s := make(map[string]bool)
	for w := range words {
		if len(w) > 4 {
			s[w[:4]] = true
		}
	}
	return s
}

func intersection(a, b map[string]bool) map[string]bool {
	r := make(map[string]bool)
	for k := range a {
		if b[k] {
			r[k] = true
		}
	}
	return r
}

// max is a builtin since Go 1.21; no local definition needed.
