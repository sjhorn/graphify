package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractDartFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractDart(filepath.Join(fixturesDir, "sample.dart"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "DataProcessor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractDart() labels = %v; want DataProcessor", labels)
	}
}

func TestExtractDartFindsMixin(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractDart(filepath.Join(fixturesDir, "sample.dart"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Loggable" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractDart() labels = %v; want Loggable", labels)
	}
}

func TestExtractDartFindsEnum(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractDart(filepath.Join(fixturesDir, "sample.dart"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Status" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractDart() labels = %v; want Status", labels)
	}
}

func TestExtractDartFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractDart(filepath.Join(fixturesDir, "sample.dart"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".process()" || label == ".validate()" || label == ".log()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractDart() labels = %v; want .process() or .validate() or .log()", labels)
	}
}

func TestExtractDartFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractDart(filepath.Join(fixturesDir, "sample.dart"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "dart_async" || label == "material" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractDart() labels = %v; want dart_async or material", labels)
	}
}

func TestExtractDartNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractDart(filepath.Join(fixturesDir, "sample.dart"))
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

func TestExtractDartStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractDart(filepath.Join(fixturesDir, "sample.dart"))
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

func TestExtractDartFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractDart(filepath.Join(fixturesDir, "sample.dart"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractDart() should find calls edges")
	}
}
