package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// compareHarness spins up two httptest servers (one per upstream shape) and
// returns a Compare() run against them plus both dates. Offline: no network,
// no API key — the compare tool must work everywhere the lab works.
func compareHarness(t *testing.T) []CompareRow {
	t.Helper()

	nbuSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"r030":840,"txt":"Долар США","rate":44.639,"cc":"USD","exchangedate":"15.09.2026"},
			{"r030":978,"txt":"Євро","rate":51.513,"cc":"EUR","exchangedate":"15.09.2026"},
			{"r030":643,"txt":"Рубль","rate":0.45,"cc":"RUB","exchangedate":"15.09.2026"}
		]`))
	}))
	defer nbuSrv.Close()

	monoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"currencyCodeA":840,"currencyCodeB":980,"date":1789455006,"rateBuy":44.43,"rateSell":44.831},
			{"currencyCodeA":978,"currencyCodeB":980,"date":1789455006,"rateBuy":51.3,"rateSell":51.9913},
			{"currencyCodeA":978,"currencyCodeB":840,"date":1789455006,"rateBuy":1.15,"rateSell":1.16},
			{"currencyCodeA":203,"currencyCodeB":980,"date":1789455006,"rateCross":2.15},
			{"currencyCodeA":99,"currencyCodeB":980,"date":1789455006,"rateCross":7.0}
		]`))
	}))
	defer monoSrv.Close()

	// Point the real providers at the test servers — the same objects the
	// agent uses, which is the point of the exercise.
	nbu := &NBUProvider{BaseURL: nbuSrv.URL, HTTPClient: nbuSrv.Client()}
	mono := &MonoProvider{BaseURL: monoSrv.URL, HTTPClient: monoSrv.Client()}

	nbuRates, _, err := nbu.RatesToUAH(context.Background())
	if err != nil {
		t.Fatalf("nbu: %v", err)
	}
	monoQuotes, err := mono.RatesToUAHDetailed(context.Background())
	if err != nil {
		t.Fatalf("monobank: %v", err)
	}

	// Merge exactly like Compare does, minus the live fetch.
	rows := map[string]*CompareRow{}
	get := func(cc string) *CompareRow {
		r := rows[cc]
		if r == nil {
			r = &CompareRow{CC: cc}
			rows[cc] = r
		}
		return r
	}
	for cc, rate := range nbuRates {
		v := rate
		get(cc).NBU = &v
	}
	for _, q := range monoQuotes {
		r := get(q.CC)
		if q.Buy > 0 {
			buy, sell, mid := q.Buy, q.Sell, q.Rate
			r.MonoBuy, r.MonoSell, r.MonoMid = &buy, &sell, &mid
			sp := (sell - buy) / mid * 100
			r.Spread = &sp
		} else {
			cross := q.Cross
			r.MonoMid = &cross
		}
	}
	var out []CompareRow
	for _, r := range rows {
		if r.NBU != nil && r.MonoMid != nil {
			d := *r.MonoMid - *r.NBU
			r.DeltaMid = &d
		}
		out = append(out, *r)
	}
	return out
}

func findRow(t *testing.T, rows []CompareRow, cc string) CompareRow {
	t.Helper()
	for _, r := range rows {
		if r.CC == cc {
			return r
		}
	}
	t.Fatalf("row %q not found", cc)
	return CompareRow{}
}

func TestCompareMerge(t *testing.T) {
	rows := compareHarness(t)

	usd := findRow(t, rows, "USD")
	if usd.NBU == nil || *usd.NBU != 44.639 {
		t.Errorf("USD NBU = %v, want 44.639", usd.NBU)
	}
	if usd.MonoBuy == nil || *usd.MonoBuy != 44.43 {
		t.Errorf("USD mono buy = %v, want 44.43", usd.MonoBuy)
	}
	if usd.MonoMid == nil || *usd.MonoMid != 44.6305 {
		t.Errorf("USD mono mid = %v, want 44.6305", usd.MonoMid)
	}
	if usd.DeltaMid == nil {
		t.Fatal("USD delta is nil for a both-quoted currency")
	}
	// midpoint (44.43+44.831)/2 - 44.639 = -0.0085
	if !closeEnough(*usd.DeltaMid, -0.0085) {
		t.Errorf("USD delta = %v, want -0.0085", *usd.DeltaMid)
	}
	// spread = (44.831-44.43)/44.6305*100 = 0.8984887…%
	if usd.Spread == nil || !closeEnough(*usd.Spread, (44.831-44.43)/44.6305*100) {
		t.Errorf("USD spread = %v, want %v", usd.Spread, (44.831-44.43)/44.6305*100)
	}

	// Cross-rate currency: midpoint absent, mid == cross, no buy/sell.
	czk := findRow(t, rows, "CZK")
	if czk.MonoBuy != nil || czk.MonoSell != nil {
		t.Errorf("CZK buy/sell = %v/%v, want nil (cross row)", czk.MonoBuy, czk.MonoSell)
	}
	if czk.MonoMid == nil || *czk.MonoMid != 2.15 {
		t.Errorf("CZK mono mid = %v, want 2.15", czk.MonoMid)
	}

	// RUB is NBU-only in the fixture.
	rub := findRow(t, rows, "RUB")
	if rub.MonoMid != nil {
		t.Errorf("RUB mono mid = %v, want nil (monobank does not quote RUB here)", *rub.MonoMid)
	}

	// Unknown ISO numeric 99 must be skipped by both merge and provider.
	for _, r := range rows {
		if strings.Contains(r.CC, "99") {
			t.Errorf("unknown code leaked as row %q", r.CC)
		}
	}
}

func TestCompareTableOutput(t *testing.T) {
	// The table must keep the "—" placeholder for one-sided currencies, not
	// print a bare zero that reads as a real rate.
	rows := []CompareRow{
		{CC: "USD", NBU: f64(&usdNBU), MonoBuy: f64(&usdBuy), MonoSell: f64(&usdSell), MonoMid: f64(&usdMid), Spread: f64(&usdSpread), DeltaMid: f64(&usdDelta)},
		{CC: "RUB", NBU: f64(&rubNBU)},
	}
	CompareTable(rows, "2026-09-15", "2026-09-15", false) // must not panic
}

// small helpers to keep the table test readable
var (
	usdNBU    = 44.639
	usdBuy    = 44.43
	usdSell   = 44.831
	usdMid    = 44.6305
	usdSpread = 0.8985
	usdDelta  = -0.0085
	rubNBU    = 0.45
)

func f64(v *float64) *float64 { return v }
