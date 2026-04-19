package extract

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjhorn/graphify/internal/treesitter/swift"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractSwift extracts classes, methods, and imports from a Swift file using tree-sitter AST parsing.
func ExtractSwift(path string) *Extraction {
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
	parser := tree_sitter.NewParser()
	defer parser.Close()
	err = parser.SetLanguage(swift.GetLanguage())
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

	// Walk AST
	swiftWalkTopLevelNew(root, content, fileNID, seenIDs, addNode, addEdge, &funcBodies)

	// Caller-scoped call detection
	edges = append(edges, resolveCallerScopedCalls(funcBodies, content, nodes, swiftCollectCalls)...)

	return &Extraction{Nodes: nodes, Edges: edges}
}

func swiftWalkTopLevelNew(node *tree_sitter.Node, content []byte, parentNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	if node == nil {
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "import_declaration":
			swiftExtractImportNew(child, content, parentNID, addNode, addEdge)
		case "class_declaration":
			swiftExtractClassDeclNew(child, content, parentNID, seenIDs, addNode, addEdge, funcBodies)
		case "protocol_declaration":
			swiftExtractProtocolNew(child, content, parentNID, seenIDs, addNode, addEdge, funcBodies)
		case "function_declaration":
			swiftExtractTopFuncNew(child, content, parentNID, addNode, addEdge, funcBodies)
		default:
			// Recurse for source_file etc
			swiftWalkTopLevelNew(child, content, parentNID, seenIDs, addNode, addEdge, funcBodies)
		}
	}
}

func swiftExtractImportNew(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	line := int(node.StartPosition().Row) + 1
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc.Kind() == "simple_identifier" {
					name := string(gc.Utf8Text(content))
					nid := MakeId(name)
					addNode(nid, name, line)
					addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
					return
				}
			}
			name := string(child.Utf8Text(content))
			nid := MakeId(name)
			addNode(nid, name, line)
			addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
			return
		}
		if child.Kind() == "simple_identifier" {
			name := string(child.Utf8Text(content))
			nid := MakeId(name)
			addNode(nid, name, line)
			addEdge(fileNID, nid, "imports", line, "EXTRACTED", 1.0)
			return
		}
	}
}

func swiftExtractClassDeclNew(node *tree_sitter.Node, content []byte, parentNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1

	// Determine if this is an extension
	isExtension := false
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		text := string(child.Utf8Text(content))
		if text == "extension" {
			isExtension = true
			break
		}
	}

	var className string
	if isExtension {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "user_type" {
				for j := uint(0); j < child.ChildCount(); j++ {
					gc := child.Child(j)
					if gc.Kind() == "type_identifier" {
						className = string(gc.Utf8Text(content))
					}
				}
			}
		}
	} else {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "type_identifier" {
				className = string(child.Utf8Text(content))
				break
			}
		}
	}

	if className == "" {
		return
	}

	classNID := MakeId(className)
	addNode(classNID, className, line)
	addEdge(parentNID, classNID, "contains", line, "EXTRACTED", 1.0)

	// Inheritance/conformance: look for inheritance_specifier → user_type/type_identifier
	swiftExtractInheritance(node, content, classNID, line, addNode, addEdge)

	// Find methods in class_body or enum_class_body
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "class_body" || child.Kind() == "enum_class_body" {
			swiftExtractMethodsFromBodyNew(child, content, classNID, className, addNode, addEdge, funcBodies)
		}
	}
}

func swiftExtractProtocolNew(node *tree_sitter.Node, content []byte, parentNID string,
	seenIDs map[string]bool,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var className string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "type_identifier" {
			className = string(child.Utf8Text(content))
			break
		}
	}
	if className == "" {
		return
	}

	classNID := MakeId(className)
	addNode(classNID, className, line)
	addEdge(parentNID, classNID, "contains", line, "EXTRACTED", 1.0)

	// Inheritance
	swiftExtractInheritance(node, content, classNID, line, addNode, addEdge)

	// Methods
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "protocol_body" {
			swiftExtractMethodsFromBodyNew(child, content, classNID, className, addNode, addEdge, funcBodies)
		}
	}
}

func swiftExtractInheritance(node *tree_sitter.Node, content []byte, classNID string, line int,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "inheritance_specifier" || child.Kind() == "type_constraints" {
			swiftExtractTypeRefsForInheritance(child, content, classNID, line, addNode, addEdge)
		}
	}
}

func swiftExtractTypeRefsForInheritance(node *tree_sitter.Node, content []byte, classNID string, line int,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "user_type":
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc.Kind() == "type_identifier" {
					baseName := string(gc.Utf8Text(content))
					baseNID := MakeId(baseName)
					addNode(baseNID, baseName, line)
					addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
					return // Only first type_identifier in user_type
				}
			}
		case "type_identifier":
			baseName := string(child.Utf8Text(content))
			baseNID := MakeId(baseName)
			addNode(baseNID, baseName, line)
			addEdge(classNID, baseNID, "inherits", line, "EXTRACTED", 1.0)
		case "inheritance_specifier":
			// Recurse into nested inheritance_specifier
			swiftExtractTypeRefsForInheritance(child, content, classNID, line, addNode, addEdge)
		}
	}
}

func swiftExtractMethodsFromBodyNew(body *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Kind() {
		case "function_declaration":
			swiftExtractMethodNew(child, content, classNID, className, addNode, addEdge, funcBodies)
		case "init_declaration":
			line := int(child.StartPosition().Row) + 1
			nid := MakeId(className, "init")
			addNode(nid, ".init()", line)
			addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)
			// Init body
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc.Kind() == "function_body" || gc.Kind() == "statements" {
					*funcBodies = append(*funcBodies, funcBody{nid, gc})
					break
				}
			}
		case "deinit_declaration":
			line := int(child.StartPosition().Row) + 1
			nid := MakeId(className, "deinit")
			addNode(nid, ".deinit()", line)
			addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)
			// Deinit body
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc.Kind() == "function_body" || gc.Kind() == "statements" {
					*funcBodies = append(*funcBodies, funcBody{nid, gc})
					break
				}
			}
		case "subscript_declaration":
			line := int(child.StartPosition().Row) + 1
			nid := MakeId(className, "subscript")
			addNode(nid, ".subscript()", line)
			addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)
		case "enum_entry":
			// Enum cases
			swiftExtractEnumCases(child, content, classNID, className, addNode, addEdge)
		}
	}
}

func swiftExtractEnumCases(node *tree_sitter.Node, content []byte, enumNID, enumName string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64)) {

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "simple_identifier" {
			caseName := string(child.Utf8Text(content))
			line := int(child.StartPosition().Row) + 1
			nid := MakeId(enumName, caseName)
			addNode(nid, "."+caseName, line)
			addEdge(enumNID, nid, "case_of", line, "EXTRACTED", 1.0)
		}
	}
}

func swiftExtractMethodNew(node *tree_sitter.Node, content []byte, classNID, className string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var funcName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "simple_identifier" {
			name := string(child.Utf8Text(content))
			if name != "if" && name != "for" && name != "while" && name != "switch" && name != "guard" {
				funcName = name
			}
			break
		}
	}

	if funcName == "" {
		return
	}

	nid := MakeId(className, funcName)
	addNode(nid, "."+funcName+"()", line)
	addEdge(classNID, nid, "method", line, "EXTRACTED", 1.0)

	// Capture function body
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "function_body" || child.Kind() == "statements" {
			*funcBodies = append(*funcBodies, funcBody{nid, child})
			break
		}
	}
}

func swiftExtractTopFuncNew(node *tree_sitter.Node, content []byte, fileNID string,
	addNode func(string, string, int), addEdge func(string, string, string, int, string, float64),
	funcBodies *[]funcBody) {

	line := int(node.StartPosition().Row) + 1
	var funcName string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "simple_identifier" {
			name := string(child.Utf8Text(content))
			if name != "if" && name != "for" && name != "while" && name != "switch" && name != "guard" {
				funcName = name
			}
			break
		}
	}

	if funcName == "" {
		return
	}

	nid := MakeId(funcName)
	addNode(nid, funcName+"()", line)
	addEdge(fileNID, nid, "contains", line, "EXTRACTED", 1.0)

	// Capture function body
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "function_body" || child.Kind() == "statements" {
			*funcBodies = append(*funcBodies, funcBody{nid, child})
			break
		}
	}
}

func swiftCollectCalls(node *tree_sitter.Node, content []byte, calls map[string]bool) {
	if node == nil {
		return
	}

	if node.Kind() == "call_expression" {
		if node.ChildCount() >= 1 {
			funcChild := node.Child(0)
			if funcChild != nil {
				if funcChild.Kind() == "simple_identifier" {
					name := string(funcChild.Utf8Text(content))
					if name != "if" && name != "for" && name != "while" && name != "switch" &&
						name != "guard" && name != "return" && name != "print" {
						calls[name] = true
					}
				}
				if funcChild.Kind() == "navigation_expression" {
					for i := uint(0); i < funcChild.ChildCount(); i++ {
						gc := funcChild.Child(i)
						if gc.Kind() == "simple_identifier" {
							calls[string(gc.Utf8Text(content))] = true
						}
					}
				}
			}
		}
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		swiftCollectCalls(node.Child(i), content, calls)
	}
}
