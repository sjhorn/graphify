package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhorn/graphify/internal/treesitter/julia"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractJulia extracts modules, functions, and imports from a Julia file using tree-sitter AST parsing.
func ExtractJulia(path string) *Extraction {
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
	err = parser.SetLanguage(julia.GetLanguage())
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
	juliaWalkNodeWithBody(root, content, imports, classes, fileNID, addNode, addEdge, &funcBodies)

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

	// Add class nodes (modules, structs)
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
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, juliaCollectCalls)...)

	return &Extraction{
		Nodes: nodes,
		Edges: edges,
	}
}

func juliaWalkNodeWithBody(node *tree_sitter.Node, content []byte, imports, classes map[string]int,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	if node == nil {
		return
	}

	switch node.Kind() {
	case "module_definition":
		juliaExtractModuleWithBody(node, content, imports, classes, fileNID, addNode, addEdge, funcBodies)
		return // Don't recurse again, extractModule handles children
	case "using_statement":
		juliaExtractUsing(node, content, imports)
	case "import_statement":
		juliaExtractImport(node, content, imports)
	case "struct_definition":
		juliaExtractStruct(node, content, classes)
	case "abstract_definition":
		juliaExtractAbstract(node, content, classes)
	case "function_definition":
		juliaExtractFunctionWithBody(node, content, fileNID, addNode, addEdge, funcBodies)
	case "assignment":
		juliaExtractShortFunctionWithBody(node, content, fileNID, addNode, addEdge, funcBodies)
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		juliaWalkNodeWithBody(node.Child(i), content, imports, classes, fileNID, addNode, addEdge, funcBodies)
	}
}

func juliaExtractModuleWithBody(node *tree_sitter.Node, content []byte, imports, classes map[string]int,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	// Find module name - identifier child
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			classes[string(child.Utf8Text(content))] = line
			break
		}
	}
	// Recurse into module body for contained declarations
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		juliaWalkNodeWithBody(child, content, imports, classes, fileNID, addNode, addEdge, funcBodies)
	}
}

func juliaExtractUsing(node *tree_sitter.Node, content []byte, imports map[string]int) {
	line := int(node.StartPosition().Row) + 1
	// Find identifier children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			imports[string(child.Utf8Text(content))] = line
		}
	}
}

func juliaExtractImport(node *tree_sitter.Node, content []byte, imports map[string]int) {
	line := int(node.StartPosition().Row) + 1
	// Look for identifiers in the import statement
	juliaExtractImportIdentifiers(node, content, imports, line)
}

func juliaExtractImportIdentifiers(node *tree_sitter.Node, content []byte, imports map[string]int, line int) {
	if node.Kind() == "identifier" {
		name := string(node.Utf8Text(content))
		// Skip common punctuation-like identifiers
		if name != ":" {
			imports[name] = line
		}
	}
	if node.Kind() == "selected_import" || node.Kind() == "import_statement" {
		for i := uint(0); i < node.ChildCount(); i++ {
			juliaExtractImportIdentifiers(node.Child(i), content, imports, line)
		}
	}
}

func juliaExtractStruct(node *tree_sitter.Node, content []byte, classes map[string]int) {
	line := int(node.StartPosition().Row) + 1
	// Look for identifier or type_head
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			classes[string(child.Utf8Text(content))] = line
			return
		}
		if child.Kind() == "type_head" {
			// Extract identifier from type_head
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub.Kind() == "identifier" {
					classes[string(sub.Utf8Text(content))] = line
					return
				}
			}
			// If no identifier child, use full text
			text := string(child.Utf8Text(content))
			// Extract just the name part (before <: or where)
			parts := strings.Fields(text)
			if len(parts) > 0 {
				classes[parts[0]] = line
			}
			return
		}
	}
}

func juliaExtractAbstract(node *tree_sitter.Node, content []byte, classes map[string]int) {
	line := int(node.StartPosition().Row) + 1
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			classes[string(child.Utf8Text(content))] = line
			return
		}
		if child.Kind() == "type_head" {
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub.Kind() == "identifier" {
					classes[string(sub.Utf8Text(content))] = line
					return
				}
			}
		}
	}
}

func juliaExtractFunctionWithBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var funcName string
	// Look for signature, then identifier within it
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "signature" || child.Kind() == "call_expression" {
			funcName = juliaGetFuncNameFromSignature(child, content)
			if funcName != "" {
				break
			}
		}
		if child.Kind() == "identifier" {
			name := string(child.Utf8Text(content))
			if name != "function" {
				funcName = name
				break
			}
		}
	}
	if funcName == "" {
		return
	}

	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	// Use the entire function_definition as body
	*funcBodies = append(*funcBodies, funcBody{nid, node})
}

func juliaGetFuncNameFromSignature(node *tree_sitter.Node, content []byte) string {
	// Look for identifier in the signature
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			return string(child.Utf8Text(content))
		}
		if child.Kind() == "call_expression" {
			return juliaGetFuncNameFromSignature(child, content)
		}
		// For typed parameters like f(x::T)
		if child.Kind() == "typed_expression" {
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub.Kind() == "identifier" {
					return string(sub.Utf8Text(content))
				}
			}
		}
	}
	return ""
}

func juliaExtractShortFunctionWithBody(node *tree_sitter.Node, content []byte,
	fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	// Short function: name(args) = expr
	// Left side should be a call_expression
	if node.ChildCount() < 1 {
		return
	}
	left := node.Child(0)
	if left.Kind() != "call_expression" {
		return
	}
	funcName := juliaGetFuncNameFromSignature(left, content)
	if funcName == "" || juliaIsKeyword(funcName) {
		return
	}

	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	// Use the entire assignment as body
	*funcBodies = append(*funcBodies, funcBody{nid, node})
}

func juliaCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "call_expression" {
		// Get function name
		if node.ChildCount() > 0 {
			first := node.Child(0)
			if first.Kind() == "identifier" {
				name := string(first.Utf8Text(content))
				if !juliaIsKeyword(name) {
					calls[name] = true
				}
			}
			if first.Kind() == "field_expression" {
				// Get last identifier
				var last string
				for i := uint(0); i < first.ChildCount(); i++ {
					child := first.Child(i)
					if child.Kind() == "identifier" {
						last = string(child.Utf8Text(content))
					}
				}
				if last != "" && !juliaIsKeyword(last) {
					calls[last] = true
				}
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		juliaCollectCalls(node.Child(i), content, calls)
	}
}

func juliaIsKeyword(name string) bool {
	keywords := []string{
		"if", "for", "while", "begin", "return", "function",
		"struct", "module", "using", "import", "end", "mutable",
		"abstract", "type", "do",
	}
	for _, kw := range keywords {
		if name == kw {
			return true
		}
	}
	return false
}
