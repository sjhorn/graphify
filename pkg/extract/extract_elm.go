package extract

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjhorn/graphify/internal/treesitter/elm"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractElm extracts functions, types, and imports from an Elm file using tree-sitter AST parsing.
func ExtractElm(path string) *Extraction {
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
	parser := tree_sitter.NewParser()
	defer parser.Close()
	err = parser.SetLanguage(elm.GetLanguage())
	if err != nil {
		return &Extraction{}
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return &Extraction{}
	}
	defer tree.Close()
	root := tree.RootNode()

	// Walk the AST
	elmWalkTopLevelWithBody(root, content, imports, fileNID, addNode, addEdge, &funcBodies)

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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, elmCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func elmWalkTopLevelWithBody(node *tree_sitter.Node, content []byte, imports map[string]int,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	// Track function names seen via type_annotation so value_declaration doesn't duplicate
	funcsSeen := make(map[string]bool)

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "import_clause":
			elmExtractImport(child, content, imports)
		case "type_annotation":
			elmExtractTypeAnnotationWithBody(child, content, fileNID, addNode, addEdge, funcsSeen)
		case "value_declaration":
			elmExtractValueDeclarationWithBody(child, content, fileNID, addNode, addEdge, funcsSeen, funcBodies)
		}
	}
}

func elmExtractImport(node *tree_sitter.Node, content []byte, imports map[string]int) {
	line := int(node.StartPosition().Row) + 1
	// Find upper_case_qid which contains the module name (e.g., "Html.Events")
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "upper_case_qid" {
			moduleName := string(child.Utf8Text(content))
			imports[moduleName] = line
			return
		}
	}
}

func elmExtractTypeAnnotationWithBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcsSeen map[string]bool) {

	line := int(node.StartPosition().Row) + 1
	// First child should be lower_case_identifier (function name)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "lower_case_identifier" {
			funcName := string(child.Utf8Text(content))
			if !elmIsKeyword(funcName) {
				funcsSeen[funcName] = true
				nid := MakeId(funcName)
				addNode(nid, funcName+"()", line)
				addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)
			}
			return
		}
	}
}

func elmExtractValueDeclarationWithBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcsSeen map[string]bool, funcBodies *[]funcBody) {

	// Look for function_declaration_left → lower_case_identifier
	var funcName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "function_declaration_left" {
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub.Kind() == "lower_case_identifier" {
					funcName = string(sub.Utf8Text(content))
					break
				}
			}
			break
		}
	}

	if funcName == "" || elmIsKeyword(funcName) {
		return
	}

	nid := MakeId(funcName)

	// Only add if not already found via type_annotation
	if !funcsSeen[funcName] {
		line := int(node.StartPosition().Row) + 1
		addNode(nid, funcName+"()", line)
		addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)
	}

	// Use the entire value_declaration node as body for call scanning
	*funcBodies = append(*funcBodies, funcBody{nid, node})
}

func elmCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "function_call_expr" {
		// Get function name from first child
		if node.ChildCount() > 0 {
			first := node.Child(0)
			name := elmGetCallName(first, content)
			if name != "" && !elmIsKeyword(name) {
				calls[name] = true
			}
		}
	}
	// Also catch record field references like { init = init, view = view }
	if node.Kind() == "field" {
		// field → lower_case_identifier = expression
		// The expression value could reference a function
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "value_expr" || child.Kind() == "lower_case_identifier" {
				name := elmGetCallName(child, content)
				if name != "" && !elmIsKeyword(name) {
					calls[name] = true
				}
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		elmCollectCalls(node.Child(i), content, calls)
	}
}

func elmGetCallName(node *tree_sitter.Node, content []byte) string {
	if node.Kind() == "lower_case_identifier" {
		return string(node.Utf8Text(content))
	}
	if node.Kind() == "value_expr" {
		// Look for value_qid → lower_case_identifier
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "value_qid" {
				// Get the last lower_case_identifier
				var last string
				for j := uint(0); j < child.ChildCount(); j++ {
					sub := child.Child(j)
					if sub.Kind() == "lower_case_identifier" {
						last = string(sub.Utf8Text(content))
					}
				}
				return last
			}
			if child.Kind() == "lower_case_identifier" {
				return string(child.Utf8Text(content))
			}
			if child.Kind() == "upper_case_qid" {
				return string(child.Utf8Text(content))
			}
		}
	}
	// Recurse to find identifier
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "lower_case_identifier" {
			return string(child.Utf8Text(content))
		}
	}
	return ""
}

func elmIsKeyword(name string) bool {
	keywords := []string{
		"if", "then", "else", "case", "of", "let",
		"in", "where", "type", "module", "import",
		"exposing", "as", "port",
	}
	for _, kw := range keywords {
		if name == kw {
			return true
		}
	}
	return false
}
