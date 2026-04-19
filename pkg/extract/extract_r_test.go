package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractRFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractR(filepath.Join(fixturesDir, "sample.R"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "analyze_data()" || label == "plot_results()" || label == "DataProcessor()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractR() labels = %v; want analyze_data() or plot_results() or DataProcessor()", labels)
	}
}

func TestExtractRFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractR(filepath.Join(fixturesDir, "sample.R"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "dplyr" || label == "ggplot2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractR() labels = %v; want dplyr or ggplot2", labels)
	}
}

func TestExtractRFindsClasses(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractR(filepath.Join(fixturesDir, "sample.R"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "DataProcessor()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractR() labels = %v; want DataProcessor()", labels)
	}
}

func TestExtractRNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractR(filepath.Join(fixturesDir, "sample.R"))
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

func TestExtractRStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractR(filepath.Join(fixturesDir, "sample.R"))
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

func TestExtractRFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractR(filepath.Join(fixturesDir, "sample.R"))
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
		t.Error("ExtractR() should find calls edges")
	}
}
