package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sjhorn/graphify/pkg/extract"
	"github.com/sjhorn/graphify/pkg/graph"
)

// TestCLIEndToEnd runs the full CLI pipeline and verifies output
func TestCLIEndToEnd(t *testing.T) {
	// Create temp directory for output
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "graphify-out")

	// Build the CLI first
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "graphify"), "./cmd/graphify")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("CLI build failed: %v", err)
	}

	// Run the CLI
	cmd := exec.Command(filepath.Join(tmpDir, "graphify"), "-out", outDir, "testdata/fixtures/")
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\nOutput: %s", err, output)
	}

	// Verify output directory was created
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		t.Fatal("Output directory was not created")
	}

	// Verify graph.json was created
	jsonPath := filepath.Join(outDir, "graph.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Fatal("graph.json was not created")
	}

	// Verify graph.json is valid JSON with expected structure
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}

	var graph map[string]interface{}
	if err := json.Unmarshal(data, &graph); err != nil {
		t.Fatalf("Invalid JSON in graph.json: %v", err)
	}

	if _, ok := graph["nodes"]; !ok {
		t.Error("graph.json missing 'nodes' field")
	}
	if _, ok := graph["links"]; !ok {
		t.Error("graph.json missing 'links' field")
	}

	// Verify GRAPH_REPORT.md was created
	reportPath := filepath.Join(outDir, "GRAPH_REPORT.md")
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Fatal("GRAPH_REPORT.md was not created")
	}

	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}

	// Verify report contains expected sections
	expectedSections := []string{
		"# Graph Report",
		"## Executive Summary",
		"## God Nodes",
		"## Surprising Connections",
		"## Communities",
	}
	for _, section := range expectedSections {
		if !contains(string(report), section) {
			t.Errorf("Report missing section: %s", section)
		}
	}

	// Verify graph.html was created
	htmlPath := filepath.Join(outDir, "graph.html")
	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		t.Fatal("graph.html was not created")
	}
}

// TestCLIVerboseFlag tests the -verbose flag
func TestCLIVerboseFlag(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "graphify-out")

	// Build the CLI first
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "graphify"), "./cmd/graphify")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("CLI build failed: %v", err)
	}

	// Run the CLI
	cmd := exec.Command(filepath.Join(tmpDir, "graphify"), "-out", outDir, "-verbose", "testdata/fixtures/")
	cmd.Dir = filepath.Join("..", "..")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\nOutput: %s", err, output)
	}

	// Verify verbose output contains file counts
	outputStr := string(output)
	if !contains(outputStr, "code files") {
		t.Error("Verbose output should mention 'code files'")
	}
}

// TestExtractPythonIntegration tests Python extraction end-to-end
func TestExtractPythonIntegration(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	samplePy := filepath.Join(fixturesDir, "sample.py")

	// Check if sample.py exists
	if _, err := os.Stat(samplePy); os.IsNotExist(err) {
		t.Skip("sample.py not found, skipping integration test")
	}

	// Extract
	result := extract.ExtractPython(samplePy)

	// Verify nodes were extracted
	if len(result.Nodes) == 0 {
		t.Error("Expected nodes to be extracted")
	}

	// Verify edges were extracted
	if len(result.Edges) == 0 {
		t.Error("Expected edges to be extracted")
	}

	// Verify Transformer class was found
	foundTransformer := false
	for _, node := range result.Nodes {
		if node.Label == "Transformer" {
			foundTransformer = true
			break
		}
	}
	if !foundTransformer {
		t.Error("Expected Transformer class to be extracted")
	}

	// Verify forward method was found (methods have .name() label)
	foundForward := false
	for _, node := range result.Nodes {
		if node.Label == ".forward()" {
			foundForward = true
			break
		}
	}
	if !foundForward {
		t.Error("Expected forward method to be extracted")
	}
}

// TestMakeIdIntegration tests the MakeId function with various inputs
func TestMakeIdIntegration(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"_auth"}, "auth"},
		{[]string{".httpx", "_client"}, "httpx_client"},
		{[]string{"foo", "Bar"}, "foo_bar"},
		{[]string{"__init__"}, "init"},
		{[]string{"Transformer", "forward"}, "transformer_forward"},
	}

	for _, tt := range tests {
		result := extract.MakeId(tt.input...)
		if result != tt.expected {
			t.Errorf("MakeId(%v) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

// TestGraphJSONStructure verifies the JSON output structure matches expectations
func TestGraphJSONStructure(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "graphify-out")

	// Build the CLI first
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "graphify"), "./cmd/graphify")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("CLI build failed: %v", err)
	}

	// Run the CLI
	cmd := exec.Command(filepath.Join(tmpDir, "graphify"), "-out", outDir, "testdata/fixtures/")
	cmd.Dir = filepath.Join("..", "..")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	jsonPath := filepath.Join(outDir, "graph.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}

	var graph map[string]interface{}
	if err := json.Unmarshal(data, &graph); err != nil {
		t.Fatal(err)
	}

	// Verify nodes structure
	nodes, ok := graph["nodes"].([]interface{})
	if !ok {
		t.Fatal("nodes should be an array")
	}

	for i, node := range nodes {
		n, ok := node.(map[string]interface{})
		if !ok {
			t.Errorf("Node %d should be an object", i)
			continue
		}
		if _, ok := n["id"]; !ok {
			t.Errorf("Node %d missing 'id'", i)
		}
		if _, ok := n["label"]; !ok {
			t.Errorf("Node %d missing 'label'", i)
		}
		if _, ok := n["community"]; !ok {
			t.Errorf("Node %d missing 'community'", i)
		}
	}

	// Verify links structure
	links, ok := graph["links"].([]interface{})
	if !ok {
		t.Fatal("links should be an array")
	}

	for i, link := range links {
		l, ok := link.(map[string]interface{})
		if !ok {
			t.Errorf("Link %d should be an object", i)
			continue
		}
		if _, ok := l["source"]; !ok {
			t.Errorf("Link %d missing 'source'", i)
		}
		if _, ok := l["target"]; !ok {
			t.Errorf("Link %d missing 'target'", i)
		}
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// --- Unit tests for main.go ---

func TestLabelScore(t *testing.T) {
	tests := []struct {
		label    string
		nodeType string
		degree   int
		wantSign int // 1 = positive, -1 = negative, 0 = don't check
	}{
		{"MyClass", "class", 5, 1},
		{"MyEnum", "enum", 3, 1},
		{"helper()", "function", 2, 1},
		{".process()", "method", 1, 1},
		{"main()", "function", 10, -1},
		{"mylib", "module", 10, -1},
		{"_wrapHelper", "function", 5, -1},
	}

	for _, tt := range tests {
		score := labelScore(tt.label, tt.nodeType, tt.degree)
		if tt.wantSign > 0 && score <= 0 {
			t.Errorf("labelScore(%q, %q, %d) = %d; want positive", tt.label, tt.nodeType, tt.degree, score)
		}
		if tt.wantSign < 0 && score >= 0 {
			t.Errorf("labelScore(%q, %q, %d) = %d; want negative", tt.label, tt.nodeType, tt.degree, score)
		}
	}

	// Class should beat function
	classScore := labelScore("MyClass", "class", 0)
	funcScore := labelScore("helper()", "function", 0)
	if classScore <= funcScore {
		t.Errorf("class score %d should beat function score %d", classScore, funcScore)
	}
}

func TestHumanizeTestFile(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"document_selection_overlay_test.dart", "document_selection_overlay tests"},
		{"widget_test.dart", "widget tests"},
		{"plain.dart", "plain tests"},
	}
	for _, tt := range tests {
		got := humanizeTestFile(tt.input)
		if got != tt.want {
			t.Errorf("humanizeTestFile(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateCommunityLabels(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("n1", "UserService", "class", "services/user.go")
	g.AddNode("n2", ".process()", "method", "services/user.go")
	g.AddNode("n3", "main()", "function", "cmd/main.go")
	g.AddEdge("n1", "n2", "method", "EXTRACTED", 1.0)

	communities := map[int][]string{
		0: {"n1", "n2"},
		1: {"n3"},
	}

	labels := generateCommunityLabels(g, communities)

	if len(labels) != 2 {
		t.Fatalf("generateCommunityLabels() returned %d labels; want 2", len(labels))
	}
	// Community 0 should prefer UserService (class) over .process() (method)
	if !strings.Contains(labels[0], "UserService") {
		t.Errorf("Community 0 label = %q; want to contain UserService", labels[0])
	}
}

func TestRunClaudeCLAUDE(t *testing.T) {
	dir := t.TempDir()

	// Creates CLAUDE.md
	runClaude([]string{dir}, "CLAUDE.md")
	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md not created: %v", err)
	}
	if !strings.Contains(string(data), "## Codebase exploration with graphify") {
		t.Error("CLAUDE.md missing graphify prompt")
	}

	// Idempotent — doesn't duplicate
	runClaude([]string{dir}, "CLAUDE.md")
	data2, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if len(data2) != len(data) {
		t.Errorf("runClaude should be idempotent; file grew from %d to %d bytes", len(data), len(data2))
	}
}

func TestRunClaudeAGENTS(t *testing.T) {
	dir := t.TempDir()

	runClaude([]string{dir}, "AGENTS.md")
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not created: %v", err)
	}
	if !strings.Contains(string(data), "## Codebase exploration with graphify") {
		t.Error("AGENTS.md missing graphify prompt")
	}
}

func TestRunClaudeAppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	existing := "# My Project\n\nExisting content.\n"
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(existing), 0644)

	runClaude([]string{dir}, "CLAUDE.md")
	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	content := string(data)

	if !strings.HasPrefix(content, "# My Project") {
		t.Error("runClaude should preserve existing content")
	}
	if !strings.Contains(content, "## Codebase exploration with graphify") {
		t.Error("runClaude should append graphify prompt")
	}
}

// --- Unit tests for query.go ---

func TestExtractTerms(t *testing.T) {
	terms := extractTerms("How does authentication work?")
	if len(terms) == 0 {
		t.Fatal("extractTerms() returned no terms")
	}
	// "how", "does" should be filtered as stopwords
	for _, term := range terms {
		if term == "how" || term == "does" {
			t.Errorf("extractTerms() should filter stopword %q", term)
		}
	}
	// "authentication" should be present
	found := false
	for _, term := range terms {
		if term == "authentication" {
			found = true
		}
	}
	if !found {
		t.Errorf("extractTerms() = %v; want to contain 'authentication'", terms)
	}
}

func TestExtractTermsShortWords(t *testing.T) {
	terms := extractTerms("go is ok")
	// All words are <= 2 chars or stopwords
	if len(terms) != 0 {
		t.Errorf("extractTerms('go is ok') = %v; want empty", terms)
	}
}

func TestMatchScore(t *testing.T) {
	// Exact match should score higher than substring
	exact := matchScore("Auth", []string{"auth"})
	partial := matchScore("AuthService", []string{"auth"})
	if exact <= partial {
		t.Errorf("exact match score %d should beat partial %d", exact, partial)
	}

	// No match
	none := matchScore("Database", []string{"auth"})
	if none != 0 {
		t.Errorf("matchScore(Database, auth) = %d; want 0", none)
	}
}

func TestFindNode(t *testing.T) {
	nodes := map[string]*jsonNode{
		"n1": {ID: "n1", Label: "AuthService"},
		"n2": {ID: "n2", Label: "UserRepository"},
		"n3": {ID: "n3", Label: "main()"},
	}

	// Exact match
	if id := findNode(nodes, "AuthService"); id != "n1" {
		t.Errorf("findNode(AuthService) = %q; want n1", id)
	}

	// Case-insensitive exact
	if id := findNode(nodes, "authservice"); id != "n1" {
		t.Errorf("findNode(authservice) = %q; want n1", id)
	}

	// Fuzzy substring
	if id := findNode(nodes, "Auth"); id != "n1" {
		t.Errorf("findNode(Auth) = %q; want n1", id)
	}

	// No match
	if id := findNode(nodes, "NonExistent"); id != "" {
		t.Errorf("findNode(NonExistent) = %q; want empty", id)
	}
}

// writeTestGraph creates a minimal graph.json for testing query functions.
func writeTestGraph(t *testing.T, dir string) string {
	t.Helper()
	graphData := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "n1", "label": "AuthService", "type": "class", "file": "auth.go"},
			{"id": "n2", "label": "UserRepo", "type": "class", "file": "user.go"},
			{"id": "n3", "label": ".login()", "type": "method", "file": "auth.go"},
			{"id": "n4", "label": "Database", "type": "class", "file": "db.go"},
		},
		"links": []map[string]interface{}{
			{"source": "n1", "target": "n3", "relation": "method", "confidence": "EXTRACTED"},
			{"source": "n1", "target": "n2", "relation": "calls", "confidence": "EXTRACTED"},
			{"source": "n2", "target": "n4", "relation": "calls", "confidence": "EXTRACTED"},
		},
	}
	data, _ := json.MarshalIndent(graphData, "", "  ")
	path := filepath.Join(dir, "graph.json")
	os.WriteFile(path, data, 0644)
	return path
}

func TestQueryGraph(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	err := queryGraph(graphPath, "How does auth work?", "bfs", 2000)
	if err != nil {
		t.Fatalf("queryGraph() error: %v", err)
	}
}

func TestQueryGraphDFS(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	err := queryGraph(graphPath, "AuthService", "dfs", 2000)
	if err != nil {
		t.Fatalf("queryGraph(dfs) error: %v", err)
	}
}

func TestQueryGraphNoMatch(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	// Should not error, just print "no matching nodes"
	err := queryGraph(graphPath, "zzzznonexistent", "bfs", 2000)
	if err != nil {
		t.Fatalf("queryGraph(no match) error: %v", err)
	}
}

func TestQueryGraphMissingFile(t *testing.T) {
	err := queryGraph("/nonexistent/graph.json", "test", "bfs", 2000)
	if err == nil {
		t.Error("queryGraph() should error on missing file")
	}
}

func TestQueryPath(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	err := queryPath(graphPath, "AuthService", "Database")
	if err != nil {
		t.Fatalf("queryPath() error: %v", err)
	}
}

func TestQueryPathNoPath(t *testing.T) {
	dir := t.TempDir()
	// Graph with disconnected nodes
	graphData := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "n1", "label": "Alpha", "type": "class", "file": "a.go"},
			{"id": "n2", "label": "Beta", "type": "class", "file": "b.go"},
		},
		"links": []map[string]interface{}{},
	}
	data, _ := json.MarshalIndent(graphData, "", "  ")
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, data, 0644)

	// Should not error, just print "no path found"
	err := queryPath(graphPath, "Alpha", "Beta")
	if err != nil {
		t.Fatalf("queryPath(no path) error: %v", err)
	}
}

func TestQueryPathBadNode(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	err := queryPath(graphPath, "NonExistent", "Database")
	if err == nil {
		t.Error("queryPath() should error on unknown node")
	}
}

func TestQueryExplain(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	err := queryExplain(graphPath, "AuthService")
	if err != nil {
		t.Fatalf("queryExplain() error: %v", err)
	}
}

func TestQueryExplainBadNode(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	err := queryExplain(graphPath, "NonExistent")
	if err == nil {
		t.Error("queryExplain() should error on unknown node")
	}
}

// --- Pipeline tests ---

func TestRunPipeline(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	err := runPipeline("../../testdata/fixtures", outDir, false)
	if err != nil {
		t.Fatalf("runPipeline() error: %v", err)
	}

	// Verify outputs exist
	for _, name := range []string{"graph.json", "graph.html", "GRAPH_REPORT.md"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); os.IsNotExist(err) {
			t.Errorf("runPipeline() did not create %s", name)
		}
	}
}

func TestRunPipelineVerbose(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	err := runPipeline("../../testdata/fixtures", outDir, true)
	if err != nil {
		t.Fatalf("runPipeline(verbose) error: %v", err)
	}
}

func TestRunPipelineCacheHits(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	// First run populates cache
	if err := runPipeline("../../testdata/fixtures", outDir, false); err != nil {
		t.Fatalf("first run error: %v", err)
	}
	// Second run should hit cache
	if err := runPipeline("../../testdata/fixtures", outDir, true); err != nil {
		t.Fatalf("second run error: %v", err)
	}
}

func TestRunPipelineEmptyDir(t *testing.T) {
	emptyDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")
	err := runPipeline(emptyDir, outDir, false)
	if err != nil {
		t.Fatalf("runPipeline(empty) error: %v", err)
	}
}

func TestPrintUsage(t *testing.T) {
	// Just verify it doesn't panic
	printUsage()
}

// --- Additional coverage for labelScore ---

func TestLabelScoreAllTypes(t *testing.T) {
	// Cover all type branches
	types := []struct {
		nodeType string
		wantGT   int // score should be greater than this (with degree=0)
	}{
		{"class", 50},
		{"mixin", 50},
		{"extension", 50},
		{"enum", 50},
		{"function", 20},
		{"method", 10},
		{"variable", 0},
		{"file", -100},
	}
	for _, tt := range types {
		score := labelScore("Test", tt.nodeType, 0)
		if score <= tt.wantGT {
			t.Errorf("labelScore(Test, %q, 0) = %d; want > %d", tt.nodeType, score, tt.wantGT)
		}
	}
}

func TestLabelScorePenalties(t *testing.T) {
	// Cover all penalty patterns
	penalties := []string{"_wrapFoo", "_makeFoo", "_buildFoo", "_ctxFoo", "_docFoo"}
	for _, label := range penalties {
		score := labelScore(label, "function", 0)
		if score >= 0 {
			t.Errorf("labelScore(%q) = %d; want negative", label, score)
		}
	}

	// _ prefix penalty (smaller, but still reduces score)
	withUnderscore := labelScore("_helper", "variable", 0)
	withoutUnderscore := labelScore("helper", "variable", 0)
	if withUnderscore >= withoutUnderscore {
		t.Errorf("_ prefix should reduce score: %d vs %d", withUnderscore, withoutUnderscore)
	}
}

// --- Additional coverage for generateCommunityLabels ---

func TestGenerateCommunityLabelsFileNode(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("n1", "widget_test.dart", "file", "test/widget_test.dart")
	communities := map[int][]string{0: {"n1"}}
	labels := generateCommunityLabels(g, communities)
	if !strings.Contains(labels[0], "widget tests") {
		t.Errorf("label = %q; want to contain 'widget tests'", labels[0])
	}
}

func TestGenerateCommunityLabelsEmpty(t *testing.T) {
	g := graph.NewGraph()
	communities := map[int][]string{0: {"nonexistent"}}
	labels := generateCommunityLabels(g, communities)
	if !strings.Contains(labels[0], "Community") {
		t.Errorf("label = %q; want fallback 'Community 0'", labels[0])
	}
}

func TestGenerateCommunityLabelsDirOnly(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("n1", "utils", "module", "lib/utils.go")
	communities := map[int][]string{0: {"n1"}}
	labels := generateCommunityLabels(g, communities)
	// module nodes are rejected by labelScore, so label should be dir-only
	if labels[0] == "" {
		t.Error("label should not be empty")
	}
}

func TestGenerateCommunityLabelsNodeOnly(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("n1", "Helper", "class", "") // no file → no dir
	communities := map[int][]string{0: {"n1"}}
	labels := generateCommunityLabels(g, communities)
	if labels[0] != "Helper" {
		t.Errorf("label = %q; want 'Helper'", labels[0])
	}
}

// --- Additional coverage for runClaude ---

func TestRunClaudeAppendsNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	// Existing file without trailing newline
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# No newline at end"), 0644)
	runClaude([]string{dir}, "CLAUDE.md")
	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if !strings.Contains(string(data), "## Codebase exploration with graphify") {
		t.Error("should append prompt even without trailing newline")
	}
}

// --- Additional coverage for queryGraph ---

func TestQueryGraphBudgetTruncation(t *testing.T) {
	dir := t.TempDir()
	// Create a large graph to trigger budget truncation
	graphData := map[string]interface{}{
		"nodes": make([]map[string]interface{}, 0),
		"links": make([]map[string]interface{}, 0),
	}
	nodes := graphData["nodes"].([]map[string]interface{})
	links := graphData["links"].([]map[string]interface{})
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("n%d", i)
		nodes = append(nodes, map[string]interface{}{
			"id": id, "label": fmt.Sprintf("ServiceNode%d", i), "type": "class", "file": fmt.Sprintf("pkg/svc%d.go", i),
		})
		if i > 0 {
			links = append(links, map[string]interface{}{
				"source": fmt.Sprintf("n%d", i-1), "target": id, "relation": "calls", "confidence": "EXTRACTED",
			})
		}
	}
	graphData["nodes"] = nodes
	graphData["links"] = links
	data, _ := json.MarshalIndent(graphData, "", "  ")
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, data, 0644)

	// Very small budget to trigger truncation
	err := queryGraph(graphPath, "ServiceNode0", "bfs", 50)
	if err != nil {
		t.Fatalf("queryGraph(small budget) error: %v", err)
	}
}

func TestQueryGraphDFSTruncation(t *testing.T) {
	dir := t.TempDir()
	graphData := map[string]interface{}{
		"nodes": make([]map[string]interface{}, 0),
		"links": make([]map[string]interface{}, 0),
	}
	nodes := graphData["nodes"].([]map[string]interface{})
	links := graphData["links"].([]map[string]interface{})
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("n%d", i)
		nodes = append(nodes, map[string]interface{}{
			"id": id, "label": fmt.Sprintf("Node%d", i), "type": "class", "file": fmt.Sprintf("pkg/%d.go", i),
		})
		if i > 0 {
			links = append(links, map[string]interface{}{
				"source": fmt.Sprintf("n%d", i-1), "target": id, "relation": "calls", "confidence": "EXTRACTED",
			})
		}
	}
	graphData["nodes"] = nodes
	graphData["links"] = links
	data, _ := json.MarshalIndent(graphData, "", "  ")
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, data, 0644)

	err := queryGraph(graphPath, "Node0", "dfs", 50)
	if err != nil {
		t.Fatalf("queryGraph(dfs, small budget) error: %v", err)
	}
}

func TestQueryGraphNoTerms(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	err := queryGraph(graphPath, "is a", "bfs", 2000)
	if err == nil {
		t.Error("queryGraph() should error when all words are stopwords")
	}
}

func TestQueryGraphInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, []byte("not json"), 0644)

	err := queryGraph(graphPath, "test", "bfs", 2000)
	if err == nil {
		t.Error("queryGraph() should error on invalid JSON")
	}
}

// --- Additional coverage for queryPath ---

func TestQueryPathMissingFile(t *testing.T) {
	err := queryPath("/nonexistent/graph.json", "A", "B")
	if err == nil {
		t.Error("queryPath() should error on missing file")
	}
}

func TestQueryPathInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, []byte("bad"), 0644)

	err := queryPath(graphPath, "A", "B")
	if err == nil {
		t.Error("queryPath() should error on invalid JSON")
	}
}

func TestQueryPathBadTarget(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	err := queryPath(graphPath, "AuthService", "NonExistent")
	if err == nil {
		t.Error("queryPath() should error on unknown target node")
	}
}

// --- Additional coverage for queryExplain ---

func TestQueryExplainMissingFile(t *testing.T) {
	err := queryExplain("/nonexistent/graph.json", "test")
	if err == nil {
		t.Error("queryExplain() should error on missing file")
	}
}

func TestQueryExplainInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, []byte("bad"), 0644)

	err := queryExplain(graphPath, "test")
	if err == nil {
		t.Error("queryExplain() should error on invalid JSON")
	}
}

func TestQueryExplainIncomingEdges(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	// UserRepo has both incoming (from AuthService) and outgoing (to Database) edges
	err := queryExplain(graphPath, "UserRepo")
	if err != nil {
		t.Fatalf("queryExplain(UserRepo) error: %v", err)
	}
}

// --- Additional coverage for queryPath ---

func TestQueryPathReverseDirection(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)

	// Database → AuthService follows incoming edges
	err := queryPath(graphPath, "Database", "AuthService")
	if err != nil {
		t.Fatalf("queryPath(reverse) error: %v", err)
	}
}

// --- Additional coverage for findNode ---

func TestFindNodePartialPrefersShorter(t *testing.T) {
	nodes := map[string]*jsonNode{
		"n1": {ID: "n1", Label: "Auth"},
		"n2": {ID: "n2", Label: "AuthService"},
		"n3": {ID: "n3", Label: "AuthServiceManager"},
	}

	// Should prefer the shorter match "Auth"
	id := findNode(nodes, "Auth")
	if id != "n1" {
		t.Errorf("findNode(Auth) = %q; want n1 (exact match)", id)
	}
}

// --- CLI integration tests for subcommands ---

func TestCLIQuerySubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "graphify-out")

	// Build and run pipeline first
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "graphify"), "./cmd/graphify")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("CLI build failed: %v", err)
	}

	cmd := exec.Command(filepath.Join(tmpDir, "graphify"), "-out", outDir, "testdata/fixtures/")
	cmd.Dir = filepath.Join("..", "..")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Test query subcommand
	queryCmd := exec.Command(filepath.Join(tmpDir, "graphify"), "query", "--dir", outDir, "Transformer")
	queryCmd.Dir = filepath.Join("..", "..")
	output, err := queryCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("query failed: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(string(output), "Transformer") {
		t.Errorf("query output should mention Transformer: %s", output)
	}
}

func TestCLIPathSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "graphify-out")

	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "graphify"), "./cmd/graphify")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("CLI build failed: %v", err)
	}

	cmd := exec.Command(filepath.Join(tmpDir, "graphify"), "-out", outDir, "testdata/fixtures/")
	cmd.Dir = filepath.Join("..", "..")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	pathCmd := exec.Command(filepath.Join(tmpDir, "graphify"), "path", "--dir", outDir, "Transformer", "forward")
	pathCmd.Dir = filepath.Join("..", "..")
	output, err := pathCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("path failed: %v\nOutput: %s", err, output)
	}
}

func TestCLIExplainSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "graphify-out")

	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "graphify"), "./cmd/graphify")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("CLI build failed: %v", err)
	}

	cmd := exec.Command(filepath.Join(tmpDir, "graphify"), "-out", outDir, "testdata/fixtures/")
	cmd.Dir = filepath.Join("..", "..")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	explainCmd := exec.Command(filepath.Join(tmpDir, "graphify"), "explain", "--dir", outDir, "Transformer")
	explainCmd.Dir = filepath.Join("..", "..")
	output, err := explainCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("explain failed: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(string(output), "Transformer") {
		t.Errorf("explain output should mention Transformer: %s", output)
	}
}

func TestCLIClaudeSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(targetDir, 0755)

	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "graphify"), "./cmd/graphify")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("CLI build failed: %v", err)
	}

	cmd := exec.Command(filepath.Join(tmpDir, "graphify"), "claude", targetDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("claude failed: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(string(output), "CLAUDE.md") {
		t.Errorf("output should mention CLAUDE.md: %s", output)
	}
}

func TestCLIAgentsSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(targetDir, 0755)

	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "graphify"), "./cmd/graphify")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("CLI build failed: %v", err)
	}

	cmd := exec.Command(filepath.Join(tmpDir, "graphify"), "agents", targetDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("agents failed: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(string(output), "AGENTS.md") {
		t.Errorf("output should mention AGENTS.md: %s", output)
	}
}

// --- Unit tests for run*E wrappers ---

func TestRunQueryE(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)
	graphDir := filepath.Dir(graphPath)

	err := runQueryE([]string{"--dir", graphDir, "AuthService"})
	if err != nil {
		t.Fatalf("runQueryE() error: %v", err)
	}
}

func TestRunQueryEDFS(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)
	graphDir := filepath.Dir(graphPath)

	err := runQueryE([]string{"--dir", graphDir, "--dfs", "AuthService"})
	if err != nil {
		t.Fatalf("runQueryE(dfs) error: %v", err)
	}
}

func TestRunQueryENoArgs(t *testing.T) {
	err := runQueryE([]string{})
	if err == nil {
		t.Error("runQueryE() should error with no args")
	}
}

func TestRunPathE(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)
	graphDir := filepath.Dir(graphPath)

	err := runPathE([]string{"--dir", graphDir, "AuthService", "Database"})
	if err != nil {
		t.Fatalf("runPathE() error: %v", err)
	}
}

func TestRunPathENoArgs(t *testing.T) {
	err := runPathE([]string{"nodeA"})
	if err == nil {
		t.Error("runPathE() should error with < 2 args")
	}
}

func TestRunExplainE(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeTestGraph(t, dir)
	graphDir := filepath.Dir(graphPath)

	err := runExplainE([]string{"--dir", graphDir, "AuthService"})
	if err != nil {
		t.Fatalf("runExplainE() error: %v", err)
	}
}

func TestRunExplainENoArgs(t *testing.T) {
	err := runExplainE([]string{})
	if err == nil {
		t.Error("runExplainE() should error with no args")
	}
}

func TestCLIHelpSubcommand(t *testing.T) {
	tmpDir := t.TempDir()

	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "graphify"), "./cmd/graphify")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("CLI build failed: %v", err)
	}

	cmd := exec.Command(filepath.Join(tmpDir, "graphify"), "help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("help failed: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(string(output), "Usage:") {
		t.Errorf("help output should contain Usage: %s", output)
	}
}
