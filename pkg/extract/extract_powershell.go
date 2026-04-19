package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhorn/graphify/internal/treesitter/powershell"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractPowerShell extracts functions, classes, and imports from a PowerShell file using tree-sitter AST parsing.
func ExtractPowerShell(path string) *Extraction {
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
	err = parser.SetLanguage(powershell.GetLanguage())
	if err != nil {
		return &Extraction{}
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return &Extraction{}
	}
	defer tree.Close()
	root := tree.RootNode()

	// The root may contain a statement_list, or direct children
	psWalkStatementsWithBody(root, content, imports, classes, fileNID, addNode, addEdge, &funcBodies)

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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, psCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func psWalkStatementsWithBody(node *tree_sitter.Node, content []byte, imports map[string]int, classes map[string]int,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	if node == nil {
		return
	}

	switch node.Kind() {
	case "function_statement":
		psExtractFunctionWithBody(node, content, fileNID, addNode, addEdge, funcBodies)
	case "class_statement":
		psExtractClassWithBody(node, content, classes, fileNID, addNode, addEdge, funcBodies)
	case "pipeline", "pipeline_chain":
		// Check if this is a using/Import-Module statement
		psExtractImportFromPipeline(node, content, imports)
	case "command":
		psExtractImportFromCommand(node, content, imports)
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() != "function_statement" && child.Kind() != "class_statement" {
			psWalkStatementsWithBody(child, content, imports, classes, fileNID, addNode, addEdge, funcBodies)
		} else {
			// Process these directly
			switch child.Kind() {
			case "function_statement":
				psExtractFunctionWithBody(child, content, fileNID, addNode, addEdge, funcBodies)
			case "class_statement":
				psExtractClassWithBody(child, content, classes, fileNID, addNode, addEdge, funcBodies)
			}
		}
	}
}

func psExtractFunctionWithBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var name string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "function_name" {
			name = string(child.Utf8Text(content))
			break
		}
	}
	if name == "" {
		return
	}

	nid := MakeId(name)
	// No "()" suffix for PowerShell functions
	addNode(nid, name, line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	// Use the entire function_statement as body
	*funcBodies = append(*funcBodies, funcBody{nid, node})
}

func psExtractClassWithBody(node *tree_sitter.Node, content []byte, classes map[string]int,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var className string
	var classNID string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "simple_name" {
			className = string(child.Utf8Text(content))
			classNID = MakeId(className)
			classes[className] = line
		}
		if child.Kind() == "class_method_definition" {
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc.Kind() == "simple_name" {
					methodName := string(gc.Utf8Text(content))
					methodLine := int(child.StartPosition().Row) + 1
					if className != "" {
						nid := MakeId(className, methodName)
						addNode(nid, "."+methodName+"()", methodLine)
						addEdge(classNID, nid, "method", methodLine, "EXTRACTED", 1.0)
						// Use the class_method_definition as body
						*funcBodies = append(*funcBodies, funcBody{nid, child})
					}
				}
			}
		}
	}
}

func psExtractImportFromPipeline(node *tree_sitter.Node, content []byte, imports map[string]int) {
	// Check text for "using" or "Import-Module"
	text := string(node.Utf8Text(content))
	if strings.HasPrefix(text, "using ") || strings.HasPrefix(text, "Import-Module ") {
		psExtractImportFromText(text, node, imports)
	}
	// Also walk children for command nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "command" {
			psExtractImportFromCommand(child, content, imports)
		}
		if child.Kind() == "pipeline" || child.Kind() == "pipeline_chain" {
			psExtractImportFromPipeline(child, content, imports)
		}
	}
}

func psExtractImportFromCommand(node *tree_sitter.Node, content []byte, imports map[string]int) {
	text := string(node.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1

	if strings.HasPrefix(text, "using namespace ") {
		parts := strings.Fields(text)
		if len(parts) >= 3 {
			imports[parts[2]] = line
		}
	} else if strings.HasPrefix(text, "using module ") {
		parts := strings.Fields(text)
		if len(parts) >= 3 {
			imports[parts[2]] = line
		}
	} else if strings.HasPrefix(text, "Import-Module ") {
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			imports[parts[1]] = line
		}
	}
}

func psExtractImportFromText(text string, node *tree_sitter.Node, imports map[string]int) {
	line := int(node.StartPosition().Row) + 1
	if strings.HasPrefix(text, "using namespace ") {
		parts := strings.Fields(text)
		if len(parts) >= 3 {
			imports[parts[2]] = line
		}
	} else if strings.HasPrefix(text, "using module ") {
		parts := strings.Fields(text)
		if len(parts) >= 3 {
			imports[parts[2]] = line
		}
	} else if strings.HasPrefix(text, "Import-Module ") {
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			imports[parts[1]] = line
		}
	}
}

func psCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}

	// Look for command nodes that look like Verb-Noun cmdlet calls
	if node.Kind() == "command" {
		text := string(node.Utf8Text(content))
		firstWord := strings.Fields(text)
		if len(firstWord) > 0 {
			cmd := firstWord[0]
			// Verb-Noun pattern
			if strings.Contains(cmd, "-") && !strings.HasPrefix(cmd, "using") && !strings.HasPrefix(cmd, "Import-Module") {
				calls[cmd] = true
			}
		}
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		psCollectCalls(node.Child(i), content, calls)
	}
}
