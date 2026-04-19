package analyze

import (
	"strings"
	"testing"

	"github.com/sjhorn/graphify/pkg/graph"
)

func TestComputeDirectoryStats(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "ClassA", "class", "lib/src/widgets/a.dart")
	g.AddNode("b", "ClassB", "class", "lib/src/widgets/b.dart")
	g.AddNode("c", "ClassC", "class", "lib/src/models/c.dart")

	stats := ComputeDirectoryStats(g)
	if len(stats) == 0 {
		t.Fatal("expected directory stats")
	}
	// widgets should have 2 nodes
	if stats[0].Directory != "widgets" {
		t.Errorf("expected top dir to be widgets, got %s", stats[0].Directory)
	}
	if stats[0].NodeCount != 2 {
		t.Errorf("expected 2 nodes in widgets, got %d", stats[0].NodeCount)
	}
}

func TestGenerateSummary(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "ClassA", "class", "lib/src/widgets/a.dart")
	g.AddNode("b", "ClassB", "class", "lib/src/models/b.dart")
	g.AddEdge("a", "b", "imports", "EXTRACTED", 1.0)

	communities := map[int][]string{
		0: {"a", "b"},
	}
	godNodes := []GodNode{
		{ID: "a", Label: "ClassA", Degree: 5},
	}
	detection := DetectResultInfo{TotalFiles: 2, TotalWords: 500}
	layers := &LayerAnalysisResult{}

	summary := GenerateSummary(g, communities, godNodes, detection, nil, layers, nil)

	if !strings.Contains(summary, "2 files") {
		t.Error("summary should mention file count")
	}
	if !strings.Contains(summary, "ClassA") {
		t.Error("summary should mention core abstractions")
	}
	if !strings.Contains(summary, "clean layering") {
		t.Error("summary should mention clean layering")
	}
}

func TestGenerateSummaryWithCycles(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "A", "class", "a.dart")
	g.AddEdge("a", "a", "calls", "EXTRACTED", 1.0)

	layers := &LayerAnalysisResult{
		Cycles: [][]string{{"widgets", "models", "widgets"}},
	}
	detection := DetectResultInfo{TotalFiles: 1}

	summary := GenerateSummary(g, nil, nil, detection, nil, layers, nil)

	if !strings.Contains(summary, "1 dependency cycle") {
		t.Error("summary should mention cycles")
	}
}
