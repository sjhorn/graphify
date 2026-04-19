package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractCFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractC(filepath.Join(fixturesDir, "sample.c"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "validate()" || label == "process()" || label == "main()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractC() labels = %v; want validate() or process() or main()", labels)
	}
}

func TestExtractCFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractC(filepath.Join(fixturesDir, "sample.c"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "stdio" || label == "stdlib" || label == "string" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractC() labels = %v; want stdio or stdlib or string", labels)
	}
}

func TestExtractCNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractC(filepath.Join(fixturesDir, "sample.c"))
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

func TestExtractCStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractC(filepath.Join(fixturesDir, "sample.c"))
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

func TestExtractCFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractC(filepath.Join(fixturesDir, "sample.c"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractC() should find calls edges")
	}
}
