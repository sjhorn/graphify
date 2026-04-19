package extract

import (
	"fmt"
	"os"
	"path/filepath"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// ExtractJava extracts classes, methods, and imports from a Java file.
func ExtractJava(path string) *Extraction {
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
	lang := tree_sitter.NewLanguage(tree_sitter_java.Language())
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		return &Extraction{}
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return &Extraction{}
	}
	defer tree.Close()
	root := tree.RootNode()

	var funcBodies []funcBody

	// Walk top-level AST nodes
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Kind() {
		case "import_declaration":
			javaExtractImport(child, content, fileNID, addNode, addEdge)
		case "class_declaration":
			javaExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "interface_declaration":
			javaExtractInterface(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "enum_declaration":
			javaExtractEnum(child, content, fileNID, addNode, addEdge)
		}
	}

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, javaCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func javaExtractImport(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	line := int(node.StartPosition().Row) + 1
	last := javaLastIdentifier(node, content)
	if last != "" {
		nid := MakeId(last)
		addNode(nid, last, line)
		addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
	}
}

func javaLastIdentifier(node *tree_sitter.Node, content []byte) string {
	// Walk scoped_identifier chains to find the last identifier
	var last string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "identifier":
			last = string(child.Utf8Text(content))
		case "scoped_identifier":
			if found := javaLastIdentifier(child, content); found != "" {
				last = found
			}
		}
	}
	return last
}

func javaExtractClass(node *tree_sitter.Node, content []byte, parentNID string,
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

	// Inheritance
	javaExtractSuperclass(node, content, classNID, line, addNode, addEdge)
	javaExtractInterfaces(node, content, classNID, line, addNode, addEdge)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		javaExtractMethods(bodyNode, content, classNID, className, addNode, addEdge, funcBodies)
	}
}

func javaExtractInterface(node *tree_sitter.Node, content []byte, parentNID string,
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

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		javaExtractMethods(bodyNode, content, classNID, className, addNode, addEdge, funcBodies)
	}
}

func javaExtractEnum(node *tree_sitter.Node, content []byte, parentNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	enumName := string(nameNode.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1
	enumNID := MakeId(enumName)
	addNode(enumNID, enumName, line)
	addEdge(parentNID, enumNID, "contains", line, "EXTRACTED", 1.0)
}

func javaExtractMethods(bodyNode *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(i)
		switch child.Kind() {
		case "method_declaration":
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				funcName := string(nameNode.Utf8Text(content))
				line := int(child.StartPosition().Row) + 1
				nid := MakeId(className, funcName)
				addNode(nid, "."+funcName+"()", line)
				addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)

				body := child.ChildByFieldName("body")
				if body != nil {
					*funcBodies = append(*funcBodies, funcBody{nid, body})
				}
			}
		case "constructor_declaration":
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				funcName := string(nameNode.Utf8Text(content))
				line := int(child.StartPosition().Row) + 1
				nid := MakeId(className, funcName)
				addNode(nid, "."+funcName+"()", line)
				addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)

				body := child.ChildByFieldName("body")
				if body != nil {
					*funcBodies = append(*funcBodies, funcBody{nid, body})
				}
			}
		}
	}
}

func javaExtractSuperclass(node *tree_sitter.Node, content []byte, classNID string, line int,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	sup := node.ChildByFieldName("superclass")
	if sup == nil {
		return
	}
	for i := uint(0); i < sup.ChildCount(); i++ {
		child := sup.Child(i)
		if child.Kind() == "type_identifier" {
			baseName := string(child.Utf8Text(content))
			baseNID := MakeId(baseName)
			addNode(baseNID, baseName, line)
			addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
		}
	}
}

func javaExtractInterfaces(node *tree_sitter.Node, content []byte, classNID string, line int,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	ifaces := node.ChildByFieldName("interfaces")
	if ifaces == nil {
		return
	}
	for i := uint(0); i < ifaces.ChildCount(); i++ {
		child := ifaces.Child(i)
		if child.Kind() == "type_list" {
			for j := uint(0); j < child.ChildCount(); j++ {
				tc := child.Child(j)
				if tc.Kind() == "type_identifier" {
					baseName := string(tc.Utf8Text(content))
					baseNID := MakeId(baseName)
					addNode(baseNID, baseName, line)
					addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
				}
			}
		}
	}
}

var javaKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"new": true, "return": true, "class": true, "interface": true, "throw": true,
	"super": true, "this": true,
}

func javaCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "method_invocation" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			callName := string(nameNode.Utf8Text(content))
			if !javaKeywords[callName] {
				calls[callName] = true
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		javaCollectCalls(node.Child(i), content, calls)
	}
}
