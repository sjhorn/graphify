package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractPHPFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPHP(filepath.Join(fixturesDir, "sample.php"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "ApiClient" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPHP() labels = %v; want ApiClient", labels)
	}
}

func TestExtractPHPFindsFunction(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPHP(filepath.Join(fixturesDir, "sample.php"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "parseResponse()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPHP() labels = %v; want parseResponse()", labels)
	}
}

func TestExtractPHPFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPHP(filepath.Join(fixturesDir, "sample.php"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Authenticator" || label == "CacheManager" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPHP() labels = %v; want Authenticator or CacheManager", labels)
	}
}

func TestExtractPHPNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPHP(filepath.Join(fixturesDir, "sample.php"))
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

func TestExtractPHPStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPHP(filepath.Join(fixturesDir, "sample.php"))
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

func TestExtractPHPFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPHP(filepath.Join(fixturesDir, "sample.php"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractPHP() should find calls edges")
	}
}
