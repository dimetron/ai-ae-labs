// Package pipeline orchestrates the knowledge graph generation workflow
// using ADK v2 graph nodes.
//
// The pipeline consists of five stages wired as a sequential ADK workflow:
//
//	Chunk → Extract → Standardize → Infer → Visualize
//
// Each stage is an ADK FunctionNode that operates on shared PipelineState.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/dimetron/ai-knowledge-graph-go/internal/chunker"
	"github.com/dimetron/ai-knowledge-graph-go/internal/config"
	"github.com/dimetron/ai-knowledge-graph-go/internal/domain"
	"github.com/dimetron/ai-knowledge-graph-go/internal/extractor"
	"github.com/dimetron/ai-knowledge-graph-go/internal/graph"
	"github.com/dimetron/ai-knowledge-graph-go/internal/inference"
	"github.com/dimetron/ai-knowledge-graph-go/internal/standardizer"
	"github.com/dimetron/ai-knowledge-graph-go/internal/viz"
)

// NOTE: The workflow graph types (Graph, FunctionNode, etc.) are defined
// in workflow.go within this package. They mirror the ADK v2 workflow API.
// When switching to the real ADK, import "google.golang.org/adk/v2/workflow"
// and remove workflow.go.

// Pipeline encapsulates the knowledge graph generation workflow.
type Pipeline struct {
	cfg *config.Config
	llm *extractor.Extractor
}

// New creates a Pipeline with the given configuration.
func New(cfg *config.Config) *Pipeline {
	return &Pipeline{
		cfg: cfg,
		llm: extractor.New(cfg.LLM),
	}
}

// Run executes the full knowledge graph pipeline.
// It builds an ADK workflow graph, then runs it.
func (p *Pipeline) Run(ctx context.Context, inputText, outputFile string, debug bool) error {
	// Build the ADK workflow graph.
	wf := p.buildWorkflow()

	// Initialize pipeline state.
	state := &domain.PipelineState{
		InputText:  inputText,
		OutputFile: outputFile,
		Debug:      debug,
	}

	// Execute the workflow.
	slog.Info("starting knowledge graph pipeline")

	if err := wf.Execute(ctx, state); err != nil {
		return fmt.Errorf("pipeline execution: %w", err)
	}

	slog.Info("pipeline complete",
		"nodes", state.Stats.Nodes,
		"edges", state.Stats.Edges,
		"communities", state.Stats.Communities,
		"output", outputFile,
	)

	return nil
}

// buildWorkflow constructs the workflow graph.
// Uses the local Graph type which mirrors the ADK v2 workflow.Graph API.
func (p *Pipeline) buildWorkflow() *Graph[*domain.PipelineState] {
	wf := NewGraph[*domain.PipelineState]("knowledge-graph-pipeline")

	// Define nodes.
	chunkNode := NewFunctionNode("chunk", p.chunkStep)
	extractNode := NewFunctionNode("extract", p.extractStep)
	standardizeNode := NewFunctionNode("standardize", p.standardizeStep)
	inferNode := NewFunctionNode("infer", p.inferStep)
	visualizeNode := NewFunctionNode("visualize", p.visualizeStep)

	// Wire the sequential pipeline: chunk → extract → standardize → infer → visualize.
	wf.AddNode(chunkNode)
	wf.AddNode(extractNode)
	wf.AddNode(standardizeNode)
	wf.AddNode(inferNode)
	wf.AddNode(visualizeNode)

	wf.AddEdge("chunk", "extract")
	wf.AddEdge("extract", "standardize")
	wf.AddEdge("standardize", "infer")
	wf.AddEdge("infer", "visualize")

	wf.SetEntrypoint("chunk")

	return wf
}

// chunkStep splits input text into overlapping chunks.
func (p *Pipeline) chunkStep(_ context.Context, state *domain.PipelineState) (*domain.PipelineState, error) {
	chunks := chunker.Chunk(state.InputText, p.cfg.Chunking.ChunkSize, p.cfg.Chunking.Overlap)
	state.Chunks = chunks

	fmt.Println(separator)
	fmt.Println("PHASE 0: TEXT CHUNKING")
	fmt.Println(separator)
	fmt.Printf("Split text into %d chunks (size: %d words, overlap: %d words)\n",
		len(chunks), p.cfg.Chunking.ChunkSize, p.cfg.Chunking.Overlap)

	return state, nil
}

// extractStep sends each chunk to the LLM for triple extraction.
func (p *Pipeline) extractStep(ctx context.Context, state *domain.PipelineState) (*domain.PipelineState, error) {
	fmt.Println(separator)
	fmt.Println("PHASE 1: INITIAL TRIPLE EXTRACTION")
	fmt.Println(separator)

	var allTriples []domain.Triple

	for i, chunk := range state.Chunks {
		fmt.Printf("Processing chunk %d/%d (%d words)\n", i+1, len(state.Chunks), wordCount(chunk))

		triples, err := p.llm.Extract(ctx, chunk)
		if err != nil {
			slog.Warn("chunk extraction failed", "chunk", i+1, "error", err)
			fmt.Printf("Warning: Failed to extract triples from chunk %d\n", i+1)
			continue
		}

		// Tag with chunk number.
		for j := range triples {
			triples[j].Chunk = i + 1
		}

		allTriples = append(allTriples, triples...)

		if state.Debug {
			data, _ := json.MarshalIndent(triples, "", "  ")
			fmt.Printf("Chunk %d extracted %d triples:\n%s\n", i+1, len(triples), string(data))
		}
	}

	state.Triples = allTriples
	fmt.Printf("\nExtracted a total of %d triples from all chunks\n", len(allTriples))

	return state, nil
}

// standardizeStep normalizes entity names across triples.
func (p *Pipeline) standardizeStep(ctx context.Context, state *domain.PipelineState) (*domain.PipelineState, error) {
	if !p.cfg.Standardization.Enabled {
		fmt.Println("Entity standardization disabled, skipping")
		return state, nil
	}

	fmt.Println(separator)
	fmt.Println("PHASE 2: ENTITY STANDARDIZATION")
	fmt.Println(separator)
	fmt.Printf("Starting with %d triples and %d unique entities\n",
		len(state.Triples), countEntities(state.Triples))

	state.Triples = standardizer.Standardize(ctx, state.Triples, p.cfg, p.llm)

	fmt.Printf("After standardization: %d triples and %d unique entities\n",
		len(state.Triples), countEntities(state.Triples))

	return state, nil
}

// inferStep discovers additional relationships between entities.
func (p *Pipeline) inferStep(ctx context.Context, state *domain.PipelineState) (*domain.PipelineState, error) {
	if !p.cfg.Inference.Enabled {
		fmt.Println("Relationship inference disabled, skipping")
		return state, nil
	}

	fmt.Println(separator)
	fmt.Println("PHASE 3: RELATIONSHIP INFERENCE")
	fmt.Println(separator)
	fmt.Printf("Starting with %d triples\n", len(state.Triples))

	before := len(state.Triples)
	state.Triples = inference.Infer(ctx, state.Triples, p.cfg, p.llm)

	inferred := 0
	for _, t := range state.Triples {
		if t.Inferred {
			inferred++
		}
	}

	fmt.Printf("Added %d inferred relationships\n", len(state.Triples)-before)
	fmt.Printf("Final knowledge graph: %d triples (%d inferred)\n", len(state.Triples), inferred)

	return state, nil
}

// visualizeStep builds the graph and renders the HTML visualization.
func (p *Pipeline) visualizeStep(_ context.Context, state *domain.PipelineState) (*domain.PipelineState, error) {
	fmt.Println(separator)
	fmt.Println("PHASE 4: VISUALIZATION")
	fmt.Println(separator)

	// Save raw JSON data.
	jsonOutput := state.OutputFile
	if len(jsonOutput) > 5 && jsonOutput[len(jsonOutput)-5:] == ".html" {
		jsonOutput = jsonOutput[:len(jsonOutput)-5] + ".json"
	}

	data, err := json.MarshalIndent(state.Triples, "", "  ")
	if err == nil {
		if err := os.WriteFile(jsonOutput, data, 0o644); err != nil {
			slog.Warn("could not save JSON data", "file", jsonOutput, "error", err)
		} else {
			fmt.Printf("Saved raw knowledge graph data to %s\n", jsonOutput)
		}
	}

	// Build the graph structure.
	kg := graph.Build(state.Triples)
	state.Stats = kg.Stats

	// Render HTML visualization.
	edgeSmooth := p.cfg.Visualization.EdgeSmooth
	if err := viz.Render(kg, state.OutputFile, edgeSmooth); err != nil {
		return state, fmt.Errorf("render visualization: %w", err)
	}

	fmt.Println("\nKnowledge Graph Statistics:")
	fmt.Printf("Nodes: %d\n", kg.Stats.Nodes)
	fmt.Printf("Edges: %d\n", kg.Stats.Edges)
	fmt.Printf("Communities: %d\n", kg.Stats.Communities)
	fmt.Printf("\nTo view the visualization, open: file://%s\n", absPath(state.OutputFile))

	return state, nil
}

// -- helpers --

const separator = "=================================================="

func wordCount(text string) int {
	count := 0
	inWord := false
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}

func countEntities(triples []domain.Triple) int {
	entities := make(map[string]bool)
	for _, t := range triples {
		entities[t.Subject] = true
		entities[t.Object] = true
	}
	return len(entities)
}

func absPath(path string) string {
	if abs, err := os.Getwd(); err == nil {
		if path[0] != '/' {
			return abs + "/" + path
		}
	}
	return path
}
