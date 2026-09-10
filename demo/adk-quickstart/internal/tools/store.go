package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Store holds the indexed catalog of providers, labs, and models.
type Store struct {
	providers       map[string]RawProvider
	providerList    []ProviderInfo
	labs            map[string]*LabInfo
	labList         []LabInfo
	models          []indexedModel
	modelByID       map[string][]int // model ID (lowercase) -> indices in models
	modelByName     map[string][]int // model Name (lowercase) -> indices in models
	modelsByProvMod map[string]int   // provider:modelID (lowercase) -> index in models
}

type indexedModel struct {
	raw          RawModel
	providerID   string
	providerName string
	lab          string
}

// NewStoreFromJSON parses models.json bytes and indexes providers, labs, and models.
func NewStoreFromJSON(data []byte) (*Store, error) {
	var raw map[string]RawProvider
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal models catalog: %w", err)
	}

	s := &Store{
		providers:       raw,
		labs:            make(map[string]*LabInfo),
		modelByID:       make(map[string][]int),
		modelByName:     make(map[string][]int),
		modelsByProvMod: make(map[string]int),
	}

	familyCountsByLab := make(map[string]map[string]int)

	for pID, p := range raw {
		pName := p.Name
		if pName == "" {
			pName = pID
		}
		pInfo := ProviderInfo{
			ID:          pID,
			Name:        pName,
			API:         p.API,
			Doc:         p.Doc,
			Env:         p.Env,
			ModelsCount: len(p.Models),
		}
		s.providerList = append(s.providerList, pInfo)

		for mID, m := range p.Models {
			lab := extractLab(mID, pID)
			if _, exists := s.labs[lab]; !exists {
				s.labs[lab] = &LabInfo{
					ID:   lab,
					Name: formatLabName(lab),
				}
				familyCountsByLab[lab] = make(map[string]int)
			}
			s.labs[lab].ModelsCount++
			if m.Family != "" {
				familyCountsByLab[lab][m.Family]++
			}

			idx := len(s.models)
			im := indexedModel{
				raw:          m,
				providerID:   pID,
				providerName: pName,
				lab:          lab,
			}
			s.models = append(s.models, im)

			mIDLower := strings.ToLower(mID)
			mNameLower := strings.ToLower(m.Name)
			provModKey := strings.ToLower(pID + ":" + mID)

			s.modelByID[mIDLower] = append(s.modelByID[mIDLower], idx)
			if mNameLower != "" && mNameLower != mIDLower {
				s.modelByName[mNameLower] = append(s.modelByName[mNameLower], idx)
			}
			s.modelsByProvMod[provModKey] = idx
		}
	}

	// Sort providers alphabetically
	sort.Slice(s.providerList, func(i, j int) bool {
		return strings.ToLower(s.providerList[i].Name) < strings.ToLower(s.providerList[j].Name)
	})

	// Build labList with top families, sorted by model count desc
	for labID, labInfo := range s.labs {
		fCounts := familyCountsByLab[labID]
		type fCount struct {
			fam string
			cnt int
		}
		var fList []fCount
		for f, c := range fCounts {
			fList = append(fList, fCount{fam: f, cnt: c})
		}
		sort.Slice(fList, func(i, j int) bool {
			return fList[i].cnt > fList[j].cnt
		})
		for i := 0; i < len(fList) && i < 3; i++ {
			labInfo.TopFamilies = append(labInfo.TopFamilies, fList[i].fam)
		}
		s.labList = append(s.labList, *labInfo)
	}

	sort.Slice(s.labList, func(i, j int) bool {
		if s.labList[i].ModelsCount == s.labList[j].ModelsCount {
			return s.labList[i].ID < s.labList[j].ID
		}
		return s.labList[i].ModelsCount > s.labList[j].ModelsCount
	})

	return s, nil
}

func extractLab(modelID, providerID string) string {
	if slashIdx := strings.Index(modelID, "/"); slashIdx > 0 {
		return strings.ToLower(modelID[:slashIdx])
	}
	return strings.ToLower(providerID)
}

func formatLabName(lab string) string {
	known := map[string]string{
		"openai":      "OpenAI",
		"anthropic":   "Anthropic",
		"google":      "Google",
		"deepseek":    "DeepSeek",
		"deepseek-ai": "DeepSeek",
		"meta":        "Meta",
		"meta-llama":  "Meta (Llama)",
		"mistralai":   "Mistral AI",
		"mistral":     "Mistral AI",
		"moonshotai":  "Moonshot AI (Kimi)",
		"qwen":        "Qwen (Alibaba)",
		"alibaba":     "Alibaba Cloud",
		"minimax":     "MiniMax",
		"cohere":      "Cohere",
		"xai":         "xAI",
		"amazon":      "Amazon",
		"microsoft":   "Microsoft",
	}
	if name, ok := known[strings.ToLower(lab)]; ok {
		return name
	}
	parts := strings.Split(lab, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// ListProviders filters and lists available inference providers.
func (s *Store) ListProviders(in ListProvidersInput) ListProvidersOutput {
	limit := in.Limit
	if limit <= 0 {
		limit = 30
	} else if limit > 100 {
		limit = 100
	}

	q := strings.ToLower(strings.TrimSpace(in.Query))
	var matched []ProviderInfo

	for _, p := range s.providerList {
		if q == "" || strings.Contains(strings.ToLower(p.ID), q) || strings.Contains(strings.ToLower(p.Name), q) {
			matched = append(matched, p)
		}
	}

	total := len(matched)
	if len(matched) > limit {
		matched = matched[:limit]
	}

	return ListProvidersOutput{
		Total:     total,
		Providers: matched,
	}
}

// ListLabs filters and lists model creator labs.
func (s *Store) ListLabs(in ListLabsInput) ListLabsOutput {
	limit := in.Limit
	if limit <= 0 {
		limit = 30
	} else if limit > 100 {
		limit = 100
	}

	q := strings.ToLower(strings.TrimSpace(in.Query))
	var matched []LabInfo

	for _, l := range s.labList {
		if q == "" || strings.Contains(strings.ToLower(l.ID), q) || strings.Contains(strings.ToLower(l.Name), q) {
			matched = append(matched, l)
		}
	}

	total := len(matched)
	if len(matched) > limit {
		matched = matched[:limit]
	}

	return ListLabsOutput{
		Total: total,
		Labs:  matched,
	}
}

// ListModels searches, filters, and summarizes models in the catalog.
func (s *Store) ListModels(in ListModelsInput) ListModelsOutput {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	q := strings.ToLower(strings.TrimSpace(in.Query))
	filterProv := strings.ToLower(strings.TrimSpace(in.Provider))
	filterLab := strings.ToLower(strings.TrimSpace(in.Lab))
	filterFam := strings.ToLower(strings.TrimSpace(in.Family))

	var results []ModelSummary

	// Seen set for deduplication if provider is not specifically filtered
	seen := make(map[string]bool)

	for _, im := range s.models {
		m := im.raw

		if filterProv != "" && strings.ToLower(im.providerID) != filterProv {
			continue
		}
		if filterLab != "" && strings.ToLower(im.lab) != filterLab && !strings.Contains(strings.ToLower(im.lab), filterLab) {
			continue
		}
		if filterFam != "" && !strings.Contains(strings.ToLower(m.Family), filterFam) {
			continue
		}
		if in.Reasoning != nil && m.Reasoning != *in.Reasoning {
			continue
		}
		if in.ToolCall != nil && m.ToolCall != *in.ToolCall {
			continue
		}
		if in.StructuredOutput != nil && m.StructuredOutput != *in.StructuredOutput {
			continue
		}
		if in.OpenWeights != nil && m.OpenWeights != *in.OpenWeights {
			continue
		}
		if in.MinContext > 0 && m.Limit.Context < in.MinContext {
			continue
		}
		if in.MaxInputCost != nil && m.Cost.Input > *in.MaxInputCost {
			continue
		}

		if q != "" {
			mID := strings.ToLower(m.ID)
			mName := strings.ToLower(m.Name)
			mDesc := strings.ToLower(m.Description)
			if !strings.Contains(mID, q) && !strings.Contains(mName, q) && !strings.Contains(mDesc, q) && !strings.Contains(strings.ToLower(m.Family), q) {
				continue
			}
		}

		// Deduplicate same model across multiple aggregators if no provider filter was specified
		dedupKey := im.lab + ":" + m.ID
		if filterProv == "" {
			if seen[dedupKey] {
				continue
			}
			seen[dedupKey] = true
		}

		summary := ModelSummary{
			ID:               m.ID,
			Name:             m.Name,
			Lab:              im.lab,
			ProviderID:       im.providerID,
			ProviderName:     im.providerName,
			Family:           m.Family,
			ContextTokens:    m.Limit.Context,
			OutputTokens:     m.Limit.Output,
			CostInput:        m.Cost.Input,
			CostOutput:       m.Cost.Output,
			Reasoning:        m.Reasoning,
			ToolCall:         m.ToolCall,
			StructuredOutput: m.StructuredOutput,
			OpenWeights:      m.OpenWeights,
			Description:      m.Description,
		}
		results = append(results, summary)
	}

	total := len(results)
	if len(results) > limit {
		results = results[:limit]
	}

	return ListModelsOutput{
		Total:  total,
		Models: results,
	}
}

// GetModelDetails returns detailed specs for a specific model ID or name.
func (s *Store) GetModelDetails(in GetModelDetailsInput) (*ModelDetails, error) {
	reqID := strings.ToLower(strings.TrimSpace(in.ModelID))
	if reqID == "" {
		return nil, fmt.Errorf("model_id is required")
	}

	reqProv := strings.ToLower(strings.TrimSpace(in.Provider))

	// 1. Direct provider:model lookup
	if reqProv != "" {
		key := reqProv + ":" + reqID
		if idx, ok := s.modelsByProvMod[key]; ok {
			return s.toModelDetails(s.models[idx]), nil
		}
	}

	// 2. Exact Model ID lookup
	if indices, ok := s.modelByID[reqID]; ok && len(indices) > 0 {
		return s.toModelDetails(s.models[indices[0]]), nil
	}

	// 3. Name lookup
	if indices, ok := s.modelByName[reqID]; ok && len(indices) > 0 {
		return s.toModelDetails(s.models[indices[0]]), nil
	}

	// 4. Substring / suffix lookup (e.g. 'gpt-5.5' matching 'openai/gpt-5.5' or 'claude-opus-4.7')
	for _, im := range s.models {
		mID := strings.ToLower(im.raw.ID)
		mName := strings.ToLower(im.raw.Name)
		if strings.HasSuffix(mID, "/"+reqID) || strings.EqualFold(mID, reqID) || strings.EqualFold(mName, reqID) || strings.Contains(mID, reqID) {
			if reqProv == "" || strings.EqualFold(im.providerID, reqProv) {
				return s.toModelDetails(im), nil
			}
		}
	}

	return nil, fmt.Errorf("model %q not found in catalog (try listing models with list_models)", in.ModelID)
}

func (s *Store) toModelDetails(im indexedModel) *ModelDetails {
	m := im.raw
	p := s.providers[im.providerID]

	return &ModelDetails{
		ID:               m.ID,
		Name:             m.Name,
		Lab:              im.lab,
		ProviderID:       im.providerID,
		ProviderName:     im.providerName,
		ProviderAPI:      p.API,
		ProviderDoc:      p.Doc,
		Description:      m.Description,
		Family:           m.Family,
		Reasoning:        m.Reasoning,
		ReasoningOptions: m.ReasoningOptions,
		ToolCall:         m.ToolCall,
		StructuredOutput: m.StructuredOutput,
		OpenWeights:      m.OpenWeights,
		Modalities:       m.Modalities,
		Limit:            m.Limit,
		Cost:             m.Cost,
		KnowledgeCutoff:  m.Knowledge,
		ReleaseDate:      m.ReleaseDate,
		LastUpdated:      m.LastUpdated,
		Status:           m.Status,
	}
}

// RecommendModels suggests best-fit models for a task based on requirements and heuristics.
func (s *Store) RecommendModels(in RecommendModelsInput) RecommendModelsOutput {
	limit := in.Limit
	if limit <= 0 {
		limit = 5
	} else if limit > 20 {
		limit = 20
	}

	taskLower := strings.ToLower(in.Task)
	isCoding := strings.Contains(taskLower, "code") || strings.Contains(taskLower, "coding") || strings.Contains(taskLower, "dev") || strings.Contains(taskLower, "software")
	isReasoning := in.RequireReasoning || strings.Contains(taskLower, "reason") || strings.Contains(taskLower, "math") || strings.Contains(taskLower, "complex") || strings.Contains(taskLower, "thinking")
	isAgent := in.RequireToolCall || strings.Contains(taskLower, "agent") || strings.Contains(taskLower, "tool") || strings.Contains(taskLower, "function")
	isVision := in.RequireVision || strings.Contains(taskLower, "vision") || strings.Contains(taskLower, "image") || strings.Contains(taskLower, "multimodal")
	isCheap := strings.Contains(taskLower, "cheap") || strings.Contains(taskLower, "economy") || strings.Contains(taskLower, "budget") || strings.Contains(taskLower, "low cost") || strings.Contains(taskLower, "scale")
	isLongCtx := in.MinContext >= 200000 || strings.Contains(taskLower, "long context") || strings.Contains(taskLower, "document") || strings.Contains(taskLower, "book") || strings.Contains(taskLower, "million")

	type candidate struct {
		im        indexedModel
		score     float64
		fitReason string
	}

	var candidates []candidate
	seen := make(map[string]bool)

	for _, im := range s.models {
		m := im.raw

		// Hard filters
		if in.MinContext > 0 && m.Limit.Context < in.MinContext {
			continue
		}
		if in.MaxInputCost != nil && m.Cost.Input > *in.MaxInputCost {
			continue
		}
		if in.RequireReasoning && !m.Reasoning {
			continue
		}
		if in.RequireToolCall && !m.ToolCall {
			continue
		}
		if in.RequireOpenWeights && !m.OpenWeights {
			continue
		}
		if in.RequireVision {
			hasVision := false
			for _, mod := range m.Modalities.Input {
				if mod == "image" || mod == "vision" {
					hasVision = true
					break
				}
			}
			if !hasVision {
				continue
			}
		}

		// Deduplicate identical model IDs across multiple provider endpoints
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true

		score := 10.0
		var reasons []string

		// Reasoning relevance
		if m.Reasoning {
			if isReasoning {
				score += 25
				reasons = append(reasons, "advanced reasoning/thinking")
			} else {
				score += 5
			}
		}

		// Tool calling relevance
		if m.ToolCall {
			if isAgent {
				score += 20
				reasons = append(reasons, "reliable tool calling")
			} else {
				score += 5
			}
		}

		// Coding relevance
		descLower := strings.ToLower(m.Description + " " + m.Name + " " + m.Family)
		if isCoding && (strings.Contains(descLower, "code") || strings.Contains(descLower, "coding")) {
			score += 20
			reasons = append(reasons, "optimized for coding & software workflows")
		}

		// Vision support
		for _, mod := range m.Modalities.Input {
			if mod == "image" || mod == "vision" {
				if isVision {
					score += 20
					reasons = append(reasons, "multimodal/vision input support")
				}
				break
			}
		}

		// Open weights
		if m.OpenWeights {
			if in.RequireOpenWeights || strings.Contains(taskLower, "open") || strings.Contains(taskLower, "local") {
				score += 25
				reasons = append(reasons, "open weights architecture")
			}
		}

		// Long context
		if m.Limit.Context >= 1000000 {
			if isLongCtx {
				score += 25
				reasons = append(reasons, fmt.Sprintf("1M+ token context (%d tokens)", m.Limit.Context))
			} else {
				score += 5
			}
		} else if m.Limit.Context >= 200000 {
			if isLongCtx {
				score += 15
				reasons = append(reasons, fmt.Sprintf("large context window (%d tokens)", m.Limit.Context))
			}
		}

		// Cost efficiency
		if m.Cost.Input > 0 {
			if isCheap {
				if m.Cost.Input <= 0.5 {
					score += 30
					reasons = append(reasons, fmt.Sprintf("ultra-low cost ($%.2f/1M tokens)", m.Cost.Input))
				} else if m.Cost.Input <= 2.0 {
					score += 15
					reasons = append(reasons, fmt.Sprintf("economical ($%.2f/1M tokens)", m.Cost.Input))
				}
			} else {
				// Gentle penalty for very expensive models if not needed
				if m.Cost.Input > 10.0 && !isReasoning {
					score -= 5
				}
			}
		}

		// Baseline reason if none matched specifically
		if len(reasons) == 0 {
			reasons = append(reasons, "strong general-purpose performance")
		}

		candidates = append(candidates, candidate{
			im:        im,
			score:     score,
			fitReason: strings.Join(reasons, ", "),
		})
	}

	// Sort candidates by score desc
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].im.raw.Limit.Context > candidates[j].im.raw.Limit.Context
		}
		return candidates[i].score > candidates[j].score
	})

	var recs []ModelRecommendation
	for i := 0; i < len(candidates) && i < limit; i++ {
		c := candidates[i]
		m := c.im.raw
		recs = append(recs, ModelRecommendation{
			ModelID:       m.ID,
			ModelName:     m.Name,
			Lab:           c.im.lab,
			ProviderName:  c.im.providerName,
			ContextTokens: m.Limit.Context,
			CostInput:     m.Cost.Input,
			CostOutput:    m.Cost.Output,
			Reasoning:     m.Reasoning,
			ToolCall:      m.ToolCall,
			OpenWeights:   m.OpenWeights,
			FitReason:     c.fitReason,
			Score:         c.score,
		})
	}

	return RecommendModelsOutput{
		Task:            in.Task,
		Recommendations: recs,
	}
}
