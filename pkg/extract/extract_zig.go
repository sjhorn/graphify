package extract

import (
	"fmt"
	"os"
	"path/filepath"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_zig "github.com/tree-sitter-grammars/tree-sitter-zig/bindings/go"
)

// ExtractZig extracts structs, functions, and imports from a Zig file.
func ExtractZig(path string) *Extraction {
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
	lang := tree_sitter.NewLanguage(tree_sitter_zig.Language())
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

	// Walk top-level AST nodes
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Kind() {
		case "variable_declaration":
			zigExtractVarDeclWithBody(child, content, imports, classes, fileNID, addNode, addEdge, &funcBodies)
		case "function_declaration":
			zigExtractFuncDeclWithBody(child, content, fileNID, addNode, addEdge, &funcBodies)
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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, zigCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func zigExtractVarDeclWithBody(node *tree_sitter.Node, content []byte, imports map[string]int, classes map[string]int,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	// variable_declaration → const/var + identifier + = + value
	var varName string
	var valueNode *tree_sitter.Node
	line := int(node.StartPosition().Row) + 1

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" && varName == "" {
			varName = string(child.Utf8Text(content))
		}
		if child.Kind() == "builtin_function" || child.Kind() == "field_expression" ||
			child.Kind() == "struct_declaration" || child.Kind() == "enum_declaration" ||
			child.Kind() == "union_declaration" {
			valueNode = child
		}
	}

	if varName == "" || valueNode == nil {
		return
	}

	switch valueNode.Kind() {
	case "builtin_function":
		// @import("std") → import
		if zigIsImport(valueNode, content) {
			imports[varName] = line
		}
	case "field_expression":
		// @import("std").mem → import
		if zigContainsImport(valueNode, content) {
			imports[varName] = line
		}
	case "struct_declaration":
		classes[varName] = line
		zigExtractStructFunctionsWithBody(valueNode, content, fileNID, addNode, addEdge, funcBodies)
	case "enum_declaration":
		classes[varName] = line
	case "union_declaration":
		classes[varName] = line
	}
}

func zigIsImport(node *tree_sitter.Node, content []byte) bool {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "builtin_identifier" && string(child.Utf8Text(content)) == "@import" {
			return true
		}
	}
	return false
}

func zigContainsImport(node *tree_sitter.Node, content []byte) bool {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "builtin_function" && zigIsImport(child, content) {
			return true
		}
	}
	return false
}

func zigExtractStructFunctionsWithBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "function_declaration" {
			zigExtractFuncDeclWithBody(child, content, fileNID, addNode, addEdge, funcBodies)
		}
	}
}

func zigExtractFuncDeclWithBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	var funcName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			funcName = string(child.Utf8Text(content))
			break
		}
	}
	if funcName == "" {
		return
	}

	line := int(node.StartPosition().Row) + 1
	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	// Find body - Zig function_declaration has a block child
	body := node.ChildByFieldName("body")
	if body == nil {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "block" {
				body = child
				break
			}
		}
	}
	if body != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, body})
	}
}

var zigKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true,
	"comptime": true, "struct": true, "enum": true, "union": true,
	"fn": true, "const": true, "var": true, "pub": true,
}

func zigCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "call_expression" {
		// call_expression → function_ref + arguments
		if node.ChildCount() > 0 {
			funcRef := node.Child(0)
			var callName string
			switch funcRef.Kind() {
			case "identifier":
				callName = string(funcRef.Utf8Text(content))
			case "field_expression":
				// e.g., std.debug.print → get last identifier
				callName = zigLastFieldName(funcRef, content)
			}
			if callName != "" && !zigKeywords[callName] {
				calls[callName] = true
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		zigCollectCalls(node.Child(i), content, calls)
	}
}

func zigLastFieldName(node *tree_sitter.Node, content []byte) string {
	var last string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			last = string(child.Utf8Text(content))
		}
	}
	return last
}
