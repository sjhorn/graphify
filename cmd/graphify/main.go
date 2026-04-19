package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhorn/graphify/pkg/analyze"
	"github.com/sjhorn/graphify/pkg/cache"
	"github.com/sjhorn/graphify/pkg/cluster"
	"github.com/sjhorn/graphify/pkg/detect"
	"github.com/sjhorn/graphify/pkg/export"
	"github.com/sjhorn/graphify/pkg/extract"
	"github.com/sjhorn/graphify/pkg/graph"
	"github.com/sjhorn/graphify/pkg/report"
)

func main() {
	// Check for subcommands before parsing flags
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "query":
			runQuery(os.Args[2:])
			return
		case "path":
			runPath(os.Args[2:])
			return
		case "explain":
			runExplain(os.Args[2:])
			return
		case "claude":
			runClaude(os.Args[2:], "CLAUDE.md")
			return
		case "agents":
			runClaude(os.Args[2:], "AGENTS.md")
			return
		case "help":
			printUsage()
			return
		}
	}

	outDir := flag.String("out", "graphify-out", "Output directory")
	verbose := flag.Bool("verbose", false, "Verbose output")
	flag.Parse()

	if flag.NArg() < 1 {
		printUsage()
		os.Exit(1)
	}

	root := flag.Arg(0)

	// Detect files
	fmt.Println("Detecting files...")
	result := detect.CollectFiles(root)

	if *verbose {
		fmt.Printf("Found %d code files, %d documents\n",
			len(result.Files[detect.FileTypeCode]),
			len(result.Files[detect.FileTypeDocument]))
	}

	// Get all source files
	var allFiles []string
	allFiles = append(allFiles, result.Files[detect.FileTypeCode]...)
	allFiles = append(allFiles, result.Files[detect.FileTypeDocument]...)

	// Extract nodes and edges
	fmt.Println("Extracting nodes and edges...")
	var allNodes []extract.Node
	var allEdges []extract.Edge

	var cacheHits, cacheMisses int
	for _, file := range allFiles {
		if cached, ok := cache.LoadCached(file, *outDir); ok {
			allNodes = append(allNodes, cached.Nodes...)
			allEdges = append(allEdges, cached.Edges...)
			cacheHits++
			continue
		}
		extraction := extract.Extract([]string{file}, "")
		allNodes = append(allNodes, extraction.Nodes...)
		allEdges = append(allEdges, extraction.Edges...)
		_ = cache.SaveCached(file, extraction, *outDir)
		cacheMisses++
	}

	if *verbose {
		fmt.Printf("Cache: %d hits, %d misses\n", cacheHits, cacheMisses)
	}

	// Build graph
	fmt.Println("Building graph...")
	absRoot, _ := filepath.Abs(root)
	g := graph.NewGraph()
	for _, node := range allNodes {
		file := node.File
		if file != "" {
			// filepath.Rel requires both paths to be absolute or both relative.
			// Extractors return paths as given by detect (relative to cwd),
			// so we must make them absolute first.
			absFile, err := filepath.Abs(file)
			if err == nil {
				if rel, err := filepath.Rel(absRoot, absFile); err == nil {
					file = rel
				}
			}
		}
		g.AddNode(node.ID, node.Label, node.Type, file)
	}
	for _, edge := range allEdges {
		g.AddEdge(edge.Source, edge.Target, edge.Relation, edge.Confidence, edge.Weight)
	}

	fmt.Printf("Graph: %d nodes, %d edges\n", g.NodeCount(), len(allEdges))

	// Cluster
	fmt.Println("Clustering...")
	clusterResult := cluster.Cluster(g)
	clusterResult = cluster.SplitLargeCommunities(g, clusterResult, 100)
	clusterResult = cluster.MergeTinyCommunities(g, clusterResult, 3)
	cohesionScores := cluster.ScoreAll(g, clusterResult.Communities)

	fmt.Printf("Found %d communities\n", len(clusterResult.Communities))

	// Analyze
	fmt.Println("Analyzing...")
	detectInfo := analyze.DetectResultInfo{
		TotalFiles: result.TotalFiles,
		TotalWords: result.TotalWords,
		CodeFiles:  len(result.Files[detect.FileTypeCode]),
		DocFiles:   len(result.Files[detect.FileTypeDocument]),
	}
	analysis := analyze.Analyze(g, clusterResult.Communities, detectInfo)

	// Extract docstrings from source files for god nodes
	analyze.ExtractDocstrings(analysis.GodNodeDetails, absRoot, g)

	fmt.Printf("Found %d god nodes, %d surprising connections\n",
		len(analysis.GodNodes), len(analysis.SurprisingConnections))

	// Create output directory
	os.MkdirAll(*outDir, 0755)

	// Export
	fmt.Println("Exporting...")
	jsonPath := filepath.Join(*outDir, "graph.json")
	if err := export.ToJSON(g, clusterResult, jsonPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error exporting JSON: %v\n", err)
		os.Exit(1)
	}

	htmlPath := filepath.Join(*outDir, "graph.html")
	labels := generateCommunityLabels(g, clusterResult.Communities)
	if err := export.ToHTML(g, clusterResult, htmlPath, labels); err != nil {
		fmt.Fprintf(os.Stderr, "Error exporting HTML: %v\n", err)
		os.Exit(1)
	}

	// Generate report
	fmt.Println("Generating report...")
	detection := report.DetectionResult{
		TotalFiles: result.TotalFiles,
		TotalWords: result.TotalWords,
		NeedsGraph: result.NeedsGraph,
	}
	tokens := report.TokenInfo{Input: 0, Output: 0}
	reportContent := report.Generate(g, clusterResult.Communities, cohesionScores, labels,
		analysis, detection, tokens, root)

	reportPath := filepath.Join(*outDir, "GRAPH_REPORT.md")
	if err := os.WriteFile(reportPath, []byte(reportContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDone! Output written to %s/\n", *outDir)
	fmt.Printf("  - graph.json\n")
	fmt.Printf("  - graph.html\n")
	fmt.Printf("  - GRAPH_REPORT.md\n")
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: graphify [options] <root>

Analyze a codebase and generate a knowledge graph.

Commands:
  claude [<dir>]                       Append graphify prompt to CLAUDE.md (default: current dir)
  agents [<dir>]                       Append graphify prompt to AGENTS.md (default: current dir)
  query "<question>" [--dfs] [--budget N]  Search the graph with a natural language question
  path "<nodeA>" "<nodeB>"             Find shortest path between two nodes
  explain "<node>"                     Show details and connections for a node
  help                                 Show this help message

Options:
  -out <dir>      Output directory (default: graphify-out)
  -verbose        Show detailed progress including cache hit/miss stats`)
}

// labelScore returns a priority score for how good a node is as a community label.
// Higher is better. Classes/enums beat functions beat methods. main() and test helpers score low.
func labelScore(label, nodeType string, degree int) int {
	// Reject module nodes entirely
	if nodeType == "module" {
		return -1
	}

	score := degree

	// Type bonus: prefer classes and enums as labels
	switch nodeType {
	case "class", "mixin", "extension":
		score += 100
	case "enum":
		score += 90
	case "function":
		score += 50
	case "method":
		score += 30
	case "variable":
		score += 10
	case "file":
		score -= 50
	}

	// Penalize main() and common test helper patterns
	if label == "main()" {
		score -= 200
	}
	if strings.HasPrefix(label, "_wrap") || strings.HasPrefix(label, "_make") ||
		strings.HasPrefix(label, "_build") || strings.HasPrefix(label, "_ctx") ||
		strings.HasPrefix(label, "_doc") {
		score -= 80
	}
	// Penalize private helpers (start with _)
	if strings.HasPrefix(label, "_") {
		score -= 20
	}

	return score
}

// humanizeTestFile converts a test filename like "document_selection_overlay_test.dart"
// into a readable label like "document_selection_overlay tests".
func humanizeTestFile(filename string) string {
	name := strings.TrimSuffix(filename, ".dart")
	name = strings.TrimSuffix(name, "_test")
	return name + " tests"
}

// generateCommunityLabels derives meaningful labels for communities based on
// the dominant directory and the best representative node.
func generateCommunityLabels(g *graph.Graph, communities map[int][]string) map[int]string {
	labels := make(map[int]string)
	for cid, nodeIDs := range communities {
		dirCounts := make(map[string]int)
		var bestNode string
		var bestType string
		bestScore := -999

		for _, nid := range nodeIDs {
			node := g.GetNode(nid)
			if node == nil {
				continue
			}
			if node.File != "" {
				dir := filepath.Dir(node.File)
				parts := strings.Split(dir, string(filepath.Separator))
				if len(parts) > 0 {
					dirCounts[parts[len(parts)-1]]++
				}
			}
			score := labelScore(node.Label, node.Type, g.GetNodeDegree(nid))
			if score > bestScore {
				bestScore = score
				bestNode = node.Label
				bestType = node.Type
			}
		}

		// If the best label is a test file, humanize it
		if bestType == "file" && strings.HasSuffix(bestNode, "_test.dart") {
			bestNode = humanizeTestFile(bestNode)
		}

		// Find dominant directory
		var topDir string
		topDirCount := 0
		for dir, count := range dirCounts {
			if count > topDirCount {
				topDirCount = count
				topDir = dir
			}
		}

		// Build label
		switch {
		case topDir != "" && bestNode != "":
			labels[cid] = fmt.Sprintf("%s / %s", topDir, bestNode)
		case topDir != "":
			labels[cid] = topDir
		case bestNode != "":
			labels[cid] = bestNode
		default:
			labels[cid] = fmt.Sprintf("Community %d", cid)
		}
	}
	return labels
}

func runQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	dfs := fs.Bool("dfs", false, "Use DFS traversal instead of BFS")
	budget := fs.Int("budget", 2000, "Token budget for output")
	graphDir := fs.String("dir", "graphify-out", "Directory containing graph.json")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: graphify query \"<question>\" [--dfs] [--budget N]")
		os.Exit(1)
	}

	question := fs.Arg(0)
	mode := "bfs"
	if *dfs {
		mode = "dfs"
	}

	graphPath := filepath.Join(*graphDir, "graph.json")
	if err := queryGraph(graphPath, question, mode, *budget); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runPath(args []string) {
	fs := flag.NewFlagSet("path", flag.ExitOnError)
	graphDir := fs.String("dir", "graphify-out", "Directory containing graph.json")
	fs.Parse(args)

	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "Usage: graphify path \"<nodeA>\" \"<nodeB>\"")
		os.Exit(1)
	}

	graphPath := filepath.Join(*graphDir, "graph.json")
	if err := queryPath(graphPath, fs.Arg(0), fs.Arg(1)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

const claudePrompt = `## Codebase exploration with graphify

When exploring this codebase for the first time, or when researching how components connect:

1. Build the knowledge graph (once per session):
   ` + "```" + `bash
   graphify .
   ` + "```" + `

2. Read the architecture overview:
   - ` + "`" + `graphify-out/GRAPH_REPORT.md` + "`" + ` — god nodes, communities, design patterns, dependency layers

3. Use subcommands for targeted exploration:
   ` + "```" + `bash
   # Natural language graph traversal (budget = max output tokens)
   graphify query "How does authentication work?" --budget 3000

   # Trace connections between two entities
   graphify path "AuthService" "UserRepository"

   # Deep dive on a specific node
   graphify explain "DatabaseClient"
   ` + "```" + `

Prefer graphify over grepping when the question is architectural ("how does X connect to Y?",
"what depends on Z?", "what are the core abstractions?").
`

func runClaude(args []string, targetFile string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	targetPath := filepath.Join(dir, targetFile)

	// Check if section already exists
	existing, err := os.ReadFile(targetPath)
	if err == nil && strings.Contains(string(existing), "## Codebase exploration with graphify") {
		fmt.Printf("%s already contains graphify prompt — skipping.\n", targetFile)
		return
	}

	f, err := os.OpenFile(targetPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", targetPath, err)
		os.Exit(1)
	}
	defer f.Close()

	// Add a newline separator if appending to existing content
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		f.WriteString("\n")
	}
	if len(existing) > 0 {
		f.WriteString("\n")
	}

	if _, err := f.WriteString(claudePrompt); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", targetPath, err)
		os.Exit(1)
	}

	if len(existing) == 0 {
		fmt.Printf("Created %s with graphify prompt.\n", targetPath)
	} else {
		fmt.Printf("Appended graphify prompt to %s.\n", targetPath)
	}
}

func runExplain(args []string) {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	graphDir := fs.String("dir", "graphify-out", "Directory containing graph.json")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: graphify explain \"<node>\"")
		os.Exit(1)
	}

	graphPath := filepath.Join(*graphDir, "graph.json")
	if err := queryExplain(graphPath, fs.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
