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

func TestGetNodeNeighborsOutgoing(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddNode("c", "C", "class", "c.py")
	g.AddEdge("a", "b", "calls", "EXTRACTED", 1.0)
	g.AddEdge("a", "c", "imports", "EXTRACTED", 1.0)

	neighbors := g.GetNodeNeighbors("a")
	if len(neighbors) != 2 {
		t.Errorf("GetNodeNeighbors(a) = %d; want 2", len(neighbors))
	}
}

func TestGetNodeNeighborsIncoming(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddEdge("b", "a", "calls", "EXTRACTED", 1.0)

	neighbors := g.GetNodeNeighbors("a")
	if len(neighbors) != 1 {
		t.Errorf("GetNodeNeighbors(a) = %d; want 1", len(neighbors))
	}
	if neighbors[0] != "b" {
		t.Errorf("GetNodeNeighbors(a)[0] = %s; want b", neighbors[0])
	}
}

func TestGetNodeNeighborsNoNeighbors(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")

	neighbors := g.GetNodeNeighbors("a")
	if len(neighbors) != 0 {
		t.Errorf("GetNodeNeighbors(a) = %d; want 0", len(neighbors))
	}
}

func TestGetNodeDegree(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddNode("c", "C", "class", "c.py")
	g.AddEdge("a", "b", "calls", "EXTRACTED", 1.0)
	g.AddEdge("c", "a", "imports", "EXTRACTED", 1.0)

	degree := g.GetNodeDegree("a")
	if degree != 2 {
		t.Errorf("GetNodeDegree(a) = %d; want 2", degree)
	}
}

func TestGetNodeDegreeZero(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	degree := g.GetNodeDegree("a")
	if degree != 0 {
		t.Errorf("GetNodeDegree(a) = %d; want 0", degree)
	}
}

func TestHasEdgeTrue(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddEdge("a", "b", "calls", "EXTRACTED", 1.0)

	if !g.HasEdge("a", "b") {
		t.Error("HasEdge(a, b) = false; want true")
	}
}

func TestHasEdgeFalse(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")

	if g.HasEdge("a", "b") {
		t.Error("HasEdge(a, b) = true; want false")
	}
}

func TestHasEdgeDirectional(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddEdge("a", "b", "calls", "EXTRACTED", 1.0)

	if g.HasEdge("b", "a") {
		t.Error("HasEdge(b, a) = true; want false (directed)")
	}
}

func TestBuildCombinesExtractions(t *testing.T) {
	ext1 := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "label": "A", "type": "class", "source_file": "a.py"},
		},
		"edges": []interface{}{},
	}
	ext2 := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n2", "label": "B", "type": "class", "source_file": "b.py"},
		},
		"edges": []interface{}{
			map[string]interface{}{"source": "n1", "target": "n2", "relation": "calls", "confidence": "EXTRACTED", "weight": 1.0},
		},
	}

	g := Build([]map[string]interface{}{ext1, ext2})
	if g.NodeCount() != 2 {
		t.Errorf("Build() nodes = %d; want 2", g.NodeCount())
	}
	if g.EdgeCount() != 1 {
		t.Errorf("Build() edges = %d; want 1", g.EdgeCount())
	}
}

func TestBuildEmpty(t *testing.T) {
	g := Build(nil)
	if g.NodeCount() != 0 {
		t.Errorf("Build(nil) nodes = %d; want 0", g.NodeCount())
	}
}

func TestNodesReturnsAllNodes(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")

	nodes := g.Nodes()
	if len(nodes) != 2 {
		t.Errorf("Nodes() = %d; want 2", len(nodes))
	}
}

func TestEdgesReturnsAllEdges(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddEdge("a", "b", "calls", "EXTRACTED", 1.0)

	edges := g.Edges()
	if len(edges) != 1 {
		t.Errorf("Edges() = %d; want 1", len(edges))
	}
}

func TestGetNodeNilForMissing(t *testing.T) {
	g := NewGraph()
	if g.GetNode("nonexistent") != nil {
		t.Error("GetNode(nonexistent) should return nil")
	}
}

func TestGetEdgeNilForMissing(t *testing.T) {
	g := NewGraph()
	if g.GetEdge("a", "b") != nil {
		t.Error("GetEdge(a, b) should return nil")
	}
}

func TestBuildFromJSONInvalidNodes(t *testing.T) {
	g := BuildFromJSON(map[string]interface{}{
		"nodes": "not a list",
		"edges": []interface{}{},
	})
	if g.NodeCount() != 0 {
		t.Errorf("BuildFromJSON with invalid nodes should have 0 nodes, got %d", g.NodeCount())
	}
}

func TestBuildFromJSONInvalidEdges(t *testing.T) {
	g := BuildFromJSON(map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "label": "A", "type": "class", "source_file": "a.py"},
		},
		"edges": "not a list",
	})
	if g.NodeCount() != 1 {
		t.Errorf("BuildFromJSON with invalid edges should have 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("BuildFromJSON with invalid edges should have 0 edges, got %d", g.EdgeCount())
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
