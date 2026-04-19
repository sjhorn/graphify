package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractElmFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElm(filepath.Join(fixturesDir, "Main.elm"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "init()" || label == "update()" || label == "view()" || label == "main()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractElm() labels = %v; want init() or update() or view() or main()", labels)
	}
}

func TestExtractElmFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElm(filepath.Join(fixturesDir, "Main.elm"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Browser" || label == "Html" || label == "Html.Events" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractElm() labels = %v; want Browser or Html or Html.Events", labels)
	}
}

func TestExtractElmNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElm(filepath.Join(fixturesDir, "Main.elm"))
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

func TestExtractElmStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElm(filepath.Join(fixturesDir, "Main.elm"))
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

func TestExtractElmFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElm(filepath.Join(fixturesDir, "Main.elm"))
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
		t.Error("ExtractElm() should find calls edges")
	}
}
