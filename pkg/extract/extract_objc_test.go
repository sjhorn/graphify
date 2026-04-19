package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractObjectiveCFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractObjectiveC(filepath.Join(fixturesDir, "sample.m"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Animal" || label == "Dog" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractObjectiveC() labels = %v; want Animal or Dog", labels)
	}
}

func TestExtractObjectiveCFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractObjectiveC(filepath.Join(fixturesDir, "sample.m"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".initWithName()" || label == ".speak()" || label == ".fetch()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractObjectiveC() labels = %v; want .initWithName() or .speak() or .fetch()", labels)
	}
}

func TestExtractObjectiveCFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractObjectiveC(filepath.Join(fixturesDir, "sample.m"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Foundation" || label == "SampleDelegate" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractObjectiveC() labels = %v; want Foundation or SampleDelegate", labels)
	}
}

func TestExtractObjectiveCNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractObjectiveC(filepath.Join(fixturesDir, "sample.m"))
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

func TestExtractObjectiveCStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractObjectiveC(filepath.Join(fixturesDir, "sample.m"))
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

func TestExtractObjectiveCFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractObjectiveC(filepath.Join(fixturesDir, "sample.m"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractObjectiveC() should find calls edges")
	}
}
