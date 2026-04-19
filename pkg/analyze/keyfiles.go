package analyze

import (
	"sort"
	"strings"

	"github.com/sjhorn/graphify/pkg/graph"
)

// KeyFile represents a high-value source file worth reading.
type KeyFile struct {
	Path        string
	TotalDegree int // sum of degrees of all nodes in this file
	NodeCount   int
	ClassCount  int
	MethodCount int
	ClassNames  []string // class labels in this file
}

// RuntimeDep represents a cross-file runtime call dependency.
type RuntimeDep struct {
	FromFile string
	ToFile   string
	Count    int
}

// FindKeyFiles ranks source files by information density and returns the top N.
// Excludes test, benchmark, example, and native platform files.
func FindKeyFiles(g *graph.Graph, topN int) []KeyFile {
	// Group nodes by file
	fileNodes := make(map[string][]string)
	for _, node := range g.Nodes() {
		if node.File != "" {
			fileNodes[node.File] = append(fileNodes[node.File], node.ID)
		}
	}

	// Compute degree per node
	nodeDegree := make(map[string]int)
	for _, edge := range g.Edges() {
		nodeDegree[edge.Source]++
		nodeDegree[edge.Target]++
	}

	// Score each file
	var files []KeyFile
	for path, nodeIDs := range fileNodes {
		if isNonSourceFile(path) {
			continue
		}

		totalDegree := 0
		classCount := 0
		methodCount := 0
		var classNames []string

		for _, nid := range nodeIDs {
			totalDegree += nodeDegree[nid]
			node := g.GetNode(nid)
			if node != nil {
				switch node.Type {
				case "class", "mixin", "extension":
					classCount++
					classNames = append(classNames, node.Label)
				case "method":
					methodCount++
				}
			}
		}

		if totalDegree < 10 {
			continue
		}

		sort.Strings(classNames)
		files = append(files, KeyFile{
			Path:        path,
			TotalDegree: totalDegree,
			NodeCount:   len(nodeIDs),
			ClassCount:  classCount,
			MethodCount: methodCount,
			ClassNames:  classNames,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].TotalDegree > files[j].TotalDegree
	})

	if len(files) > topN {
		files = files[:topN]
	}

	return files
}

// FindRuntimeDeps finds cross-file "calls" edges and aggregates them.
func FindRuntimeDeps(g *graph.Graph, topN int) []RuntimeDep {
	depCounts := make(map[string]int) // "from|to" -> count

	for _, edge := range g.Edges() {
		if edge.Relation != "calls" {
			continue
		}
		srcNode := g.GetNode(edge.Source)
		tgtNode := g.GetNode(edge.Target)
		if srcNode == nil || tgtNode == nil {
			continue
		}
		if srcNode.File == "" || tgtNode.File == "" || srcNode.File == tgtNode.File {
			continue
		}
		if isNonSourceFile(srcNode.File) || isNonSourceFile(tgtNode.File) {
			continue
		}
		key := srcNode.File + "|" + tgtNode.File
		depCounts[key]++
	}

	var deps []RuntimeDep
	for key, count := range depCounts {
		parts := strings.SplitN(key, "|", 2)
		deps = append(deps, RuntimeDep{FromFile: parts[0], ToFile: parts[1], Count: count})
	}

	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Count > deps[j].Count
	})

	if len(deps) > topN {
		deps = deps[:topN]
	}

	return deps
}

func isNonSourceFile(path string) bool {
	lower := strings.ToLower(path)
	// Skip test/benchmark/example files
	if strings.Contains(lower, "/test/") || strings.HasPrefix(lower, "test/") {
		return true
	}
	if strings.Contains(lower, "/benchmark/") || strings.HasPrefix(lower, "benchmark/") {
		return true
	}
	if strings.Contains(lower, "/example/") || strings.HasPrefix(lower, "example/") {
		return true
	}
	// Skip native platform files
	for _, ext := range []string{".swift", ".m", ".kt", ".java", ".cpp", ".h", ".cs"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	// Skip generated files
	if strings.Contains(lower, "generated") || strings.Contains(lower, "runner/") {
		return true
	}
	return false
}
