package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractJuliaFindsModule(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJulia(filepath.Join(fixturesDir, "sample.jl"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Geometry" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJulia() labels = %v; want Geometry", labels)
	}
}

func TestExtractJuliaFindsStruct(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJulia(filepath.Join(fixturesDir, "sample.jl"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Point" || label == "Circle" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJulia() labels = %v; want Point or Circle", labels)
	}
}

func TestExtractJuliaFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJulia(filepath.Join(fixturesDir, "sample.jl"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "area()" || label == "distance()" || label == "perimeter()" || label == "describe()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJulia() labels = %v; want area() or distance() or perimeter() or describe()", labels)
	}
}

func TestExtractJuliaFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJulia(filepath.Join(fixturesDir, "sample.jl"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "LinearAlgebra" || label == "Base" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJulia() labels = %v; want LinearAlgebra or Base", labels)
	}
}

func TestExtractJuliaNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJulia(filepath.Join(fixturesDir, "sample.jl"))
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

func TestExtractJuliaStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJulia(filepath.Join(fixturesDir, "sample.jl"))
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

func TestExtractJuliaFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJulia(filepath.Join(fixturesDir, "sample.jl"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractJulia() should find calls edges")
	}
}
