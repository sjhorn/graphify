package analyze

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sjhorn/graphify/pkg/graph"
)

// DetectResultInfo contains corpus detection metadata for the summary.
type DetectResultInfo struct {
	TotalFiles int
	TotalWords int
	CodeFiles  int
	DocFiles   int
}

// DirectoryStats contains per-directory counts.
type DirectoryStats struct {
	Directory string
	FileCount int
	NodeCount int
}

// ComputeDirectoryStats groups nodes by their 2nd-level directory and counts
// unique files and total nodes per directory.
func ComputeDirectoryStats(g *graph.Graph) []DirectoryStats {
	dirFiles := make(map[string]map[string]bool)
	dirNodes := make(map[string]int)

	for _, node := range g.Nodes() {
		dir := secondLevelDir(node.File)
		if dir == "" {
			continue
		}
		if dirFiles[dir] == nil {
			dirFiles[dir] = make(map[string]bool)
		}
		if node.File != "" {
			dirFiles[dir][node.File] = true
		}
		dirNodes[dir]++
	}

	var stats []DirectoryStats
	for dir, files := range dirFiles {
		stats = append(stats, DirectoryStats{
			Directory: dir,
			FileCount: len(files),
			NodeCount: dirNodes[dir],
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].NodeCount > stats[j].NodeCount
	})
	return stats
}

// GenerateSummary builds a concise executive summary paragraph from analysis data.
func GenerateSummary(
	g *graph.Graph,
	communities map[int][]string,
	godNodes []GodNode,
	detection DetectResultInfo,
	patterns []DesignPattern,
	layers *LayerAnalysisResult,
	dirStats []DirectoryStats,
) string {
	var parts []string

	// Project scale
	singletons := 0
	for _, nodes := range communities {
		if len(nodes) == 1 {
			singletons++
		}
	}
	nonSingletonCount := len(communities) - singletons

	parts = append(parts, fmt.Sprintf(
		"Project contains %d files with %d nodes and %d edges, organized into %d communities",
		detection.TotalFiles, g.NodeCount(), g.EdgeCount(), nonSingletonCount,
	))
	if singletons > 0 {
		parts[len(parts)-1] += fmt.Sprintf(" (plus %d isolated nodes)", singletons)
	}
	parts[len(parts)-1] += "."

	// Directory breakdown (top 3-4)
	if len(dirStats) > 0 {
		limit := 4
		if len(dirStats) < limit {
			limit = len(dirStats)
		}
		var dirParts []string
		for _, ds := range dirStats[:limit] {
			dirParts = append(dirParts, fmt.Sprintf("%s (%d files, %d nodes)", ds.Directory, ds.FileCount, ds.NodeCount))
		}
		parts = append(parts, "Largest directories: "+strings.Join(dirParts, "; ")+".")
	}

	// Core abstractions
	if len(godNodes) > 0 {
		limit := 3
		if len(godNodes) < limit {
			limit = len(godNodes)
		}
		var names []string
		for _, gn := range godNodes[:limit] {
			names = append(names, gn.Label)
		}
		parts = append(parts, "Core abstractions: "+strings.Join(names, ", ")+".")
	}

	// Architecture
	if layers != nil {
		if len(layers.Cycles) == 0 {
			parts = append(parts, "Architecture shows clean layering with no dependency cycles.")
		} else {
			parts = append(parts, fmt.Sprintf(
				"Architecture has %d dependency cycle(s) that may warrant refactoring.",
				len(layers.Cycles),
			))
		}
	}

	// Patterns
	if len(patterns) > 0 {
		var patNames []string
		for _, p := range patterns {
			patNames = append(patNames, fmt.Sprintf("%s (%s)", p.Name, p.AnchorNode))
		}
		parts = append(parts, "Design patterns detected: "+strings.Join(patNames, ", ")+".")
	}

	return strings.Join(parts, " ")
}
