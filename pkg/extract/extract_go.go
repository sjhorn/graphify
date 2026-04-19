package extract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ExtractGo extracts functions, methods, types, and imports from a Go file.
func ExtractGo(path string) *Extraction {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return &Extraction{}
	}

	stem := strings.TrimSuffix(filepath.Base(path), ".go")
	strPath := path
	pkgScope := file.Name.Name

	var nodes []Node
	var edges []Edge
	seenIDs := make(map[string]bool)
	var functions []*ast.FuncDecl

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

	fileNID := MakeId(strPath)
	addNode(fileNID, filepath.Base(path), 1)

	// Track imported packages
	importedPackages := make(map[string]string) // package name -> import path

	// Walk the AST
	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			if decl.Recv == nil {
				// Regular function
				funcNID := MakeId(stem, decl.Name.Name)
				line := fset.Position(decl.Pos()).Line
				addNode(funcNID, decl.Name.Name+"()", line)
				addEdge(fileNID, funcNID, "contains", line, "EXTRACTED", 1.0)
				functions = append(functions, decl)
			} else {
				// Method
				receiver := decl.Recv.List[0]
				var receiverType string
				if ident, ok := receiver.Type.(*ast.Ident); ok {
					receiverType = ident.Name
				} else if star, ok := receiver.Type.(*ast.StarExpr); ok {
					if ident, ok := star.X.(*ast.Ident); ok {
						receiverType = ident.Name
					}
				}
				if receiverType != "" {
					parentNID := MakeId(pkgScope, receiverType)
					line := fset.Position(decl.Pos()).Line
					addNode(parentNID, receiverType, line)
					methodNID := MakeId(parentNID, decl.Name.Name)
					addNode(methodNID, "."+decl.Name.Name+"()", line)
					addEdge(parentNID, methodNID, "method", line, "EXTRACTED", 1.0)
					functions = append(functions, decl)
				}
			}

		case *ast.TypeSpec:
			typeNID := MakeId(pkgScope, decl.Name.Name)
			line := fset.Position(decl.Pos()).Line
			addNode(typeNID, decl.Name.Name, line)
			addEdge(fileNID, typeNID, "contains", line, "EXTRACTED", 1.0)

		case *ast.ImportSpec:
			importPath := strings.Trim(decl.Path.Value, "\"")
			parts := strings.Split(importPath, "/")
			moduleName := parts[len(parts)-1]
			line := fset.Position(decl.Pos()).Line

			// Handle aliased imports
			name := moduleName
			if decl.Name != nil {
				name = decl.Name.Name
			}
			importedPackages[name] = moduleName

			tgtNID := MakeId(moduleName)
			addEdge(fileNID, tgtNID, "imports_from", line, "EXTRACTED", 1.0)
		}
		return true
	})

	// Build label to NID map
	labelToNID := make(map[string]string)
	for _, n := range nodes {
		raw := n.Label
		normalized := strings.TrimSuffix(strings.TrimPrefix(raw, "."), "()")
		labelToNID[strings.ToLower(normalized)] = n.ID
	}

	// Walk function bodies for call expressions
	seenCallPairs := make(map[string]bool)

	for _, fn := range functions {
		callerNID := MakeId(stem, fn.Name.Name)
		if fn.Recv != nil {
			// For methods, use receiver type + method name
			receiver := fn.Recv.List[0]
			if ident, ok := receiver.Type.(*ast.Ident); ok {
				callerNID = MakeId(pkgScope, ident.Name, fn.Name.Name)
			}
		}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				var calleeName string
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					calleeName = fun.Name
				case *ast.SelectorExpr:
					calleeName = fun.Sel.Name
				}
				if calleeName != "" {
					tgtNID := labelToNID[strings.ToLower(calleeName)]
					if tgtNID != "" && tgtNID != callerNID {
						pair := callerNID + "->" + tgtNID
						if !seenCallPairs[pair] {
							seenCallPairs[pair] = true
							edges = append(edges, Edge{
								Source:     callerNID,
								Target:     tgtNID,
								Relation:   "calls",
								Confidence: "EXTRACTED",
								Weight:     1.0,
							})
						}
					}
				}
			}
			return true
		})
	}

	// Filter edges
	validIDs := seenIDs
	cleanEdges := make([]Edge, 0)
	for _, edge := range edges {
		if validIDs[edge.Source] && (validIDs[edge.Target] || edge.Relation == "imports" || edge.Relation == "imports_from") {
			cleanEdges = append(cleanEdges, edge)
		}
	}

	return &Extraction{
		Nodes: nodes,
		Edges: cleanEdges,
	}
}
