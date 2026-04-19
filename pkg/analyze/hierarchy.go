package analyze

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sjhorn/graphify/pkg/graph"
)

// GodNodeDetail extends GodNode with structural context.
type GodNodeDetail struct {
	GodNode
	ParentClass    string
	MethodCount    int
	InheritorCount int
	InheritorNames []string
	Docstring      string // extracted from source file
}

// InheritanceTree represents a class hierarchy rooted at a base class.
type InheritanceTree struct {
	Root     string
	RootFile string
	Children []InheritanceChild
	Depth    int
}

// InheritanceChild represents a direct subclass.
type InheritanceChild struct {
	Label       string
	File        string
	MethodCount int
	Children    []string // grandchildren labels
}

// EnrichGodNodes adds structural context to god nodes.
func EnrichGodNodes(g *graph.Graph, godNodes []GodNode) []GodNodeDetail {
	details := make([]GodNodeDetail, len(godNodes))
	for i, gn := range godNodes {
		details[i] = GodNodeDetail{GodNode: gn}

		// Find parent class (outgoing inherits edge)
		for _, edge := range g.Edges() {
			if edge.Source == gn.ID && edge.Relation == "inherits" {
				parent := g.GetNode(edge.Target)
				if parent != nil {
					details[i].ParentClass = parent.Label
					break
				}
			}
		}

		// Count methods
		methodCount := 0
		for _, edge := range g.Edges() {
			if edge.Source == gn.ID && edge.Relation == "method" {
				methodCount++
			}
		}
		details[i].MethodCount = methodCount

		// Count and list inheritors
		var inhNames []string
		for _, edge := range g.Edges() {
			if edge.Target == gn.ID && edge.Relation == "inherits" {
				child := g.GetNode(edge.Source)
				if child != nil {
					inhNames = append(inhNames, child.Label)
				}
			}
		}
		sort.Strings(inhNames)
		details[i].InheritorCount = len(inhNames)
		details[i].InheritorNames = inhNames
	}
	return details
}

// BuildInheritanceTrees finds the most significant inheritance hierarchies in the graph.
func BuildInheritanceTrees(g *graph.Graph, godNodes []GodNode) []InheritanceTree {
	// Build parent -> children map
	children := make(map[string][]string) // parent ID -> child IDs
	for _, edge := range g.Edges() {
		if edge.Relation == "inherits" {
			children[edge.Target] = append(children[edge.Target], edge.Source)
		}
	}

	// Find trees rooted at god nodes that have 3+ inheritors
	seen := make(map[string]bool)
	var trees []InheritanceTree

	for _, gn := range godNodes {
		kids := children[gn.ID]
		if len(kids) < 3 {
			continue
		}
		if seen[gn.ID] {
			continue
		}
		seen[gn.ID] = true

		tree := InheritanceTree{
			Root:     gn.Label,
			RootFile: gn.File,
		}

		maxDepth := 1
		for _, kidID := range kids {
			kid := g.GetNode(kidID)
			if kid == nil {
				continue
			}

			// Count methods on this child
			methodCount := 0
			for _, edge := range g.Edges() {
				if edge.Source == kidID && edge.Relation == "method" {
					methodCount++
				}
			}

			// Find grandchildren
			var grandkids []string
			for _, gkID := range children[kidID] {
				gk := g.GetNode(gkID)
				if gk != nil {
					grandkids = append(grandkids, gk.Label)
				}
			}
			if len(grandkids) > 0 {
				maxDepth = 2
			}

			tree.Children = append(tree.Children, InheritanceChild{
				Label:       kid.Label,
				File:        kid.File,
				MethodCount: methodCount,
				Children:    grandkids,
			})
		}

		tree.Depth = maxDepth
		sort.Slice(tree.Children, func(i, j int) bool {
			return len(tree.Children[i].Children) > len(tree.Children[j].Children)
		})
		trees = append(trees, tree)
	}

	sort.Slice(trees, func(i, j int) bool {
		return len(trees[i].Children) > len(trees[j].Children)
	})

	// Limit to top 5
	if len(trees) > 5 {
		trees = trees[:5]
	}

	return trees
}

// RenderInheritanceTree formats a tree as indented text.
func RenderInheritanceTree(tree InheritanceTree) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("`%s` (%s)", tree.Root, tree.RootFile))

	limit := 10
	if len(tree.Children) < limit {
		limit = len(tree.Children)
	}
	for i, child := range tree.Children[:limit] {
		prefix := "├──"
		if i == limit-1 && len(tree.Children) <= limit {
			prefix = "└──"
		}
		methodNote := ""
		if child.MethodCount > 0 {
			methodNote = fmt.Sprintf(" [%d methods]", child.MethodCount)
		}
		lines = append(lines, fmt.Sprintf("  %s `%s`%s", prefix, child.Label, methodNote))

		// Show grandchildren
		for j, gk := range child.Children {
			gkPrefix := "│   ├──"
			if j == len(child.Children)-1 {
				gkPrefix = "│   └──"
			}
			lines = append(lines, fmt.Sprintf("  %s `%s`", gkPrefix, gk))
		}
	}
	if len(tree.Children) > limit {
		lines = append(lines, fmt.Sprintf("  └── (+%d more)", len(tree.Children)-limit))
	}
	return strings.Join(lines, "\n")
}
