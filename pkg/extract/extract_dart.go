package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhorn/graphify/internal/treesitter/dart"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractDart extracts classes, methods, and imports from a Dart file using tree-sitter AST parsing.
func ExtractDart(path string) *Extraction {
	content, err := os.ReadFile(path)
	if err != nil {
		return &Extraction{}
	}

	var nodes []Node
	var edges []Edge
	seenIDs := make(map[string]bool)
	strPath := path

	fileNID := MakeId(strPath)
	addNode := func(nid, label, nodeType string, line int) {
		if !seenIDs[nid] {
			seenIDs[nid] = true
			nodes = append(nodes, Node{
				ID: nid, Label: label, Type: nodeType, File: strPath,
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

	addNode(fileNID, filepath.Base(path), "file", 1)

	// Parse with tree-sitter
	parser := tree_sitter.NewParser()
	defer parser.Close()
	err = parser.SetLanguage(dart.GetLanguage())
	if err != nil {
		return &Extraction{}
	}
	tree := parser.Parse(content, nil)
	if tree == nil {
		return &Extraction{}
	}
	defer tree.Close()
	root := tree.RootNode()

	var funcBodies []funcBody

	// Walk the AST
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Kind() {
		case "import_or_export":
			walkForImports(child, content, fileNID, addNode, addEdge)
		case "import_specification":
			dartExtractImportSpec(child, content, fileNID, addNode, addEdge)
		case "export_specification":
			dartExtractExportSpec(child, content, fileNID, addNode, addEdge)
		case "part_directive":
			dartExtractPartDirective(child, content, fileNID, addNode, addEdge)
		case "class_definition":
			dartExtractClassDef(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "mixin_declaration":
			dartExtractMixinDecl(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "extension_declaration":
			dartExtractExtensionDecl(child, content, fileNID, addNode, addEdge, &funcBodies)
		case "enum_declaration":
			dartExtractEnumDecl(child, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)
		case "function_signature":
			// Check if next sibling is function_body
			var bodyNode *tree_sitter.Node
			if i+1 < root.ChildCount() {
				next := root.Child(i + 1)
				if next.Kind() == "function_body" {
					bodyNode = next
				}
			}
			dartExtractTopFuncWithBody(child, bodyNode, content, fileNID, addNode, addEdge, &funcBodies)
		case "top_level_variable_declaration":
			dartExtractTopVar(child, content, fileNID, addNode, addEdge)
		}
	}

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, collectCalls)...)

	return &Extraction{Nodes: nodes, Edges: edges}
}

func walkForImports(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64)) {
	if node.Kind() == "import_specification" {
		dartExtractImportSpec(node, content, fileNID, addNode, addEdge)
		return
	}
	if node.Kind() == "export_specification" {
		dartExtractExportSpec(node, content, fileNID, addNode, addEdge)
		return
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		walkForImports(node.Child(i), content, fileNID, addNode, addEdge)
	}
}

func dartExtractImportSpec(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64)) {
	importPath := extractImportURI(node, content)
	if importPath == "" {
		return
	}
	moduleName := importModuleName(importPath)
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(moduleName)
	addNode(nid, moduleName, "module", line)
	addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
}

func dartExtractExportSpec(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64)) {
	importPath := extractImportURI(node, content)
	if importPath == "" {
		return
	}
	moduleName := importModuleName(importPath)
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(moduleName)
	addNode(nid, moduleName, "module", line)
	addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
}

func dartExtractPartDirective(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64)) {
	importPath := extractImportURI(node, content)
	if importPath == "" {
		return
	}
	moduleName := importModuleName(importPath)
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(moduleName)
	addNode(nid, moduleName, "module", line)
	addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
}

func extractImportURI(node *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "configurable_uri" || child.Kind() == "uri" {
			return extractImportURI(child, content)
		}
		if child.Kind() == "string_literal" {
			return unquoteString(string(child.Utf8Text(content)))
		}
	}
	return ""
}

func importModuleName(importPath string) string {
	parts := strings.Split(importPath, "/")
	name := strings.TrimSuffix(parts[len(parts)-1], ".dart")
	name = strings.ReplaceAll(name, ":", "_")
	return name
}

func unquoteString(s string) string {
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")
	return s
}

func dartExtractClassDef(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	className := string(nameNode.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1
	classNID := MakeId(className)
	addNode(classNID, className, "class", line)
	addEdge(fileNID, classNID, "contains", line, "EXTRACTED", 1.0)

	// Inheritance: superclass, mixins, interfaces
	dartExtractInheritance(node, content, classNID, line, addNode, addEdge)

	// Methods in class body
	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		dartExtractClassBody(bodyNode, content, classNID, className, addNode, addEdge, funcBodies)
	}
}

func dartExtractMixinDecl(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	var className string
	line := int(node.StartPosition().Row) + 1
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			className = string(child.Utf8Text(content))
			break
		}
	}
	if className == "" {
		return
	}
	classNID := MakeId(className)
	addNode(classNID, className, "mixin", line)
	addEdge(fileNID, classNID, "contains", line, "EXTRACTED", 1.0)

	// Inheritance from "on" clause and "implements"
	dartExtractInheritance(node, content, classNID, line, addNode, addEdge)

	// Methods - mixin uses positional class_body child, not a "body" field
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "class_body" {
			dartExtractClassBody(child, content, classNID, className, addNode, addEdge, funcBodies)
			break
		}
	}
}

func dartExtractExtensionDecl(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	// Extension name is a positional identifier child, not a "name" field
	var className string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			className = string(child.Utf8Text(content))
			break
		}
	}
	if className == "" {
		return
	}
	line := int(node.StartPosition().Row) + 1
	classNID := MakeId(className)
	addNode(classNID, className, "extension", line)
	addEdge(fileNID, classNID, "contains", line, "EXTRACTED", 1.0)

	// Methods - extension uses positional extension_body child, not a "body" field
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "extension_body" {
			dartExtractClassBody(child, content, classNID, className, addNode, addEdge, funcBodies)
			break
		}
	}
}

func dartExtractEnumDecl(node *tree_sitter.Node, content []byte, fileNID string,
	seenIDs map[string]bool,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	enumName := string(nameNode.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1
	enumNID := MakeId(enumName)
	addNode(enumNID, enumName, "enum", line)
	addEdge(fileNID, enumNID, "contains", line, "EXTRACTED", 1.0)

	// Inheritance (enums can implement interfaces)
	dartExtractInheritance(node, content, enumNID, line, addNode, addEdge)

	// Methods - enum uses positional enum_body child, not a "body" field
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "enum_body" {
			dartExtractClassBody(child, content, enumNID, enumName, addNode, addEdge, funcBodies)
			break
		}
	}
}

func dartExtractInheritance(node *tree_sitter.Node, content []byte, classNID string, line int,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "superclass":
			// extends clause: superclass → type_identifier
			dartExtractTypeRefs(child, content, classNID, line, addNode, addEdge)
		case "mixins":
			// with clause: mixins → type_identifier(s)
			dartExtractTypeRefs(child, content, classNID, line, addNode, addEdge)
		case "interfaces":
			// implements clause: interfaces → type_identifier(s)
			dartExtractTypeRefs(child, content, classNID, line, addNode, addEdge)
		}
	}
}

func dartExtractTypeRefs(node *tree_sitter.Node, content []byte, classNID string, line int,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "type_identifier":
			baseName := string(child.Utf8Text(content))
			baseNID := MakeId(baseName)
			addNode(baseNID, baseName, "class", line)
			addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
		case "identifier":
			baseName := string(child.Utf8Text(content))
			if baseName != "extends" && baseName != "with" && baseName != "implements" && baseName != "on" {
				baseNID := MakeId(baseName)
				addNode(baseNID, baseName, "class", line)
				addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
			}
		}
		// Recurse for nested type nodes
		if child.Kind() != "type_identifier" && child.Kind() != "identifier" {
			dartExtractTypeRefs(child, content, classNID, line, addNode, addEdge)
		}
	}
}

func dartExtractClassBody(body *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Kind() {
		case "method_signature":
			// Check if next sibling is function_body
			var methodBody *tree_sitter.Node
			if i+1 < body.ChildCount() {
				next := body.Child(i + 1)
				if next.Kind() == "function_body" {
					methodBody = next
				}
			}
			dartExtractMethodSig(child, methodBody, content, classNID, className, addNode, addEdge, funcBodies)
		case "constructor_declaration":
			dartExtractConstructorDecl(child, content, classNID, className, addNode, addEdge, funcBodies)
		case "enum_constant":
			dartExtractEnumConstant(child, content, classNID, className, addNode, addEdge)
		}
	}
}

func dartExtractMethodSig(node *tree_sitter.Node, siblingBody *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	var funcName string
	var bodyNode *tree_sitter.Node

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "function_signature":
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				funcName = string(nameNode.Utf8Text(content))
			}
			bodyNode = child.ChildByFieldName("body")
		case "identifier":
			if funcName == "" {
				funcName = string(child.Utf8Text(content))
			}
		case "constructor_signature":
			nameChild := child.Child(uint(0))
			if nameChild != nil {
				funcName = string(nameChild.Utf8Text(content))
			}
		case "factory_constructor_signature":
			// Walk children to find the SECOND identifier (first is class name, second is factory method name)
			idCount := 0
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc.Kind() == "identifier" {
					idCount++
					if idCount == 2 {
						funcName = string(gc.Utf8Text(content))
						break
					}
				}
			}
		case "function_body", "block":
			bodyNode = child
		}
	}

	// Use sibling body if no inline body found
	if bodyNode == nil {
		bodyNode = siblingBody
	}

	if funcName != "" {
		line := int(node.StartPosition().Row) + 1
		nid := MakeId(className, funcName)
		addNode(nid, "."+funcName+"()", "method", line)
		addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)
		if bodyNode != nil {
			*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
		}
	}
}

func dartExtractConstructorDecl(node *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	var funcName string
	var bodyNode *tree_sitter.Node

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "function_signature" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				funcName = string(nameNode.Utf8Text(content))
			}
			bodyNode = child.ChildByFieldName("body")
		}
		if child.Kind() == "function_body" || child.Kind() == "block" {
			bodyNode = child
		}
	}

	if funcName != "" {
		line := int(node.StartPosition().Row) + 1
		nid := MakeId(className, funcName)
		addNode(nid, "."+funcName+"()", "method", line)
		addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)
		if bodyNode != nil {
			*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
		}
	}
}

func dartExtractEnumConstant(node *tree_sitter.Node, content []byte, enumNID, enumName string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			caseName := string(child.Utf8Text(content))
			line := int(node.StartPosition().Row) + 1
			nid := MakeId(enumName, caseName)
			addNode(nid, "."+caseName, "enum_value", line)
			addEdge(enumNID, nid, "case_of", line, "EXTRACTED", 1.0)
			break
		}
	}
}

func dartExtractTopFuncWithBody(node *tree_sitter.Node, bodyNode *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	funcName := string(nameNode.Utf8Text(content))
	line := int(node.StartPosition().Row) + 1
	nid := MakeId(fileNID, funcName)
	addNode(nid, funcName+"()", "function", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	// Try body from parameter first, then from field
	if bodyNode == nil {
		bodyNode = node.ChildByFieldName("body")
	}
	if bodyNode != nil {
		*funcBodies = append(*funcBodies, funcBody{nid, bodyNode})
	}
}

func dartExtractTopVar(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			funcName := string(child.Utf8Text(content))
			line := int(node.StartPosition().Row) + 1
			nid := MakeId(fileNID, funcName)
			addNode(nid, funcName, "variable", line)
			addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)
			break
		}
	}
}

func collectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}

	if node.Kind() == "expression_statement" || node.Kind() == "return_statement" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)

			// Handle cascade sections: ..add() ..remove() ..clear()
			if child.Kind() == "cascade_section" {
				collectCascadeCalls(child, content, calls)
				continue
			}

			if child.Kind() == "identifier" {
				// Check for direct call: identifier + selector(argument_part)
				if hasArgumentPartSibling(node, i+1) {
					callName := string(child.Utf8Text(content))
					if !isKeyword(callName) {
						calls[callName] = true
					}
				}
				// Scan all selector chains for method calls ending in argument_part.
				// This handles obj.method(args) and obj.a.b.method(args).
				collectChainedCalls(node, i+1, content, calls)
			}
		}
	}

	// Handle calls in initialized_variable_definition: identifier + selector(argument_part)
	if node.Kind() == "initialized_variable_definition" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "identifier" && hasArgumentPartSibling(node, i+1) {
				callName := string(child.Utf8Text(content))
				if !isKeyword(callName) {
					calls[callName] = true
				}
			}
			// Also scan selector chains in variable initializers
			if child.Kind() == "identifier" {
				collectChainedCalls(node, i+1, content, calls)
			}
		}
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		collectCalls(node.Child(i), content, calls)
	}
}

// collectCascadeCalls extracts method names from cascade sections (..add() ..remove()).
func collectCascadeCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "cascade_selector" {
			for j := uint(0); j < child.ChildCount(); j++ {
				id := child.Child(j)
				if id.Kind() == "identifier" {
					callName := string(id.Utf8Text(content))
					if !isKeyword(callName) {
						calls[callName] = true
					}
				}
			}
		}
	}
}

// collectChainedCalls scans a selector chain starting at startIdx and extracts
// any identifier immediately before an argument_part as a call.
// Handles: obj.method(args), obj.a.b.method(args), obj.a.method(x).other(y).
func collectChainedCalls(parent *tree_sitter.Node, startIdx uint, content []byte, calls map[string]bool) {
	var lastIdentifier string
	for i := startIdx; i < parent.ChildCount(); i++ {
		child := parent.Child(i)
		if child.Kind() != "selector" {
			lastIdentifier = ""
			continue
		}
		// Check what's inside this selector
		hasArgs := false
		for j := uint(0); j < child.ChildCount(); j++ {
			gc := child.Child(j)
			if gc.Kind() == "unconditional_assignable_selector" {
				// Extract the identifier — this might be the method name
				for k := uint(0); k < gc.ChildCount(); k++ {
					id := gc.Child(k)
					if id.Kind() == "identifier" {
						lastIdentifier = string(id.Utf8Text(content))
					}
				}
			}
			if gc.Kind() == "argument_part" {
				hasArgs = true
			}
		}
		if hasArgs && lastIdentifier != "" && !isKeyword(lastIdentifier) {
			calls[lastIdentifier] = true
			lastIdentifier = ""
		}
	}
}

func hasChildKind(node *tree_sitter.Node, kind string) bool {
	for i := uint(0); i < node.ChildCount(); i++ {
		if node.Child(i).Kind() == kind {
			return true
		}
	}
	return false
}

func hasArgumentPartSibling(parent *tree_sitter.Node, startIdx uint) bool {
	for i := startIdx; i < parent.ChildCount(); i++ {
		child := parent.Child(i)
		if child.Kind() == "selector" {
			for j := uint(0); j < child.ChildCount(); j++ {
				if child.Child(j).Kind() == "argument_part" {
					return true
				}
			}
		}
	}
	return false
}

func isKeyword(name string) bool {
	keywords := []string{
		"if", "for", "while", "switch", "catch", "return",
		"class", "enum", "extension", "mixin", "import", "export",
		"void", "int", "String", "bool", "var", "final", "const",
	}
	for _, kw := range keywords {
		if name == kw {
			return true
		}
	}
	return false
}
