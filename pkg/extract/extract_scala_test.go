package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractScalaFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractScala(filepath.Join(fixturesDir, "sample.scala"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "HttpClient" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractScala() labels = %v; want HttpClient", labels)
	}
}

func TestExtractScalaFindsCaseClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractScala(filepath.Join(fixturesDir, "sample.scala"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Config" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractScala() labels = %v; want Config", labels)
	}
}

func TestExtractScalaFindsObject(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractScala(filepath.Join(fixturesDir, "sample.scala"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "HttpClientFactory" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractScala() labels = %v; want HttpClientFactory", labels)
	}
}

func TestExtractScalaFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractScala(filepath.Join(fixturesDir, "sample.scala"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".get()" || label == ".post()" || label == ".buildRequest()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractScala() labels = %v; want .get() or .post() or .buildRequest()", labels)
	}
}

func TestExtractScalaFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractScala(filepath.Join(fixturesDir, "sample.scala"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "ListBuffer" || label == "mutable" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractScala() labels = %v; want ListBuffer or mutable", labels)
	}
}

func TestExtractScalaNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractScala(filepath.Join(fixturesDir, "sample.scala"))
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

func TestExtractScalaStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractScala(filepath.Join(fixturesDir, "sample.scala"))
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

func TestExtractScalaFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractScala(filepath.Join(fixturesDir, "sample.scala"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractScala() should find calls edges")
	}
}
