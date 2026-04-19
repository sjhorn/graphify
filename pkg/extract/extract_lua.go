package extract

import (
	"fmt"
	"os"
	"path/filepath"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_lua "github.com/tree-sitter-grammars/tree-sitter-lua/bindings/go"
)

// ExtractLua extracts functions, methods, and imports from a Lua file.
func ExtractLua(path string) *Extraction {
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
	lang := tree_sitter.NewLanguage(tree_sitter_lua.Language())
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
		case "variable_declaration":
			luaExtractVarDecl(child, content, imports, classes)
		case "function_declaration":
			luaExtractFuncDeclWithBody(child, content, fileNID, addNode, addEdge, &funcBodies)
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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, luaCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func luaExtractVarDecl(node *tree_sitter.Node, content []byte, imports map[string]int, classes map[string]int) {
	// variable_declaration → local + assignment_statement → variable_list → identifier, expression_list → value
	// Pattern 1: local http = require("...")  → import
	// Pattern 2: local HttpClient = {}        → class
	var varName string
	var exprNode *tree_sitter.Node

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "assignment_statement" {
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub.Kind() == "variable_list" {
					for k := uint(0); k < sub.ChildCount(); k++ {
						if sub.Child(k).Kind() == "identifier" {
							varName = string(sub.Child(k).Utf8Text(content))
							break
						}
					}
				}
				if sub.Kind() == "expression_list" {
					for k := uint(0); k < sub.ChildCount(); k++ {
						if sub.Child(k).Kind() != "=" {
							exprNode = sub.Child(k)
							break
						}
					}
				}
			}
		}
	}

	if varName == "" || exprNode == nil {
		return
	}

	line := int(node.StartPosition().Row) + 1

	if exprNode.Kind() == "function_call" && luaIsRequireCall(exprNode, content) {
		imports[varName] = line
	} else if exprNode.Kind() == "table_constructor" {
		classes[varName] = line
	}
}

func luaIsRequireCall(node *tree_sitter.Node, content []byte) bool {
	if node.ChildCount() > 0 {
		first := node.Child(0)
		if first.Kind() == "identifier" && string(first.Utf8Text(content)) == "require" {
			return true
		}
	}
	return false
}

func luaExtractFuncDeclWithBody(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	// function_declaration → method_index_expression (Class:method) or identifier (local func)
	line := int(node.StartPosition().Row) + 1

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "method_index_expression" {
			// Class:method pattern
			var className, methodName string
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub.Kind() == "identifier" {
					if className == "" {
						className = string(sub.Utf8Text(content))
					} else {
						methodName = string(sub.Utf8Text(content))
					}
				}
			}
			if className != "" && methodName != "" {
				label := className + ":" + methodName
				nid := MakeId(label)
				addNode(nid, label+"()", line)
				addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

				// Find the function body
				body := luaFindFuncBody(node)
				if body != nil {
					*funcBodies = append(*funcBodies, funcBody{nid, body})
				}
			}
			return
		}
		if child.Kind() == "dot_index_expression" {
			// Class.method pattern
			var className, methodName string
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub.Kind() == "identifier" {
					if className == "" {
						className = string(sub.Utf8Text(content))
					} else {
						methodName = string(sub.Utf8Text(content))
					}
				}
			}
			if className != "" && methodName != "" {
				label := className + "." + methodName
				nid := MakeId(label)
				addNode(nid, label+"()", line)
				addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

				body := luaFindFuncBody(node)
				if body != nil {
					*funcBodies = append(*funcBodies, funcBody{nid, body})
				}
			}
			return
		}
		if child.Kind() == "identifier" {
			funcName := string(child.Utf8Text(content))
			if funcName != "function" {
				nid := MakeId(funcName)
				addNode(nid, funcName+"()", line)
				addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

				body := luaFindFuncBody(node)
				if body != nil {
					*funcBodies = append(*funcBodies, funcBody{nid, body})
				}
				return
			}
		}
	}
}

func luaFindFuncBody(node *tree_sitter.Node) *tree_sitter.Node {
	// In Lua tree-sitter, function_declaration has a "body" field or a block child
	body := node.ChildByFieldName("body")
	if body != nil {
		return body
	}
	// Fallback: look for block child
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "block" {
			return child
		}
	}
	// Last resort: use the entire node as body for call scanning
	return node
}

var luaKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "return": true, "function": true,
	"local": true, "require": true, "print": true, "type": true,
	"setmetatable": true, "new": true, "end": true, "do": true, "then": true,
}

func luaCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "function_call" {
		if node.ChildCount() > 0 {
			first := node.Child(0)
			var callName string
			switch first.Kind() {
			case "identifier":
				callName = string(first.Utf8Text(content))
			case "method_index_expression":
				// Object:method() - get just the method name
				for i := uint(0); i < first.ChildCount(); i++ {
					sub := first.Child(i)
					if sub.Kind() == "identifier" {
						callName = string(sub.Utf8Text(content))
					}
				}
			case "dot_index_expression":
				for i := uint(0); i < first.ChildCount(); i++ {
					sub := first.Child(i)
					if sub.Kind() == "identifier" {
						callName = string(sub.Utf8Text(content))
					}
				}
			}
			if callName != "" && !luaKeywords[callName] {
				calls[callName] = true
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		luaCollectCalls(node.Child(i), content, calls)
	}
}
