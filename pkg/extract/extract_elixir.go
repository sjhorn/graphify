package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhorn/graphify/internal/treesitter/elixir"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractElixir extracts modules, functions, and imports from an Elixir file using tree-sitter AST parsing.
func ExtractElixir(path string) *Extraction {
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
	err = parser.SetLanguage(elixir.GetLanguage())
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
	elixirWalkNodeWithBody(root, content, imports, classes, fileNID, addNode, addEdge, &funcBodies)

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

	// Add class nodes (modules)
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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, elixirCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func elixirWalkNodeWithBody(node *tree_sitter.Node, content []byte, imports, classes map[string]int,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	if node == nil {
		return
	}
	if node.Kind() == "call" {
		elixirProcessCallWithBody(node, content, imports, classes, fileNID, addNode, addEdge, funcBodies)
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		elixirWalkNodeWithBody(node.Child(i), content, imports, classes, fileNID, addNode, addEdge, funcBodies)
	}
}

func elixirProcessCallWithBody(node *tree_sitter.Node, content []byte, imports, classes map[string]int,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	// Get the call identifier
	callID := elixirGetCallIdentifier(node, content)
	line := int(node.StartPosition().Row) + 1

	switch callID {
	case "defmodule":
		// First argument is the module name (alias node)
		name := elixirGetFirstArgText(node, content)
		if name != "" {
			classes[name] = line
		}
	case "def", "defp":
		// First argument has the function name
		name := elixirGetDefName(node, content)
		if name != "" {
			nid := MakeId(name)
			// No "()" suffix for Elixir functions
			addNode(nid, name, line)
			addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

			// Use the entire def/defp call node as body for call scanning
			*funcBodies = append(*funcBodies, funcBody{nid, node})
		}
	case "alias":
		name := elixirGetFirstArgText(node, content)
		if name != "" {
			// Use last part for alias (e.g., MyApp.Repo -> Repo)
			parts := strings.Split(name, ".")
			imports[parts[len(parts)-1]] = line
		}
	case "import":
		name := elixirGetFirstArgText(node, content)
		if name != "" {
			imports[name] = line
		}
	}
}

func elixirGetCallIdentifier(node *tree_sitter.Node, content []byte) string {
	// The first child of a call node is typically the identifier
	if node.ChildCount() == 0 {
		return ""
	}
	first := node.Child(0)
	if first.Kind() == "identifier" {
		return string(first.Utf8Text(content))
	}
	return ""
}

func elixirGetFirstArgText(node *tree_sitter.Node, content []byte) string {
	// Look for arguments node, then get text of first argument
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "arguments" {
			if child.ChildCount() > 0 {
				arg := child.Child(0)
				return string(arg.Utf8Text(content))
			}
		}
		// Sometimes the argument is a direct child (alias node)
		if child.Kind() == "alias" {
			return string(child.Utf8Text(content))
		}
	}
	return ""
}

func elixirGetDefName(node *tree_sitter.Node, content []byte) string {
	// For def/defp, the function name is in the arguments
	// It could be: def create(attrs) or def validate(user, attrs)
	// The arguments contain a call node with the function name
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "arguments" {
			if child.ChildCount() > 0 {
				arg := child.Child(0)
				// The arg could be a call (function with params) or identifier (no params)
				if arg.Kind() == "call" {
					return elixirGetCallIdentifier(arg, content)
				}
				if arg.Kind() == "identifier" {
					return string(arg.Utf8Text(content))
				}
			}
		}
	}
	return ""
}

func elixirCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "call" {
		callID := elixirGetCallIdentifier(node, content)
		if callID != "" && !elixirIsKeyword(callID) {
			calls[callID] = true
		}
		// Also check for qualified calls like Repo.insert()
		if node.ChildCount() > 0 {
			first := node.Child(0)
			if first.Kind() == "dot" {
				// Get the function name from dot access
				for j := uint(0); j < first.ChildCount(); j++ {
					sub := first.Child(j)
					if sub.Kind() == "identifier" {
						name := string(sub.Utf8Text(content))
						if !elixirIsKeyword(name) {
							calls[name] = true
						}
					}
				}
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		elixirCollectCalls(node.Child(i), content, calls)
	}
}

func elixirIsKeyword(name string) bool {
	keywords := []string{
		"if", "for", "while", "case", "cond", "receive",
		"def", "defp", "defmodule", "defstruct", "defimpl",
		"alias", "import", "require", "use", "do", "end",
	}
	for _, kw := range keywords {
		if name == kw {
			return true
		}
	}
	return false
}
