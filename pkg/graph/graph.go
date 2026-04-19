package graph

import (
	"fmt"
)

// Node represents a node in the graph.
type Node struct {
	ID       string
	Label    string
	Type     string
	File     string
	Location string
}

// Edge represents an edge between nodes.
type Edge struct {
	Source     string
	Target     string
	Relation   string
	Confidence string
	Weight     float64
}

// Graph represents a knowledge graph.
type Graph struct {
	nodes    map[string]*Node
	edges    map[string]*Edge // key: "source:target"
	adjList  map[string][]string
}

// NewGraph creates a new empty graph.
func NewGraph() *Graph {
	return &Graph{
		nodes:   make(map[string]*Node),
		edges:   make(map[string]*Edge),
		adjList: make(map[string][]string),
	}
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int {
	return len(g.nodes)
}

// EdgeCount returns the number of edges in the graph.
func (g *Graph) EdgeCount() int {
	return len(g.edges)
}

// Nodes returns all nodes in the graph.
func (g *Graph) Nodes() []*Node {
	nodes := make([]*Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// Edges returns all edges in the graph.
func (g *Graph) Edges() []*Edge {
	edges := make([]*Edge, 0, len(g.edges))
	for _, edge := range g.edges {
		edges = append(edges, edge)
	}
	return edges
}

// GetNode returns a node by ID.
func (g *Graph) GetNode(id string) *Node {
	return g.nodes[id]
}

// GetEdge returns an edge between two nodes.
func (g *Graph) GetEdge(source, target string) *Edge {
	key := fmt.Sprintf("%s:%s", source, target)
	return g.edges[key]
}

// GetNodeNeighbors returns the neighbors of a node (undirected).
func (g *Graph) GetNodeNeighbors(nodeID string) []string {
	neighbors := make(map[string]bool)
	// Outgoing edges
	for _, neighbor := range g.adjList[nodeID] {
		neighbors[neighbor] = true
	}
	// Incoming edges (find nodes that point to this node)
	for src, targets := range g.adjList {
		for _, tgt := range targets {
			if tgt == nodeID {
				neighbors[src] = true
			}
		}
	}
	result := make([]string, 0, len(neighbors))
	for n := range neighbors {
		result = append(result, n)
	}
	return result
}

// GetNodeDegree returns the degree of a node (both incoming and outgoing edges).
func (g *Graph) GetNodeDegree(nodeID string) int {
	return len(g.GetNodeNeighbors(nodeID))
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(id string, label string, nodeType string, file string) {
	g.nodes[id] = &Node{
		ID:    id,
		Label: label,
		Type:  nodeType,
		File:  file,
	}
}

// AddEdge adds an edge to the graph.
func (g *Graph) AddEdge(source, target, relation, confidence string, weight float64) {
	key := fmt.Sprintf("%s:%s", source, target)
	g.edges[key] = &Edge{
		Source:     source,
		Target:     target,
		Relation:   relation,
		Confidence: confidence,
		Weight:     weight,
	}

	// Update adjacency list
	g.adjList[source] = append(g.adjList[source], target)
}

// HasEdge returns true if an edge exists.
func (g *Graph) HasEdge(source, target string) bool {
	key := fmt.Sprintf("%s:%s", source, target)
	_, exists := g.edges[key]
	return exists
}

// BuildFromJSON builds a graph from an extraction map.
func BuildFromJSON(extraction map[string]interface{}) *Graph {
	g := NewGraph()

	nodes, ok := extraction["nodes"].([]interface{})
	if !ok {
		return g
	}

	for _, n := range nodes {
		node, ok := n.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := node["id"].(string)
		label, _ := node["label"].(string)
		nodeType, _ := node["type"].(string)
		file, _ := node["source_file"].(string)
		g.AddNode(id, label, nodeType, file)
	}

	edges, ok := extraction["edges"].([]interface{})
	if !ok {
		return g
	}

	for _, e := range edges {
		edge, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		source, _ := edge["source"].(string)
		target, _ := edge["target"].(string)
		relation, _ := edge["relation"].(string)
		confidence, _ := edge["confidence"].(string)
		weight, _ := edge["weight"].(float64)
		g.AddEdge(source, target, relation, confidence, weight)
	}

	return g
}

// Build builds a graph from multiple extractions.
func Build(extractions []map[string]interface{}) *Graph {
	combined := make(map[string]interface{})
	combined["nodes"] = []interface{}{}
	combined["edges"] = []interface{}{}

	for _, ext := range extractions {
		if nodes, ok := ext["nodes"].([]interface{}); ok {
			combined["nodes"] = append(combined["nodes"].([]interface{}), nodes...)
		}
		if edges, ok := ext["edges"].([]interface{}); ok {
			combined["edges"] = append(combined["edges"].([]interface{}), edges...)
		}
	}

	return BuildFromJSON(combined)
}

// BuildFromExtractions builds a graph from typed extractions.
func BuildFromExtractions(extractions []*Extraction) *Graph {
	g := NewGraph()

	for _, ext := range extractions {
		for _, node := range ext.Nodes {
			g.AddNode(node.ID, node.Label, node.Type, node.File)
		}
		for _, edge := range ext.Edges {
			g.AddEdge(edge.Source, edge.Target, edge.Relation, edge.Confidence, edge.Weight)
		}
	}

	return g
}

// Extraction represents extraction data.
type Extraction struct {
	Nodes []Node
	Edges []Edge
}
