package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractJavaFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJava(filepath.Join(fixturesDir, "sample.java"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "DataProcessor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJava() labels = %v; want DataProcessor", labels)
	}
}

func TestExtractJavaFindsInterface(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJava(filepath.Join(fixturesDir, "sample.java"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Processor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJava() labels = %v; want Processor", labels)
	}
}

func TestExtractJavaFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJava(filepath.Join(fixturesDir, "sample.java"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".process()" || label == ".addItem()" || label == ".validate()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJava() labels = %v; want .process() or .addItem() or .validate()", labels)
	}
}

func TestExtractJavaFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJava(filepath.Join(fixturesDir, "sample.java"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "List" || label == "ArrayList" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJava() labels = %v; want List or ArrayList", labels)
	}
}

func TestExtractJavaNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJava(filepath.Join(fixturesDir, "sample.java"))
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

func TestExtractJavaStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJava(filepath.Join(fixturesDir, "sample.java"))
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

func TestExtractJavaFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJava(filepath.Join(fixturesDir, "sample.java"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractJava() should find calls edges")
	}
}
