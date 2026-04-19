package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractKotlinFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractKotlin(filepath.Join(fixturesDir, "sample.kt"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "HttpClient" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractKotlin() labels = %v; want HttpClient", labels)
	}
}

func TestExtractKotlinFindsDataClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractKotlin(filepath.Join(fixturesDir, "sample.kt"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Config" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractKotlin() labels = %v; want Config", labels)
	}
}

func TestExtractKotlinFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractKotlin(filepath.Join(fixturesDir, "sample.kt"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".get()" || label == ".post()" || label == ".buildRequest()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractKotlin() labels = %v; want .get() or .post() or .buildRequest()", labels)
	}
}

func TestExtractKotlinFindsTopLevelFunction(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractKotlin(filepath.Join(fixturesDir, "sample.kt"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "createClient()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractKotlin() labels = %v; want createClient()", labels)
	}
}

func TestExtractKotlinFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractKotlin(filepath.Join(fixturesDir, "sample.kt"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "delay" || label == "max" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractKotlin() labels = %v; want delay or max", labels)
	}
}

func TestExtractKotlinNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractKotlin(filepath.Join(fixturesDir, "sample.kt"))
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

func TestExtractKotlinStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractKotlin(filepath.Join(fixturesDir, "sample.kt"))
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

func TestExtractKotlinFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractKotlin(filepath.Join(fixturesDir, "sample.kt"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractKotlin() should find calls edges")
	}
}
