package analyze

import (
	"testing"

	"github.com/sjhorn/graphify/pkg/graph"
)

func TestDetectCommandPattern(t *testing.T) {
	g := graph.NewGraph()

	// Base command class
	g.AddNode("cmd", "Command", "class", "lib/src/commands/command.dart")
	g.AddNode("cmd_exec", "execute", "method", "lib/src/commands/command.dart")
	g.AddEdge("cmd", "cmd_exec", "method", "EXTRACTED", 1.0)

	// 3 concrete commands
	for _, name := range []string{"EditCommand", "DeleteCommand", "CopyCommand"} {
		id := "cmd_" + name
		g.AddNode(id, name, "class", "lib/src/commands/"+name+".dart")
		g.AddEdge(id, "cmd", "inherits", "EXTRACTED", 1.0)
		execID := id + "_exec"
		g.AddNode(execID, "execute", "method", "lib/src/commands/"+name+".dart")
		g.AddEdge(id, execID, "method", "EXTRACTED", 1.0)
	}

	patterns := DetectPatterns(g)

	found := false
	for _, p := range patterns {
		if p.Name == "Command" && p.AnchorNode == "Command" {
			found = true
			if p.Participants < 4 {
				t.Errorf("expected >= 4 participants, got %d", p.Participants)
			}
		}
	}
	if !found {
		t.Error("expected Command pattern to be detected")
	}
}

func TestDetectBuilderFactoryPattern(t *testing.T) {
	g := graph.NewGraph()

	// Base builder class
	g.AddNode("builder", "ComponentBuilder", "class", "lib/src/builders/builder.dart")
	g.AddNode("builder_create", "createComponent", "method", "lib/src/builders/builder.dart")
	g.AddNode("builder_build", "buildLayout", "method", "lib/src/builders/builder.dart")
	g.AddEdge("builder", "builder_create", "method", "EXTRACTED", 1.0)
	g.AddEdge("builder", "builder_build", "method", "EXTRACTED", 1.0)

	// 2 concrete builders
	for _, name := range []string{"TextBuilder", "ImageBuilder"} {
		id := "builder_" + name
		g.AddNode(id, name, "class", "lib/src/builders/"+name+".dart")
		g.AddEdge(id, "builder", "inherits", "EXTRACTED", 1.0)
	}

	patterns := DetectPatterns(g)

	found := false
	for _, p := range patterns {
		if p.Name == "Builder/Factory" && p.AnchorNode == "ComponentBuilder" {
			found = true
			if p.Participants < 3 {
				t.Errorf("expected >= 3 participants, got %d", p.Participants)
			}
		}
	}
	if !found {
		t.Error("expected Builder/Factory pattern to be detected")
	}
}

func TestDetectObserverPattern(t *testing.T) {
	g := graph.NewGraph()

	g.AddNode("editor", "Editor", "class", "lib/src/editor/editor.dart")
	g.AddNode("addListener", "addListener", "method", "lib/src/editor/editor.dart")
	g.AddNode("removeListener", "removeListener", "method", "lib/src/editor/editor.dart")
	g.AddNode("notifyAll", "notifyAll", "method", "lib/src/editor/editor.dart")
	g.AddEdge("editor", "addListener", "method", "EXTRACTED", 1.0)
	g.AddEdge("editor", "removeListener", "method", "EXTRACTED", 1.0)
	g.AddEdge("editor", "notifyAll", "method", "EXTRACTED", 1.0)

	patterns := DetectPatterns(g)

	found := false
	for _, p := range patterns {
		if p.Name == "Observer" && p.AnchorNode == "Editor" {
			found = true
		}
	}
	if !found {
		t.Error("expected Observer pattern to be detected")
	}
}

func TestDetectStrategyPattern(t *testing.T) {
	g := graph.NewGraph()

	// 4 classes that share the same 3-method interface but are not in an inheritance tree
	for _, name := range []string{"PenTool", "BrushTool", "EraserTool", "FillTool"} {
		id := "tool_" + name
		g.AddNode(id, name, "class", "lib/src/tools/"+name+".dart")
		for _, method := range []string{"activate", "deactivate", "onPointerDown"} {
			mid := id + "_" + method
			g.AddNode(mid, "."+method+"()", "method", "lib/src/tools/"+name+".dart")
			g.AddEdge(id, mid, "method", "EXTRACTED", 1.0)
		}
	}

	patterns := DetectPatterns(g)

	found := false
	for _, p := range patterns {
		if p.Name == "Strategy" {
			found = true
			if p.Participants < 4 {
				t.Errorf("expected >= 4 participants, got %d", p.Participants)
			}
		}
	}
	if !found {
		t.Error("expected Strategy pattern to be detected")
	}
}

func TestUniqueStrings(t *testing.T) {
	result := uniqueStrings([]string{"b", "a", "b", "c", "a"})
	if len(result) != 3 {
		t.Errorf("uniqueStrings() = %d; want 3", len(result))
	}
	// Should be sorted
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("uniqueStrings() = %v; want [a b c]", result)
	}
}

func TestTruncateNamesShort(t *testing.T) {
	result := truncateNames([]string{"a", "b"}, 5)
	if len(result) != 2 {
		t.Errorf("truncateNames(2, 5) = %d; want 2", len(result))
	}
}

func TestTruncateNamesLong(t *testing.T) {
	result := truncateNames([]string{"a", "b", "c", "d", "e", "f"}, 3)
	if len(result) != 4 { // 3 names + "(+3 more)"
		t.Errorf("truncateNames(6, 3) = %d; want 4", len(result))
	}
	if result[3] != "(+3 more)" {
		t.Errorf("last element = %q; want (+3 more)", result[3])
	}
}

func TestNormalizeMethodName(t *testing.T) {
	tests := map[string]string{
		".build()":  "build",
		"process()": "process",
		".foo":      "foo",
		"bar":       "bar",
	}
	for input, expected := range tests {
		got := normalizeMethodName(input)
		if got != expected {
			t.Errorf("normalizeMethodName(%q) = %q; want %q", input, got, expected)
		}
	}
}

func TestMatchesAny(t *testing.T) {
	methods := []string{".execute()", ".validate()", ".render()"}
	if matchesAny(methods, []string{"run", "handle"}) != "" {
		t.Error("matchesAny should return empty for no match")
	}
	if matchesAny(methods, []string{"execute"}) != "execute" {
		t.Error("matchesAny should match execute")
	}
}

func TestMatchesPrefix(t *testing.T) {
	methods := []string{".createWidget()", ".buildLayout()", ".render()"}
	matched := matchesPrefix(methods, []string{"create", "build"})
	if len(matched) != 2 {
		t.Errorf("matchesPrefix() = %d; want 2", len(matched))
	}
}

func TestDetectPatternsNoFalsePositives(t *testing.T) {
	g := graph.NewGraph()

	// A simple class with no pattern indicators
	g.AddNode("util", "StringUtil", "class", "lib/src/utils/string_util.dart")
	g.AddNode("trim", "trim", "method", "lib/src/utils/string_util.dart")
	g.AddEdge("util", "trim", "method", "EXTRACTED", 1.0)

	patterns := DetectPatterns(g)
	if len(patterns) != 0 {
		t.Errorf("expected no patterns, got %d: %v", len(patterns), patterns)
	}
}
