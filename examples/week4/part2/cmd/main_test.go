package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/adk/v2/agent"
)

func TestClassifyError_Transient(t *testing.T) {
	if class := classifyError(ErrTransient); class != "transient" {
		t.Errorf("expected 'transient', got %q", class)
	}
}

func TestClassifyError_Semantic(t *testing.T) {
	if class := classifyError(ErrSemantic); class != "semantic" {
		t.Errorf("expected 'semantic', got %q", class)
	}
}

func TestClassifyError_Wrapped(t *testing.T) {
	wrapped := fmt.Errorf("wrapped: %w", ErrTransient)
	if class := classifyError(wrapped); class != "transient" {
		t.Errorf("expected 'transient' for wrapped error, got %q", class)
	}
}

func TestFlakyCheck_EmptyInput(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	_, err := flakyCheck(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !errors.Is(err, ErrSemantic) {
		t.Errorf("expected ErrSemantic, got %v", err)
	}
}

func TestFlakyCheck_UndefinedInput(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	_, err := flakyCheck(ctx, "undefined")
	if err == nil {
		t.Fatal("expected error for 'undefined' input")
	}
	if !errors.Is(err, ErrSemantic) {
		t.Errorf("expected ErrSemantic, got %v", err)
	}
}

func TestToolHarness_SemanticError(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	result, err := toolHarness(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure for empty input")
	}
	if result.ErrorClass != "semantic" {
		t.Errorf("expected 'semantic' error class, got %q", result.ErrorClass)
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt for semantic error, got %d", result.Attempts)
	}
}

func TestToolHarness_ValidInput(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	result, err := toolHarness(ctx, "valid input")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success && result.ErrorClass == "semantic" {
		t.Fatal("unexpected semantic error for valid input")
	}
}

func TestErrorSentinelWrapping(t *testing.T) {
	wrapped := fmt.Errorf("upstream: %w", ErrTransient)
	if !errors.Is(wrapped, ErrTransient) {
		t.Error("errors.Is should match through wrapping")
	}
}
