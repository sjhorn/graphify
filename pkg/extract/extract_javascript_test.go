package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractJavaScriptFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJavaScript(filepath.Join(fixturesDir, "sample.ts"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "HttpClient" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJavaScript() labels = %v; want HttpClient", labels)
	}
}

func TestExtractJavaScriptFindsFunction(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJavaScript(filepath.Join(fixturesDir, "sample.ts"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "buildHeaders()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJavaScript() labels = %v; want buildHeaders()", labels)
	}
}

func TestExtractJavaScriptFindsMethod(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJavaScript(filepath.Join(fixturesDir, "sample.ts"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".get()" || label == ".post()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJavaScript() labels = %v; want .get() or .post()", labels)
	}
}

func TestExtractJavaScriptFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJavaScript(filepath.Join(fixturesDir, "sample.ts"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "./models" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractJavaScript() labels = %v; want ./models", labels)
	}
}

func TestExtractJavaScriptNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJavaScript(filepath.Join(fixturesDir, "sample.ts"))
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

func TestExtractJavaScriptStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJavaScript(filepath.Join(fixturesDir, "sample.ts"))
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

func TestExtractJavaScriptFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractJavaScript(filepath.Join(fixturesDir, "sample.ts"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractJavaScript() should find calls edges")
	}
}
