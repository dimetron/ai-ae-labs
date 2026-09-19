// Command currency_agent is the Week 1 Part 2 lab: a typed function-calling
// agent over live exchange rates (NBU or monobank, both key-free).
//
// Run it:
//
//	go run . console                # NBU rates, needs a provider key, see below
//	go run . -provider monobank console  # monobank public rates, still key-free for the model
//	go run . -offline console       # fixture rates, no key and no network
//	go run . compare                # NBU vs monobank side-by-side table, no model key
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
	"google.golang.org/adk/v2/tool"
)

func main() {
	// compare is a plain side-by-side utility, not an agent run: it needs no
	// model key, so it must not consume the -offline/-provider flags.
	if len(flag.Args()) == 0 && len(os.Args) > 1 && os.Args[1] == "compare" {
		runCompare(os.Args[2:])
		return
	}

	offline := flag.Bool("offline", false, "use fixture rates instead of a live API")
	providerName := flag.String("provider", "nbu", "rate provider: nbu | monobank")
	// The external MCP tool boundary (mono-go-mcp over stdio, official Go
	// SDK MCP) is attached when the server binary is resolvable — an
	// explicit, visible decision, never a silent "maybe". -no-mcp opts out.
	noMCP := flag.Bool("no-mcp", false, "run with only the local rate tool (no mono-go-mcp toolset)")
	flag.Parse()

	LoadEnv()

	ctx := context.Background()

	var provider Provider = &NBUProvider{}
	switch *providerName {
	case "nbu":
		provider = &NBUProvider{}
	case "monobank":
		provider = &MonoProvider{}
	default:
		log.Fatalf("unknown provider %q (want nbu or monobank)", *providerName)
	}
	if *offline {
		provider = &FixtureProvider{
			Rates: map[string]float64{"USD": 41.5, "EUR": 45.0, "PLN": 10.0},
			Date:  "2026-07-29",
		}
		fmt.Fprintln(os.Stderr, "rates: offline fixture (USD/EUR/PLN)")
	} else {
		fmt.Fprintf(os.Stderr, "rates: %s\n", provider.Name())
	}

	// Extra toolsets: the local rate tool is always there; the external MCP
	// toolset is added only when its binary is resolvable. The model sees
	// exactly the tools we wired — never a "maybe" (least agency).
	var extraToolsets []tool.Toolset
	if !*noMCP {
		if ts, err := MonoMCPToolset(); err != nil {
			fmt.Fprintf(os.Stderr, "mcp: %v (continuing with the local tool only)\n", err)
		} else {
			extraToolsets = append(extraToolsets, ts)
			fmt.Fprintln(os.Stderr, "mcp: mono-go-mcp toolset attached (tools/list decides what the model sees)")
		}
	}

	m, choice, err := BuildModel(ctx)
	if err != nil {
		log.Fatalf("model: %v", err)
	}
	fmt.Fprintf(os.Stderr, "model: %s (%s) — %s\n", choice.Model, choice.Provider, choice.Reason)

	a, err := NewAgent(m, provider, extraToolsets...)
	if err != nil {
		log.Fatalf("agent: %v", err)
	}

	l := full.NewLauncher()
	cfg := &launcher.Config{AgentLoader: agent.NewSingleLoader(a)}
	if err := l.Execute(ctx, cfg, flag.Args()); err != nil {
		log.Fatalf("run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}

// runCompare executes the side-by-side rate comparison. It uses a fresh
// FlagSet because the agent's own flags (-offline, -provider) do not apply.
func runCompare(args []string) {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "also print the comparison as JSON")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	rows, nbuDate, monoDate, err := Compare(context.Background())
	if err != nil {
		log.Fatalf("compare: %v", err)
	}
	CompareTable(rows, nbuDate, monoDate, *jsonOut)
}
