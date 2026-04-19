package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractZigFindsStruct(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractZig(filepath.Join(fixturesDir, "sample.zig"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Point" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractZig() labels = %v; want Point", labels)
	}
}

func TestExtractZigFindsEnum(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractZig(filepath.Join(fixturesDir, "sample.zig"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Color" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractZig() labels = %v; want Color", labels)
	}
}

func TestExtractZigFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractZig(filepath.Join(fixturesDir, "sample.zig"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "distance()" || label == "add()" || label == "multiply()" || label == "main()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractZig() labels = %v; want distance() or add() or multiply() or main()", labels)
	}
}

func TestExtractZigFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractZig(filepath.Join(fixturesDir, "sample.zig"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "std" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractZig() labels = %v; want std", labels)
	}
}

func TestExtractZigNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractZig(filepath.Join(fixturesDir, "sample.zig"))
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

func TestExtractZigStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractZig(filepath.Join(fixturesDir, "sample.zig"))
	structural := map[string]bool{
		"contains": true,
		"imports":  true,
	}
	for _, edge := range result.Edges {
		if structural[edge.Relation] {
			if edge.Confidence != "EXTRACTED" {
				t.Errorf("Structural edge %s should be EXTRACTED, got %s", edge.Relation, edge.Confidence)
			}
		}
	}
}

func TestExtractZigFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractZig(filepath.Join(fixturesDir, "sample.zig"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractZig() should find calls edges")
	}
}
