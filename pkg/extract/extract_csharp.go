package extract

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjhorn/graphify/internal/treesitter/csharp"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractCSharp extracts classes, methods, and imports from a C# file using tree-sitter AST parsing.
func ExtractCSharp(path string) *Extraction {
	content, err := os.ReadFile(path)
	if err != nil {
		return &Extraction{}
	}

	var nodes []Node
	var edges []Edge
	seenIDs := make(map[string]bool)
	strPath := path

	fileNID := MakeId(strPath)
	addNode := func(nid, label string, line int) {
		if !seenIDs[nid] {
			seenIDs[nid] = true
			nodes = append(nodes, Node{
				ID: nid, Label: label, File: strPath,
				Location: fmt.Sprintf("L%d", line),
			})
		}
	}
	addEdge := func(src, tgt, relation string, line int, confidence string, weight float64) {
		edges = append(edges, Edge{
			Source: src, Target: tgt, Relation: relation,
			Confidence: confidence, Weight: weight,
		})
	}

	addNode(fileNID, filepath.Base(path), 1)

	// Parse with tree-sitter
	parser := tree_sitter.NewParser()
	defer parser.Close()
	err = parser.SetLanguage(csharp.GetLanguage())
	if err != nil {
		return &Extraction{}
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return &Extraction{}
	}
	defer tree.Close()
	root := tree.RootNode()

	var funcBodies []funcBody

	// Walk the AST
	csWalkTopLevel(root, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, csharpCollectCalls)...)

	return &Extraction{Nodes: nodes, Edges: edges}
}

func csWalkTopLevel(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "using_directive":
			csExtractUsing(child, content, fileNID, addNode, addEdge)
		case "namespace_declaration":
			csWalkNamespace(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		case "class_declaration":
			csExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		case "interface_declaration":
			csExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		}
	}
}

func csExtractUsing(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	line := int(node.StartPosition().Row) + 1
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "identifier":
			name := string(child.Utf8Text(content))
			nid := MakeId(name)
			addNode(nid, name, line)
			addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
		case "qualified_name":
			name := csharpLastIdentifier(child, content)
			if name != "" {
				nid := MakeId(name)
				addNode(nid, name, line)
				addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
			}
		}
	}
}

func csWalkNamespace(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1

	// Extract namespace name from identifier or qualified_name child
	var nsName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "identifier":
			nsName = string(child.Utf8Text(content))
		case "qualified_name":
			nsName = string(child.Utf8Text(content))
		}
		if nsName != "" {
			break
		}
	}

	// Create a node for the namespace and a file→namespace contains edge
	parentNID := fileNID
	if nsName != "" {
		nsNID := MakeId(nsName)
		addNode(nsNID, nsName, line)
		addEdge(fileNID, nsNID, "contains", line, "EXTRACTED", 1.0)
		parentNID = nsNID
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "declaration_list" {
			for j := uint(0); j < child.ChildCount(); j++ {
				member := child.Child(j)
				switch member.Kind() {
				case "class_declaration":
					csExtractClass(member, content, parentNID, seenIDs, addNode, addEdge, funcBodies)
				case "interface_declaration":
					csExtractClass(member, content, parentNID, seenIDs, addNode, addEdge, funcBodies)
				}
			}
		}
	}
}

func csExtractClass(node *tree_sitter.Node, content []byte, parentNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	className := string(nameNode.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1
	classNID := MakeId(className)
	addNode(classNID, className, line)
	addEdge(parentNID, classNID, "contains", line, "EXTRACTED", 1.0)

	// Inheritance: base_list → identifier/generic_name
	csExtractBaseList(node, content, classNID, line, addNode, addEdge)

	// Methods in declaration_list
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "declaration_list" {
			for j := uint(0); j < child.ChildCount(); j++ {
				member := child.Child(j)
				if member.Kind() == "method_declaration" {
					csExtractMethod(member, content, classNID, className, addNode, addEdge, funcBodies)
				}
			}
		}
	}
}

func csExtractBaseList(node *tree_sitter.Node, content []byte, classNID string, line int,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "base_list" {
			for j := uint(0); j < child.ChildCount(); j++ {
				baseChild := child.Child(j)
				switch baseChild.Kind() {
				case "identifier":
					baseName := string(baseChild.Utf8Text(content))
					baseNID := MakeId(baseName)
					addNode(baseNID, baseName, line)
					addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
				case "generic_name":
					nameNode := baseChild.ChildByFieldName("name")
					if nameNode == nil {
						// Fallback to first identifier
						for k := uint(0); k < baseChild.ChildCount(); k++ {
							gc := baseChild.Child(k)
							if gc.Kind() == "identifier" {
								baseName := string(gc.Utf8Text(content))
								baseNID := MakeId(baseName)
								addNode(baseNID, baseName, line)
								addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
								break
							}
						}
					} else {
						baseName := string(nameNode.Utf8Text(content))
						baseNID := MakeId(baseName)
						addNode(baseNID, baseName, line)
						addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
					}
				case "qualified_name":
					baseName := csharpLastIdentifier(baseChild, content)
					if baseName != "" {
						baseNID := MakeId(baseName)
						addNode(baseNID, baseName, line)
						addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
					}
				}
			}
		}
	}
}

func csExtractMethod(node *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	mName := node.ChildByFieldName("name")
	if mName == nil {
		return
	}
	funcName := string(mName.Utf8Text(content))
	// Skip constructors
	if funcName == className {
		return
	}
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(className, funcName)
	addNode(nid, "."+funcName+"()", line)
	addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)

	// Capture method body
	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
	}
}

func csharpLastIdentifier(node *tree_sitter.Node, content []byte) string {
	var last string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			last = string(child.Utf8Text(content))
		} else if child.Kind() == "qualified_name" {
			sub := csharpLastIdentifier(child, content)
			if sub != "" {
				last = sub
			}
		}
	}
	return last
}

func csharpCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "invocation_expression" {
		name := csharpGetCallName(node, content)
		if name != "" && !csharpIsKeyword(name) {
			calls[name] = true
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		csharpCollectCalls(node.Child(i), content, calls)
	}
}

func csharpGetCallName(node *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			return string(child.Utf8Text(content))
		}
		if child.Kind() == "member_access_expression" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				return string(nameNode.Utf8Text(content))
			}
		}
	}
	return ""
}

func csharpIsKeyword(name string) bool {
	keywords := map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"return": true, "class": true, "interface": true, "new": true, "foreach": true,
		"using": true, "void": true, "int": true, "string": true, "bool": true, "var": true,
	}
	return keywords[name]
}
