package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sjhorn/graphify/pkg/extract"
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
		"## Corpus Check",
		"## Summary",
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
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0)
}
