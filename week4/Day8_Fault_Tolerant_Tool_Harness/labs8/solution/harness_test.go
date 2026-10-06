package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.temporal.io/sdk/temporal"

	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/ledger"
)

func TestClassifyError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want Class
	}{
		{"semantic type", &ledger.SemanticError{Field: "merchant_id", Reason: "undefined"}, ClassSurfaceToModel},
		{"semantic wrapped with %w", fmt.Errorf("tool: %w", &ledger.SemanticError{Field: "x"}), ClassSurfaceToModel},
		{"semantic sentinel", fmt.Errorf("bad: %w", ledger.ErrSemantic), ClassSurfaceToModel},
		{"transient", ledger.Unavailable.Err("open_refund_case"), ClassRetryTransient},
		{"deadline", fmt.Errorf("call: %w", context.DeadlineExceeded), ClassRetryTransient},
		{"cancelled is fatal", fmt.Errorf("call: %w", context.Canceled), ClassFatal},
		{"unknown is fatal", errors.New("disk on fire"), ClassFatal},
		{"wrapped with %v loses the chain", fmt.Errorf("tool: %v", ledger.ErrTransient), ClassFatal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got, _ := classifyError(tc.err); got != tc.want {
				t.Errorf("classifyError(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestActivityErrorMapping(t *testing.T) {
	t.Parallel()
	if activityError(nil) != nil {
		t.Fatal("nil must stay nil")
	}
	tests := []struct {
		name      string
		err       error
		typ       string
		retryable bool
		delay     time.Duration
	}{
		{"semantic", &ledger.SemanticError{Field: "merchant_id", Reason: "r"}, ErrTypeSemantic, false, 0},
		{"transient", ledger.Unavailable.Err("op"), ErrTypeTransient, true, 0},
		{"rate limit honours Retry-After", ledger.RateLimited.Err("op"), ErrTypeTransient, true, time.Second},
		{"fatal", errors.New("boom"), ErrTypeFatal, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var appErr *temporal.ApplicationError
			if !errors.As(activityError(tc.err), &appErr) {
				t.Fatal("not an ApplicationError")
			}
			if appErr.Type() != tc.typ || appErr.NonRetryable() == tc.retryable || appErr.NextRetryDelay() != tc.delay {
				t.Errorf("type=%s nonRetryable=%v delay=%v", appErr.Type(), appErr.NonRetryable(), appErr.NextRetryDelay())
			}
		})
	}
	var appErr *temporal.ApplicationError
	_ = errors.As(activityError(&ledger.SemanticError{Field: "merchant_id"}), &appErr)
	var field string
	if err := appErr.Details(&field); err != nil || field != "merchant_id" {
		t.Errorf("semantic details = %q, %v", field, err)
	}
}

func TestBudgetObserve(t *testing.T) {
	t.Parallel()
	b := &budget{}
	sem := temporal.NewNonRetryableApplicationError("invalid merchant_id", ErrTypeSemantic, nil, "merchant_id")
	for i := 1; i <= maxSelfCorrections; i++ {
		obs, err := b.observe(sem)
		if err != nil || obs.Status != "invalid_argument" || obs.Attempt != i || obs.Field != "merchant_id" || !strings.Contains(obs.Observation, "please correct") {
			t.Fatalf("attempt %d: %+v, %v", i, obs, err)
		}
	}
	if b.exhausted() != nil {
		t.Fatal("exhausted too early")
	}
	if obs, _ := b.observe(sem); obs.Status != "stopped" {
		t.Errorf("past the limit: %+v", obs)
	}
	ex := b.exhausted()
	var appErr *temporal.ApplicationError
	if !errors.As(ex, &appErr) || appErr.Type() != ErrTypeSelfCorrection || !errors.Is(ex, ErrSelfCorrectionExhausted) {
		t.Errorf("exhausted = %v", ex)
	}

	if obs, err := (&budget{}).observe(temporal.NewApplicationError("503", ErrTypeTransient)); err != nil || obs.Status != "unavailable" {
		t.Errorf("transient after retries = %+v, %v", obs, err)
	}
	fatal := temporal.NewNonRetryableApplicationError("boom", ErrTypeFatal, nil)
	if _, err := (&budget{}).observe(fatal); err != fatal {
		t.Errorf("fatal must pass through, got %v", err)
	}
	plain := errors.New("workflow cancelled")
	if _, err := (&budget{}).observe(plain); err != plain {
		t.Errorf("non-activity error must pass through, got %v", err)
	}
}
