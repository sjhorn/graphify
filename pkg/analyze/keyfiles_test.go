package analyze

import (
	"testing"

	"github.com/sjhorn/graphify/pkg/graph"
)

func TestFindKeyFilesRankedByDegree(t *testing.T) {
	g := graph.NewGraph()
	// File 1: 3 nodes with high degree
	g.AddNode("a1", "ClassA", "class", "lib/src/widgets/widget.dart")
	g.AddNode("a2", ".build()", "method", "lib/src/widgets/widget.dart")
	g.AddNode("a3", ".render()", "method", "lib/src/widgets/widget.dart")
	g.AddEdge("a1", "a2", "method", "EXTRACTED", 1.0)
	g.AddEdge("a1", "a3", "method", "EXTRACTED", 1.0)
	// Add more edges to boost degree
	for i := 0; i < 10; i++ {
		callerID := "caller_" + string(rune('a'+i))
		g.AddNode(callerID, "Caller", "function", "lib/src/other/other.dart")
		g.AddEdge(callerID, "a1", "calls", "EXTRACTED", 1.0)
	}

	// File 2: 1 node with low degree
	g.AddNode("b1", "Helper", "function", "lib/src/utils/helper.dart")
	g.AddEdge("b1", "a1", "calls", "EXTRACTED", 1.0)

	files := FindKeyFiles(g, 5)
	if len(files) == 0 {
		t.Fatal("FindKeyFiles() returned empty")
	}
	if files[0].Path != "lib/src/widgets/widget.dart" {
		t.Errorf("top file = %q; want widget.dart", files[0].Path)
	}
}

func TestFindKeyFilesExcludesTestFiles(t *testing.T) {
	g := graph.NewGraph()
	for i := 0; i < 15; i++ {
		id := "t_" + string(rune('a'+i))
		g.AddNode(id, "TestFn", "function", "test/widget_test.dart")
	}
	// Add edges to boost degree
	for i := 0; i < 14; i++ {
		g.AddEdge("t_"+string(rune('a'+i)), "t_"+string(rune('a'+i+1)), "calls", "EXTRACTED", 1.0)
	}

	files := FindKeyFiles(g, 5)
	for _, f := range files {
		if f.Path == "test/widget_test.dart" {
			t.Error("FindKeyFiles() should exclude test files")
		}
	}
}

func TestFindKeyFilesLimitsToTopN(t *testing.T) {
	g := graph.NewGraph()
	for i := 0; i < 20; i++ {
		dir := "lib/src/pkg" + string(rune('a'+i))
		file := dir + "/file.dart"
		for j := 0; j < 15; j++ {
			id := dir + "_n" + string(rune('a'+j))
			g.AddNode(id, "Node", "class", file)
			if j > 0 {
				prev := dir + "_n" + string(rune('a'+j-1))
				g.AddEdge(prev, id, "calls", "EXTRACTED", 1.0)
			}
		}
	}

	files := FindKeyFiles(g, 3)
	if len(files) > 3 {
		t.Errorf("FindKeyFiles(3) returned %d; want <= 3", len(files))
	}
}

func TestFindRuntimeDeps(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("fn1", "process()", "function", "lib/src/proc/processor.dart")
	g.AddNode("fn2", "validate()", "function", "lib/src/val/validator.dart")
	g.AddNode("fn3", "log()", "function", "lib/src/log/logger.dart")
	g.AddEdge("fn1", "fn2", "calls", "EXTRACTED", 1.0)
	g.AddEdge("fn1", "fn3", "calls", "EXTRACTED", 1.0)

	deps := FindRuntimeDeps(g, 10)
	if len(deps) != 2 {
		t.Errorf("FindRuntimeDeps() = %d; want 2", len(deps))
	}
}

func TestFindRuntimeDepsSkipsSameFile(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "foo()", "function", "lib/src/same/file.dart")
	g.AddNode("b", "bar()", "function", "lib/src/same/file.dart")
	g.AddEdge("a", "b", "calls", "EXTRACTED", 1.0)

	deps := FindRuntimeDeps(g, 10)
	if len(deps) != 0 {
		t.Errorf("FindRuntimeDeps() = %d; want 0 (same file)", len(deps))
	}
}

func TestFindRuntimeDepsSkipsNonCalls(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "ClassA", "class", "lib/src/a/a.dart")
	g.AddNode("b", "ClassB", "class", "lib/src/b/b.dart")
	g.AddEdge("a", "b", "inherits", "EXTRACTED", 1.0)

	deps := FindRuntimeDeps(g, 10)
	if len(deps) != 0 {
		t.Errorf("FindRuntimeDeps() = %d; want 0 (inherits, not calls)", len(deps))
	}
}

func TestIsNonSourceFile(t *testing.T) {
	tests := map[string]bool{
		"test/widget_test.dart":         true,
		"benchmark/bench.dart":          true,
		"example/main.dart":             true,
		"lib/src/widget.dart":           false,
		"lib/src/generated/proto.dart":  true,
		"lib/src/model.dart":            false,
	}
	for path, expected := range tests {
		got := isNonSourceFile(path)
		if got != expected {
			t.Errorf("isNonSourceFile(%q) = %v; want %v", path, got, expected)
		}
	}
}
