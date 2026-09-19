package main

import "testing"

// TestResolveModelName pins the one invariant the whole Lab 1 benchmark rests
// on: the model name comes from the MODEL environment variable, and falls back
// to a non-expiring alias when it is absent.
//
// Why this is worth a test at all: Завдання 5 asks the learner to run the same
// prompt on two models by changing ONE variable. If MODEL were ignored, the
// benchmark would silently compare a model against itself and produce a
// perfectly plausible table — the worst kind of wrong, because nothing fails.
//
// The default is deliberately the floating alias `gemini-flash-latest` rather
// than a dated id like `gemini-3.7-flash`: a starter that ships a dated model
// id breaks for every learner the day that id is retired.
func TestResolveModelName(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "empty falls back to the non-expiring alias", env: "", want: defaultModel},
		{name: "explicit model wins", env: "gemini-3.7-flash", want: "gemini-3.7-flash"},
		{name: "another provider's id passes through untouched", env: "gpt-5.6-luna", want: "gpt-5.6-luna"},
		{name: "whitespace is not treated as a value", env: "   ", want: defaultModel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveModelName(tt.env); got != tt.want {
				t.Errorf("resolveModelName(%q) = %q, want %q", tt.env, got, tt.want)
			}
		})
	}
}
