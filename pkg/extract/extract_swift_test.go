package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractSwiftFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractSwift(filepath.Join(fixturesDir, "sample.swift"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "DataProcessor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractSwift() labels = %v; want DataProcessor", labels)
	}
}

func TestExtractSwiftFindsStruct(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractSwift(filepath.Join(fixturesDir, "sample.swift"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Config" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractSwift() labels = %v; want Config", labels)
	}
}

func TestExtractSwiftFindsEnum(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractSwift(filepath.Join(fixturesDir, "sample.swift"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "NetworkError" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractSwift() labels = %v; want NetworkError", labels)
	}
}

func TestExtractSwiftFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractSwift(filepath.Join(fixturesDir, "sample.swift"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".process()" || label == ".validate()" || label == ".addItem()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractSwift() labels = %v; want .process() or .validate() or .addItem()", labels)
	}
}

func TestExtractSwiftFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractSwift(filepath.Join(fixturesDir, "sample.swift"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Foundation" || label == "UIKit" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractSwift() labels = %v; want Foundation or UIKit", labels)
	}
}

func TestExtractSwiftNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractSwift(filepath.Join(fixturesDir, "sample.swift"))
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

func TestExtractSwiftStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractSwift(filepath.Join(fixturesDir, "sample.swift"))
	structural := map[string]bool{
		"contains": true,
		"imports":  true,
		"method":   true,
		"inherits": true,
		"case_of":  true,
	}
	for _, edge := range result.Edges {
		if structural[edge.Relation] {
			if edge.Confidence != "EXTRACTED" {
				t.Errorf("Structural edge %s should be EXTRACTED, got %s", edge.Relation, edge.Confidence)
			}
		}
	}
}

func TestExtractSwiftFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractSwift(filepath.Join(fixturesDir, "sample.swift"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractSwift() should find calls edges")
	}
}
