package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
)

// ExtractC extracts functions and imports from a C file.
func ExtractC(path string) *Extraction {
	content, err := os.ReadFile(path)
	if err != nil {
		return &Extraction{}
	}

	var nodes []Node
	var edges []Edge
	seenIDs := make(map[string]bool)

	strPath := path

	imports := make(map[string]int)
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
	lang := tree_sitter.NewLanguage(tree_sitter_c.Language())
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
		case "preproc_include":
			cExtractInclude(child, content, imports)
		case "function_definition":
			cExtractFunctionDefWithBody(child, content, fileNID, addNode, addEdge, &funcBodies)
		case "declaration":
			cExtractFunctionDeclNode(child, content, fileNID, addNode, addEdge)
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

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, cCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func cExtractInclude(node *tree_sitter.Node, content []byte, imports map[string]int) {
	line := int(node.StartPosition().Row) + 1
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "system_lib_string" || child.Kind() == "string_literal" {
			headerPath := string(child.Utf8Text(content))
			// Strip <> or ""
			headerPath = strings.Trim(headerPath, "<>\"")
			// Get filename without extension
			base := strings.TrimSuffix(filepath.Base(headerPath), ".h")
			imports[base] = line
		}
	}
}

func cExtractFunctionDefWithBody(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	funcName := cFindFuncName(node, content)
	if funcName == "" {
		return
	}
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	body := node.ChildByFieldName("body")
	if body != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, body})
	}
}

func cExtractFunctionDeclNode(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	funcName := cFindFuncName(node, content)
	if funcName == "" {
		return
	}
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)
}

func cFindFuncName(node *tree_sitter.Node, content []byte) string {
	declarator := node.ChildByFieldName("declarator")
	if declarator == nil {
		return ""
	}
	return cExtractNameFromDeclarator(declarator, content)
}

func cExtractNameFromDeclarator(node *tree_sitter.Node, content []byte) string {
	switch node.Kind() {
	case "function_declarator":
		nameNode := node.ChildByFieldName("declarator")
		if nameNode != nil && nameNode.Kind() == "identifier" {
			return string(nameNode.Utf8Text(content))
		}
	case "pointer_declarator":
		// pointer_declarator → function_declarator
		inner := node.ChildByFieldName("declarator")
		if inner != nil {
			return cExtractNameFromDeclarator(inner, content)
		}
	}
	return ""
}

var cKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true,
	"sizeof": true, "typedef": true, "struct": true, "enum": true, "union": true,
}

func cCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "call_expression" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil && funcNode.Kind() == "identifier" {
			callName := string(funcNode.Utf8Text(content))
			if !cKeywords[callName] {
				calls[callName] = true
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		cCollectCalls(node.Child(i), content, calls)
	}
}
