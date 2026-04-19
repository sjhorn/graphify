package analyze

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sjhorn/graphify/pkg/graph"
)

// GodNode represents a highly-connected node in the graph.
type GodNode struct {
	ID     string
	Label  string
	Degree int
	File   string
}

// SurprisingConnection represents a non-obvious connection between nodes.
type SurprisingConnection struct {
	Source      string
	Target      string
	SourceFiles []string
	Score       int
	Confidence  string
	Relation    string
	Why         string
}

// Analysis contains the results of graph analysis.
type Analysis struct {
	GodNodes              []GodNode
	GodNodeDetails        []GodNodeDetail
	InheritanceTrees      []InheritanceTree
	SurprisingConnections []SurprisingConnection
	SuggestedQuestions    []SuggestedQuestion
	Patterns              []DesignPattern
	Enums                 []EnumSummary
	KeyFiles              []KeyFile
	RuntimeDeps           []RuntimeDep
	Layers                *LayerAnalysisResult
	Summary               string
	DirectoryStats        []DirectoryStats
	SingletonCount        int
}

// SuggestedQuestion represents a question about the graph.
type SuggestedQuestion struct {
	Type     string
	Question string
	Why      string
}

// GodNodes returns the top N most-connected real entities.
func GodNodes(g *graph.Graph, topN int) []GodNode {
	degreeMap := make(map[string]int)
	for _, node := range g.Nodes() {
		neighbors := g.GetNodeNeighbors(node.ID)
		degreeMap[node.ID] = len(neighbors)
	}

	// Sort nodes by degree
	type nodeDegree struct {
		ID     string
		Degree int
	}
	sorted := make([]nodeDegree, 0, len(degreeMap))
	for id, deg := range degreeMap {
		sorted = append(sorted, nodeDegree{ID: id, Degree: deg})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Degree > sorted[j].Degree
	})

	result := make([]GodNode, 0, topN)
	for _, nd := range sorted {
		if len(result) >= topN {
			break
		}
		node := g.GetNode(nd.ID)
		if node == nil {
			continue
		}
		// Skip file-level hub nodes, concept nodes, and external framework types
		if isFileNode(g, nd.ID) || isConceptNode(g, nd.ID) || isExternalType(g, nd.ID) {
			continue
		}
		result = append(result, GodNode{
			ID:     nd.ID,
			Label:  node.Label,
			Degree: nd.Degree,
			File:   node.File,
		})
	}
	return result
}

// isFileNode returns true if this node is a file-level hub node.
func isFileNode(g *graph.Graph, nodeID string) bool {
	node := g.GetNode(nodeID)
	if node == nil {
		return false
	}

	// Use type field if available
	if node.Type == "file" {
		return true
	}

	// Fallback: label matches the actual source filename
	if node.File != "" {
		baseName := filepathBase(node.File)
		if node.Label == baseName {
			return true
		}
	}

	return false
}

// isConceptNode returns true if this node is a semantic concept node (no source file).
func isConceptNode(g *graph.Graph, nodeID string) bool {
	node := g.GetNode(nodeID)
	if node == nil {
		return false
	}
	if node.Type == "module" {
		return true
	}
	if node.File == "" {
		return true
	}
	return false
}

// isExternalType returns true if a node appears to be an external framework type —
// it has only incoming inherits edges and no outgoing edges of its own.
func isExternalType(g *graph.Graph, nodeID string) bool {
	node := g.GetNode(nodeID)
	if node == nil || node.Type != "class" {
		return false
	}
	// Check outgoing edges — if any, it's defined in the project
	for _, edge := range g.Edges() {
		if edge.Source == nodeID {
			return false
		}
	}
	// Check incoming edges — must be only inherits
	hasInherits := false
	for _, edge := range g.Edges() {
		if edge.Target == nodeID {
			if edge.Relation != "inherits" {
				return false
			}
			hasInherits = true
		}
	}
	return hasInherits
}

// FileCategory returns the category of a file based on its extension.
func FileCategory(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	codeExtensions := map[string]bool{
		".py": true, ".go": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".java": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".rb": true, ".cs": true, ".kt": true, ".scala": true, ".php": true,
		".swift": true, ".ex": true, ".exs": true, ".m": true, ".mm": true,
		".lua": true, ".zig": true, ".jl": true, ".r": true, ".rmd": true,
		".ps1": true, ".sh": true, ".bash": true, ".zsh": true,
		".rs": true, ".hs": true, ".clj": true, ".erl": true, ".elm": true,
	}
	paperExtensions := map[string]bool{
		".pdf": true, ".tex": true, ".bib": true, ".md": true,
	}
	imageExtensions := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true,
	}

	if codeExtensions[ext] {
		return "code"
	}
	if paperExtensions[ext] {
		return "paper"
	}
	if imageExtensions[ext] {
		return "image"
	}
	return "doc"
}

// TopLevelDir returns the first path component.
func TopLevelDir(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return path
}

// SurpriseScore calculates how surprising a connection is.
func SurpriseScore(g *graph.Graph, u, v string, confidence string, nodeCommunity map[string]int, uSource, vSource string) (int, []string) {
	score := 0
	reasons := []string{}

	// 1. Confidence weight
	confBonus := map[string]int{
		"AMBIGUOUS": 3,
		"INFERRED":  2,
		"EXTRACTED": 1,
	}
	if bonus, ok := confBonus[confidence]; ok {
		score += bonus
		if confidence == "AMBIGUOUS" || confidence == "INFERRED" {
			reasons = append(reasons, fmt.Sprintf("%s connection - not explicitly stated in source", confidence))
		}
	}

	// 1b. Relation weight — calls are more surprising than inherits across files
	uEdge := g.GetEdge(u, v)
	if uEdge == nil {
		uEdge = g.GetEdge(v, u)
	}
	if uEdge != nil && uEdge.Relation == "calls" {
		score += 2
		reasons = append(reasons, "runtime call dependency")
	}

	// 2. Cross file-type bonus
	catU := FileCategory(uSource)
	catV := FileCategory(vSource)
	if catU != catV {
		score += 2
		reasons = append(reasons, fmt.Sprintf("crosses file types (%s <-> %s)", catU, catV))
	}

	// 3. Cross-repo bonus
	if TopLevelDir(uSource) != TopLevelDir(vSource) {
		score += 2
		reasons = append(reasons, "connects across different repos/directories")
	}

	// 4. Cross-community bonus
	cidU := nodeCommunity[u]
	cidV := nodeCommunity[v]
	if cidU != cidV {
		score += 1
		reasons = append(reasons, "bridges separate communities")
	}

	// 5. Peripheral->hub
	degU := g.GetNodeDegree(u)
	degV := g.GetNodeDegree(v)
	minDeg := min(degU, degV)
	maxDeg := max(degU, degV)
	if minDeg <= 2 && maxDeg >= 5 {
		score += 1
		reasons = append(reasons, fmt.Sprintf("peripheral node reaches hub"))
	}

	return score, reasons
}

// SurprisingConnections finds non-obvious connections in the graph.
func SurprisingConnections(g *graph.Graph, communities map[int][]string, topN int) []SurprisingConnection {
	// Build node -> community map
	nodeCommunity := make(map[string]int)
	for cid, nodes := range communities {
		for _, node := range nodes {
			nodeCommunity[node] = cid
		}
	}

	// Collect unique source files
	sourceFiles := make(map[string]bool)
	for _, node := range g.Nodes() {
		if node.File != "" {
			sourceFiles[node.File] = true
		}
	}
	_ = len(sourceFiles) // may be used later

	var candidates []SurprisingConnection

	for _, edge := range g.Edges() {
		u := edge.Source
		v := edge.Target
		relation := edge.Relation
		confidence := edge.Confidence

		// Skip structural edges
		if relation == "imports" || relation == "imports_from" || relation == "contains" || relation == "method" || relation == "case_of" {
			continue
		}

		// Penalize inherits edges — they're common and rarely surprising
		if relation == "inherits" {
			// Skip inherits edges to common base types (moderate+ in-degree)
			if g.GetNodeDegree(v) > 5 {
				continue
			}
		}

		// Skip concept nodes and external framework types
		if isConceptNode(g, u) || isConceptNode(g, v) {
			continue
		}
		if isExternalType(g, u) || isExternalType(g, v) {
			continue
		}

		// Skip file nodes
		if isFileNode(g, u) || isFileNode(g, v) {
			continue
		}

		uNode := g.GetNode(u)
		vNode := g.GetNode(v)
		if uNode == nil || vNode == nil {
			continue
		}

		uSource := uNode.File
		vSource := vNode.File

		// Same file - skip for multi-source
		if uSource == vSource {
			continue
		}

		score, reasons := SurpriseScore(g, u, v, confidence, nodeCommunity, uSource, vSource)

		candidates = append(candidates, SurprisingConnection{
			Source:      uNode.Label,
			Target:      vNode.Label,
			SourceFiles: []string{uSource, vSource},
			Score:       score,
			Confidence:  confidence,
			Relation:    relation,
			Why:         strings.Join(reasons, "; "),
		})
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Deduplicate by exact pair AND by pattern (same target + relation).
	// This prevents e.g. 7 variants of "XBuilder → ComponentBuilder [inherits]".
	seenPair := make(map[string]bool)
	seenPattern := make(map[string]int) // target+relation → count
	maxPerPattern := 2                  // allow at most 2 examples per pattern
	var deduped []SurprisingConnection
	for _, c := range candidates {
		// Exact pair dedup
		key := c.Source + "→" + c.Target
		reverseKey := c.Target + "→" + c.Source
		if seenPair[key] || seenPair[reverseKey] {
			continue
		}
		// Pattern dedup: same target + same relation
		patternKey := c.Target + ":" + c.Relation
		if seenPattern[patternKey] >= maxPerPattern {
			continue
		}
		seenPair[key] = true
		seenPattern[patternKey]++
		deduped = append(deduped, c)
		if len(deduped) >= topN {
			break
		}
	}

	return deduped
}

// Analyze runs the full graph analysis.
func Analyze(g *graph.Graph, communities map[int][]string, detection DetectResultInfo) *Analysis {
	godNodes := GodNodes(g, 10)
	godNodeDetails := EnrichGodNodes(g, godNodes)
	inheritanceTrees := BuildInheritanceTrees(g, godNodes)
	surprises := SurprisingConnections(g, communities, 10)
	patterns := DetectPatterns(g)
	enums := DetectEnums(g)
	keyFiles := FindKeyFiles(g, 8)
	runtimeDeps := FindRuntimeDeps(g, 10)
	layers := AnalyzeLayers(g)
	dirStats := ComputeDirectoryStats(g)

	singletons := 0
	for _, nodes := range communities {
		if len(nodes) == 1 {
			singletons++
		}
	}

	summary := GenerateSummary(g, communities, godNodes, detection, patterns, layers, dirStats)

	return &Analysis{
		GodNodes:              godNodes,
		GodNodeDetails:        godNodeDetails,
		InheritanceTrees:      inheritanceTrees,
		SurprisingConnections: surprises,
		Patterns:              patterns,
		Enums:                 enums,
		KeyFiles:              keyFiles,
		RuntimeDeps:           runtimeDeps,
		Layers:                layers,
		Summary:               summary,
		DirectoryStats:        dirStats,
		SingletonCount:        singletons,
	}
}

// NewGraph creates a new empty graph for testing.
func NewGraph() *graph.Graph {
	return graph.NewGraph()
}

// Helper functions

func filepathBase(path string) string {
	// Simple base path extraction
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash >= 0 {
		return path[lastSlash+1:]
	}
	return path
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
