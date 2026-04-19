package analyze

import (
	"strings"
	"testing"

	"github.com/sjhorn/graphify/pkg/graph"
)

func TestEnrichGodNodesParentClass(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("child", "ChildClass", "class", "child.py")
	g.AddNode("parent", "ParentClass", "class", "parent.py")
	g.AddEdge("child", "parent", "inherits", "EXTRACTED", 1.0)

	godNodes := []GodNode{{ID: "child", Label: "ChildClass", Degree: 5, File: "child.py"}}
	details := EnrichGodNodes(g, godNodes)

	if len(details) != 1 {
		t.Fatalf("EnrichGodNodes() returned %d details; want 1", len(details))
	}
	if details[0].ParentClass != "ParentClass" {
		t.Errorf("ParentClass = %q; want ParentClass", details[0].ParentClass)
	}
}

func TestEnrichGodNodesMethodCount(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("cls", "MyClass", "class", "my.py")
	g.AddNode("m1", ".method1()", "method", "my.py")
	g.AddNode("m2", ".method2()", "method", "my.py")
	g.AddEdge("cls", "m1", "method", "EXTRACTED", 1.0)
	g.AddEdge("cls", "m2", "method", "EXTRACTED", 1.0)

	godNodes := []GodNode{{ID: "cls", Label: "MyClass", Degree: 4, File: "my.py"}}
	details := EnrichGodNodes(g, godNodes)

	if details[0].MethodCount != 2 {
		t.Errorf("MethodCount = %d; want 2", details[0].MethodCount)
	}
}

func TestEnrichGodNodesInheritors(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("base", "Base", "class", "base.py")
	g.AddNode("sub1", "Sub1", "class", "sub1.py")
	g.AddNode("sub2", "Sub2", "class", "sub2.py")
	g.AddEdge("sub1", "base", "inherits", "EXTRACTED", 1.0)
	g.AddEdge("sub2", "base", "inherits", "EXTRACTED", 1.0)

	godNodes := []GodNode{{ID: "base", Label: "Base", Degree: 5, File: "base.py"}}
	details := EnrichGodNodes(g, godNodes)

	if details[0].InheritorCount != 2 {
		t.Errorf("InheritorCount = %d; want 2", details[0].InheritorCount)
	}
	if len(details[0].InheritorNames) != 2 {
		t.Errorf("InheritorNames = %d; want 2", len(details[0].InheritorNames))
	}
}

func TestBuildInheritanceTreesRequiresThreeInheritors(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("base", "Base", "class", "base.py")
	g.AddNode("sub1", "Sub1", "class", "sub1.py")
	g.AddNode("sub2", "Sub2", "class", "sub2.py")
	g.AddEdge("sub1", "base", "inherits", "EXTRACTED", 1.0)
	g.AddEdge("sub2", "base", "inherits", "EXTRACTED", 1.0)

	godNodes := []GodNode{{ID: "base", Label: "Base", Degree: 5, File: "base.py"}}
	trees := BuildInheritanceTrees(g, godNodes)

	// Only 2 inheritors — should not produce a tree
	if len(trees) != 0 {
		t.Errorf("BuildInheritanceTrees() returned %d trees; want 0 (need 3+ inheritors)", len(trees))
	}
}

func TestBuildInheritanceTreesWithThreeInheritors(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("base", "Base", "class", "base.py")
	for i := 0; i < 3; i++ {
		id := strings.Replace("sub_X", "X", string(rune('A'+i)), 1)
		label := strings.Replace("SubX", "X", string(rune('A'+i)), 1)
		g.AddNode(id, label, "class", label+".py")
		g.AddEdge(id, "base", "inherits", "EXTRACTED", 1.0)
	}

	godNodes := []GodNode{{ID: "base", Label: "Base", Degree: 6, File: "base.py"}}
	trees := BuildInheritanceTrees(g, godNodes)

	if len(trees) != 1 {
		t.Fatalf("BuildInheritanceTrees() = %d trees; want 1", len(trees))
	}
	if trees[0].Root != "Base" {
		t.Errorf("Root = %q; want Base", trees[0].Root)
	}
	if len(trees[0].Children) != 3 {
		t.Errorf("Children = %d; want 3", len(trees[0].Children))
	}
}

func TestBuildInheritanceTreesGrandchildren(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("base", "Base", "class", "base.py")
	g.AddNode("mid1", "Mid1", "class", "mid1.py")
	g.AddNode("mid2", "Mid2", "class", "mid2.py")
	g.AddNode("mid3", "Mid3", "class", "mid3.py")
	g.AddNode("gc1", "GrandChild1", "class", "gc1.py")
	g.AddEdge("mid1", "base", "inherits", "EXTRACTED", 1.0)
	g.AddEdge("mid2", "base", "inherits", "EXTRACTED", 1.0)
	g.AddEdge("mid3", "base", "inherits", "EXTRACTED", 1.0)
	g.AddEdge("gc1", "mid1", "inherits", "EXTRACTED", 1.0)

	godNodes := []GodNode{{ID: "base", Label: "Base", Degree: 6, File: "base.py"}}
	trees := BuildInheritanceTrees(g, godNodes)

	if len(trees) != 1 {
		t.Fatalf("BuildInheritanceTrees() = %d trees; want 1", len(trees))
	}
	if trees[0].Depth != 2 {
		t.Errorf("Depth = %d; want 2 (has grandchildren)", trees[0].Depth)
	}
}

func TestRenderInheritanceTree(t *testing.T) {
	tree := InheritanceTree{
		Root:     "Base",
		RootFile: "base.py",
		Children: []InheritanceChild{
			{Label: "Sub1", File: "sub1.py", MethodCount: 3},
			{Label: "Sub2", File: "sub2.py", MethodCount: 0, Children: []string{"GrandChild"}},
		},
		Depth: 2,
	}

	output := RenderInheritanceTree(tree)

	if !strings.Contains(output, "`Base`") {
		t.Error("should contain root label")
	}
	if !strings.Contains(output, "`Sub1`") {
		t.Error("should contain child label")
	}
	if !strings.Contains(output, "[3 methods]") {
		t.Error("should show method count")
	}
	if !strings.Contains(output, "`GrandChild`") {
		t.Error("should show grandchild")
	}
}

func TestRenderInheritanceTreeManyChildren(t *testing.T) {
	tree := InheritanceTree{
		Root:     "Big",
		RootFile: "big.py",
		Depth:    1,
	}
	for i := 0; i < 12; i++ {
		tree.Children = append(tree.Children, InheritanceChild{Label: strings.Replace("ChildX", "X", string(rune('A'+i)), 1), File: "c.py"})
	}

	output := RenderInheritanceTree(tree)
	if !strings.Contains(output, "+2 more") {
		t.Error("should truncate with +N more")
	}
}
