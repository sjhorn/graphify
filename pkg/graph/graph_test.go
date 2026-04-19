package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadExtraction() map[string]interface{} {
	fixturesDir := "../../testdata/fixtures"
	data, _ := os.ReadFile(filepath.Join(fixturesDir, "extraction.json"))
	var extraction map[string]interface{}
	json.Unmarshal(data, &extraction)
	return extraction
}

func TestBuildFromJsonNodeCount(t *testing.T) {
	extraction := loadExtraction()
	g := BuildFromJSON(extraction)
	if g.NodeCount() != 4 {
		t.Errorf("BuildFromJSON() nodes = %d; want 4", g.NodeCount())
	}
}

func TestBuildFromJsonEdgeCount(t *testing.T) {
	extraction := loadExtraction()
	g := BuildFromJSON(extraction)
	if g.EdgeCount() != 4 {
		t.Errorf("BuildFromJSON() edges = %d; want 4", g.EdgeCount())
	}
}

func TestNodesHaveLabel(t *testing.T) {
	extraction := loadExtraction()
	g := BuildFromJSON(extraction)
	node := g.GetNode("n_transformer")
	if node.Label != "Transformer" {
		t.Errorf("Node n_transformer label = %s; want Transformer", node.Label)
	}
}

func TestEdgesHaveConfidence(t *testing.T) {
	extraction := loadExtraction()
	g := BuildFromJSON(extraction)
	edge := g.GetEdge("n_attention", "n_concept_attn")
	if edge.Confidence != "INFERRED" {
		t.Errorf("Edge confidence = %s; want INFERRED", edge.Confidence)
	}
}

func TestAmbiguousEdgePreserved(t *testing.T) {
	extraction := loadExtraction()
	g := BuildFromJSON(extraction)
	edge := g.GetEdge("n_layernorm", "n_concept_attn")
	if edge.Confidence != "AMBIGUOUS" {
		t.Errorf("Edge confidence = %s; want AMBIGUOUS", edge.Confidence)
	}
}

func TestBuildMergesMultipleExtractions(t *testing.T) {
	extractions := []*Extraction{
		{
			Nodes: []Node{
				{ID: "n1", Label: "A", File: "a.py", Location: "L1"},
			},
			Edges: []Edge{},
		},
		{
			Nodes: []Node{
				{ID: "n2", Label: "B", File: "b.md", Location: "L1"},
			},
			Edges: []Edge{
				{Source: "n1", Target: "n2", Relation: "references", Confidence: "INFERRED", Weight: 1.0},
			},
		},
	}
	g := BuildFromExtractions(extractions)
	if g.NodeCount() != 2 {
		t.Errorf("BuildFromExtractions() nodes = %d; want 2", g.NodeCount())
	}
	if g.EdgeCount() != 1 {
		t.Errorf("BuildFromExtractions() edges = %d; want 1", g.EdgeCount())
	}
}
