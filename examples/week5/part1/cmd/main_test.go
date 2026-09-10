package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/adk/v2/agent"
)

func TestRememberFact(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	result, err := rememberFact(ctx, "integration=legacy")
	if err != nil {
		t.Fatalf("rememberFact failed: %v", err)
	}
	if !strings.Contains(result, "integration") {
		t.Errorf("expected key in result, got: %s", result)
	}
}

func TestRememberFact_InvalidFormat(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	_, err := rememberFact(ctx, "invalid-format-no-equals")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestRecallFact_Found(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	longTermMemory.Set("test_key", "test_value")
	result, err := recallFact(ctx, "test_key")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "test_value") {
		t.Errorf("expected value in result, got: %s", result)
	}
}

func TestRecallFact_NotFound(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	result, err := recallFact(ctx, "nonexistent_key")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No memory found") {
		t.Errorf("expected 'not found' message, got: %s", result)
	}
}

func TestRecallFact_PrefixSearch(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	longTermMemory.Set("user:integration", "legacy")
	longTermMemory.Set("user:recommended_path", "legacy-fix")
	result, err := recallFact(ctx, "user:")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "legacy") {
		t.Errorf("expected prefix search results, got: %s", result)
	}
}

func TestFormatMemory(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	longTermMemory.Set("fmt_test", "fmt_value")
	result, err := formatMemory(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "fmt_test") {
		t.Errorf("expected key in formatted output, got: %s", result)
	}
}

func TestMemoryStore_Concurrent(t *testing.T) {
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			longTermMemory.Set(fmt.Sprintf("concurrent_key_%d", i), fmt.Sprintf("value_%d", i))
			longTermMemory.Get(fmt.Sprintf("concurrent_key_%d", i))
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
