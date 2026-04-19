package cluster

import (
	"math/rand"

	"gonum.org/v1/gonum/graph/community"
	"gonum.org/v1/gonum/graph/simple"

	"github.com/sjhorn/graphify/pkg/graph"
)

// ClusterResult contains the results of community detection.
type ClusterResult struct {
	Communities   map[int][]string // community ID -> node IDs
	NodeCommunity map[string]int   // node ID -> community ID
}

// Cluster runs community detection using the Louvain method.
func Cluster(g *graph.Graph) *ClusterResult {
	// Convert to gonum graph
	gonumGraph := simple.NewUndirectedGraph()
	nodeMap := make(map[string]int64)
	reverseMap := make(map[int64]string)

	// Add nodes
	for _, node := range g.Nodes() {
		n := gonumGraph.NewNode()
		gonumGraph.AddNode(n)
		nodeMap[node.ID] = n.ID()
		reverseMap[n.ID()] = node.ID
	}

	// Add edges
	for _, edge := range g.Edges() {
		srcID := nodeMap[edge.Source]
		tgtID := nodeMap[edge.Target]
		// Skip self-edges
		if srcID == tgtID {
			continue
		}
		gonumGraph.SetEdge(gonumGraph.NewEdge(gonumGraph.Node(srcID), gonumGraph.Node(tgtID)))
	}

	// Run Louvain community detection using Modularize
	// Use resolution 1.0 and seeded random source for deterministic results
	reduced := community.Modularize(gonumGraph, 1.0, rand.New(rand.NewSource(42)))
	communities := reduced.Communities()

	// Convert result to our format
	clusterResult := &ClusterResult{
		Communities:   make(map[int][]string),
		NodeCommunity: make(map[string]int),
	}

	for i, nodeSet := range communities {
		for _, node := range nodeSet {
			originalID := reverseMap[node.ID()]
			clusterResult.Communities[i] = append(clusterResult.Communities[i], originalID)
			clusterResult.NodeCommunity[originalID] = i
		}
	}

	return clusterResult
}

// SplitLargeCommunities re-clusters communities that exceed maxSize by running
// Louvain on the subgraph of each oversized community.
func SplitLargeCommunities(g *graph.Graph, result *ClusterResult, maxSize int) *ClusterResult {
	if maxSize <= 0 {
		maxSize = 100
	}

	newCommunities := make(map[int][]string)
	newNodeCommunity := make(map[string]int)
	nextCID := 0

	for _, nodes := range result.Communities {
		if len(nodes) <= maxSize {
			// Keep as-is
			for _, nid := range nodes {
				newCommunities[nextCID] = append(newCommunities[nextCID], nid)
				newNodeCommunity[nid] = nextCID
			}
			nextCID++
			continue
		}

		// Build subgraph for this community
		subGraph := simple.NewUndirectedGraph()
		nodeSet := make(map[string]bool)
		subNodeMap := make(map[string]int64)
		subReverseMap := make(map[int64]string)

		for _, nid := range nodes {
			nodeSet[nid] = true
		}
		for _, nid := range nodes {
			n := subGraph.NewNode()
			subGraph.AddNode(n)
			subNodeMap[nid] = n.ID()
			subReverseMap[n.ID()] = nid
		}
		for _, edge := range g.Edges() {
			if nodeSet[edge.Source] && nodeSet[edge.Target] && edge.Source != edge.Target {
				srcID := subNodeMap[edge.Source]
				tgtID := subNodeMap[edge.Target]
				if !subGraph.HasEdgeBetween(srcID, tgtID) {
					subGraph.SetEdge(subGraph.NewEdge(subGraph.Node(srcID), subGraph.Node(tgtID)))
				}
			}
		}

		// Re-cluster with slightly higher resolution to break into sub-communities
		reduced := community.Modularize(subGraph, 1.5, rand.New(rand.NewSource(42)))
		subCommunities := reduced.Communities()

		for _, subNodeSet := range subCommunities {
			for _, node := range subNodeSet {
				originalID := subReverseMap[node.ID()]
				newCommunities[nextCID] = append(newCommunities[nextCID], originalID)
				newNodeCommunity[originalID] = nextCID
			}
			nextCID++
		}
	}

	return &ClusterResult{
		Communities:   newCommunities,
		NodeCommunity: newNodeCommunity,
	}
}

// MergeTinyCommunities absorbs communities smaller than minSize into the
// neighboring community they share the most edges with.
func MergeTinyCommunities(g *graph.Graph, result *ClusterResult, minSize int) *ClusterResult {
	if minSize <= 0 {
		minSize = 3
	}

	// Work on a mutable copy of the node→community mapping
	nodeCommunity := make(map[string]int)
	for nid, cid := range result.NodeCommunity {
		nodeCommunity[nid] = cid
	}

	// Repeatedly merge until no tiny communities remain
	changed := true
	for changed {
		changed = false
		// Rebuild communities from nodeCommunity
		communities := make(map[int][]string)
		for nid, cid := range nodeCommunity {
			communities[cid] = append(communities[cid], nid)
		}

		for cid, nodes := range communities {
			if len(nodes) >= minSize {
				continue
			}
			// Find the neighboring community with the most connections
			neighborCounts := make(map[int]int)
			for _, nid := range nodes {
				for _, neighbor := range g.GetNodeNeighbors(nid) {
					ncid := nodeCommunity[neighbor]
					if ncid != cid {
						neighborCounts[ncid]++
					}
				}
			}
			if len(neighborCounts) == 0 {
				continue // isolated, leave as-is
			}
			// Pick the neighbor community with most connections
			bestCID := -1
			bestCount := 0
			for ncid, count := range neighborCounts {
				if count > bestCount {
					bestCount = count
					bestCID = ncid
				}
			}
			if bestCID >= 0 {
				for _, nid := range nodes {
					nodeCommunity[nid] = bestCID
				}
				changed = true
			}
		}
	}

	// Rebuild final result with contiguous IDs
	oldToNew := make(map[int]int)
	newCommunities := make(map[int][]string)
	newNodeCommunity := make(map[string]int)
	nextCID := 0

	for nid, cid := range nodeCommunity {
		newCID, exists := oldToNew[cid]
		if !exists {
			newCID = nextCID
			oldToNew[cid] = newCID
			nextCID++
		}
		newCommunities[newCID] = append(newCommunities[newCID], nid)
		newNodeCommunity[nid] = newCID
	}

	return &ClusterResult{
		Communities:   newCommunities,
		NodeCommunity: newNodeCommunity,
	}
}

// CohesionScore calculates the cohesion score for a community.
func CohesionScore(g *graph.Graph, nodes []string) float64 {
	if len(nodes) <= 1 {
		return 1.0
	}

	// Count internal edges
	internalEdges := 0
	nodeSet := make(map[string]bool)
	for _, node := range nodes {
		nodeSet[node] = true
	}

	for _, node := range nodes {
		neighbors := g.GetNodeNeighbors(node)
		for _, neighbor := range neighbors {
			if nodeSet[neighbor] {
				internalEdges++
			}
		}
	}

	// Each edge is counted twice (once from each end)
	internalEdges /= 2

	// Maximum possible edges
	maxEdges := len(nodes) * (len(nodes) - 1) / 2

	if maxEdges == 0 {
		return 1.0
	}

	return float64(internalEdges) / float64(maxEdges)
}

// ScoreAll calculates cohesion scores for all communities.
func ScoreAll(g *graph.Graph, communities map[int][]string) map[int]float64 {
	scores := make(map[int]float64)
	for commID, nodes := range communities {
		scores[commID] = CohesionScore(g, nodes)
	}
	return scores
}
