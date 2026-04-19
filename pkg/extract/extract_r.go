package extract

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjhorn/graphify/internal/treesitter/r"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractR extracts functions, methods, and imports from an R file using tree-sitter AST parsing.
func ExtractR(path string) *Extraction {
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
	err = parser.SetLanguage(r.GetLanguage())
	if err != nil {
		return &Extraction{}
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return &Extraction{}
	}
	defer tree.Close()
	root := tree.RootNode()

	// Walk AST top-level
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Kind() {
		case "call":
			rExtractLibraryOrCall(child, content, imports)
		case "binary_operator":
			rExtractFunctionDefWithBody(child, content, fileNID, addNode, addEdge, &funcBodies)
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

	// Add class nodes (with "()" suffix)
	for className, line := range classes {
		id := MakeId(className)
		if !seenIDs[id] {
			seenIDs[id] = true
			nodes = append(nodes, Node{
				ID:       id,
				Label:    className + "()",
				File:     strPath,
				Location: fmt.Sprintf("L%d", line),
			})
		}
		addEdge(fileNID, id, "contains", line, "EXTRACTED", 1.0)
	}

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, rCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func rExtractLibraryOrCall(node *tree_sitter.Node, content []byte, imports map[string]int) {
	// call node: first child is identifier (function name), then arguments
	if node.ChildCount() < 2 {
		return
	}
	funcChild := node.Child(0)
	if funcChild == nil || funcChild.Kind() != "identifier" {
		return
	}
	funcName := string(funcChild.Utf8Text(content))
	if funcName != "library" && funcName != "require" {
		return
	}

	line := int(node.StartPosition().Row) + 1
	// Get the arguments node
	argsNode := node.Child(1)
	if argsNode == nil {
		return
	}
	// Walk arguments to find the package name
	for i := uint(0); i < argsNode.ChildCount(); i++ {
		child := argsNode.Child(i)
		if child.Kind() == "argument" {
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc.Kind() == "identifier" {
					imports[string(gc.Utf8Text(content))] = line
					return
				}
			}
		}
		if child.Kind() == "identifier" {
			imports[string(child.Utf8Text(content))] = line
			return
		}
	}
}

func rExtractFunctionDefWithBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	// binary_operator: left <- right
	// Check if operator is <- and right side is function_definition
	if node.ChildCount() < 3 {
		return
	}

	left := node.Child(0)
	right := node.Child(node.ChildCount() - 1)

	if left == nil || right == nil {
		return
	}

	if left.Kind() != "identifier" {
		return
	}

	if right.Kind() != "function_definition" {
		return
	}

	funcName := string(left.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1

	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	// Use the function_definition node (right side) as body for call scanning
	// Look for the body child within the function_definition
	body := right.ChildByFieldName("body")
	if body == nil {
		// Fallback: use the entire function_definition
		body = right
	}
	*funcBodies = append(*funcBodies, funcBody{nid, body})
}

func rCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}

	if node.Kind() == "call" {
		if node.ChildCount() >= 1 {
			funcChild := node.Child(0)
			if funcChild != nil && funcChild.Kind() == "identifier" {
				name := string(funcChild.Utf8Text(content))
				// Skip library/require and common keywords
				if name != "library" && name != "require" && name != "if" &&
					name != "for" && name != "while" && name != "return" &&
					name != "list" && name != "c" && name != "function" &&
					name != "class" {
					calls[name] = true
				}
			}
		}
	}

	// Also detect $method() calls
	if node.Kind() == "extract_operator" {
		// pattern: expr$name — check if followed by call
		if node.ChildCount() >= 3 {
			lastChild := node.Child(node.ChildCount() - 1)
			if lastChild != nil && lastChild.Kind() == "identifier" {
				name := string(lastChild.Utf8Text(content))
				calls[name] = true
			}
		}
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		rCollectCalls(node.Child(i), content, calls)
	}
}
