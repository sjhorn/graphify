package extract

import (
	"path/filepath"
	"testing"
)

func TestMakeIdStripsDotsAndUnderscores(t *testing.T) {
	if MakeId("_auth") != "auth" {
		t.Errorf("MakeId(\"_auth\") = %s; want \"auth\"", MakeId("_auth"))
	}
	if MakeId(".httpx._client") != "httpx_client" {
		t.Errorf("MakeId(\".httpx._client\") = %s; want \"httpx_client\"", MakeId(".httpx._client"))
	}
}

func TestMakeIdConsistent(t *testing.T) {
	if MakeId("foo", "Bar") != MakeId("foo", "Bar") {
		t.Error("MakeId should be consistent for same input")
	}
}

func TestMakeIdNoLeadingTrailingUnderscores(t *testing.T) {
	result := MakeId("__init__")
	if len(result) == 0 {
		t.Error("MakeId(\"__init__\") should not be empty")
	}
	if result[0] == '_' || result[len(result)-1] == '_' {
		t.Errorf("MakeId(\"__init__\") = %s; should not start/end with underscore", result)
	}
}

func TestExtractPythonFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPython(filepath.Join(fixturesDir, "sample.py"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Transformer" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPython() labels = %v; want Transformer", labels)
	}
}

func TestExtractPythonFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPython(filepath.Join(fixturesDir, "sample.py"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".__init__()" || label == ".forward()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPython() labels = %v; want .__init__() or .forward()", labels)
	}
}

func TestExtractPythonNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPython(filepath.Join(fixturesDir, "sample.py"))
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

func TestStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPython(filepath.Join(fixturesDir, "sample.py"))
	structural := map[string]bool{
		"contains": true, "method": true, "inherits": true,
		"imports": true, "imports_from": true,
	}
	for _, edge := range result.Edges {
		if structural[edge.Relation] {
			if edge.Confidence != "EXTRACTED" {
				t.Errorf("Structural edge %s should be EXTRACTED, got %s", edge.Relation, edge.Confidence)
			}
		}
	}
}

func TestExtractMergesMultipleFiles(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	files := []string{filepath.Join(fixturesDir, "sample.py")}
	result := Extract(files, fixturesDir)
	if len(result.Nodes) == 0 {
		t.Error("Extract() should return nodes")
	}
}

func TestCollectFilesFromDir(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	files := CollectFiles(fixturesDir)
	if len(files) == 0 {
		t.Error("CollectFiles() should return files")
	}
}

func TestCollectFilesSkipsHidden(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	files := CollectFiles(fixturesDir)
	for _, f := range files {
		if filepath.Base(f)[0] == '.' {
			t.Errorf("CollectFiles() included hidden file: %s", f)
		}
	}
}

func TestNoDanglingEdgesOnExtract(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	files := []string{filepath.Join(fixturesDir, "sample.py")}
	result := Extract(files, fixturesDir)
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
