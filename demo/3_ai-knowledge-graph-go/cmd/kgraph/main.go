// Package main provides the CLI entry point for the knowledge graph generator.
//
// Usage:
//
//	kgraph --input data/industrial-revolution.txt --output graph.html
//	kgraph --test --output sample.html
//	kgraph --input data/article.txt --config custom.toml --debug
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/dimetron/ai-knowledge-graph-go/internal/config"
	"github.com/dimetron/ai-knowledge-graph-go/internal/domain"
	"github.com/dimetron/ai-knowledge-graph-go/internal/graph"
	"github.com/dimetron/ai-knowledge-graph-go/internal/pipeline"
	"github.com/dimetron/ai-knowledge-graph-go/internal/viz"
)

func main() {
	var (
		testMode        = flag.Bool("test", false, "Generate a test visualization with sample data")
		configPath      = flag.String("config", "config.toml", "Path to configuration file")
		outputFile      = flag.String("output", "knowledge_graph.html", "Output HTML file path")
		inputFile       = flag.String("input", "", "Path to input text file (required unless --test)")
		debug           = flag.Bool("debug", false, "Enable debug output")
		noStandardize   = flag.Bool("no-standardize", false, "Disable entity standardization")
		noInference     = flag.Bool("no-inference", false, "Disable relationship inference")
	)
	flag.Parse()

	// Configure structured logging.
	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config from %s: %v\n", *configPath, err)
		os.Exit(1)
	}

	// Override config with CLI flags.
	if *noStandardize {
		cfg.Standardization.Enabled = false
	}
	if *noInference {
		cfg.Inference.Enabled = false
	}

	// Test mode: generate sample visualization and exit.
	if *testMode {
		if err := runTestMode(*outputFile, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Normal mode: input file is required.
	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --input is required unless --test is used")
		flag.Usage()
		os.Exit(1)
	}

	// Read input text.
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file %s: %v\n", *inputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Using input text from file: %s\n", *inputFile)

	// Set up cancellation context.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Run the pipeline.
	p := pipeline.New(cfg)
	if err := p.Run(ctx, string(data), *outputFile, *debug); err != nil {
		fmt.Fprintf(os.Stderr, "Pipeline error: %v\n", err)
		os.Exit(1)
	}
}

// runTestMode generates a visualization using sample data.
func runTestMode(outputFile string, cfg *config.Config) error {
	fmt.Println("Generating sample data visualization...")

	triples := sampleTriples()
	kg := graph.Build(triples)

	if err := viz.Render(kg, outputFile, cfg.Visualization.EdgeSmooth); err != nil {
		return fmt.Errorf("render sample: %w", err)
	}

	// Also save the JSON data.
	jsonOutput := outputFile
	if len(jsonOutput) > 5 && jsonOutput[len(jsonOutput)-5:] == ".html" {
		jsonOutput = jsonOutput[:len(jsonOutput)-5] + ".json"
	}
	data, _ := json.MarshalIndent(triples, "", "  ")
	_ = os.WriteFile(jsonOutput, data, 0o644)

	fmt.Printf("\nSample visualization saved to %s\n", outputFile)
	abs, _ := os.Getwd()
	fmt.Printf("To view: file://%s/%s\n", abs, outputFile)
	return nil
}

// sampleTriples returns sample data matching the Python version's test data.
func sampleTriples() []domain.Triple {
	return []domain.Triple{
		{Subject: "industrial revolution", Predicate: "began in", Object: "great britain"},
		{Subject: "industrial revolution", Predicate: "characterized by", Object: "machine manufacturing"},
		{Subject: "industrial revolution", Predicate: "led to", Object: "urbanization"},
		{Subject: "industrial revolution", Predicate: "led to", Object: "rise of capitalism"},
		{Subject: "industrial revolution", Predicate: "led to", Object: "new labor movements"},
		{Subject: "industrial revolution", Predicate: "fueled by", Object: "technological innovations"},
		{Subject: "james watt", Predicate: "developed", Object: "steam engine"},
		{Subject: "james watt", Predicate: "born in", Object: "scotland"},
		{Subject: "scotland", Predicate: "a country in", Object: "europe"},
		{Subject: "steam engine", Predicate: "revolutionized", Object: "transportation"},
		{Subject: "steam engine", Predicate: "revolutionized", Object: "manufacturing processes"},
		{Subject: "steam engine", Predicate: "spread to", Object: "europe"},
		{Subject: "steam engine", Predicate: "led to", Object: "industrial revolution"},
		{Subject: "steam engine", Predicate: "spread to", Object: "north america"},
		{Subject: "technological innovations", Predicate: "led to", Object: "digital computers"},
		{Subject: "digital computers", Predicate: "enabled", Object: "artificial intelligence"},
		{Subject: "artificial intelligence", Predicate: "will replace", Object: "humanity"},
		{Subject: "artificial intelligence", Predicate: "led to", Object: "llms"},
		{Subject: "robert mcdermott", Predicate: "likes", Object: "llms"},
		{Subject: "robert mcdermott", Predicate: "owns", Object: "digital computers"},
		{Subject: "robert mcdermott", Predicate: "lives in", Object: "north america"},
	}
}
