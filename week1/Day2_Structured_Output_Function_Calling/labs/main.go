// Command currency_agent is the Week 1 Part 2 lab: a typed function-calling
// agent over live National Bank of Ukraine exchange rates.
//
// Run it:
//
//	go run . console           # needs a provider key, see below
//	go run . -offline console  # fixture rates, no key and no network
//
// Provider selection (first match wins), loaded from apps/.env automatically:
//
//	AGENTGATEWAY_BASE_URL -> openaimodel via the local agentgateway (routes
//	                         every call through the gateway so traces, metrics
//	                         and realized USD cost are complete)
//	OLLAMA_BASE_URL  -> openaimodel with a custom BaseURL (self-hosted)
//	GOOGLE_API_KEY   -> gemini
//	OPENAI_API_KEY   -> openaimodel
//
// ADK Go v2.4.0 has no Anthropic backend; an ANTHROPIC_API_KEY alone will not
// run this lab, and the error says so explicitly.
//
// Verified against google.golang.org/adk/v2 v2.4.0 (released 2026-09-11,
// requires Go 1.27) on 2026-08-26. Re-check before recording.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
)

func main() {
	offline := flag.Bool("offline", false, "use fixture rates instead of the live NBU API")
	flag.Parse()

	LoadEnv()

	ctx := context.Background()

	var provider Provider = &NBUProvider{}
	if *offline {
		provider = &FixtureProvider{
			Rates: map[string]float64{"USD": 41.5, "EUR": 45.0, "PLN": 10.0},
			Date:  "2026-07-29",
		}
		fmt.Fprintln(os.Stderr, "rates: offline fixture (USD/EUR/PLN)")
	}

	m, choice, err := BuildModel(ctx)
	if err != nil {
		log.Fatalf("model: %v", err)
	}
	fmt.Fprintf(os.Stderr, "model: %s (%s) — %s\n", choice.Model, choice.Provider, choice.Reason)

	a, err := NewAgent(m, provider)
	if err != nil {
		log.Fatalf("agent: %v", err)
	}

	l := full.NewLauncher()
	cfg := &launcher.Config{AgentLoader: agent.NewSingleLoader(a)}
	if err := l.Execute(ctx, cfg, flag.Args()); err != nil {
		log.Fatalf("run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
