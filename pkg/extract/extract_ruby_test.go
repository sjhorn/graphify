package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractRubyFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRuby(filepath.Join(fixturesDir, "sample.rb"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "ApiClient" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractRuby() labels = %v; want ApiClient", labels)
	}
}

func TestExtractRubyFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRuby(filepath.Join(fixturesDir, "sample.rb"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".get()" || label == ".post()" || label == ".fetch()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractRuby() labels = %v; want .get() or .post() or .fetch()", labels)
	}
}

func TestExtractRubyFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRuby(filepath.Join(fixturesDir, "sample.rb"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "json" || label == "net_http" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractRuby() labels = %v; want json or net_http", labels)
	}
}

func TestExtractRubyNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRuby(filepath.Join(fixturesDir, "sample.rb"))
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

func TestExtractRubyStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRuby(filepath.Join(fixturesDir, "sample.rb"))
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

func TestExtractRubyFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractRuby(filepath.Join(fixturesDir, "sample.rb"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractRuby() should find calls edges")
	}
}
