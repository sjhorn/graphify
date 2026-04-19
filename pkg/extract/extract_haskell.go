package extract

import (
	"fmt"
	"os"
	"path/filepath"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_haskell "github.com/tree-sitter/tree-sitter-haskell/bindings/go"
)

// ExtractHaskell extracts functions, types, and imports from a Haskell file.
func ExtractHaskell(path string) *Extraction {
	content, err := os.ReadFile(path)
	if err != nil {
		return &Extraction{}
	}

	var nodes []Node
	var edges []Edge
	seenIDs := make(map[string]bool)

	strPath := path

	imports := make(map[string]int)
	types := make(map[string]int)
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
	lang := tree_sitter.NewLanguage(tree_sitter_haskell.Language())
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
		case "imports":
			hsExtractImports(child, content, imports)
		case "declarations":
			hsExtractDeclarationsWithBody(child, content, fileNID, addNode, addEdge, types, &funcBodies)
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

	// Add type nodes
	for typeName, line := range types {
		id := MakeId(typeName)
		if !seenIDs[id] {
			seenIDs[id] = true
			nodes = append(nodes, Node{
				ID:       id,
				Label:    typeName,
				File:     strPath,
				Location: fmt.Sprintf("L%d", line),
			})
		}
		addEdge(fileNID, id, "contains", line, "EXTRACTED", 1.0)
	}

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, hsCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func hsExtractImports(node *tree_sitter.Node, content []byte, imports map[string]int) {
	// imports → import* → module → module_id.module_id...
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "import" {
			line := int(child.StartPosition().Row) + 1
			// Find the module node (the one after "import" / "qualified" keywords)
			for j := uint(0); j < child.ChildCount(); j++ {
				modNode := child.Child(j)
				if modNode.Kind() == "module" {
					moduleName := string(modNode.Utf8Text(content))
					imports[moduleName] = line
					break
				}
			}
		}
	}
}

var hsKeywords = map[string]bool{
	"if": true, "then": true, "else": true, "case": true, "of": true,
	"let": true, "in": true, "where": true, "data": true, "type": true,
	"newtype": true, "do": true, "return": true, "module": true, "import": true,
}

func hsExtractDeclarationsWithBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	types map[string]int, funcBodies *[]funcBody) {

	// Track function names from signatures so we can match them with their definitions
	funcNames := make(map[string]bool)

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "signature":
			// signature → variable (function name) + :: + type
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub.Kind() == "variable" {
					funcName := string(sub.Utf8Text(content))
					if !hsKeywords[funcName] {
						funcNames[funcName] = true
						line := int(child.StartPosition().Row) + 1
						nid := MakeId(funcName)
						addNode(nid, funcName+"()", line)
						addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)
					}
					break
				}
			}
		case "function":
			// function definition - extract body for call scanning
			hsExtractFunctionBody(child, content, fileNID, addNode, addEdge, funcNames, funcBodies)
		case "bind":
			// bind is another form of function definition in Haskell
			hsExtractFunctionBody(child, content, fileNID, addNode, addEdge, funcNames, funcBodies)
		case "data_type", "type_alias", "newtype":
			// data/type/newtype → name
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				types[string(nameNode.Utf8Text(content))] = int(child.StartPosition().Row) + 1
			} else {
				// Try to find a "name" kind child
				for j := uint(0); j < child.ChildCount(); j++ {
					sub := child.Child(j)
					if sub.Kind() == "name" {
						types[string(sub.Utf8Text(content))] = int(child.StartPosition().Row) + 1
						break
					}
				}
			}
		}
	}
}

func hsExtractFunctionBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcNames map[string]bool, funcBodies *[]funcBody) {

	// Find the function name (first variable child)
	var funcName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "variable" {
			funcName = string(child.Utf8Text(content))
			break
		}
	}
	if funcName == "" || hsKeywords[funcName] {
		return
	}

	nid := MakeId(funcName)

	// If this function wasn't already added via signature, add it now
	if !funcNames[funcName] {
		line := int(node.StartPosition().Row) + 1
		addNode(nid, funcName+"()", line)
		addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)
	}

	// Use the entire function/bind node as the body for call scanning
	*funcBodies = append(*funcBodies, funcBody{nid, node})
}

func hsCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	// In Haskell, function application is an "apply" node
	// apply → function_name arg1 arg2 ...
	if node.Kind() == "apply" {
		if node.ChildCount() > 0 {
			first := node.Child(0)
			if first.Kind() == "variable" {
				callName := string(first.Utf8Text(content))
				if !hsKeywords[callName] {
					calls[callName] = true
				}
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		hsCollectCalls(node.Child(i), content, calls)
	}
}
