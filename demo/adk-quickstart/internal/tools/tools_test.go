package tools

import (
	"strings"
	"testing"
)

func TestAll(t *testing.T) {
	all := All()
	if len(all) != 6 {
		t.Fatalf("expected 6 tools from tools.All(), got %d", len(all))
	}

	names := make(map[string]bool)
	for _, tool := range all {
		names[tool.Name()] = true
	}

	expected := []string{"list_models", "list_providers", "list_labs", "get_model_details", "recommend_models", "list_course_models"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestCourseModels(t *testing.T) {
	doc := CourseModels()
	if len(doc.Models) < 7 {
		t.Fatalf("expected at least 7 course models, got %d", len(doc.Models))
	}
	if doc.DefaultModel != "gemini-3.7-flash" {
		t.Errorf("expected default model gemini-3.7-flash, got %s", doc.DefaultModel)
	}

	foundGemini := false
	foundSonnet := false
	foundDeepSeek := false
	for _, m := range doc.Models {
		if m.ID == "gemini-3.7-flash" {
			foundGemini = true
		}
		if m.ID == "claude-sonnet-5" {
			foundSonnet = true
		}
		if strings.Contains(m.ID, "deepseek-v4-flash") {
			foundDeepSeek = true
		}
	}
	if !foundGemini || !foundSonnet || !foundDeepSeek {
		t.Errorf("missing expected course models: gemini=%v sonnet=%v deepseek=%v", foundGemini, foundSonnet, foundDeepSeek)
	}
}

func TestStore_ListProviders(t *testing.T) {
	store := DefaultStore()
	if store == nil {
		t.Fatal("DefaultStore() returned nil")
	}

	all := store.ListProviders(ListProvidersInput{Limit: 50})
	if all.Total == 0 || len(all.Providers) == 0 {
		t.Fatalf("expected providers, got 0")
	}

	filtered := store.ListProviders(ListProvidersInput{Query: "open", Limit: 10})
	if filtered.Total == 0 {
		t.Errorf("expected providers matching 'open', got 0")
	}
}

func TestStore_ListLabs(t *testing.T) {
	store := DefaultStore()

	labs := store.ListLabs(ListLabsInput{Limit: 20})
	if labs.Total == 0 || len(labs.Labs) == 0 {
		t.Fatalf("expected labs, got 0")
	}

	foundOpenAI := false
	foundAnthropic := false
	for _, l := range labs.Labs {
		if l.ID == "openai" {
			foundOpenAI = true
		}
		if l.ID == "anthropic" {
			foundAnthropic = true
		}
	}
	if !foundOpenAI || !foundAnthropic {
		t.Errorf("expected openai and anthropic in top labs: openai=%v anthropic=%v", foundOpenAI, foundAnthropic)
	}
}

func TestStore_ListModels_Filters(t *testing.T) {
	store := DefaultStore()

	reasoningTrue := true
	reasoningModels := store.ListModels(ListModelsInput{Reasoning: &reasoningTrue, Limit: 10})
	if reasoningModels.Total == 0 {
		t.Fatal("expected reasoning models, got 0")
	}
	for _, m := range reasoningModels.Models {
		if !m.Reasoning {
			t.Errorf("expected reasoning=true for model %s", m.ID)
		}
	}
}

func TestStore_GetModelDetails(t *testing.T) {
	store := DefaultStore()

	details, err := store.GetModelDetails(GetModelDetailsInput{
		ModelID: "anthropic/claude-opus-4.7",
	})
	if err != nil {
		t.Fatalf("GetModelDetails failed: %v", err)
	}
	if details == nil {
		t.Fatal("got nil details")
	}
	if !strings.Contains(strings.ToLower(details.Lab), "anthropic") {
		t.Errorf("lab = %s, want anthropic", details.Lab)
	}
}

func TestStore_RecommendModels(t *testing.T) {
	store := DefaultStore()

	recs := store.RecommendModels(RecommendModelsInput{
		Task:            "coding agent with fast execution and tool use",
		RequireToolCall: true,
		Limit:           5,
	})
	if len(recs.Recommendations) == 0 {
		t.Fatal("expected recommendations, got 0")
	}
}
