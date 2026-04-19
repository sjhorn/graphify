package analyze

import (
	"testing"

	"github.com/sjhorn/graphify/pkg/graph"
)

func TestDetectEnumsFindsEnums(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("color", "Color", "enum", "colors.dart")
	g.AddNode("red", "Red", "enum_value", "colors.dart")
	g.AddNode("green", "Green", "enum_value", "colors.dart")
	g.AddNode("blue", "Blue", "enum_value", "colors.dart")
	g.AddEdge("color", "red", "case_of", "EXTRACTED", 1.0)
	g.AddEdge("color", "green", "case_of", "EXTRACTED", 1.0)
	g.AddEdge("color", "blue", "case_of", "EXTRACTED", 1.0)

	enums := DetectEnums(g)
	if len(enums) != 1 {
		t.Fatalf("DetectEnums() = %d; want 1", len(enums))
	}
	if enums[0].Label != "Color" {
		t.Errorf("Label = %q; want Color", enums[0].Label)
	}
	if enums[0].CaseCount != 3 {
		t.Errorf("CaseCount = %d; want 3", enums[0].CaseCount)
	}
}

func TestDetectEnumsFiltersTooFew(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("small", "SmallEnum", "enum", "small.dart")
	g.AddNode("val1", "Val1", "enum_value", "small.dart")
	g.AddNode("val2", "Val2", "enum_value", "small.dart")
	g.AddEdge("small", "val1", "case_of", "EXTRACTED", 1.0)
	g.AddEdge("small", "val2", "case_of", "EXTRACTED", 1.0)

	enums := DetectEnums(g)
	if len(enums) != 0 {
		t.Errorf("DetectEnums() = %d; want 0 (fewer than 3 cases)", len(enums))
	}
}

func TestDetectEnumsSortedByCaseCount(t *testing.T) {
	g := graph.NewGraph()

	// Enum with 4 cases
	g.AddNode("big", "BigEnum", "enum", "big.dart")
	for _, c := range []string{"A", "B", "C", "D"} {
		id := "big_" + c
		g.AddNode(id, c, "enum_value", "big.dart")
		g.AddEdge("big", id, "case_of", "EXTRACTED", 1.0)
	}

	// Enum with 3 cases
	g.AddNode("small", "SmallEnum", "enum", "small.dart")
	for _, c := range []string{"X", "Y", "Z"} {
		id := "small_" + c
		g.AddNode(id, c, "enum_value", "small.dart")
		g.AddEdge("small", id, "case_of", "EXTRACTED", 1.0)
	}

	enums := DetectEnums(g)
	if len(enums) != 2 {
		t.Fatalf("DetectEnums() = %d; want 2", len(enums))
	}
	if enums[0].CaseCount < enums[1].CaseCount {
		t.Error("DetectEnums() should be sorted by CaseCount descending")
	}
}

func TestDetectEnumsSkipsNonEnum(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("cls", "MyClass", "class", "my.py")
	g.AddNode("m1", "method1", "method", "my.py")
	g.AddEdge("cls", "m1", "case_of", "EXTRACTED", 1.0) // wrong type

	enums := DetectEnums(g)
	if len(enums) != 0 {
		t.Errorf("DetectEnums() = %d; want 0 (not an enum type)", len(enums))
	}
}
