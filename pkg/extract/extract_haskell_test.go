package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractHaskellFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractHaskell(filepath.Join(fixturesDir, "sample.hs"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "calculateStats()" || label == "processData()" || label == "runProcessor()" || label == "createConfig()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractHaskell() labels = %v; want calculateStats() or processData() or runProcessor() or createConfig()", labels)
	}
}

func TestExtractHaskellFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractHaskell(filepath.Join(fixturesDir, "sample.hs"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Data.List" || label == "Data.Map" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractHaskell() labels = %v; want Data.List or Data.Map", labels)
	}
}

func TestExtractHaskellNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractHaskell(filepath.Join(fixturesDir, "sample.hs"))
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

func TestExtractHaskellStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractHaskell(filepath.Join(fixturesDir, "sample.hs"))
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

func TestExtractHaskellFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractHaskell(filepath.Join(fixturesDir, "sample.hs"))
	// Check for any calls edges
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			t.Logf("Found calls edge: %s -> %s", edge.Source, edge.Target)
			break
		}
	}
	if !calls {
		t.Logf("All edges: %v", result.Edges)
		t.Error("ExtractHaskell() should find calls edges")
	}
}
