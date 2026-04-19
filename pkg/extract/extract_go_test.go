package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractGoFindsType(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractGo(filepath.Join(fixturesDir, "sample.go"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Server" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractGo() labels = %v; want Server", labels)
	}
}

func TestExtractGoFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractGo(filepath.Join(fixturesDir, "sample.go"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".Start()" || label == ".Stop()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractGo() labels = %v; want .Start() or .Stop()", labels)
	}
}

func TestExtractGoFindsFunction(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractGo(filepath.Join(fixturesDir, "sample.go"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "main()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractGo() labels = %v; want main()", labels)
	}
}

func TestExtractGoNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractGo(filepath.Join(fixturesDir, "sample.go"))
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

func TestExtractGoStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractGo(filepath.Join(fixturesDir, "sample.go"))
	structural := map[string]bool{
		"contains":     true,
		"method":       true,
		"inherits":     true,
		"imports":      true,
		"imports_from": true,
	}
	for _, edge := range result.Edges {
		if structural[edge.Relation] {
			if edge.Confidence != "EXTRACTED" {
				t.Errorf("Structural edge %s should be EXTRACTED, got %s", edge.Relation, edge.Confidence)
			}
		}
	}
}

func TestExtractGoFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractGo(filepath.Join(fixturesDir, "sample.go"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractGo() should find calls edges")
	}
}

func TestExtractGoCallsEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractGo(filepath.Join(fixturesDir, "sample.go"))
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			if edge.Confidence != "EXTRACTED" {
				t.Errorf("Calls edge should be EXTRACTED, got %s", edge.Confidence)
			}
			if edge.Weight != 1.0 {
				t.Errorf("Calls edge weight should be 1.0, got %f", edge.Weight)
			}
		}
	}
}

func TestExtractGoCallsNoSelfLoops(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractGo(filepath.Join(fixturesDir, "sample.go"))
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			if edge.Source == edge.Target {
				t.Errorf("Self-loop found: %s", edge.Source)
			}
		}
	}
}

func TestExtractGoCallsDeduplication(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractGo(filepath.Join(fixturesDir, "sample.go"))
	callPairs := make(map[string]bool)
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			pair := edge.Source + "->" + edge.Target
			if callPairs[pair] {
				t.Errorf("Duplicate calls edge: %s", pair)
			}
			callPairs[pair] = true
		}
	}
}
