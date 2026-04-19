package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// funcBody tracks a function's node ID and AST body node for caller-scoped call detection.
// Shared across all tree-sitter based extractors.
type funcBody struct {
	nid  string
	body *tree_sitter.Node
}

// resolveCallerScopedCalls walks each function body for calls and returns caller→callee edges.
func resolveCallerScopedCalls(funcBodies []funcBody, content []byte, nodes []Node,
	collectCalls func(*tree_sitter.Node, []byte, map[string]bool)) []Edge {

	labelToNID := make(map[string]string)
	for _, n := range nodes {
		normalized := strings.TrimSuffix(strings.TrimPrefix(n.Label, "."), "()")
		labelToNID[strings.ToLower(normalized)] = n.ID
		// Also index by the part after ":" for Lua-style Class:method labels
		if idx := strings.Index(normalized, ":"); idx != -1 {
			short := normalized[idx+1:]
			if short != "" {
				labelToNID[strings.ToLower(short)] = n.ID
			}
		}
	}

	var callEdges []Edge
	seenCallPairs := make(map[string]bool)
	for _, fb := range funcBodies {
		calls := make(map[string]bool)
		collectCalls(fb.body, content, calls)
		for callName := range calls {
			tgtNID := labelToNID[strings.ToLower(callName)]
			if tgtNID != "" {
				pair := fb.nid + "|" + tgtNID
				if !seenCallPairs[pair] {
					seenCallPairs[pair] = true
					callEdges = append(callEdges, Edge{
						Source: fb.nid, Target: tgtNID,
						Relation: "calls", Confidence: "EXTRACTED", Weight: 1.0,
					})
				}
			}
		}
	}
	return callEdges
}

// ExtractPython extracts classes, functions, imports, and calls from a Python file using tree-sitter.
func ExtractPython(path string) *Extraction {
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
				ID: nid, Label: label, File: strPath,
				Location: fmt.Sprintf("L%d", line),
			})
		}
	}
	addEdge := func(src, tgt, relation string, line int, confidence string, weight float64) {
		edges = append(edges, Edge{
			Source: src, Target: tgt, Relation: relation,
			Confidence: confidence, Weight: weight,
		})
	}

	addNode(fileNID, filepath.Base(path), 1)

	// Parse with tree-sitter
	lang := tree_sitter.NewLanguage(tree_sitter_python.Language())
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

	var funcBodies []funcBody

	// Walk top-level AST nodes
	pyWalkTopLevel(root, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, pyCollectCalls)...)

	return &Extraction{Nodes: nodes, Edges: edges}
}

func pyWalkTopLevel(node *tree_sitter.Node, content []byte, parentNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "import_statement":
			pyExtractImportStmt(child, content, parentNID, addNode, addEdge)
		case "import_from_statement":
			pyExtractFromImportStmt(child, content, parentNID, addNode, addEdge)
		case "class_definition":
			pyExtractClassDef(child, content, parentNID, seenIDs, addNode, addEdge, funcBodies)
		case "decorated_definition":
			for j := uint(0); j < child.ChildCount(); j++ {
				inner := child.Child(j)
				switch inner.Kind() {
				case "class_definition":
					pyExtractClassDef(inner, content, parentNID, seenIDs, addNode, addEdge, funcBodies)
				case "function_definition":
					pyExtractTopFunc(inner, content, parentNID, addNode, addEdge, funcBodies)
				}
			}
		case "function_definition":
			pyExtractTopFunc(child, content, parentNID, addNode, addEdge, funcBodies)
		}
	}
}

func pyExtractTopFunc(node *tree_sitter.Node, content []byte, fileNID string,
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

	body := node.ChildByFieldName("body")
	if body != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, body})
	}
}

func pyExtractClassDef(node *tree_sitter.Node, content []byte, parentNID string,
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
	addEdge(parentNID, classNID, "contains", line, "EXTRACTED", 1.0)

	// Inheritance from superclasses
	superclasses := node.ChildByFieldName("superclasses")
	if superclasses != nil {
		for i := uint(0); i < superclasses.ChildCount(); i++ {
			child := superclasses.Child(i)
			var baseName string
			switch child.Kind() {
			case "identifier":
				baseName = string(child.Utf8Text(content))
			case "attribute":
				attr := child.ChildByFieldName("attribute")
				if attr != nil {
					baseName = string(attr.Utf8Text(content))
				}
			}
			if baseName != "" {
				baseNID := MakeId(baseName)
				addNode(baseNID, baseName, line)
				addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
			}
		}
	}

	// Methods in class body
	body := node.ChildByFieldName("body")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			child := body.Child(i)
			switch child.Kind() {
			case "function_definition":
				pyExtractMethod(child, content, classNID, className, addNode, addEdge, funcBodies)
			case "decorated_definition":
				for j := uint(0); j < child.ChildCount(); j++ {
					inner := child.Child(j)
					switch inner.Kind() {
					case "function_definition":
						pyExtractMethod(inner, content, classNID, className, addNode, addEdge, funcBodies)
					case "class_definition":
						pyExtractClassDef(inner, content, classNID, seenIDs, addNode, addEdge, funcBodies)
					}
				}
			case "class_definition":
				pyExtractClassDef(child, content, classNID, seenIDs, addNode, addEdge, funcBodies)
			}
		}
	}
}

func pyExtractMethod(node *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	methodName := string(nameNode.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(className, methodName)
	addNode(nid, "."+methodName+"()", line)
	addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)

	body := node.ChildByFieldName("body")
	if body != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, body})
	}
}

func pyExtractImportStmt(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	line := int(node.StartPosition().Row) + 1
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "dotted_name":
			name := pyDottedNameLastPart(child, content)
			if name != "" {
				nid := MakeId(name)
				addNode(nid, name, line)
				addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
			}
		case "aliased_import":
			alias := child.ChildByFieldName("alias")
			if alias != nil {
				name := string(alias.Utf8Text(content))
				nid := MakeId(name)
				addNode(nid, name, line)
				addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
			} else {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					name := pyDottedNameLastPart(nameNode, content)
					if name != "" {
						nid := MakeId(name)
						addNode(nid, name, line)
						addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
					}
				}
			}
		}
	}
}

func pyExtractFromImportStmt(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	line := int(node.StartPosition().Row) + 1
	moduleName := node.ChildByFieldName("module_name")
	if moduleName == nil {
		return
	}

	var modName string
	switch moduleName.Kind() {
	case "dotted_name":
		modName = pyDottedNameLastPart(moduleName, content)
	case "relative_import":
		for i := uint(0); i < moduleName.ChildCount(); i++ {
			child := moduleName.Child(i)
			if child.Kind() == "dotted_name" {
				modName = pyDottedNameLastPart(child, content)
				break
			}
		}
	}

	if modName != "" {
		nid := MakeId(modName)
		addNode(nid, modName, line)
		addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
	}
}

func pyDottedNameLastPart(node *tree_sitter.Node, content []byte) string {
	var last string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			last = string(child.Utf8Text(content))
		}
	}
	if last == "" {
		last = string(node.Utf8Text(content))
	}
	return last
}

var pyKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "with": true,
	"return": true, "print": true, "raise": true, "yield": true,
	"assert": true, "del": true, "pass": true, "break": true,
	"continue": true, "lambda": true,
}

func pyCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "call" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			switch funcNode.Kind() {
			case "identifier":
				name := string(funcNode.Utf8Text(content))
				if !pyKeywords[name] {
					calls[name] = true
				}
			case "attribute":
				attr := funcNode.ChildByFieldName("attribute")
				if attr != nil {
					name := string(attr.Utf8Text(content))
					if !pyKeywords[name] {
						calls[name] = true
					}
				}
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		pyCollectCalls(node.Child(i), content, calls)
	}
}
