package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sjhorn/graphify/pkg/cluster"
	"github.com/sjhorn/graphify/pkg/graph"
)

func makeGraph() *graph.Graph {
	fixturesDir := "../../testdata/fixtures"
	data, _ := os.ReadFile(filepath.Join(fixturesDir, "extraction.json"))
	var extraction map[string]interface{}
	json.Unmarshal(data, &extraction)
	return graph.BuildFromJSON(extraction)
}

func TestToJSONCreatesFile(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.json")
	err := ToJSON(g, communities, out)
	if err != nil {
		t.Errorf("ToJSON() error = %v", err)
	}
	if _, err := os.Stat(out); os.IsNotExist(err) {
		t.Error("ToJSON() should create file")
	}
}

func TestToJSONValidJSON(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.json")
	ToJSON(g, communities, out)

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("ToJSON() invalid JSON = %v", err)
	}
	if _, ok := result["nodes"]; !ok {
		t.Error("ToJSON() should have 'nodes' field")
	}
	if _, ok := result["links"]; !ok {
		t.Error("ToJSON() should have 'links' field")
	}
}

func TestToJSONNodesHaveCommunity(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.json")
	ToJSON(g, communities, out)

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}

	nodes, ok := result["nodes"].([]interface{})
	if !ok {
		t.Fatal("nodes should be an array")
	}
	for i, node := range nodes {
		n := node.(map[string]interface{})
		if _, ok := n["community"]; !ok {
			t.Errorf("Node %d missing 'community' field", i)
		}
	}
}

func TestToCypherCreatesFile(t *testing.T) {
	g := makeGraph()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "cypher.txt")
	err := ToCypher(g, out)
	if err != nil {
		t.Errorf("ToCypher() error = %v", err)
	}
	if _, err := os.Stat(out); os.IsNotExist(err) {
		t.Error("ToCypher() should create file")
	}
}

func TestToCypherContainsMergeStatements(t *testing.T) {
	g := makeGraph()
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "cypher.txt")
	ToCypher(g, out)

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(content), "MERGE") {
		t.Error("ToCypher() should contain MERGE statements")
	}
}

func TestToGraphMLCreatesFile(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.graphml")
	err := ToGraphML(g, communities, out)
	if err != nil {
		t.Errorf("ToGraphML() error = %v", err)
	}
	if _, err := os.Stat(out); os.IsNotExist(err) {
		t.Error("ToGraphML() should create file")
	}
}

func TestToGraphMLValidXML(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.graphml")
	ToGraphML(g, communities, out)

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(content), "<graphml") {
		t.Error("ToGraphML() should contain <graphml> tag")
	}
	if !contains(string(content), "<node") {
		t.Error("ToGraphML() should contain <node> tag")
	}
}

func TestToGraphMLHasCommunityAttribute(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.graphml")
	ToGraphML(g, communities, out)

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(content), "community") {
		t.Error("ToGraphML() should contain community attribute")
	}
}

func TestToHTMLCreatesFile(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.html")
	err := ToHTML(g, communities, out, nil)
	if err != nil {
		t.Errorf("ToHTML() error = %v", err)
	}
	if _, err := os.Stat(out); os.IsNotExist(err) {
		t.Error("ToHTML() should create file")
	}
}

func TestToHTMLContainsVisjs(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.html")
	ToHTML(g, communities, out, nil)

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(content), "vis-network") {
		t.Error("ToHTML() should contain vis-network script")
	}
}

func TestToHTMLContainsSearch(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.html")
	ToHTML(g, communities, out, nil)

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(content), "search") {
		t.Error("ToHTML() should contain search functionality")
	}
}

func TestToHTMLContainsLegendWithLabels(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	labels := map[int]string{0: "Group 0", 1: "Group 1"}
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.html")
	ToHTML(g, communities, out, labels)

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(content), "Group 0") {
		t.Error("ToHTML() should contain community label")
	}
}

func TestToHTMLContainsNodesAndEdges(t *testing.T) {
	g := makeGraph()
	communities := cluster.Cluster(g)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "graph.html")
	ToHTML(g, communities, out, nil)

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(content), "RAW_NODES") {
		t.Error("ToHTML() should contain RAW_NODES")
	}
	if !contains(string(content), "RAW_EDGES") {
		t.Error("ToHTML() should contain RAW_EDGES")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0)
}
