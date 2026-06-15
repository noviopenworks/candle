package graph

import (
	"encoding/json"
	"io"
)

// Node mirrors a Graphify graph.json node.
type Node struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	FileType       string `json:"file_type"`
	SourceFile     string `json:"source_file"`
	SourceLocation string `json:"source_location"`
	SourceURL      string `json:"source_url"`
	CapturedAt     string `json:"captured_at"`
	Author         string `json:"author"`
	Contributor    string `json:"contributor"`
}

// Edge mirrors a Graphify graph.json edge.
type Edge struct {
	Source          string  `json:"source"`
	Target          string  `json:"target"`
	Relation        string  `json:"relation"`
	Confidence      string  `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	Weight          float64 `json:"weight"`
	SourceFile      string  `json:"source_file"`
}

// Hyperedge mirrors a Graphify graph.json hyperedge.
type Hyperedge struct {
	ID              string   `json:"id"`
	Label           string   `json:"label"`
	Nodes           []string `json:"nodes"`
	Relation        string   `json:"relation"`
	Confidence      string   `json:"confidence"`
	ConfidenceScore float64  `json:"confidence_score"`
	SourceFile      string   `json:"source_file"`
}

// Graph is a parsed Graphify graph.json document.
type Graph struct {
	Nodes      []Node      `json:"nodes"`
	Edges      []Edge      `json:"edges"`
	Hyperedges []Hyperedge `json:"hyperedges"`
}

// Parse decodes a Graphify graph.json document from r.
func Parse(r io.Reader) (*Graph, error) {
	var g Graph
	if err := json.NewDecoder(r).Decode(&g); err != nil {
		return nil, err
	}
	return &g, nil
}

// ParseBytes decodes a graph.json document from b.
func ParseBytes(b []byte) (*Graph, error) {
	var g Graph
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, err
	}
	return &g, nil
}
