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

func TestDetectCyclesFindsSimpleCycle(t *testing.T) {
	adj := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	}
	allDirs := map[string]bool{"a": true, "b": true, "c": true}

	cycles := detectCycles(adj, allDirs)
	if len(cycles) == 0 {
		t.Error("detectCycles() should find the a->b->c->a cycle")
	}
}

func TestDetectCyclesNoCycle(t *testing.T) {
	adj := map[string][]string{
		"a": {"b"},
		"b": {"c"},
	}
	allDirs := map[string]bool{"a": true, "b": true, "c": true}

	cycles := detectCycles(adj, allDirs)
	if len(cycles) != 0 {
		t.Errorf("detectCycles() = %d; want 0 (no cycles)", len(cycles))
	}
}

func TestTopoSortLinear(t *testing.T) {
	adj := map[string][]string{
		"a": {"b"},
		"b": {"c"},
	}
	allDirs := map[string]bool{"a": true, "b": true, "c": true}

	order := topoSort(adj, allDirs)
	if len(order) != 3 {
		t.Fatalf("topoSort() = %d; want 3", len(order))
	}
	// "a" should come before "b", "b" before "c"
	aIdx, bIdx, cIdx := -1, -1, -1
	for i, d := range order {
		switch d {
		case "a":
			aIdx = i
		case "b":
			bIdx = i
		case "c":
			cIdx = i
		}
	}
	if aIdx > bIdx || bIdx > cIdx {
		t.Errorf("topoSort() = %v; want a before b before c", order)
	}
}

func TestTopoSortWithCycle(t *testing.T) {
	adj := map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}
	allDirs := map[string]bool{"a": true, "b": true}

	order := topoSort(adj, allDirs)
	if len(order) != 2 {
		t.Fatalf("topoSort() = %d; want 2 (includes cycle nodes)", len(order))
	}
}

func TestNormalizeCycle(t *testing.T) {
	cycle1 := []string{"b", "c", "a", "b"}
	cycle2 := []string{"a", "b", "c", "a"}
	// Both represent the same cycle, should normalize the same
	if normalizeCycle(cycle1) != normalizeCycle(cycle2) {
		t.Errorf("normalizeCycle should produce same result for rotated cycles: %q vs %q",
			normalizeCycle(cycle1), normalizeCycle(cycle2))
	}
}

func TestAnalyzeLayersSourceDepsExcludeTestDirs(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "Widget", "class", "lib/src/widgets/widget.dart")
	g.AddNode("b", "WidgetTest", "class", "test/widget_test.dart")
	g.AddEdge("b", "a", "imports", "EXTRACTED", 1.0)

	result := AnalyzeLayers(g)
	if len(result.SourceDeps) != 0 {
		t.Errorf("SourceDeps should exclude test dirs, got %d", len(result.SourceDeps))
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
