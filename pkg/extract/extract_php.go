package extract

import (
	"fmt"
	"os"
	"path/filepath"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

// ExtractPHP extracts classes, methods, and imports from a PHP file.
func ExtractPHP(path string) *Extraction {
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
	lang := tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())
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
		case "namespace_use_declaration":
			phpExtractUse(child, content, imports)
		case "class_declaration":
			phpExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "interface_declaration":
			phpExtractInterface(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "trait_declaration":
			phpExtractTrait(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "function_definition":
			phpExtractTopFunction(child, content, fileNID, addNode, addEdge, &funcBodies)
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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, phpCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func phpExtractUse(node *tree_sitter.Node, content []byte, imports map[string]int) {
	// namespace_use_declaration → namespace_use_clause → qualified_name → last name
	line := int(node.StartPosition().Row) + 1
	last := phpLastName(node, content)
	if last != "" {
		imports[last] = line
	}
}

func phpLastName(node *tree_sitter.Node, content []byte) string {
	var last string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "name" {
			last = string(child.Utf8Text(content))
		} else if child.ChildCount() > 0 {
			if found := phpLastName(child, content); found != "" {
				last = found
			}
		}
	}
	return last
}

func phpExtractClass(node *tree_sitter.Node, content []byte, fileNID string,
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
	addEdge(fileNID, classNID, "contains", line, "EXTRACTED", 1.0)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		phpExtractMethods(bodyNode, content, classNID, className, addNode, addEdge, funcBodies)
	}
}

func phpExtractInterface(node *tree_sitter.Node, content []byte, fileNID string,
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
	addEdge(fileNID, classNID, "contains", line, "EXTRACTED", 1.0)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		phpExtractMethods(bodyNode, content, classNID, className, addNode, addEdge, funcBodies)
	}
}

func phpExtractTrait(node *tree_sitter.Node, content []byte, fileNID string,
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
	addEdge(fileNID, classNID, "contains", line, "EXTRACTED", 1.0)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		phpExtractMethods(bodyNode, content, classNID, className, addNode, addEdge, funcBodies)
	}
}

func phpExtractMethods(bodyNode *tree_sitter.Node, content []byte, parentNID, parentName string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(i)
		if child.Kind() == "method_declaration" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				methodName := string(nameNode.Utf8Text(content))
				line := int(child.StartPosition().Row) + 1
				nid := MakeId(parentName, methodName)
				addNode(nid, "."+methodName+"()", line)
				addEdge(parentNID, nid, "method", line, "EXTRACTED", 1.0)

				methodBody := child.ChildByFieldName("body")
				if methodBody != nil {
					*funcBodies = append(*funcBodies, funcBody{nid, methodBody})
				}
			}
		}
	}
}

func phpExtractTopFunction(node *tree_sitter.Node, content []byte, fileNID string,
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

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
	}
}

var phpKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"new": true, "return": true, "class": true, "interface": true, "foreach": true,
	"function": true, "array": true, "echo": true, "isset": true, "empty": true,
}

func phpCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "function_call_expression" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			var callName string
			if funcNode.Kind() == "name" {
				callName = string(funcNode.Utf8Text(content))
			} else if funcNode.Kind() == "qualified_name" {
				callName = phpLastName(funcNode, content)
			}
			if callName != "" && !phpKeywords[callName] {
				calls[callName] = true
			}
		}
	}
	if node.Kind() == "member_call_expression" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			callName := string(nameNode.Utf8Text(content))
			if !phpKeywords[callName] {
				calls[callName] = true
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		phpCollectCalls(node.Child(i), content, calls)
	}
}
