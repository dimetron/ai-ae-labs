package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fixture returns a provider with a small, stable rate table.
func fixture() *FixtureProvider {
	return &FixtureProvider{
		Rates: map[string]float64{"USD": 41.5, "EUR": 45.0, "PLN": 10.0},
		Date:  "2026-07-29",
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		in       RateInput
		wantRate float64
	}{
		{name: "USD to UAH", in: RateInput{Base: "USD", Target: "UAH"}, wantRate: 41.5},
		{name: "UAH to USD", in: RateInput{Base: "UAH", Target: "USD"}, wantRate: 1 / 41.5},
		{name: "cross rate EUR to USD", in: RateInput{Base: "EUR", Target: "USD"}, wantRate: 45.0 / 41.5},
		{name: "same currency is 1", in: RateInput{Base: "USD", Target: "USD"}, wantRate: 1},
		{name: "lowercase is normalized", in: RateInput{Base: "usd", Target: "uah"}, wantRate: 41.5},
		{name: "surrounding space is trimmed", in: RateInput{Base: " USD ", Target: "UAH"}, wantRate: 41.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Convert(context.Background(), fixture(), tc.in)
			if err != nil {
				t.Fatalf("Convert() error = %v", err)
			}
			if !closeEnough(got.Rate, tc.wantRate) {
				t.Errorf("Rate = %v, want %v", got.Rate, tc.wantRate)
			}
			if got.AsOf != "2026-07-29" {
				t.Errorf("AsOf = %q, want %q", got.AsOf, "2026-07-29")
			}
			if got.Source == "" {
				t.Error("Source is empty; provenance must always be populated")
			}
		})
	}
}

// closeEnough compares floats with a tolerance. Comparing computed rates with
// == is the classic way to make a green test go red on a different CPU.
func closeEnough(got, want float64) bool {
	const epsilon = 1e-9
	d := got - want
	return d < epsilon && d > -epsilon
}

func TestConvertErrors(t *testing.T) {
	tests := []struct {
		name string
		in   RateInput
		want error
	}{
		{name: "unknown base", in: RateInput{Base: "XYZ", Target: "UAH"}, want: ErrUnknownCurrency},
		{name: "unknown target", in: RateInput{Base: "USD", Target: "XYZ"}, want: ErrUnknownCurrency},
		{name: "empty base", in: RateInput{Base: "", Target: "UAH"}, want: ErrInvalidCode},
		{name: "too short", in: RateInput{Base: "US", Target: "UAH"}, want: ErrInvalidCode},
		{name: "too long", in: RateInput{Base: "USDD", Target: "UAH"}, want: ErrInvalidCode},
		{name: "digits rejected", in: RateInput{Base: "US1", Target: "UAH"}, want: ErrInvalidCode},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Convert(context.Background(), fixture(), tc.in)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Convert() error = %v, want %v", err, tc.want)
			}
			// The message is what the model sees. It must name the offending
			// value, or the model cannot correct its own call.
			if err != nil && tc.in.Base != "" && !contains(err.Error(), "USD", "XYZ", "US", "USDD", "US1") {
				t.Errorf("error %q names no offending value", err)
			}
		})
	}
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && len(s) >= len(sub) && indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestConvertUpstreamFailure(t *testing.T) {
	p := &FixtureProvider{Err: errors.New("connection refused")}

	_, err := Convert(context.Background(), p, RateInput{Base: "USD", Target: "UAH"})
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("error = %v, want ErrUpstream", err)
	}
}

func TestConvertRejectsNonPositiveRate(t *testing.T) {
	// A zero rate would yield +Inf, which JSON-encodes as an error or, worse,
	// reaches the learner as a confident nonsense answer.
	p := &FixtureProvider{Rates: map[string]float64{"USD": 41.5, "BAD": 0}, Date: "2026-07-29"}

	_, err := Convert(context.Background(), p, RateInput{Base: "USD", Target: "BAD"})
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("error = %v, want ErrUpstream for a zero rate", err)
	}
}

const nbuBody = `[
  {"txt":"Долар США","rate":41.5,"cc":"USD","exchangedate":"29.07.2026"},
  {"txt":"Євро","rate":45.0,"cc":"EUR","exchangedate":"29.07.2026"}
]`

func TestNBUProvider(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(nbuBody))
	}))
	defer srv.Close()

	p := &NBUProvider{BaseURL: srv.URL, HTTPClient: srv.Client()}
	rates, asOf, err := p.RatesToUAH(context.Background())
	if err != nil {
		t.Fatalf("RatesToUAH() error = %v", err)
	}
	if gotQuery != "json" {
		t.Errorf("query = %q, want %q", gotQuery, "json")
	}
	if rates["USD"] != 41.5 || rates["EUR"] != 45.0 {
		t.Errorf("rates = %v, want USD=41.5 EUR=45.0", rates)
	}
	if asOf != "2026-07-29" {
		t.Errorf("asOf = %q, want ISO %q", asOf, "2026-07-29")
	}
}

func TestNBUProviderErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{name: "server error", status: http.StatusInternalServerError, body: "boom", wantErr: "unexpected status"},
		{name: "malformed json", status: http.StatusOK, body: "{not json", wantErr: "decode rates"},
		{name: "empty directory", status: http.StatusOK, body: "[]", wantErr: "empty rate directory"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			p := &NBUProvider{BaseURL: srv.URL, HTTPClient: srv.Client()}
			_, _, err := p.RatesToUAH(context.Background())
			if err == nil {
				t.Fatal("RatesToUAH() error = nil, want error")
			}
			if indexOf(err.Error(), tc.wantErr) < 0 {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestNBUProviderRespectsContextCancellation(t *testing.T) {
	// Without this, a cancelled agent run leaks an in-flight HTTP request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(nbuBody))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := &NBUProvider{BaseURL: srv.URL, HTTPClient: srv.Client()}
	if _, _, err := p.RatesToUAH(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestIsoDate(t *testing.T) {
	tests := []struct{ in, want string }{
		{in: "29.07.2026", want: "2026-07-29"},
		{in: "01.01.2026", want: "2026-01-01"},
		{in: "garbage", want: "garbage"}, // pass through, never invent a date
		{in: "", want: ""},
	}
	for _, tc := range tests {
		if got := isoDate(tc.in); got != tc.want {
			t.Errorf("isoDate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
