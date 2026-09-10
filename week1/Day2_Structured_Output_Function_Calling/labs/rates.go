// Week 1 Part 2 — Structured Output & Function Calling.
//
// This file is the ADK-free core: currency-rate lookup with a real HTTP
// provider, input validation, and typed errors. It has no dependency on the
// agent framework at all, which is the point — the logic a tool wraps should be
// testable without a model, a key, or a network.
//
// The tool contract itself lives in agent.go.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Sentinel errors. These matter more than they look: the text of a tool error
// goes back to the model as an observation, and the model decides whether to
// retry with corrected arguments or give up. A vague error ("request failed")
// produces a vague retry; a specific one ("unknown currency code XYZ") lets the
// model fix its own call. That is the week 4 self-correction path in miniature.
var (
	// ErrUnknownCurrency reports a syntactically valid but unsupported code.
	ErrUnknownCurrency = errors.New("unknown currency code")
	// ErrInvalidCode reports a malformed code (not three letters).
	ErrInvalidCode = errors.New("invalid currency code")
	// ErrUpstream reports that the rate provider failed.
	ErrUpstream = errors.New("rate provider unavailable")
)

// RateInput is the tool's input contract.
//
// The `jsonschema` tag value is a DESCRIPTION, not a constraint list. This is
// the single most common mistake with ADK Go tool schemas: writing
// `jsonschema:"required,enum=UAH"` produces a tool whose description is the
// literal string "required,enum=UAH", and an empty `jsonschema:""` is a hard
// error in jsonschema-go. Constraints belong in an explicit
// functiontool.Config.InputSchema, not in the tag.
type RateInput struct {
	Base   string `json:"base" jsonschema:"ISO 4217 code of the base currency, e.g. USD"`
	Target string `json:"target" jsonschema:"ISO 4217 code of the target currency, e.g. UAH"`
}

// RateOutput is the tool's output contract.
//
// Source is deliberately included: an agent answer that cannot say where a
// number came from is not auditable, and provenance is a week 3 theme that
// starts here.
type RateOutput struct {
	Base   string  `json:"base"`
	Target string  `json:"target"`
	Rate   float64 `json:"rate" jsonschema:"How many units of target one unit of base buys"`
	AsOf   string  `json:"as_of" jsonschema:"Rate date, YYYY-MM-DD"`
	Source string  `json:"source" jsonschema:"Provenance of the rate, e.g. nbu or fixture"`
}

// Provider fetches rates against UAH, which is the axis the National Bank of
// Ukraine publishes. Keeping this an interface is what lets the tests run
// offline and the binary run against the real bank.
type Provider interface {
	// RatesToUAH returns how many UAH one unit of each listed currency buys,
	// plus the date the rates are valid for.
	RatesToUAH(ctx context.Context) (map[string]float64, string, error)
}

// normalizeCode upper-cases and validates a currency code.
func normalizeCode(code string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(code))
	if len(c) != 3 {
		return "", fmt.Errorf("%w: %q must be three letters (ISO 4217)", ErrInvalidCode, code)
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return "", fmt.Errorf("%w: %q must be three letters (ISO 4217)", ErrInvalidCode, code)
		}
	}
	return c, nil
}

// Convert computes the base→target rate by cross-rating through UAH.
//
// UAH itself is treated as having rate 1.0, so UAH→EUR and EUR→UAH both work
// without the provider needing to publish a UAH row.
func Convert(ctx context.Context, p Provider, in RateInput) (RateOutput, error) {
	base, err := normalizeCode(in.Base)
	if err != nil {
		return RateOutput{}, err
	}
	target, err := normalizeCode(in.Target)
	if err != nil {
		return RateOutput{}, err
	}

	rates, asOf, err := p.RatesToUAH(ctx)
	if err != nil {
		return RateOutput{}, fmt.Errorf("%w: %s", ErrUpstream, err)
	}

	baseUAH, err := rateToUAH(base, rates)
	if err != nil {
		return RateOutput{}, err
	}
	targetUAH, err := rateToUAH(target, rates)
	if err != nil {
		return RateOutput{}, err
	}

	return RateOutput{
		Base:   base,
		Target: target,
		Rate:   baseUAH / targetUAH,
		AsOf:   asOf,
		Source: "nbu",
	}, nil
}

// rateToUAH resolves one currency's UAH value, treating UAH as the unit.
func rateToUAH(code string, rates map[string]float64) (float64, error) {
	if code == "UAH" {
		return 1, nil
	}
	rate, ok := rates[code]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrUnknownCurrency, code)
	}
	if rate <= 0 {
		// A zero or negative rate would produce +Inf or a negative price. Fail
		// loudly rather than hand the model a nonsense number it will happily
		// present as fact.
		return 0, fmt.Errorf("%w: %s has non-positive rate %v", ErrUpstream, code, rate)
	}
	return rate, nil
}

// NBUProvider reads the National Bank of Ukraine's public rate directory.
//
// Chosen deliberately for this course: it needs no API key, so the production
// path runs for every learner on day one (§2b "no surprise prerequisites"),
// and it is a real endpoint with real failure modes rather than a toy.
type NBUProvider struct {
	// BaseURL defaults to the NBU statistics service. Tests point it at an
	// httptest server.
	BaseURL string
	// HTTPClient defaults to a client with a timeout. Never use
	// http.DefaultClient for outbound calls in production: it has no timeout,
	// so one hung upstream leaks a goroutine per request forever.
	HTTPClient *http.Client
}

const defaultNBUBaseURL = "https://bank.gov.ua/NBUStatService/v1/statdirectory/exchange"

// nbuRow is one row of the NBU response.
type nbuRow struct {
	Rate         float64 `json:"rate"`
	CC           string  `json:"cc"`
	ExchangeDate string  `json:"exchangedate"` // DD.MM.YYYY
}

// RatesToUAH implements Provider.
func (p *NBUProvider) RatesToUAH(ctx context.Context) (map[string]float64, string, error) {
	base := p.BaseURL
	if base == "" {
		base = defaultNBUBaseURL
	}
	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?json", nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("get rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	var rows []nbuRow
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, "", fmt.Errorf("decode rates: %w", err)
	}
	if len(rows) == 0 {
		return nil, "", errors.New("empty rate directory")
	}

	out := make(map[string]float64, len(rows))
	for _, row := range rows {
		out[strings.ToUpper(row.CC)] = row.Rate
	}
	return out, isoDate(rows[0].ExchangeDate), nil
}

// isoDate converts NBU's DD.MM.YYYY to YYYY-MM-DD, leaving anything
// unparseable untouched rather than inventing a date.
func isoDate(ddmmyyyy string) string {
	t, err := time.Parse("02.01.2006", ddmmyyyy)
	if err != nil {
		return ddmmyyyy
	}
	return t.Format("2006-01-02")
}

// FixtureProvider is an offline Provider for tests and key-free demos.
type FixtureProvider struct {
	Rates map[string]float64
	Date  string
	Err   error
}

// RatesToUAH implements Provider.
func (p *FixtureProvider) RatesToUAH(context.Context) (map[string]float64, string, error) {
	if p.Err != nil {
		return nil, "", p.Err
	}
	return p.Rates, p.Date, nil
}
