package extract

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjhorn/graphify/internal/treesitter/scala"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractScala extracts classes, methods, and imports from a Scala file using tree-sitter AST parsing.
func ExtractScala(path string) *Extraction {
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
	err = parser.SetLanguage(scala.GetLanguage())
	if err != nil {
		return &Extraction{}
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return &Extraction{}
	}
	defer tree.Close()
	root := tree.RootNode()

	// Walk AST top-level (may be wrapped in compilation_unit or program)
	scalaWalkTopLevel(root, content, fileNID, imports, seenIDs, addNode, addEdge, &funcBodies)

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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, scalaCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func scalaWalkTopLevel(node *tree_sitter.Node, content []byte, fileNID string,
	imports map[string]int, seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	if node == nil {
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "import_declaration":
			scalaExtractImport(child, content, imports)
		case "class_definition":
			scalaExtractClassDef(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		case "object_definition":
			scalaExtractObjectDef(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		case "function_definition":
			scalaExtractTopFunc(child, content, fileNID, addNode, addEdge, funcBodies)
		default:
			// Recurse into compilation_unit, package_clause, etc.
			scalaWalkTopLevel(child, content, fileNID, imports, seenIDs, addNode, addEdge, funcBodies)
		}
	}
}

func scalaExtractImport(node *tree_sitter.Node, content []byte, imports map[string]int) {
	line := int(node.StartPosition().Row) + 1
	// Collect all identifiers in the import chain; the last one is the import name
	var lastIdent string
	scalaFindLastIdentifier(node, content, &lastIdent)
	if lastIdent != "" {
		imports[lastIdent] = line
	}
}

func scalaFindLastIdentifier(node *tree_sitter.Node, content []byte, lastIdent *string) {
	if node == nil {
		return
	}
	if node.Kind() == "identifier" {
		*lastIdent = string(node.Utf8Text(content))
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		scalaFindLastIdentifier(node.Child(i), content, lastIdent)
	}
}

func scalaExtractClassDef(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var className string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			className = string(child.Utf8Text(content))
			break
		}
	}
	if className == "" {
		return
	}
	classNID := MakeId(className)
	addNode(classNID, className, line)
	addEdge(fileNID, classNID, "contains", line, "EXTRACTED", 1.0)

	// Find template_body for methods
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "template_body" {
			scalaExtractMethodsFromBody(child, content, classNID, className, addNode, addEdge, funcBodies)
		}
	}
}

func scalaExtractObjectDef(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var objName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			objName = string(child.Utf8Text(content))
			break
		}
	}
	if objName == "" {
		return
	}
	objNID := MakeId(objName)
	addNode(objNID, objName, line)
	addEdge(fileNID, objNID, "contains", line, "EXTRACTED", 1.0)

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "template_body" {
			scalaExtractMethodsFromBody(child, content, objNID, objName, addNode, addEdge, funcBodies)
		}
	}
}

func scalaExtractMethodsFromBody(node *tree_sitter.Node, content []byte, parentNID, parentName string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "function_definition" {
			scalaExtractMethod(child, content, parentNID, parentName, addNode, addEdge, funcBodies)
		}
	}
}

func scalaExtractMethod(node *tree_sitter.Node, content []byte, parentNID, parentName string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var methodName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			name := string(child.Utf8Text(content))
			if name != "if" && name != "for" && name != "while" && name != "match" {
				methodName = name
			}
			break
		}
	}
	if methodName == "" {
		return
	}

	nid := MakeId(parentName, methodName)
	addNode(nid, "."+methodName+"()", line)
	addEdge(parentNID, nid, "method", line, "EXTRACTED", 1.0)

	// Scala function body: look for block_expression or the body field
	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
	}
}

func scalaExtractTopFunc(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var funcName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			name := string(child.Utf8Text(content))
			if name != "if" && name != "for" && name != "while" && name != "match" {
				funcName = name
			}
			break
		}
	}
	if funcName == "" {
		return
	}

	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
	}
}

func scalaCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}

	// Look for call_expression or application nodes
	if node.Kind() == "call_expression" || node.Kind() == "application" {
		if node.ChildCount() >= 1 {
			funcChild := node.Child(0)
			if funcChild != nil && funcChild.Kind() == "identifier" {
				name := string(funcChild.Utf8Text(content))
				if name != "if" && name != "for" && name != "while" && name != "match" &&
					name != "new" && name != "return" && name != "class" && name != "object" {
					calls[name] = true
				}
			}
		}
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		scalaCollectCalls(node.Child(i), content, calls)
	}
}
