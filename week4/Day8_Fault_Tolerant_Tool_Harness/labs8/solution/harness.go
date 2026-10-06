package main

import (
	"context"
	"errors"
	"fmt"

	"go.temporal.io/sdk/temporal"

	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/ledger"
)

// Class is what the harness does with an error.
type Class int

// The three classes. They are decided once, from the error chain, on the side
// of the Activity boundary where the Go error still exists.
const (
	// ClassRetryTransient: give it back to the engine — Temporal retries the
	// Activity under its RetryPolicy, and the model never hears about it.
	ClassRetryTransient Class = iota + 1
	// ClassSurfaceToModel: retrying cannot help; the model must change its
	// arguments, so the error becomes an observation.
	ClassSurfaceToModel
	// ClassFatal: stop. context.Canceled is a signal to stop, not a reason to
	// retry; an unknown error is not something to retry blindly either.
	ClassFatal
)

// Temporal ApplicationError types. A Go error type does not survive the trip
// from worker to workflow — only the type string does — so the workflow side
// classifies by these names, never by errors.Is on a local sentinel.
const (
	ErrTypeSemantic  = "SemanticError"
	ErrTypeTransient = "TransientError"
	ErrTypeFatal     = "FatalError"
	// ErrTypeSelfCorrection fails the workflow when the model ran out of tries.
	ErrTypeSelfCorrection = "SelfCorrectionExhausted"
)

// maxSelfCorrections is how many semantic errors one turn may turn into
// observations before the harness stops the run.
const maxSelfCorrections = 3

// ErrSelfCorrectionExhausted is wrapped by the error that ends such a run.
var ErrSelfCorrectionExhausted = errors.New("self-correction budget exhausted")

// classifyError decides the class of err with errors.Is / errors.As — never by
// matching err.Error(), which breaks the first time someone wraps the error.
func classifyError(err error) (Class, string) {
	var se *ledger.SemanticError
	switch {
	case errors.Is(err, context.Canceled):
		return ClassFatal, "cancelled"
	case errors.As(err, &se):
		return ClassSurfaceToModel, se.Error()
	case errors.Is(err, ledger.ErrSemantic):
		return ClassSurfaceToModel, err.Error()
	case errors.Is(err, ledger.ErrTransient), errors.Is(err, context.DeadlineExceeded):
		return ClassRetryTransient, err.Error()
	default:
		return ClassFatal, err.Error()
	}
}

// activityError converts a tool error into the ApplicationError the workflow
// will see. Semantic and fatal errors are non-retryable: Temporal must not
// spend the retry budget on a call that cannot succeed.
func activityError(err error) error {
	if err == nil {
		return nil
	}
	class, msg := classifyError(err)
	switch class {
	case ClassSurfaceToModel:
		field := ""
		var se *ledger.SemanticError
		if errors.As(err, &se) {
			field = se.Field
		}
		return temporal.NewNonRetryableApplicationError(msg, ErrTypeSemantic, err, field)
	case ClassRetryTransient:
		opts := temporal.ApplicationErrorOptions{Cause: err}
		var te *ledger.TransientError
		if errors.As(err, &te) && te.RetryAfter > 0 {
			opts.NextRetryDelay = te.RetryAfter // honour Retry-After instead of fixed backoff
		}
		return temporal.NewApplicationErrorWithOptions(msg, ErrTypeTransient, opts)
	default:
		return temporal.NewNonRetryableApplicationError(msg, ErrTypeFatal, err)
	}
}

// Observation is what the model reads after a tool call that did not succeed.
type Observation struct {
	Status      string `json:"status"` // invalid_argument | unavailable | stopped
	Field       string `json:"field,omitempty"`
	Observation string `json:"observation"`
	Attempt     int    `json:"attempt,omitempty"`
}

// budget counts self-corrections within one workflow run. It lives in workflow
// memory, so it is rebuilt identically on replay — no global, no shared state
// between concurrent runs.
type budget struct {
	used    int
	lastErr error
}

// exhausted reports the error that should end the run, or nil.
func (b *budget) exhausted() error {
	if b.used <= maxSelfCorrections {
		return nil
	}
	return temporal.NewNonRetryableApplicationError(
		fmt.Sprintf("self-correction limit exhausted after %d attempts", maxSelfCorrections),
		ErrTypeSelfCorrection,
		fmt.Errorf("%w: %w", ErrSelfCorrectionExhausted, b.lastErr),
	)
}

// observe converts the error of a finished Activity into an observation for
// the model, or returns it when the run must stop.
func (b *budget) observe(err error) (Observation, error) {
	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		// Not an Activity failure (e.g. the workflow is being cancelled).
		return Observation{}, err
	}
	switch appErr.Type() {
	case ErrTypeSemantic:
		b.used++
		b.lastErr = err
		if b.used > maxSelfCorrections {
			return Observation{Status: "stopped", Observation: "STOP: the self-correction limit is reached. Do not call the tool again; tell the user the request could not be completed."}, nil
		}
		var field string
		_ = appErr.Details(&field)
		return Observation{
			Status:      "invalid_argument",
			Field:       field,
			Observation: "ERROR: " + appErr.Message() + " — please correct your arguments and try again",
			Attempt:     b.used,
		}, nil
	case ErrTypeTransient:
		// The engine already retried and ran out of attempts.
		return Observation{Status: "unavailable", Observation: "ERROR: the acquirer is unavailable after retries; tell the user to try again later. Do not retry yourself."}, nil
	default:
		return Observation{}, err
	}
}
