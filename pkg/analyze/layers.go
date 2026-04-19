package analyze

import (
	"sort"
	"strings"

	"github.com/sjhorn/graphify/pkg/graph"
)

// DirectoryDep represents a dependency between two directories.
type DirectoryDep struct {
	From  string
	To    string
	Count int
}

// LayerAnalysisResult contains the results of dependency direction analysis.
type LayerAnalysisResult struct {
	DirectoryDeps    []DirectoryDep // all deps
	SourceDeps       []DirectoryDep // source-only (excluding test/benchmark dirs)
	Cycles           [][]string
	SourceCycles     [][]string
	LayerOrder       []string
	SourceLayerOrder []string
	Violations       []string
}

// AnalyzeLayers scans imports and calls edges to build a directory-level
// dependency graph, detect cycles, and compute a topological layer order.
func AnalyzeLayers(g *graph.Graph) *LayerAnalysisResult {
	// Build directed adjacency map between 2nd-level directories
	depCounts := make(map[string]map[string]int) // from -> to -> count
	allDirs := make(map[string]bool)

	for _, edge := range g.Edges() {
		if edge.Relation != "imports" && edge.Relation != "imports_from" && edge.Relation != "calls" {
			continue
		}
		srcNode := g.GetNode(edge.Source)
		tgtNode := g.GetNode(edge.Target)
		if srcNode == nil || tgtNode == nil {
			continue
		}
		srcDir := secondLevelDir(srcNode.File)
		tgtDir := secondLevelDir(tgtNode.File)
		if srcDir == "" || tgtDir == "" || srcDir == tgtDir {
			continue
		}
		allDirs[srcDir] = true
		allDirs[tgtDir] = true
		if depCounts[srcDir] == nil {
			depCounts[srcDir] = make(map[string]int)
		}
		depCounts[srcDir][tgtDir]++
	}

	// Build DirectoryDeps list
	var deps []DirectoryDep
	adj := make(map[string][]string)
	for from, targets := range depCounts {
		for to, count := range targets {
			deps = append(deps, DirectoryDep{From: from, To: to, Count: count})
			adj[from] = append(adj[from], to)
		}
	}
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Count > deps[j].Count
	})

	// Build source-only deps (exclude test/benchmark/integration_test/example dirs)
	nonSourceDirs := map[string]bool{
		"test": true, "tests": true, "testing": true,
		"benchmark": true, "benchmarks": true,
		"integration_test": true, "example": true, "examples": true,
	}
	var sourceDeps []DirectoryDep
	sourceAdj := make(map[string][]string)
	sourceDirs := make(map[string]bool)
	for _, dep := range deps {
		if nonSourceDirs[dep.From] || nonSourceDirs[dep.To] {
			continue
		}
		sourceDeps = append(sourceDeps, dep)
		sourceAdj[dep.From] = append(sourceAdj[dep.From], dep.To)
		sourceDirs[dep.From] = true
		sourceDirs[dep.To] = true
	}

	// Detect cycles via DFS
	cycles := detectCycles(adj, allDirs)
	sourceCycles := detectCycles(sourceAdj, sourceDirs)

	// Compute topological order (best-effort, ignoring back edges)
	layerOrder := topoSort(adj, allDirs)
	sourceLayerOrder := topoSort(sourceAdj, sourceDirs)

	// Generate violation strings
	var violations []string
	for _, cycle := range sourceCycles {
		violations = append(violations, "cycle: "+strings.Join(cycle, " -> "))
	}

	return &LayerAnalysisResult{
		DirectoryDeps:    deps,
		SourceDeps:       sourceDeps,
		Cycles:           cycles,
		SourceCycles:     sourceCycles,
		LayerOrder:       layerOrder,
		SourceLayerOrder: sourceLayerOrder,
		Violations:    violations,
	}
}

// secondLevelDir extracts the 2nd-level directory component from a file path.
// e.g. "lib/src/widgets/button.dart" -> "widgets"
// e.g. "src/models/user.go" -> "models"
// Falls back to first component if path is short.
func secondLevelDir(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	// Skip common top-level dirs like "lib", "src", "lib/src"
	for i, p := range parts {
		if p == "lib" || p == "src" || p == "pkg" || p == "app" {
			continue
		}
		// If this is a file (last part), skip it
		if i == len(parts)-1 {
			break
		}
		return p
	}
	// Fallback: use the directory immediately above the file
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return ""
}

// detectCycles finds all simple cycles in the directed graph using DFS.
func detectCycles(adj map[string][]string, allDirs map[string]bool) [][]string {
	const white, gray, black = 0, 1, 2
	color := make(map[string]int)
	parent := make(map[string]string)
	var cycles [][]string
	seen := make(map[string]bool) // dedup cycle representations

	var dfs func(u string)
	dfs = func(u string) {
		color[u] = gray
		for _, v := range adj[u] {
			if color[v] == gray {
				// Found a cycle — reconstruct
				cycle := []string{v}
				cur := u
				for cur != v {
					cycle = append([]string{cur}, cycle...)
					cur = parent[cur]
				}
				cycle = append(cycle, v) // close the cycle
				// Normalize for dedup: rotate so smallest element is first
				key := normalizeCycle(cycle)
				if !seen[key] {
					seen[key] = true
					cycles = append(cycles, cycle)
				}
			} else if color[v] == white {
				parent[v] = u
				dfs(v)
			}
		}
		color[u] = black
	}

	for dir := range allDirs {
		if color[dir] == white {
			dfs(dir)
		}
	}
	return cycles
}

func normalizeCycle(cycle []string) string {
	if len(cycle) <= 1 {
		return strings.Join(cycle, ",")
	}
	// Remove the closing element (duplicate of first)
	open := cycle[:len(cycle)-1]
	// Find index of smallest element
	minIdx := 0
	for i, s := range open {
		if s < open[minIdx] {
			minIdx = i
		}
	}
	// Rotate
	rotated := make([]string, len(open))
	for i := range open {
		rotated[i] = open[(i+minIdx)%len(open)]
	}
	return strings.Join(rotated, ",")
}

// topoSort computes a topological order, skipping back edges.
func topoSort(adj map[string][]string, allDirs map[string]bool) []string {
	inDegree := make(map[string]int)
	for dir := range allDirs {
		inDegree[dir] = 0
	}
	for _, targets := range adj {
		for _, t := range targets {
			inDegree[t]++
		}
	}

	var queue []string
	for dir, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, dir)
		}
	}
	sort.Strings(queue) // deterministic

	var order []string
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		targets := adj[u]
		sort.Strings(targets)
		for _, v := range targets {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	// Add remaining nodes (in cycles) at the end
	for dir := range allDirs {
		found := false
		for _, o := range order {
			if o == dir {
				found = true
				break
			}
		}
		if !found {
			order = append(order, dir)
		}
	}

	return order
}
