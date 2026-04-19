package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractRustFindsStruct(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRust(filepath.Join(fixturesDir, "sample.rs"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Graph" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractRust() labels = %v; want Graph", labels)
	}
}

func TestExtractRustFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRust(filepath.Join(fixturesDir, "sample.rs"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".new()" || label == ".add_node()" || label == ".add_edge()" || label == "build_graph()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractRust() labels = %v; want .new() or .add_node() or .add_edge() or build_graph()", labels)
	}
}

func TestExtractRustFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRust(filepath.Join(fixturesDir, "sample.rs"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "HashMap" || label == "collections" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractRust() labels = %v; want HashMap or collections", labels)
	}
}

func TestExtractRustNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRust(filepath.Join(fixturesDir, "sample.rs"))
	nodeIds := GetNodeIds(result)
	for _, edge := range result.Edges {
		found := false
		for _, id := range nodeIds {
			if id == edge.Source {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Dangling edge source: %s", edge.Source)
		}
	}
}

func TestExtractRustStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRust(filepath.Join(fixturesDir, "sample.rs"))
	structural := map[string]bool{
		"contains": true,
		"imports":  true,
		"method":   true,
	}
	for _, edge := range result.Edges {
		if structural[edge.Relation] {
			if edge.Confidence != "EXTRACTED" {
				t.Errorf("Structural edge %s should be EXTRACTED, got %s", edge.Relation, edge.Confidence)
			}
		}
	}
}

func TestExtractRustFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRust(filepath.Join(fixturesDir, "sample.rs"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractRust() should find calls edges")
	}
}
