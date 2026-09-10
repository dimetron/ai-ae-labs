// Package domain defines the core types for the knowledge graph pipeline.
package domain

// Triple represents a subject-predicate-object relationship.
type Triple struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
	Chunk     int    `json:"chunk,omitempty"`
	Inferred  bool   `json:"inferred,omitempty"`
}

// GraphStats holds statistics about the generated knowledge graph.
type GraphStats struct {
	Nodes         int `json:"nodes"`
	Edges         int `json:"edges"`
	OriginalEdges int `json:"original_edges"`
	InferredEdges int `json:"inferred_edges"`
	Communities   int `json:"communities"`
}

// PipelineState carries data through the ADK workflow pipeline.
type PipelineState struct {
	InputText  string   `json:"input_text"`
	Chunks     []string `json:"chunks"`
	Triples    []Triple `json:"triples"`
	Stats      GraphStats `json:"stats"`
	OutputFile string   `json:"output_file"`
	Debug      bool     `json:"debug"`
}
