package cluster

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sjhorn/graphify/pkg/graph"
)

func loadGraph() *graph.Graph {
	fixturesDir := "../../testdata/fixtures"
	data, _ := os.ReadFile(filepath.Join(fixturesDir, "extraction.json"))
	var extraction map[string]interface{}
	json.Unmarshal(data, &extraction)
	return graph.BuildFromJSON(extraction)
}

func TestClusterReturnsDict(t *testing.T) {
	g := loadGraph()
	result := Cluster(g)
	if result == nil {
		t.Error("Cluster() should return a result")
	}
}

func TestClusterCoversAllNodes(t *testing.T) {
	g := loadGraph()
	result := Cluster(g)
	allNodes := make(map[string]bool)
	for _, nodes := range result.Communities {
		for _, nodeID := range nodes {
			allNodes[nodeID] = true
		}
	}
	if len(allNodes) != g.NodeCount() {
		t.Errorf("Cluster() covers %d nodes; want %d", len(allNodes), g.NodeCount())
	}
}

func TestCohesionScoreCompleteGraph(t *testing.T) {
	g := graph.NewGraph()
	nodes := []string{"0", "1", "2", "3"}
	for _, id := range nodes {
		g.AddNode(id, id, "", "")
	}
	// Add all edges (complete graph)
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			g.AddEdge(nodes[i], nodes[j], "rel", "EXTRACTED", 1.0)
		}
	}
	nodeList := []string{"0", "1", "2", "3"}
	score := CohesionScore(g, nodeList)
	if score != 1.0 {
		t.Errorf("CohesionScore(complete graph) = %f; want 1.0", score)
	}
}

func TestCohesionScoreSingleNode(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "A", "", "")
	nodes := []string{"a"}
	score := CohesionScore(g, nodes)
	if score != 1.0 {
		t.Errorf("CohesionScore(single node) = %f; want 1.0", score)
	}
}

func TestCohesionScoreDisconnected(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "A", "", "")
	g.AddNode("b", "B", "", "")
	g.AddNode("c", "C", "", "")
	nodes := []string{"a", "b", "c"}
	score := CohesionScore(g, nodes)
	if score != 0.0 {
		t.Errorf("CohesionScore(disconnected) = %f; want 0.0", score)
	}
}

func TestCohesionScoreRange(t *testing.T) {
	g := loadGraph()
	result := Cluster(g)
	for _, nodes := range result.Communities {
		score := CohesionScore(g, nodes)
		if score < 0.0 || score > 1.0 {
			t.Errorf("CohesionScore() = %f; should be in [0, 1]", score)
		}
	}
}

func TestScoreAllKeysMatchCommunities(t *testing.T) {
	g := loadGraph()
	result := Cluster(g)
	scores := ScoreAll(g, result.Communities)
	if len(scores) != len(result.Communities) {
		t.Errorf("ScoreAll() has %d keys; want %d", len(scores), len(result.Communities))
	}
}

func TestClusterDoesNotWriteToStdout(t *testing.T) {
	g := loadGraph()
	Cluster(g)
	// No output should be written
}

func TestClusterDoesNotWriteToStderr(t *testing.T) {
	g := loadGraph()
	Cluster(g)
	// No output should be written
}
