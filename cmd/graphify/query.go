package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// queryGraph loads graph.json and runs a traversal query against it.
func queryGraph(graphPath string, question string, mode string, budget int) error {
	data, err := os.ReadFile(graphPath)
	if err != nil {
		return fmt.Errorf("cannot read graph: %w\nRun 'graphify <path>' first to build the graph", err)
	}

	var graphData struct {
		Nodes []jsonNode `json:"nodes"`
		Links []jsonLink `json:"links"`
	}
	if err := json.Unmarshal(data, &graphData); err != nil {
		return fmt.Errorf("cannot parse graph: %w", err)
	}

	// Build lookup structures
	nodes := make(map[string]*jsonNode)
	adjOut := make(map[string][]adjEntry) // outgoing
	adjIn := make(map[string][]adjEntry)  // incoming
	for i := range graphData.Nodes {
		n := &graphData.Nodes[i]
		nodes[n.ID] = n
	}
	for _, link := range graphData.Links {
		adjOut[link.Source] = append(adjOut[link.Source], adjEntry{Target: link.Target, Relation: link.Relation, Confidence: link.Confidence})
		adjIn[link.Target] = append(adjIn[link.Target], adjEntry{Target: link.Source, Relation: link.Relation, Confidence: link.Confidence})
	}

	// Find best-matching start nodes
	terms := extractTerms(question)
	if len(terms) == 0 {
		return fmt.Errorf("no search terms found in query")
	}

	type scored struct {
		Score int
		ID    string
	}
	var candidates []scored
	for _, n := range graphData.Nodes {
		score := matchScore(n.Label, terms)
		if n.Type == "file" || n.Type == "module" {
			score -= 2 // deprioritize structural nodes
		}
		if score > 0 {
			candidates = append(candidates, scored{score, n.ID})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	startNodes := make([]string, 0, 3)
	for i, c := range candidates {
		if i >= 3 {
			break
		}
		startNodes = append(startNodes, c.ID)
	}

	if len(startNodes) == 0 {
		fmt.Printf("No matching nodes found for: %s\n", strings.Join(terms, ", "))
		fmt.Println("\nAvailable node labels (top 20 by degree):")
		type nd struct {
			Label  string
			Degree int
		}
		var top []nd
		for id, n := range nodes {
			deg := len(adjOut[id]) + len(adjIn[id])
			top = append(top, nd{n.Label, deg})
		}
		sort.Slice(top, func(i, j int) bool { return top[i].Degree > top[j].Degree })
		for i, n := range top {
			if i >= 20 {
				break
			}
			fmt.Printf("  - %s (%d edges)\n", n.Label, n.Degree)
		}
		return nil
	}

	// Run traversal
	subgraphNodes := make(map[string]bool)
	type edgeRecord struct{ From, To, Relation, Confidence string }
	var subgraphEdges []edgeRecord

	if mode == "dfs" {
		// DFS: depth-limited to 4
		visited := make(map[string]bool)
		type stackEntry struct {
			ID    string
			Depth int
		}
		stack := make([]stackEntry, 0)
		for i := len(startNodes) - 1; i >= 0; i-- {
			stack = append(stack, stackEntry{startNodes[i], 0})
		}
		for len(stack) > 0 {
			entry := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[entry.ID] || entry.Depth > 4 {
				continue
			}
			visited[entry.ID] = true
			subgraphNodes[entry.ID] = true
			for _, adj := range adjOut[entry.ID] {
				if !visited[adj.Target] {
					stack = append(stack, stackEntry{adj.Target, entry.Depth + 1})
					subgraphEdges = append(subgraphEdges, edgeRecord{entry.ID, adj.Target, adj.Relation, adj.Confidence})
				}
			}
			for _, adj := range adjIn[entry.ID] {
				if !visited[adj.Target] {
					stack = append(stack, stackEntry{adj.Target, entry.Depth + 1})
					subgraphEdges = append(subgraphEdges, edgeRecord{adj.Target, entry.ID, adj.Relation, adj.Confidence})
				}
			}
		}
	} else {
		// BFS: depth 2
		for _, s := range startNodes {
			subgraphNodes[s] = true
		}
		frontier := make(map[string]bool)
		for _, s := range startNodes {
			frontier[s] = true
		}
		for depth := 0; depth < 2; depth++ {
			nextFrontier := make(map[string]bool)
			for n := range frontier {
				for _, adj := range adjOut[n] {
					if !subgraphNodes[adj.Target] {
						nextFrontier[adj.Target] = true
						subgraphEdges = append(subgraphEdges, edgeRecord{n, adj.Target, adj.Relation, adj.Confidence})
					}
				}
				for _, adj := range adjIn[n] {
					if !subgraphNodes[adj.Target] {
						nextFrontier[adj.Target] = true
						subgraphEdges = append(subgraphEdges, edgeRecord{adj.Target, n, adj.Relation, adj.Confidence})
					}
				}
			}
			for n := range nextFrontier {
				subgraphNodes[n] = true
			}
			frontier = nextFrontier
		}
	}

	// Build output with token budget
	charBudget := budget * 4
	var sb strings.Builder

	// Header
	startLabels := make([]string, 0, len(startNodes))
	for _, s := range startNodes {
		if n := nodes[s]; n != nil {
			startLabels = append(startLabels, n.Label)
		}
	}
	fmt.Fprintf(&sb, "Query: %s\nMode: %s | Start: %s | Subgraph: %d nodes, %d edges\n\n",
		question, strings.ToUpper(mode), strings.Join(startLabels, ", "),
		len(subgraphNodes), len(subgraphEdges))

	// Rank nodes by relevance
	type rankedNode struct {
		ID    string
		Score int
	}
	var ranked []rankedNode
	for nid := range subgraphNodes {
		n := nodes[nid]
		if n == nil {
			continue
		}
		score := matchScore(n.Label, terms)
		ranked = append(ranked, rankedNode{nid, score})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })

	// Output nodes
	sb.WriteString("Nodes:\n")
	for _, r := range ranked {
		n := nodes[r.ID]
		if n == nil {
			continue
		}
		line := fmt.Sprintf("  %s [type=%s file=%s]\n", n.Label, n.Type, n.File)
		if sb.Len()+len(line) > charBudget {
			sb.WriteString(fmt.Sprintf("  ... (%d more nodes truncated)\n", len(ranked)-len(strings.Split(sb.String(), "\n"))+3))
			break
		}
		sb.WriteString(line)
	}

	// Output edges
	sb.WriteString("\nEdges:\n")
	for _, e := range subgraphEdges {
		srcLabel := e.From
		tgtLabel := e.To
		if n := nodes[e.From]; n != nil {
			srcLabel = n.Label
		}
		if n := nodes[e.To]; n != nil {
			tgtLabel = n.Label
		}
		line := fmt.Sprintf("  %s --%s--> %s [%s]\n", srcLabel, e.Relation, tgtLabel, e.Confidence)
		if sb.Len()+len(line) > charBudget {
			sb.WriteString(fmt.Sprintf("  ... (truncated at ~%d token budget)\n", budget))
			break
		}
		sb.WriteString(line)
	}

	fmt.Print(sb.String())
	return nil
}

// queryPath finds the shortest path between two nodes.
func queryPath(graphPath string, nodeA string, nodeB string) error {
	data, err := os.ReadFile(graphPath)
	if err != nil {
		return fmt.Errorf("cannot read graph: %w", err)
	}

	var graphData struct {
		Nodes []jsonNode `json:"nodes"`
		Links []jsonLink `json:"links"`
	}
	if err := json.Unmarshal(data, &graphData); err != nil {
		return fmt.Errorf("cannot parse graph: %w", err)
	}

	nodes := make(map[string]*jsonNode)
	adjOut := make(map[string][]adjEntry)
	adjIn := make(map[string][]adjEntry)
	for i := range graphData.Nodes {
		n := &graphData.Nodes[i]
		nodes[n.ID] = n
	}
	for _, link := range graphData.Links {
		adjOut[link.Source] = append(adjOut[link.Source], adjEntry{Target: link.Target, Relation: link.Relation, Confidence: link.Confidence})
		adjIn[link.Target] = append(adjIn[link.Target], adjEntry{Target: link.Source, Relation: link.Relation, Confidence: link.Confidence})
	}

	src := findNode(nodes, nodeA)
	tgt := findNode(nodes, nodeB)
	if src == "" {
		return fmt.Errorf("no node found matching %q", nodeA)
	}
	if tgt == "" {
		return fmt.Errorf("no node found matching %q", nodeB)
	}

	// BFS shortest path
	parent := make(map[string]string)
	parentEdge := make(map[string]string) // node -> "relation [confidence]"
	visited := make(map[string]bool)
	queue := []string{src}
	visited[src] = true

	found := false
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if curr == tgt {
			found = true
			break
		}
		for _, adj := range adjOut[curr] {
			if !visited[adj.Target] {
				visited[adj.Target] = true
				parent[adj.Target] = curr
				parentEdge[adj.Target] = fmt.Sprintf("--%s--> [%s]", adj.Relation, adj.Confidence)
				queue = append(queue, adj.Target)
			}
		}
		for _, adj := range adjIn[curr] {
			if !visited[adj.Target] {
				visited[adj.Target] = true
				parent[adj.Target] = curr
				parentEdge[adj.Target] = fmt.Sprintf("<--%s-- [%s]", adj.Relation, adj.Confidence)
				queue = append(queue, adj.Target)
			}
		}
	}

	if !found {
		fmt.Printf("No path found between %q and %q\n", nodeA, nodeB)
		return nil
	}

	// Reconstruct path
	var path []string
	for curr := tgt; curr != ""; curr = parent[curr] {
		path = append([]string{curr}, path...)
		if curr == src {
			break
		}
	}

	fmt.Printf("Shortest path (%d hops):\n", len(path)-1)
	for i, nid := range path {
		n := nodes[nid]
		label := nid
		if n != nil {
			label = n.Label
		}
		if i < len(path)-1 {
			edge := parentEdge[path[i+1]]
			fmt.Printf("  %s %s\n", label, edge)
		} else {
			fmt.Printf("  %s\n", label)
		}
	}
	return nil
}

// queryExplain gives a comprehensive explanation of a node and its neighborhood.
func queryExplain(graphPath string, nodeName string) error {
	data, err := os.ReadFile(graphPath)
	if err != nil {
		return fmt.Errorf("cannot read graph: %w", err)
	}

	var graphData struct {
		Nodes []jsonNode `json:"nodes"`
		Links []jsonLink `json:"links"`
	}
	if err := json.Unmarshal(data, &graphData); err != nil {
		return fmt.Errorf("cannot parse graph: %w", err)
	}

	nodes := make(map[string]*jsonNode)
	for i := range graphData.Nodes {
		n := &graphData.Nodes[i]
		nodes[n.ID] = n
	}

	nodeID := findNode(nodes, nodeName)
	if nodeID == "" {
		return fmt.Errorf("no node found matching %q", nodeName)
	}

	n := nodes[nodeID]
	fmt.Printf("# %s\n\n", n.Label)
	fmt.Printf("Type: %s\n", n.Type)
	fmt.Printf("File: %s\n", n.File)
	fmt.Println()

	// Gather relationships grouped by relation type
	type rel struct {
		Direction string // "out" or "in"
		Relation  string
		Other     string
		OtherFile string
		Conf      string
	}
	var rels []rel
	for _, link := range graphData.Links {
		if link.Source == nodeID {
			otherLabel := link.Target
			otherFile := ""
			if other := nodes[link.Target]; other != nil {
				otherLabel = other.Label
				otherFile = other.File
			}
			rels = append(rels, rel{"out", link.Relation, otherLabel, otherFile, link.Confidence})
		}
		if link.Target == nodeID {
			otherLabel := link.Source
			otherFile := ""
			if other := nodes[link.Source]; other != nil {
				otherLabel = other.Label
				otherFile = other.File
			}
			rels = append(rels, rel{"in", link.Relation, otherLabel, otherFile, link.Confidence})
		}
	}

	// Group by relation type
	grouped := make(map[string][]rel)
	for _, r := range rels {
		key := r.Direction + ":" + r.Relation
		grouped[key] = append(grouped[key], r)
	}

	// Sort keys for deterministic output
	var keys []string
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("Connections (%d total):\n\n", len(rels))
	for _, key := range keys {
		items := grouped[key]
		parts := strings.SplitN(key, ":", 2)
		dir, relation := parts[0], parts[1]
		arrow := "--" + relation + "-->"
		if dir == "in" {
			arrow = "<--" + relation + "--"
		}
		fmt.Printf("  %s (%d):\n", arrow, len(items))
		for _, item := range items {
			fmt.Printf("    %s (%s)\n", item.Other, item.OtherFile)
		}
	}

	return nil
}

// Helper types
type jsonNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	File  string `json:"file"`
}

type jsonLink struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Relation   string `json:"relation"`
	Confidence string `json:"confidence"`
}

type adjEntry struct {
	Target     string
	Relation   string
	Confidence string
}

func extractTerms(question string) []string {
	words := strings.Fields(strings.ToLower(question))
	var terms []string
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"how": true, "what": true, "does": true, "do": true, "in": true,
		"to": true, "of": true, "and": true, "or": true, "with": true,
		"for": true, "this": true, "that": true, "from": true, "by": true,
	}
	for _, w := range words {
		if len(w) > 2 && !stopwords[w] {
			terms = append(terms, w)
		}
	}
	return terms
}

func matchScore(label string, terms []string) int {
	lower := strings.ToLower(label)
	score := 0
	for _, t := range terms {
		if strings.EqualFold(label, t) {
			score += 5 // exact match bonus
		} else if strings.Contains(lower, t) {
			score += 2
		}
	}
	return score
}

func findNode(nodes map[string]*jsonNode, name string) string {
	nameLower := strings.ToLower(name)
	// Try exact label match first
	for id, n := range nodes {
		if strings.EqualFold(n.Label, name) {
			return id
		}
		_ = id
	}
	// Fuzzy match
	type scored struct {
		Score int
		ID    string
	}
	var candidates []scored
	for id, n := range nodes {
		label := strings.ToLower(n.Label)
		if strings.Contains(label, nameLower) {
			score := 10 - len(label) + len(nameLower) // prefer shorter matches
			candidates = append(candidates, scored{score, id})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	if len(candidates) > 0 {
		return candidates[0].ID
	}
	return ""
}
