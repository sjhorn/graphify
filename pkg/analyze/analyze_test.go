package analyze

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sjhorn/graphify/pkg/graph"
)

func loadGraph() *graph.Graph {
	fixturesDir := "../../testdata/fixtures"
	data, _ := os.ReadFile(filepath.Join(fixturesDir, "extraction.json"))
	var extraction map[string]interface{}
	json.Unmarshal(data, &extraction)
	return graph.BuildFromJSON(extraction)
}

func TestGodNodesReturnsList(t *testing.T) {
	g := loadGraph()
	result := GodNodes(g, 3)
	if len(result) > 3 {
		t.Errorf("GodNodes() returned %d nodes; want <= 3", len(result))
	}
}

func TestGodNodesSortedByDegree(t *testing.T) {
	g := loadGraph()
	result := GodNodes(g, 10)
	for i := 1; i < len(result); i++ {
		if result[i].Degree > result[i-1].Degree {
			t.Errorf("GodNodes not sorted by degree: %d > %d", result[i].Degree, result[i-1].Degree)
		}
	}
}

func TestGodNodesHaveRequiredKeys(t *testing.T) {
	g := loadGraph()
	result := GodNodes(g, 1)
	if len(result) == 0 {
		t.Fatal("GodNodes() returned empty list")
	}
	node := result[0]
	if node.ID == "" {
		t.Error("GodNode missing ID")
	}
	if node.Label == "" {
		t.Error("GodNode missing Label")
	}
	if node.Degree <= 0 {
		t.Error("GodNode missing Degree")
	}
}

func TestSurprisingConnectionsHaveWhyField(t *testing.T) {
	g := loadGraph()
	communities := make(map[int][]string)
	communities[0] = []string{"n_transformer", "n_attention"}
	communities[1] = []string{"n_concept_attn", "n_layernorm"}
	surprises := SurprisingConnections(g, communities, 5)
	for _, s := range surprises {
		if s.Why == "" {
			t.Errorf("SurprisingConnection missing Why field")
		}
	}
}

func TestSurprisingConnectionsHaveRequiredKeys(t *testing.T) {
	g := loadGraph()
	communities := make(map[int][]string)
	surprises := SurprisingConnections(g, communities, 5)
	for _, s := range surprises {
		if s.Source == "" {
			t.Error("SurprisingConnection missing Source")
		}
		if s.Target == "" {
			t.Error("SurprisingConnection missing Target")
		}
		if len(s.SourceFiles) == 0 {
			t.Error("SurprisingConnection missing SourceFiles")
		}
	}
}

func TestFileCategory(t *testing.T) {
	tests := map[string]string{
		"model.py":       "code",
		"flash.pdf":      "paper",
		"diagram.png":    "image",
		"notes.md":       "paper",
		"app.swift":      "code",
		"plugin.lua":     "code",
		"build.zig":      "code",
		"deploy.ps1":     "code",
		"server.ex":      "code",
		"component.jsx":  "code",
		"analysis.jl":    "code",
		"view.m":         "code",
		"main.go":        "code",
		"readme.txt":     "doc",
	}
	for path, expected := range tests {
		if got := FileCategory(path); got != expected {
			t.Errorf("FileCategory(%q) = %q; want %q", path, got, expected)
		}
	}
}

func TestSurpriseScoreConfidence(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "Alpha", "", "repo1/model.py")
	g.AddNode("b", "Beta", "", "repo2/train.py")
	g.AddEdge("a", "b", "calls", "AMBIGUOUS", 1.0)

	nodeCommunity := map[string]int{"a": 0, "b": 1}
	score, reasons := SurpriseScore(g, "a", "b", "AMBIGUOUS", nodeCommunity, "repo1/model.py", "repo2/train.py")

	if score < 3 {
		t.Errorf("SurpriseScore(AMBIGUOUS) = %d; want >= 3", score)
	}
	if len(reasons) == 0 {
		t.Error("SurpriseScore should return reasons")
	}
}

func TestSurpriseScoreCrossType(t *testing.T) {
	g := NewGraph()
	g.AddNode("a", "Transformer", "", "code/model.py")
	g.AddNode("b", "FlashAttn", "", "papers/flash.pdf")
	g.AddEdge("a", "b", "references", "EXTRACTED", 1.0)

	nodeCommunity := map[string]int{"a": 0, "b": 1}
	score, reasons := SurpriseScore(g, "a", "b", "EXTRACTED", nodeCommunity, "code/model.py", "papers/flash.pdf")

	if score < 2 {
		t.Errorf("SurpriseScore(cross-type) = %d; want >= 2", score)
	}
	found := false
	for _, r := range reasons {
		if containsIgnoreCase(r, "file type") {
			found = true
			break
		}
	}
	if !found {
		t.Error("SurpriseScore should mention file type difference")
	}
}

func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		result[i] = c
	}
	return string(result)
}

func TestAnalyzeReturnsAllFields(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("cls", "MyClass", "class", "lib/src/models/my.py")
	g.AddNode("m1", ".process()", "method", "lib/src/models/my.py")
	g.AddNode("fn1", "helper()", "function", "lib/src/utils/helper.py")
	g.AddEdge("cls", "m1", "method", "EXTRACTED", 1.0)
	g.AddEdge("m1", "fn1", "calls", "EXTRACTED", 1.0)

	communities := map[int][]string{
		0: {"cls", "m1"},
		1: {"fn1"},
	}
	detection := DetectResultInfo{TotalFiles: 2, TotalWords: 500}
	analysis := Analyze(g, communities, detection)

	if analysis == nil {
		t.Fatal("Analyze() returned nil")
	}
	if analysis.Summary == "" {
		t.Error("Analyze() summary is empty")
	}
}

func TestAnalyzeCountsSingletons(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddNode("c", "C", "class", "c.py")

	communities := map[int][]string{
		0: {"a", "b"},
		1: {"c"},
	}
	detection := DetectResultInfo{TotalFiles: 3}
	analysis := Analyze(g, communities, detection)

	if analysis.SingletonCount != 1 {
		t.Errorf("SingletonCount = %d; want 1", analysis.SingletonCount)
	}
}

func TestIsFileNode(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("file1", "main.py", "file", "main.py")
	g.AddNode("cls1", "MyClass", "class", "main.py")

	if !isFileNode(g, "file1") {
		t.Error("isFileNode(file type) = false; want true")
	}
	if isFileNode(g, "cls1") {
		t.Error("isFileNode(class type) = true; want false")
	}
	if isFileNode(g, "nonexistent") {
		t.Error("isFileNode(missing) = true; want false")
	}
}

func TestIsConceptNode(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("mod", "os", "module", "")
	g.AddNode("cls", "MyClass", "class", "my.py")
	g.AddNode("nofile", "Mystery", "class", "")

	if !isConceptNode(g, "mod") {
		t.Error("isConceptNode(module) = false; want true")
	}
	if isConceptNode(g, "cls") {
		t.Error("isConceptNode(class with file) = true; want false")
	}
	if !isConceptNode(g, "nofile") {
		t.Error("isConceptNode(no file) = false; want true")
	}
}

func TestIsExternalType(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("ext", "Widget", "class", "")
	g.AddNode("child1", "MyWidget", "class", "my.dart")
	g.AddNode("child2", "OtherWidget", "class", "other.dart")
	g.AddEdge("child1", "ext", "inherits", "EXTRACTED", 1.0)
	g.AddEdge("child2", "ext", "inherits", "EXTRACTED", 1.0)

	if !isExternalType(g, "ext") {
		t.Error("isExternalType(Widget) = false; want true")
	}
	if isExternalType(g, "child1") {
		t.Error("isExternalType(MyWidget) = true; want false (has outgoing edges)")
	}
}

func TestTopLevelDir(t *testing.T) {
	tests := map[string]string{
		"model.py":          "model.py",
		"repo1/model.py":    "repo1",
		"a/b/c/file.py":     "a",
	}
	for path, expected := range tests {
		if got := TopLevelDir(path); got != expected {
			t.Errorf("TopLevelDir(%q) = %q; want %q", path, got, expected)
		}
	}
}
