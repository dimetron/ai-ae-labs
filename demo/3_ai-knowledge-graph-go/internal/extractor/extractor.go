// Package extractor uses an LLM to extract subject-predicate-object triples from text.
package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/dimetron/ai-knowledge-graph-go/internal/config"
	"github.com/dimetron/ai-knowledge-graph-go/internal/domain"
	"github.com/dimetron/ai-knowledge-graph-go/internal/prompts"
)

// Extractor calls an OpenAI-compatible LLM to extract triples from text.
type Extractor struct {
	cfg    config.LLMConfig
	client *http.Client
}

// New creates an Extractor with the given LLM configuration.
func New(cfg config.LLMConfig) *Extractor {
	return &Extractor{
		cfg:    cfg,
		client: &http.Client{},
	}
}

// chatRequest is the OpenAI-compatible chat completion request body.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Extract sends text to the LLM and returns parsed triples.
func (e *Extractor) Extract(ctx context.Context, text string) ([]domain.Triple, error) {
	systemPrompt := prompts.MainSystem()
	userPrompt := prompts.MainUser(text)

	raw, err := e.callLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("extract triples: %w", err)
	}

	triples, err := parseTriples(raw)
	if err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}

	// Validate and limit predicate length.
	var valid []domain.Triple
	for _, t := range triples {
		if t.Subject == "" || t.Predicate == "" || t.Object == "" {
			continue
		}
		t.Predicate = LimitPredicateLength(t.Predicate, 3)
		valid = append(valid, t)
	}

	return valid, nil
}

// ResolveEntities asks the LLM to standardize entity names.
func (e *Extractor) ResolveEntities(ctx context.Context, entityList string) (map[string][]string, error) {
	systemPrompt := prompts.EntityResolutionSystem()
	userPrompt := prompts.EntityResolutionUser(entityList)

	raw, err := e.callLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("resolve entities: %w", err)
	}

	cleaned := extractJSONObject(raw)
	var mapping map[string][]string
	if err := json.Unmarshal([]byte(cleaned), &mapping); err != nil {
		return nil, fmt.Errorf("parse entity mapping: %w", err)
	}

	return mapping, nil
}

// InferRelationships asks the LLM to infer cross-community relationships.
func (e *Extractor) InferRelationships(ctx context.Context, entities1, entities2, triplesText string) ([]domain.Triple, error) {
	systemPrompt := prompts.RelationshipInferenceSystem()
	userPrompt := prompts.RelationshipInferenceUser(entities1, entities2, triplesText)

	raw, err := e.callLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("infer relationships: %w", err)
	}

	triples, err := parseTriples(raw)
	if err != nil {
		return nil, fmt.Errorf("parse inferred triples: %w", err)
	}

	for i := range triples {
		triples[i].Inferred = true
		triples[i].Predicate = LimitPredicateLength(triples[i].Predicate, 3)
	}

	return triples, nil
}

// InferWithinCommunity asks the LLM to infer relationships within a community.
func (e *Extractor) InferWithinCommunity(ctx context.Context, pairsText, triplesText string) ([]domain.Triple, error) {
	systemPrompt := prompts.WithinCommunitySystem()
	userPrompt := prompts.WithinCommunityUser(pairsText, triplesText)

	raw, err := e.callLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("infer within community: %w", err)
	}

	triples, err := parseTriples(raw)
	if err != nil {
		return nil, fmt.Errorf("parse within-community triples: %w", err)
	}

	for i := range triples {
		triples[i].Inferred = true
		triples[i].Predicate = LimitPredicateLength(triples[i].Predicate, 3)
	}

	return triples, nil
}

// callLLM sends a chat completion request to the OpenAI-compatible API.
func (e *Extractor) callLLM(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody := chatRequest{
		Model:       e.cfg.Model,
		Messages:    messages,
		MaxTokens:   e.cfg.MaxTokens,
		Temperature: e.cfg.Temperature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.BaseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// parseTriples extracts a JSON array of triples from LLM text output.
func parseTriples(text string) ([]domain.Triple, error) {
	cleaned := extractJSONArray(text)
	if cleaned == "" {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	var triples []domain.Triple
	if err := json.Unmarshal([]byte(cleaned), &triples); err != nil {
		// Try fixing common issues.
		fixed := fixJSON(cleaned)
		if err2 := json.Unmarshal([]byte(fixed), &triples); err2 != nil {
			return nil, fmt.Errorf("parse JSON: %w (original: %w)", err2, err)
		}
	}

	return triples, nil
}

var codeBlockRE = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")

// extractJSONArray finds a JSON array in text, handling code blocks.
func extractJSONArray(text string) string {
	// Check for code blocks first.
	if m := codeBlockRE.FindStringSubmatch(text); len(m) > 1 {
		text = strings.TrimSpace(m[1])
	}

	// Try direct parse.
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "[") {
		return text
	}

	// Find array bounds by bracket counting.
	start := strings.Index(text, "[")
	if start == -1 {
		return ""
	}

	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}

	// Incomplete array — try to reconstruct from complete objects.
	return reconstructArray(text[start:])
}

// extractJSONObject finds a JSON object in text.
func extractJSONObject(text string) string {
	if m := codeBlockRE.FindStringSubmatch(text); len(m) > 1 {
		text = strings.TrimSpace(m[1])
	}

	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "{") {
		return text
	}

	start := strings.Index(text, "{")
	if start == -1 {
		return text
	}

	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}

	return text[start:]
}

// reconstructArray attempts to build a valid JSON array from complete objects.
func reconstructArray(text string) string {
	var objects []string
	depth := 0
	objStart := -1

	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '{':
			if depth == 0 {
				objStart = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && objStart >= 0 {
				objects = append(objects, text[objStart:i+1])
				objStart = -1
			}
		}
	}

	if len(objects) == 0 {
		return ""
	}

	return "[\n" + strings.Join(objects, ",\n") + "\n]"
}

// fixJSON attempts common JSON fixes: unquoted keys, trailing commas.
func fixJSON(text string) string {
	// Fix unquoted keys.
	re := regexp.MustCompile(`(\s*)(\w+)(\s*):(\s*)`)
	text = re.ReplaceAllString(text, `$1"$2"$3:$4`)

	// Fix trailing commas before ] or }.
	re2 := regexp.MustCompile(`,(\s*[}\]])`)
	text = re2.ReplaceAllString(text, `$1`)

	return text
}

// LimitPredicateLength enforces a maximum word limit on predicates.
func LimitPredicateLength(predicate string, maxWords int) string {
	words := strings.Fields(predicate)
	if len(words) <= maxWords {
		return predicate
	}

	shortened := strings.Join(words[:maxWords], " ")

	// Remove trailing stop words.
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "of": true, "with": true,
		"by": true, "to": true, "from": true, "in": true, "on": true, "for": true,
	}

	parts := strings.Fields(shortened)
	if len(parts) > 1 && stopWords[strings.ToLower(parts[len(parts)-1])] {
		shortened = strings.Join(parts[:len(parts)-1], " ")
	}

	slog.Debug("truncated predicate", "original", predicate, "shortened", shortened)
	return shortened
}
