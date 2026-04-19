package extract

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjhorn/graphify/internal/treesitter/kotlin"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractKotlin extracts classes, methods, and imports from a Kotlin file using tree-sitter AST parsing.
func ExtractKotlin(path string) *Extraction {
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
	parser := tree_sitter.NewParser()
	defer parser.Close()
	err = parser.SetLanguage(kotlin.GetLanguage())
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
	kotlinWalkTopLevel(root, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, kotlinCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func kotlinWalkTopLevel(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "import_list":
			kotlinExtractImportList(child, content, fileNID, addNode, addEdge)
		case "import_header":
			kotlinExtractImportHeader(child, content, fileNID, addNode, addEdge)
		case "class_declaration":
			kotlinExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		case "function_declaration":
			kotlinExtractTopFunction(child, content, fileNID, addNode, addEdge, funcBodies)
		default:
			// Recurse into source_file children that may contain declarations
			kotlinWalkTopLevel(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		}
	}
}

func kotlinExtractImportList(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "import_header" {
			kotlinExtractImportHeader(child, content, fileNID, addNode, addEdge)
		}
	}
}

func kotlinExtractImportHeader(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	line := int(node.StartPosition().Row) + 1
	// import_header -> identifier -> simple_identifier*
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			// Get last simple_identifier
			var last string
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub.Kind() == "simple_identifier" {
					last = string(sub.Utf8Text(content))
				}
			}
			if last != "" {
				nid := MakeId(last)
				addNode(nid, last, line)
				addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
			}
		}
	}
}

func kotlinExtractClass(node *tree_sitter.Node, content []byte, parentNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	// Find class name via type_identifier
	var className string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "type_identifier" {
			className = string(child.Utf8Text(content))
			break
		}
	}
	if className == "" {
		return
	}
	classNID := MakeId(className)
	addNode(classNID, className, line)
	addEdge(parentNID, classNID, "contains", line, "EXTRACTED", 1.0)

	// Find class_body -> function_declaration (methods)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "class_body" {
			for j := uint(0); j < child.ChildCount(); j++ {
				member := child.Child(j)
				if member.Kind() == "function_declaration" {
					kotlinExtractMethod(member, content, classNID, className, addNode, addEdge, funcBodies)
				}
			}
		}
	}
}

func kotlinExtractTopFunction(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	// Find simple_identifier that is the function name (comes after "fun" keyword)
	foundFun := false
	var funcName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "fun" {
			foundFun = true
			continue
		}
		if foundFun && child.Kind() == "simple_identifier" {
			funcName = string(child.Utf8Text(content))
			break
		}
	}
	if funcName == "" {
		return
	}
	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	body := kotlinFindBody(node)
	if body != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, body})
	}
}

func kotlinExtractMethod(node *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	foundFun := false
	var funcName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "fun" {
			foundFun = true
			continue
		}
		if foundFun && child.Kind() == "simple_identifier" {
			funcName = string(child.Utf8Text(content))
			break
		}
	}
	if funcName == "" {
		return
	}
	nid := MakeId(className, funcName)
	addNode(nid, "."+funcName+"()", line)
	addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)

	body := kotlinFindBody(node)
	if body != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, body})
	}
}

func kotlinFindBody(node *tree_sitter.Node) *tree_sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "function_body" {
			return child
		}
	}
	return nil
}

func kotlinCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "call_expression" {
		name := kotlinGetCallName(node, content)
		if name != "" && !kotlinIsKeyword(name) {
			calls[name] = true
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		kotlinCollectCalls(node.Child(i), content, calls)
	}
}

func kotlinGetCallName(node *tree_sitter.Node, content []byte) string {
	// First child is usually the callee expression
	if node.ChildCount() == 0 {
		return ""
	}
	first := node.Child(0)
	if first.Kind() == "simple_identifier" {
		return string(first.Utf8Text(content))
	}
	// For member access like config.baseUrl, get the last identifier
	if first.Kind() == "navigation_expression" {
		var last string
		for i := uint(0); i < first.ChildCount(); i++ {
			child := first.Child(i)
			if child.Kind() == "simple_identifier" {
				last = string(child.Utf8Text(content))
			}
		}
		return last
	}
	return ""
}

func kotlinIsKeyword(name string) bool {
	keywords := []string{
		"if", "for", "while", "when", "try", "catch",
		"return", "class", "fun", "val", "var", "object",
	}
	for _, kw := range keywords {
		if name == kw {
			return true
		}
	}
	return false
}
