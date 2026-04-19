package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractElixirFindsModule(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElixir(filepath.Join(fixturesDir, "sample.ex"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "MyApp.Accounts.User" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractElixir() labels = %v; want MyApp.Accounts.User", labels)
	}
}

func TestExtractElixirFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElixir(filepath.Join(fixturesDir, "sample.ex"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "create" || label == "find" || label == "validate" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractElixir() labels = %v; want create or find or validate", labels)
	}
}

func TestExtractElixirFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElixir(filepath.Join(fixturesDir, "sample.ex"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Repo" || label == "Ecto.Query" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractElixir() labels = %v; want Repo or Ecto.Query", labels)
	}
}

func TestExtractElixirNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElixir(filepath.Join(fixturesDir, "sample.ex"))
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

func TestExtractElixirStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElixir(filepath.Join(fixturesDir, "sample.ex"))
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

func TestExtractElixirFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractElixir(filepath.Join(fixturesDir, "sample.ex"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractElixir() should find calls edges")
	}
}
