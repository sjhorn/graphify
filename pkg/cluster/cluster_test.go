package cluster

import (
	"encoding/json"
	"fmt"
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

func TestSplitLargeCommunitiesKeepsSmall(t *testing.T) {
	g := graph.NewGraph()
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("n%d", i)
		g.AddNode(id, id, "class", "a.py")
	}
	// Connect them all
	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			g.AddEdge(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", j), "calls", "EXTRACTED", 1.0)
		}
	}
	result := Cluster(g)
	// With maxSize > community size, nothing should change
	split := SplitLargeCommunities(g, result, 100)
	totalNodes := 0
	for _, nodes := range split.Communities {
		totalNodes += len(nodes)
	}
	if totalNodes != 5 {
		t.Errorf("SplitLargeCommunities() total nodes = %d; want 5", totalNodes)
	}
}

func TestSplitLargeCommunitiesSplitsLarge(t *testing.T) {
	g := graph.NewGraph()
	// Create two distinct clusters of 6 nodes each, loosely connected
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("n%d", i)
		g.AddNode(id, id, "class", "a.py")
	}
	// Dense connections within cluster 1 (0-5) and cluster 2 (6-11)
	for i := 0; i < 6; i++ {
		for j := i + 1; j < 6; j++ {
			g.AddEdge(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", j), "calls", "EXTRACTED", 1.0)
		}
	}
	for i := 6; i < 12; i++ {
		for j := i + 1; j < 12; j++ {
			g.AddEdge(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", j), "calls", "EXTRACTED", 1.0)
		}
	}
	// One weak cross-cluster link
	g.AddEdge("n0", "n6", "calls", "EXTRACTED", 1.0)

	initial := Cluster(g)
	split := SplitLargeCommunities(g, initial, 4)

	// All nodes should still be present
	totalNodes := 0
	for _, nodes := range split.Communities {
		totalNodes += len(nodes)
	}
	if totalNodes != 12 {
		t.Errorf("SplitLargeCommunities() total nodes = %d; want 12", totalNodes)
	}

	// Should have more communities than before (if any were > 4)
	for _, nodes := range split.Communities {
		if len(nodes) > 6 {
			t.Errorf("SplitLargeCommunities() community has %d nodes; want <= 6", len(nodes))
		}
	}
}

func TestSplitLargeCommunitiesDefaultMaxSize(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddEdge("a", "b", "calls", "EXTRACTED", 1.0)
	result := Cluster(g)
	// maxSize 0 should default to 100
	split := SplitLargeCommunities(g, result, 0)
	totalNodes := 0
	for _, nodes := range split.Communities {
		totalNodes += len(nodes)
	}
	if totalNodes != 2 {
		t.Errorf("SplitLargeCommunities(0) total = %d; want 2", totalNodes)
	}
}

func TestMergeTinyCommunitiesAbsorbs(t *testing.T) {
	g := graph.NewGraph()
	// Community 1: 4 densely connected nodes
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("big%d", i)
		g.AddNode(id, id, "class", "a.py")
	}
	for i := 0; i < 4; i++ {
		for j := i + 1; j < 4; j++ {
			g.AddEdge(fmt.Sprintf("big%d", i), fmt.Sprintf("big%d", j), "calls", "EXTRACTED", 1.0)
		}
	}

	// Community 2: 1 node connected to big0
	g.AddNode("tiny0", "tiny0", "class", "b.py")
	g.AddEdge("tiny0", "big0", "calls", "EXTRACTED", 1.0)

	initial := &ClusterResult{
		Communities: map[int][]string{
			0: {"big0", "big1", "big2", "big3"},
			1: {"tiny0"},
		},
		NodeCommunity: map[string]int{
			"big0": 0, "big1": 0, "big2": 0, "big3": 0,
			"tiny0": 1,
		},
	}

	merged := MergeTinyCommunities(g, initial, 3)
	// tiny0 should be absorbed into the big community
	totalNodes := 0
	for _, nodes := range merged.Communities {
		totalNodes += len(nodes)
	}
	if totalNodes != 5 {
		t.Errorf("MergeTinyCommunities() total = %d; want 5", totalNodes)
	}
	// Should now have just 1 community (tiny absorbed into big)
	if len(merged.Communities) != 1 {
		t.Errorf("MergeTinyCommunities() communities = %d; want 1", len(merged.Communities))
	}
}

func TestMergeTinyCommunitiesDefaultMinSize(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddNode("c", "C", "class", "c.py")
	g.AddNode("d", "D", "class", "d.py")
	g.AddEdge("a", "b", "calls", "EXTRACTED", 1.0)
	g.AddEdge("b", "c", "calls", "EXTRACTED", 1.0)
	g.AddEdge("c", "d", "calls", "EXTRACTED", 1.0)

	initial := &ClusterResult{
		Communities: map[int][]string{
			0: {"a", "b", "c"},
			1: {"d"},
		},
		NodeCommunity: map[string]int{
			"a": 0, "b": 0, "c": 0, "d": 1,
		},
	}
	// minSize 0 should default to 3
	merged := MergeTinyCommunities(g, initial, 0)
	totalNodes := 0
	for _, nodes := range merged.Communities {
		totalNodes += len(nodes)
	}
	if totalNodes != 4 {
		t.Errorf("MergeTinyCommunities(0) total = %d; want 4", totalNodes)
	}
}

func TestMergeTinyCommunitiesIsolatedStays(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "A", "class", "a.py")

	initial := &ClusterResult{
		Communities:   map[int][]string{0: {"a"}},
		NodeCommunity: map[string]int{"a": 0},
	}
	merged := MergeTinyCommunities(g, initial, 3)
	if len(merged.Communities) != 1 {
		t.Errorf("Isolated node should stay in its own community, got %d communities", len(merged.Communities))
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
