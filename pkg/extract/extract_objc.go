package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhorn/graphify/internal/treesitter/objc"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractObjectiveC extracts classes, methods, and imports from an Objective-C file using tree-sitter AST parsing.
func ExtractObjectiveC(path string) *Extraction {
	content, err := os.ReadFile(path)
	if err != nil {
		return &Extraction{}
	}

	var nodes []Node
	var edges []Edge
	seenIDs := make(map[string]bool)

	strPath := path
	imports := make(map[string]int)
	classes := make(map[string]int)
	var funcBodies []funcBody

	fileNID := MakeId(strPath)
	addNode := func(nid, label string, line int) {
		if !seenIDs[nid] {
			seenIDs[nid] = true
			nodes = append(nodes, Node{
				ID:       nid,
				Label:    label,
				File:     strPath,
				Location: fmt.Sprintf("L%d", line),
			})
		}
	}

	addEdge := func(src, tgt, relation string, line int, confidence string, weight float64) {
		edges = append(edges, Edge{
			Source:     src,
			Target:     tgt,
			Relation:   relation,
			Confidence: confidence,
			Weight:     weight,
		})
	}

	addNode(fileNID, filepath.Base(path), 1)

	// Parse with tree-sitter
	parser := tree_sitter.NewParser()
	defer parser.Close()
	err = parser.SetLanguage(objc.GetLanguage())
	if err != nil {
		return &Extraction{}
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return &Extraction{}
	}
	defer tree.Close()
	root := tree.RootNode()

	// Walk AST
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Kind() {
		case "preproc_include":
			objcExtractImport(child, content, imports)
		case "class_interface":
			objcExtractClassInterfaceWithBody(child, content, classes, addNode, addEdge, &funcBodies)
		case "class_implementation":
			objcExtractClassImplWithBody(child, content, classes, addNode, addEdge, &funcBodies)
		}
	}

	// Add import nodes
	for imp, line := range imports {
		id := MakeId(imp)
		if !seenIDs[id] {
			seenIDs[id] = true
			nodes = append(nodes, Node{
				ID:       id,
				Label:    imp,
				File:     strPath,
				Location: fmt.Sprintf("L%d", line),
			})
		}
		addEdge(fileNID, id, "imports", line, "EXTRACTED", 1.0)
	}

	// Add class nodes
	for className, line := range classes {
		id := MakeId(className)
		if !seenIDs[id] {
			seenIDs[id] = true
			nodes = append(nodes, Node{
				ID:       id,
				Label:    className,
				File:     strPath,
				Location: fmt.Sprintf("L%d", line),
			})
		}
		addEdge(fileNID, id, "contains", line, "EXTRACTED", 1.0)
	}

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, objcCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func objcExtractImport(node *tree_sitter.Node, content []byte, imports map[string]int) {
	line := int(node.StartPosition().Row) + 1
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "system_lib_string":
			// e.g. <Foundation/Foundation.h>
			text := string(child.Utf8Text(content))
			text = strings.Trim(text, "<>")
			base := filepath.Base(text)
			base = strings.TrimSuffix(base, ".h")
			imports[base] = line
		case "string_literal":
			// e.g. "SampleDelegate.h" — find string_content inside
			for j := uint(0); j < child.ChildCount(); j++ {
				sc := child.Child(j)
				if sc.Kind() == "string_content" {
					text := string(sc.Utf8Text(content))
					base := filepath.Base(text)
					base = strings.TrimSuffix(base, ".h")
					imports[base] = line
				}
			}
		}
	}
}

func objcExtractClassInterfaceWithBody(node *tree_sitter.Node, content []byte, classes map[string]int,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	// The class name is the first identifier; the second identifier (after ":") is the superclass
	var className string
	var classNID string
	identCount := 0
	foundColon := false
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == ":" {
			foundColon = true
			continue
		}
		if child.Kind() == "identifier" {
			identCount++
			if identCount == 1 {
				className = string(child.Utf8Text(content))
				classNID = MakeId(className)
				classes[className] = line
			} else if identCount == 2 && foundColon {
				// Superclass reference
				superName := string(child.Utf8Text(content))
				superNID := MakeId(superName)
				addNode(superNID, superName, line)
				addEdge(classNID, superNID, "inherits", line, "EXTRACTED", 1.0)
			}
		}
		// Protocol conformance: parameterized_arguments contains type_name → type_identifier
		if child.Kind() == "parameterized_arguments" && className != "" {
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc.Kind() == "type_name" {
					for k := uint(0); k < gc.ChildCount(); k++ {
						ti := gc.Child(k)
						if ti.Kind() == "type_identifier" {
							protoName := string(ti.Utf8Text(content))
							protoNID := MakeId(protoName)
							addNode(protoNID, protoName, line)
							addEdge(classNID, protoNID, "inherits", line, "EXTRACTED", 1.0)
						}
					}
				}
			}
		}
		if child.Kind() == "method_declaration" && className != "" {
			objcExtractMethodWithBody(child, content, className, addNode, addEdge, funcBodies)
		}
	}
}

func objcExtractClassImplWithBody(node *tree_sitter.Node, content []byte, classes map[string]int,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var className string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			className = string(child.Utf8Text(content))
			classes[className] = line
			break
		}
	}
	if className == "" {
		return
	}
	// Walk for method definitions inside implementation
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "method_definition" {
			objcExtractMethodWithBody(child, content, className, addNode, addEdge, funcBodies)
		}
		if child.Kind() == "implementation_definition" {
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc.Kind() == "method_definition" {
					objcExtractMethodWithBody(gc, content, className, addNode, addEdge, funcBodies)
				}
			}
		}
	}
}

func objcExtractMethodWithBody(node *tree_sitter.Node, content []byte, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var methodName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			name := string(child.Utf8Text(content))
			if name != "void" && name != "instancetype" && name != "NSString" && name != "NSLog" {
				methodName = name
				break
			}
		}
	}
	if methodName == "" {
		return
	}

	classNID := MakeId(className)
	nid := MakeId(className, methodName)
	addNode(nid, "."+methodName+"()", line)
	addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)

	// Use the entire method node as body for call scanning
	*funcBodies = append(*funcBodies, funcBody{nid, node})
}

func objcCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}

	// Look for message_expression nodes: [receiver selector]
	if node.Kind() == "message_expression" {
		// Find the selector/identifier for the message
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "identifier" {
				name := string(child.Utf8Text(content))
				if name != "super" && name != "self" && name != "if" && name != "for" && name != "while" && name != "return" {
					calls[name] = true
				}
			}
		}
	}

	// Also look for call_expression
	if node.Kind() == "call_expression" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "identifier" {
				name := string(child.Utf8Text(content))
				calls[name] = true
			}
		}
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		objcCollectCalls(node.Child(i), content, calls)
	}
}
