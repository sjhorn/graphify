package extract

import (
	"fmt"
	"os"
	"path/filepath"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
)

// ExtractRuby extracts classes, methods, and imports from a Ruby file.
func ExtractRuby(path string) *Extraction {
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
	lang := tree_sitter.NewLanguage(tree_sitter_ruby.Language())
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
	rubyWalkTopLevel(root, content, fileNID, imports, seenIDs, addNode, addEdge, &funcBodies)

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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, rubyCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func rubyWalkTopLevel(node *tree_sitter.Node, content []byte, fileNID string,
	imports map[string]int, seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "call":
			rubyExtractRequire(child, content, imports)
		case "class":
			rubyExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		case "module":
			rubyExtractModule(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		case "method":
			rubyExtractTopMethod(child, content, fileNID, addNode, addEdge, funcBodies)
		}
	}
}

func rubyExtractRequire(node *tree_sitter.Node, content []byte, imports map[string]int) {
	// call → identifier("require") + argument_list → string → string_content
	var isRequire bool
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" && string(child.Utf8Text(content)) == "require" {
			isRequire = true
		}
	}
	if !isRequire {
		return
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "argument_list" {
			for j := uint(0); j < child.ChildCount(); j++ {
				arg := child.Child(j)
				if arg.Kind() == "string" {
					moduleName := rubyStringContent(arg, content)
					if moduleName != "" {
						imports[moduleName] = int(node.StartPosition().Row) + 1
					}
				}
			}
		}
	}
}

func rubyStringContent(node *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "string_content" {
			return string(child.Utf8Text(content))
		}
	}
	return ""
}

func rubyExtractClass(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	var className string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "constant" {
			className = string(child.Utf8Text(content))
			break
		}
	}
	if className == "" {
		return
	}
	line := int(node.StartPosition().Row) + 1
	classNID := MakeId(className)
	addNode(classNID, className, line)
	addEdge(fileNID, classNID, "contains", line, "EXTRACTED", 1.0)

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "body_statement" {
			rubyExtractMethodsFromBody(child, content, classNID, className, addNode, addEdge, funcBodies)
		}
	}
}

func rubyExtractModule(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	var moduleName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "constant" {
			moduleName = string(child.Utf8Text(content))
			break
		}
	}
	if moduleName == "" {
		return
	}
	line := int(node.StartPosition().Row) + 1
	modNID := MakeId(moduleName)
	addNode(modNID, moduleName, line)
	addEdge(fileNID, modNID, "contains", line, "EXTRACTED", 1.0)

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "body_statement" {
			rubyExtractMethodsFromBody(child, content, modNID, moduleName, addNode, addEdge, funcBodies)
		}
	}
}

func rubyExtractMethodsFromBody(body *tree_sitter.Node, content []byte, parentNID, parentName string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child.Kind() == "method" {
			rubyExtractMethod(child, content, parentNID, parentName, addNode, addEdge, funcBodies)
		}
	}
}

func rubyExtractMethod(node *tree_sitter.Node, content []byte, parentNID, parentName string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	var methodName string
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		methodName = string(nameNode.Utf8Text(content))
	} else {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "identifier" {
				methodName = string(child.Utf8Text(content))
				break
			}
		}
	}
	if methodName == "" {
		return
	}

	line := int(node.StartPosition().Row) + 1
	nid := MakeId(parentName, methodName)
	addNode(nid, "."+methodName+"()", line)
	addEdge(parentNID, nid, "method", line, "EXTRACTED", 1.0)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
	}
}

func rubyExtractTopMethod(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	var methodName string
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		methodName = string(nameNode.Utf8Text(content))
	} else {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "identifier" {
				methodName = string(child.Utf8Text(content))
				break
			}
		}
	}
	if methodName == "" {
		return
	}

	line := int(node.StartPosition().Row) + 1
	nid := MakeId(methodName)
	addNode(nid, methodName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
	}
}

var rubyKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "case": true, "rescue": true,
	"def": true, "class": true, "module": true, "return": true, "require": true,
	"puts": true, "print": true, "raise": true, "yield": true, "new": true,
}

func rubyCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "call" {
		// call → receiver.method or just method(args)
		methodNode := node.ChildByFieldName("method")
		if methodNode != nil {
			callName := string(methodNode.Utf8Text(content))
			if !rubyKeywords[callName] {
				calls[callName] = true
			}
		} else {
			// Simple call: identifier + argument_list
			for i := uint(0); i < node.ChildCount(); i++ {
				child := node.Child(i)
				if child.Kind() == "identifier" {
					callName := string(child.Utf8Text(content))
					if !rubyKeywords[callName] {
						calls[callName] = true
					}
					break
				}
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		rubyCollectCalls(node.Child(i), content, calls)
	}
}
