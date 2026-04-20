package main

import (
	"encoding/json"
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
