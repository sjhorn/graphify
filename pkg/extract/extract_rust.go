package extract

import (
	"fmt"
	"os"
	"path/filepath"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

// ExtractRust extracts structs, functions, and imports from a Rust file.
func ExtractRust(path string) *Extraction {
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
	lang := tree_sitter.NewLanguage(tree_sitter_rust.Language())
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
		case "use_declaration":
			rustExtractUse(child, content, imports)
		case "struct_item":
			rustExtractStruct(child, content, classes)
		case "enum_item":
			rustExtractEnum(child, content, classes)
		case "trait_item":
			rustExtractTrait(child, content, classes)
		case "impl_item":
			rustExtractImplWithBodies(child, content, classes, addNode, addEdge, &funcBodies)
		case "mod_item":
			rustExtractMod(child, content, classes)
		case "function_item":
			rustExtractFunctionWithBody(child, content, fileNID, addNode, addEdge, &funcBodies)
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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, rustCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func rustExtractUse(node *tree_sitter.Node, content []byte, imports map[string]int) {
	// use_declaration → scoped_identifier → last identifier
	line := int(node.StartPosition().Row) + 1
	last := rustLastIdentifier(node, content)
	if last != "" {
		imports[last] = line
	}
}

func rustLastIdentifier(node *tree_sitter.Node, content []byte) string {
	var last string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "identifier", "type_identifier":
			last = string(child.Utf8Text(content))
		case "scoped_identifier", "scoped_use_list", "use_list":
			if found := rustLastIdentifier(child, content); found != "" {
				last = found
			}
		}
	}
	return last
}

func rustExtractStruct(node *tree_sitter.Node, content []byte, classes map[string]int) {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		classes[string(nameNode.Utf8Text(content))] = int(node.StartPosition().Row) + 1
	}
}

func rustExtractEnum(node *tree_sitter.Node, content []byte, classes map[string]int) {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		classes[string(nameNode.Utf8Text(content))] = int(node.StartPosition().Row) + 1
	}
}

func rustExtractTrait(node *tree_sitter.Node, content []byte, classes map[string]int) {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		classes[string(nameNode.Utf8Text(content))] = int(node.StartPosition().Row) + 1
	}
}

func rustExtractMod(node *tree_sitter.Node, content []byte, classes map[string]int) {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		classes[string(nameNode.Utf8Text(content))] = int(node.StartPosition().Row) + 1
	}
}

func rustExtractImplWithBodies(node *tree_sitter.Node, content []byte, classes map[string]int,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	// impl_item → type_identifier, declaration_list → function_item
	nameNode := node.ChildByFieldName("type")
	var typeName string
	if nameNode != nil {
		typeName = string(nameNode.Utf8Text(content))
		classes[typeName] = int(node.StartPosition().Row) + 1
	}

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		for i := uint(0); i < bodyNode.ChildCount(); i++ {
			child := bodyNode.Child(i)
			if child.Kind() == "function_item" {
				if typeName != "" {
					rustExtractMethodWithBody(child, content, typeName, addNode, addEdge, funcBodies)
				} else {
					// Fallback: treat as top-level function (no type context)
					rustExtractFunctionWithBody(child, content, "", addNode, addEdge, funcBodies)
				}
			}
		}
	}
}

func rustExtractMethodWithBody(node *tree_sitter.Node, content []byte, typeName string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	methodName := string(nameNode.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(typeName, methodName)
	classNID := MakeId(typeName)
	addNode(nid, "."+methodName+"()", line)
	addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)

	body := node.ChildByFieldName("body")
	if body != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, body})
	}
}

func rustExtractFunctionWithBody(node *tree_sitter.Node, content []byte, parentNID string,
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
	if parentNID != "" {
		addEdge(parentNID, nid, "contains", line, "EXTRACTED", 1.0)
	}

	body := node.ChildByFieldName("body")
	if body != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, body})
	}
}

var rustKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "match": true, "return": true,
	"struct": true, "enum": true, "fn": true, "let": true, "mut": true,
	"impl": true, "trait": true, "mod": true, "use": true, "pub": true,
}

func rustCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
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
			case "scoped_identifier", "field_expression":
				// Get last identifier in path (e.g., Graph::new → new, self.validate → validate)
				callName = rustLastIdentifier(funcNode, content)
			}
			if callName != "" && !rustKeywords[callName] {
				calls[callName] = true
			}
		}
	}
	if node.Kind() == "macro_invocation" {
		macroNode := node.ChildByFieldName("macro")
		if macroNode != nil {
			callName := string(macroNode.Utf8Text(content))
			if !rustKeywords[callName] {
				calls[callName] = true
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		rustCollectCalls(node.Child(i), content, calls)
	}
}
