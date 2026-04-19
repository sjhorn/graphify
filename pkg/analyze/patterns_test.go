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
