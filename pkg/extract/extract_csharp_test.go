package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractCSharpFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractCSharp(filepath.Join(fixturesDir, "sample.cs"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "DataProcessor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractCSharp() labels = %v; want DataProcessor", labels)
	}
}

func TestExtractCSharpFindsInterface(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractCSharp(filepath.Join(fixturesDir, "sample.cs"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "IProcessor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractCSharp() labels = %v; want IProcessor", labels)
	}
}

func TestExtractCSharpFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractCSharp(filepath.Join(fixturesDir, "sample.cs"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".Process()" || label == ".Validate()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractCSharp() labels = %v; want .Process() or .Validate()", labels)
	}
}

func TestExtractCSharpFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractCSharp(filepath.Join(fixturesDir, "sample.cs"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "System" || label == "HttpClient" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractCSharp() labels = %v; want System or HttpClient", labels)
	}
}

func TestExtractCSharpNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractCSharp(filepath.Join(fixturesDir, "sample.cs"))
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

func TestExtractCSharpStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractCSharp(filepath.Join(fixturesDir, "sample.cs"))
	structural := map[string]bool{
		"contains": true,
		"imports":  true,
		"method":   true,
		"inherits": true,
	}
	for _, edge := range result.Edges {
		if structural[edge.Relation] {
			if edge.Confidence != "EXTRACTED" {
				t.Errorf("Structural edge %s should be EXTRACTED, got %s", edge.Relation, edge.Confidence)
			}
		}
	}
}

func TestExtractCSharpFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractCSharp(filepath.Join(fixturesDir, "sample.cs"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractCSharp() should find calls edges")
	}
}
