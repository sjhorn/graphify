package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractLuaFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractLua(filepath.Join(fixturesDir, "sample.lua"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "HttpClient:get()" || label == "HttpClient:post()" || label == "createClient()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractLua() labels = %v; want HttpClient:get() or HttpClient:post() or createClient()", labels)
	}
}

func TestExtractLuaFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractLua(filepath.Join(fixturesDir, "sample.lua"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "http" || label == "json" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractLua() labels = %v; want http or json", labels)
	}
}

func TestExtractLuaFindsClasses(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractLua(filepath.Join(fixturesDir, "sample.lua"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "HttpClient:buildRequest()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractLua() labels = %v; want HttpClient:buildRequest()", labels)
	}
}

func TestExtractLuaNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractLua(filepath.Join(fixturesDir, "sample.lua"))
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

func TestExtractLuaStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractLua(filepath.Join(fixturesDir, "sample.lua"))
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

func TestExtractLuaFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractLua(filepath.Join(fixturesDir, "sample.lua"))
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
		t.Error("ExtractLua() should find calls edges")
	}
}
