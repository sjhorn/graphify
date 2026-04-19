package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

// ExtractJavaScript extracts functions, classes, and imports from a JavaScript/TypeScript file.
func ExtractJavaScript(path string) *Extraction {
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
	lang := tree_sitter.NewLanguage(tree_sitter_javascript.Language())
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
		case "import_statement":
			jsExtractImport(child, content, fileNID, addNode, addEdge)
		case "class_declaration":
			jsExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "function_declaration":
			jsExtractTopFunction(child, content, fileNID, addNode, addEdge, &funcBodies)
		case "export_statement":
			// Export can wrap class/function declarations
			for j := uint(0); j < child.ChildCount(); j++ {
				inner := child.Child(j)
				switch inner.Kind() {
				case "class_declaration":
					jsExtractClass(inner, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
				case "function_declaration":
					jsExtractTopFunction(inner, content, fileNID, addNode, addEdge, &funcBodies)
				}
			}
		case "lexical_declaration", "variable_declaration":
			jsExtractVarDecl(child, content, fileNID, addNode, addEdge, &funcBodies)
		}
	}

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, jsCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func jsExtractImport(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	line := int(node.StartPosition().Row) + 1
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "string" {
			modulePath := jsStringContent(child, content)
			if modulePath != "" {
				nid := MakeId(modulePath)
				addNode(nid, modulePath, line)
				addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
			}
		}
	}
}

func jsStringContent(node *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "string_fragment" {
			return string(child.Utf8Text(content))
		}
	}
	// Fallback: strip quotes from the whole string
	s := string(node.Utf8Text(content))
	s = strings.Trim(s, "'\"")
	return s
}

func jsExtractClass(node *tree_sitter.Node, content []byte, parentNID string,
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

	// Extract methods from class body
	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		for i := uint(0); i < bodyNode.ChildCount(); i++ {
			child := bodyNode.Child(i)
			if child.Kind() == "method_definition" {
				nameChild := child.ChildByFieldName("name")
				if nameChild != nil {
					funcName := string(nameChild.Utf8Text(content))
					methodLine := int(child.StartPosition().Row) + 1
					nid := MakeId(className, funcName)
					addNode(nid, "."+funcName+"()", methodLine)
					addEdge(classNID, nid, "method", methodLine, "EXTRACTED", 1.0)

					body := child.ChildByFieldName("body")
					if body != nil {
						*funcBodies = append(*funcBodies, funcBody{nid, body})
					}
				}
			}
		}
	}
}

func jsExtractTopFunction(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	funcName := string(nameNode.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	body := node.ChildByFieldName("body")
	if body != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, body})
	}
}

func jsExtractVarDecl(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	// const foo = (...) => { ... }
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "variable_declarator" {
			nameNode := child.ChildByFieldName("name")
			valueNode := child.ChildByFieldName("value")
			if nameNode != nil && valueNode != nil && valueNode.Kind() == "arrow_function" {
				funcName := string(nameNode.Utf8Text(content))
				line := int(node.StartPosition().Row) + 1
				nid := MakeId(funcName)
				addNode(nid, funcName+"()", line)
				addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

				body := valueNode.ChildByFieldName("body")
				if body != nil {
					*funcBodies = append(*funcBodies, funcBody{nid, body})
				}
			}
		}
	}
}

var jsKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"return": true, "class": true, "interface": true, "function": true,
	"console": true, "require": true, "new": true, "typeof": true, "delete": true,
}

func jsCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "call_expression" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			var callName string
			switch funcNode.Kind() {
			case "identifier":
				callName = string(funcNode.Utf8Text(content))
			case "member_expression":
				prop := funcNode.ChildByFieldName("property")
				if prop != nil {
					callName = string(prop.Utf8Text(content))
				}
			}
			if callName != "" && !jsKeywords[callName] {
				calls[callName] = true
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		jsCollectCalls(node.Child(i), content, calls)
	}
}
