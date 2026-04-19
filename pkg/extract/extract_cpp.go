package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
)

// ExtractCpp extracts classes, methods, functions, and includes from a C++ file using tree-sitter.
func ExtractCpp(path string) *Extraction {
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
	lang := tree_sitter.NewLanguage(tree_sitter_cpp.Language())
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
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Kind() {
		case "preproc_include":
			cppExtractInclude(child, content, fileNID, addNode, addEdge)
		case "class_specifier":
			cppExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "struct_specifier":
			cppExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "function_definition":
			cppExtractTopFunc(child, content, fileNID, addNode, addEdge, &funcBodies)
		case "declaration":
			cppExtractTopDecl(child, content, fileNID, addNode, addEdge, &funcBodies)
		case "namespace_definition":
			cppWalkNamespace(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		}
	}

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, cppCollectCalls)...)

	return &Extraction{Nodes: nodes, Edges: edges}
}

func cppExtractInclude(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	line := int(node.StartPosition().Row) + 1
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "system_lib_string" || child.Kind() == "string_literal" {
			headerPath := string(child.Utf8Text(content))
			headerPath = strings.Trim(headerPath, "<>\"")
			base := strings.TrimSuffix(filepath.Base(headerPath), ".h")
			base = strings.TrimSuffix(base, ".hpp")
			nid := MakeId(base)
			addNode(nid, base, line)
			addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
		}
	}
}

func cppExtractClass(node *tree_sitter.Node, content []byte, parentNID string,
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

	// Inheritance: base_class_clause → base_specifier → type_identifier
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "base_class_clause" {
			for j := uint(0); j < child.ChildCount(); j++ {
				spec := child.Child(j)
				if spec.Kind() == "base_class_specifier" {
					cppExtractBaseClass(spec, content, classNID, line, addNode, addEdge)
				}
			}
		}
	}

	// Methods in class body
	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		cppExtractClassBody(bodyNode, content, classNID, className, addNode, addEdge, funcBodies)
	}
}

func cppExtractBaseClass(node *tree_sitter.Node, content []byte, classNID string, line int,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "type_identifier":
			baseName := string(child.Utf8Text(content))
			baseNID := MakeId(baseName)
			addNode(baseNID, baseName, line)
			addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
		case "qualified_identifier":
			name := cppQualifiedName(child, content)
			if name != "" {
				baseNID := MakeId(name)
				addNode(baseNID, name, line)
				addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
			}
		case "template_type":
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				baseName := string(nameNode.Utf8Text(content))
				baseNID := MakeId(baseName)
				addNode(baseNID, baseName, line)
				addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
			}
		}
	}
}

func cppExtractClassBody(body *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Kind() {
		case "function_definition":
			cppExtractMethod(child, content, classNID, className, addNode, addEdge, funcBodies)
		case "declaration":
			// Could be a method declaration (no body) - extract name
			cppExtractMethodDecl(child, content, classNID, className, addNode, addEdge)
		case "access_specifier":
			// public/private/protected - skip
		}
	}
}

func cppExtractMethod(node *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	funcName := cppFindFuncName(node, content)
	if funcName == "" {
		return
	}
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(className, funcName)
	addNode(nid, "."+funcName+"()", line)
	addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
	}
}

func cppExtractMethodDecl(node *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	funcName := cppFindFuncName(node, content)
	if funcName == "" || funcName == className {
		// Skip constructors and non-function declarations
		return
	}
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(className, funcName)
	addNode(nid, "."+funcName+"()", line)
	addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)
}

func cppExtractTopFunc(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	funcName := cppFindFuncName(node, content)
	if funcName == "" {
		return
	}
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
	}
}

func cppExtractTopDecl(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	// Forward declarations at file scope
	funcName := cppFindFuncName(node, content)
	if funcName != "" {
		line := int(node.StartPosition().Row) + 1
		nid := MakeId(funcName)
		addNode(nid, funcName+"()", line)
		addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)
	}
}

func cppWalkNamespace(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	bodyNode := node.ChildByFieldName("body")
	if bodyNode == nil {
		return
	}

	for i := uint(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(i)
		switch child.Kind() {
		case "class_specifier", "struct_specifier":
			cppExtractClass(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		case "function_definition":
			cppExtractTopFunc(child, content, fileNID, addNode, addEdge, funcBodies)
		case "declaration":
			cppExtractTopDecl(child, content, fileNID, addNode, addEdge, funcBodies)
		case "namespace_definition":
			cppWalkNamespace(child, content, fileNID, seenIDs, addNode, addEdge, funcBodies)
		}
	}
}

func cppFindFuncName(node *tree_sitter.Node, content []byte) string {
	declarator := node.ChildByFieldName("declarator")
	if declarator == nil {
		return ""
	}
	return cppExtractNameFromDeclarator(declarator, content)
}

func cppExtractNameFromDeclarator(node *tree_sitter.Node, content []byte) string {
	switch node.Kind() {
	case "function_declarator":
		inner := node.ChildByFieldName("declarator")
		if inner != nil {
			return cppExtractNameFromDeclarator(inner, content)
		}
	case "pointer_declarator":
		inner := node.ChildByFieldName("declarator")
		if inner != nil {
			return cppExtractNameFromDeclarator(inner, content)
		}
	case "reference_declarator":
		inner := node.ChildByFieldName("value")
		if inner != nil {
			return cppExtractNameFromDeclarator(inner, content)
		}
	case "identifier":
		return string(node.Utf8Text(content))
	case "qualified_identifier":
		return cppQualifiedName(node, content)
	case "field_identifier":
		return string(node.Utf8Text(content))
	case "destructor_name":
		return "~" + string(node.Utf8Text(content))
	case "operator_name":
		return string(node.Utf8Text(content))
	}
	return ""
}

func cppQualifiedName(node *tree_sitter.Node, content []byte) string {
	// qualified_identifier → namespace_identifier :: name
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return string(nameNode.Utf8Text(content))
	}
	// Fallback: last identifier child
	var last string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" || child.Kind() == "type_identifier" {
			last = string(child.Utf8Text(content))
		}
	}
	return last
}

var cppKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true,
	"sizeof": true, "typedef": true, "struct": true, "enum": true, "union": true,
	"class": true, "new": true, "delete": true, "throw": true, "catch": true,
	"static_cast": true, "dynamic_cast": true, "reinterpret_cast": true, "const_cast": true,
}

func cppCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind() == "call_expression" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			switch funcNode.Kind() {
			case "identifier":
				name := string(funcNode.Utf8Text(content))
				if !cppKeywords[name] {
					calls[name] = true
				}
			case "field_expression":
				// obj.method() or obj->method()
				field := funcNode.ChildByFieldName("field")
				if field != nil {
					name := string(field.Utf8Text(content))
					if !cppKeywords[name] {
						calls[name] = true
					}
				}
			case "qualified_identifier":
				name := cppQualifiedName(funcNode, content)
				if name != "" && !cppKeywords[name] {
					calls[name] = true
				}
			}
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		cppCollectCalls(node.Child(i), content, calls)
	}
}
