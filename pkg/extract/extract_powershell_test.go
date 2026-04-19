package extract

import (
	"path/filepath"
	"testing"
)

func TestExtractPowerShellFindsFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPowerShell(filepath.Join(fixturesDir, "sample.ps1"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Get-Data" || label == "Process-ITEMS" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPowerShell() labels = %v; want Get-Data or Process-ITEMS", labels)
	}
}

func TestExtractPowerShellFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPowerShell(filepath.Join(fixturesDir, "sample.ps1"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "DataProcessor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPowerShell() labels = %v; want DataProcessor", labels)
	}
}

func TestExtractPowerShellFindsImports(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPowerShell(filepath.Join(fixturesDir, "sample.ps1"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "System.IO" || label == "MyModule" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPowerShell() labels = %v; want System.IO or MyModule", labels)
	}
}

func TestExtractPowerShellNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPowerShell(filepath.Join(fixturesDir, "sample.ps1"))
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

func TestExtractPowerShellStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPowerShell(filepath.Join(fixturesDir, "sample.ps1"))
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

func TestExtractPowerShellFindsCalls(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPowerShell(filepath.Join(fixturesDir, "sample.ps1"))
	calls := false
	for _, edge := range result.Edges {
		if edge.Relation == "calls" {
			calls = true
			break
		}
	}
	if !calls {
		t.Error("ExtractPowerShell() should find calls edges")
	}
}
