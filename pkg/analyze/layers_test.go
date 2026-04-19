package analyze

import (
	"testing"

	"github.com/sjhorn/graphify/pkg/graph"
)

func TestAnalyzeLayersLinearDeps(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a1", "WidgetA", "class", "lib/src/widgets/widget_a.dart")
	g.AddNode("b1", "ModelB", "class", "lib/src/models/model_b.dart")
	g.AddNode("c1", "ServiceC", "class", "lib/src/services/service_c.dart")

	g.AddEdge("a1", "b1", "imports", "EXTRACTED", 1.0)
	g.AddEdge("b1", "c1", "imports", "EXTRACTED", 1.0)

	result := AnalyzeLayers(g)

	if len(result.Cycles) != 0 {
		t.Errorf("expected no cycles, got %d: %v", len(result.Cycles), result.Cycles)
	}
	if len(result.DirectoryDeps) == 0 {
		t.Error("expected directory deps")
	}
	if len(result.LayerOrder) == 0 {
		t.Error("expected layer order")
	}
	if len(result.Violations) != 0 {
		t.Errorf("expected no violations, got %v", result.Violations)
	}
}

func TestAnalyzeLayersCyclicDeps(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a1", "WidgetA", "class", "lib/src/widgets/widget_a.dart")
	g.AddNode("b1", "ModelB", "class", "lib/src/models/model_b.dart")
	g.AddNode("c1", "ServiceC", "class", "lib/src/services/service_c.dart")

	g.AddEdge("a1", "b1", "imports", "EXTRACTED", 1.0)
	g.AddEdge("b1", "c1", "calls", "EXTRACTED", 1.0)
	g.AddEdge("c1", "a1", "imports", "EXTRACTED", 1.0)

	result := AnalyzeLayers(g)

	if len(result.Cycles) == 0 {
		t.Error("expected at least one cycle")
	}
	if len(result.Violations) == 0 {
		t.Error("expected violations for cycles")
	}
}

func TestAnalyzeLayersSameDirectorySkipped(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a1", "WidgetA", "class", "lib/src/widgets/widget_a.dart")
	g.AddNode("a2", "WidgetB", "class", "lib/src/widgets/widget_b.dart")

	g.AddEdge("a1", "a2", "imports", "EXTRACTED", 1.0)

	result := AnalyzeLayers(g)

	if len(result.DirectoryDeps) != 0 {
		t.Errorf("expected no cross-directory deps, got %d", len(result.DirectoryDeps))
	}
}

func TestSecondLevelDir(t *testing.T) {
	tests := map[string]string{
		"lib/src/widgets/button.dart": "widgets",
		"src/models/user.go":          "models",
		"pkg/graph/graph.go":          "graph",
		"main.go":                     "",
	}
	for path, expected := range tests {
		got := secondLevelDir(path)
		if got != expected {
			t.Errorf("secondLevelDir(%q) = %q; want %q", path, got, expected)
		}
	}
}
