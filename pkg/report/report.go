package report

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sjhorn/graphify/pkg/analyze"
	"github.com/sjhorn/graphify/pkg/graph"
)

// DetectionResult contains file detection results.
type DetectionResult struct {
	TotalFiles int
	TotalWords int
	NeedsGraph bool
	Warning    string
}

// TokenInfo contains token usage information.
type TokenInfo struct {
	Input  int
	Output int
}

// Generate creates a markdown report from the graph analysis.
func Generate(
	g *graph.Graph,
	communities map[int][]string,
	cohesionScores map[int]float64,
	communityLabels map[int]string,
	analysis *analyze.Analysis,
	detection DetectionResult,
	tokens TokenInfo,
	root string,
) string {
	var lines []string

	// 1. Header
	lines = append(lines, fmt.Sprintf("# Graph Report - %s", root))
	lines = append(lines, "")

	// 2. Executive Summary
	if analysis.Summary != "" {
		lines = append(lines, "## Executive Summary")
		lines = append(lines, "")
		lines = append(lines, analysis.Summary)
		lines = append(lines, "")
	}

	// 3. Architecture (source-only deps prioritized)
	if analysis.Layers != nil && len(analysis.Layers.DirectoryDeps) > 0 {
		lines = append(lines, "## Architecture")
		lines = append(lines, "")

		// Show source-only layer order if available, fall back to full
		layerOrder := analysis.Layers.SourceLayerOrder
		if len(layerOrder) == 0 {
			layerOrder = analysis.Layers.LayerOrder
		}
		if len(layerOrder) > 0 {
			lines = append(lines, "**Source layer order** (top depends on bottom): "+strings.Join(layerOrder, " -> "))
		}

		// Show source-only deps first
		sourceDeps := analysis.Layers.SourceDeps
		if len(sourceDeps) > 0 {
			lines = append(lines, "")
			lines = append(lines, "**Source dependencies:**")
			limit := 10
			if len(sourceDeps) < limit {
				limit = len(sourceDeps)
			}
			for _, dep := range sourceDeps[:limit] {
				lines = append(lines, fmt.Sprintf("- %s -> %s (%d edges)", dep.From, dep.To, dep.Count))
			}
		}

		if len(analysis.Layers.SourceCycles) > 0 {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("**Dependency cycles (%d):**", len(analysis.Layers.SourceCycles)))
			for _, v := range analysis.Layers.Violations {
				lines = append(lines, "- "+v)
			}
		} else if len(sourceDeps) > 0 {
			lines = append(lines, "")
			lines = append(lines, "No dependency cycles detected in source code.")
		}
		lines = append(lines, "")
	}

	// 4. Design Patterns (skip if none)
	if len(analysis.Patterns) > 0 {
		lines = append(lines, "## Design Patterns")
		lines = append(lines, "")
		for _, p := range analysis.Patterns {
			lines = append(lines, fmt.Sprintf("**%s** - `%s`", p.Name, p.AnchorNode))
			if p.AnchorFile != "" {
				lines = append(lines, fmt.Sprintf("  File: %s", p.AnchorFile))
			}
			for _, e := range p.Evidence {
				lines = append(lines, "  - "+e)
			}
			lines = append(lines, fmt.Sprintf("  Participants: %d", p.Participants))
		}
		lines = append(lines, "")
	}

	// 4b. Enums (skip if none with 3+ cases)
	if len(analysis.Enums) > 0 {
		lines = append(lines, "## Enums & State Machines")
		lines = append(lines, "")
		limit := 8
		if len(analysis.Enums) < limit {
			limit = len(analysis.Enums)
		}
		for _, e := range analysis.Enums[:limit] {
			casePreview := e.Cases
			if len(casePreview) > 5 {
				casePreview = append(casePreview[:5], fmt.Sprintf("+%d more", len(casePreview)-5))
			}
			lines = append(lines, fmt.Sprintf("- `%s` (%d cases): %s", e.Label, e.CaseCount, strings.Join(casePreview, ", ")))
		}
		if len(analysis.Enums) > limit {
			lines = append(lines, fmt.Sprintf("- (+%d more enums)", len(analysis.Enums)-limit))
		}
		lines = append(lines, "")
	}

	// 5. Project Structure
	lines = append(lines, "## Project Structure")
	lines = append(lines, "")
	if detection.Warning != "" {
		lines = append(lines, "- "+detection.Warning)
	} else {
		lines = append(lines, fmt.Sprintf("- %d files · ~%d words", detection.TotalFiles, detection.TotalWords))
		lines = append(lines, "- Verdict: corpus is large enough that graph structure adds value.")
	}
	if len(analysis.DirectoryStats) > 0 {
		lines = append(lines, "")
		lines = append(lines, "| Directory | Files | Nodes |")
		lines = append(lines, "|-----------|------:|------:|")
		for _, ds := range analysis.DirectoryStats {
			lines = append(lines, fmt.Sprintf("| %s | %d | %d |", ds.Directory, ds.FileCount, ds.NodeCount))
		}
	}

	// Summary stats
	confidenceCounts := make(map[string]int)
	for _, edge := range g.Edges() {
		confidenceCounts[edge.Confidence]++
	}
	totalEdges := len(g.Edges())
	if totalEdges == 0 {
		totalEdges = 1
	}
	extPct := confidenceCounts["EXTRACTED"] * 100 / totalEdges
	infPct := confidenceCounts["INFERRED"] * 100 / totalEdges
	ambPct := confidenceCounts["AMBIGUOUS"] * 100 / totalEdges

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("- %d nodes · %d edges · %d communities detected",
		g.NodeCount(), len(g.Edges()), len(communities)))
	lines = append(lines, fmt.Sprintf("- Extraction: %d%% EXTRACTED · %d%% INFERRED · %d%% AMBIGUOUS",
		extPct, infPct, ambPct))
	lines = append(lines, fmt.Sprintf("- Token cost: %s input · %s output",
		formatNumber(tokens.Input), formatNumber(tokens.Output)))

	// 5b. Key Files (highest information density)
	if len(analysis.KeyFiles) > 0 {
		lines = append(lines, "")
		lines = append(lines, "## Key Files (read these for maximum insight)")
		for i, kf := range analysis.KeyFiles {
			classInfo := ""
			if len(kf.ClassNames) > 0 {
				names := kf.ClassNames
				if len(names) > 3 {
					names = append(names[:3], fmt.Sprintf("+%d more", len(names)-3))
				}
				classInfo = fmt.Sprintf(" — %s", strings.Join(names, ", "))
			}
			lines = append(lines, fmt.Sprintf("%d. `%s` (%d nodes, %d methods)%s",
				i+1, kf.Path, kf.NodeCount, kf.MethodCount, classInfo))
		}
	}

	// 5c. Runtime Dependencies (cross-file calls)
	if len(analysis.RuntimeDeps) > 0 {
		lines = append(lines, "")
		lines = append(lines, "## Runtime Dependencies (cross-file calls)")
		for _, rd := range analysis.RuntimeDeps {
			lines = append(lines, fmt.Sprintf("- %s -> %s (%d calls)", rd.FromFile, rd.ToFile, rd.Count))
		}
	}

	// 6. God Nodes (enriched)
	lines = append(lines, "")
	lines = append(lines, "## God Nodes (most connected - your core abstractions)")
	for i, detail := range analysis.GodNodeDetails {
		// Base line: name, type, degree
		typePart := ""
		if detail.GodNode.File != "" {
			typePart = fmt.Sprintf(" (%s)", detail.GodNode.File)
		}
		lines = append(lines, fmt.Sprintf("%d. `%s` - %d edges%s", i+1, detail.Label, detail.Degree, typePart))

		// Context line: parent, methods, inheritors
		var context []string
		if detail.ParentClass != "" {
			context = append(context, "extends "+detail.ParentClass)
		}
		if detail.MethodCount > 0 {
			context = append(context, fmt.Sprintf("%d methods", detail.MethodCount))
		}
		if detail.InheritorCount > 0 {
			names := detail.InheritorNames
			if len(names) > 3 {
				names = append(names[:3], fmt.Sprintf("+%d more", len(names)-3))
			}
			context = append(context, fmt.Sprintf("%d subclasses: %s", detail.InheritorCount, strings.Join(names, ", ")))
		}
		if len(context) > 0 {
			lines = append(lines, "   "+strings.Join(context, " · "))
		}
		if detail.Docstring != "" {
			lines = append(lines, "   > "+detail.Docstring)
		}
	}

	// 6b. Inheritance Trees
	if len(analysis.InheritanceTrees) > 0 {
		lines = append(lines, "")
		lines = append(lines, "## Class Hierarchies")
		for _, tree := range analysis.InheritanceTrees {
			lines = append(lines, "")
			lines = append(lines, analyze.RenderInheritanceTree(tree))
		}
	}

	// 7. Surprising Connections
	lines = append(lines, "")
	lines = append(lines, "## Surprising Connections (you probably didn't know these)")
	if len(analysis.SurprisingConnections) > 0 {
		absRoot, _ := filepath.Abs(root)
		for _, s := range analysis.SurprisingConnections {
			confTag := s.Confidence
			files := s.SourceFiles
			if len(files) < 2 {
				files = []string{"", ""}
			}
			f0 := relativePath(files[0], absRoot)
			f1 := relativePath(files[1], absRoot)
			lines = append(lines, fmt.Sprintf("- `%s` --%s--> `%s`  [%s]",
				s.Source, s.Relation, s.Target, confTag))
			lines = append(lines, fmt.Sprintf("  %s → %s", f0, f1))
		}
	} else {
		lines = append(lines, "- None detected - all connections are within the same source files.")
	}

	// 8. Divider
	lines = append(lines, "")
	lines = append(lines, "---")

	// 9. Communities — filter singletons
	lines = append(lines, "")
	lines = append(lines, "## Communities")

	if analysis.SingletonCount > 0 {
		lines = append(lines, fmt.Sprintf("*+ %d isolated nodes omitted*", analysis.SingletonCount))
	}

	// Sort community IDs for deterministic output
	var cids []int
	for cid := range communities {
		cids = append(cids, cid)
	}
	sort.Ints(cids)

	for _, cid := range cids {
		nodes := communities[cid]
		// Skip singletons
		if len(nodes) <= 1 {
			continue
		}
		label := communityLabels[cid]
		score := cohesionScores[cid]
		display := make([]string, 0, 8)
		for i, nodeID := range nodes {
			if i >= 8 {
				break
			}
			node := g.GetNode(nodeID)
			if node != nil {
				display = append(display, node.Label)
			}
		}
		suffix := ""
		if len(nodes) > 8 {
			suffix = fmt.Sprintf(" (+%d more)", len(nodes)-8)
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("### Community %d - \"%s\"", cid, label))
		lines = append(lines, fmt.Sprintf("Cohesion: %.2f", score))
		lines = append(lines, fmt.Sprintf("Nodes (%d): %s%s", len(nodes), strings.Join(display, ", "), suffix))
	}

	// 10. Ambiguous Edges
	ambiguousEdges := make([]*graph.Edge, 0)
	for _, edge := range g.Edges() {
		if edge.Confidence == "AMBIGUOUS" {
			ambiguousEdges = append(ambiguousEdges, edge)
		}
	}
	if len(ambiguousEdges) > 0 {
		lines = append(lines, "")
		lines = append(lines, "## Ambiguous Edges - Review These")
		for _, edge := range ambiguousEdges {
			srcNode := g.GetNode(edge.Source)
			tgtNode := g.GetNode(edge.Target)
			srcLabel := edge.Source
			tgtLabel := edge.Target
			if srcNode != nil {
				srcLabel = srcNode.Label
			}
			if tgtNode != nil {
				tgtLabel = tgtNode.Label
			}
			lines = append(lines, fmt.Sprintf("- `%s` → `%s`  [AMBIGUOUS]", srcLabel, tgtLabel))
			lines = append(lines, fmt.Sprintf("  %s · relation: %s", edge.Source, edge.Relation))
		}
	}

	return strings.Join(lines, "\n")
}

// safeCommunityName cleans a community label for use in filenames.
func safeCommunityName(label string) string {
	// Remove special characters
	specialChars := `[]\\/*?:"<>|#^`
	result := label
	for _, c := range specialChars {
		result = strings.ReplaceAll(result, string(c), "")
	}
	// Replace newlines
	result = strings.ReplaceAll(result, "\r\n", " ")
	result = strings.ReplaceAll(result, "\r", " ")
	result = strings.ReplaceAll(result, "\n", " ")
	// Remove file extension
	re := regexp.MustCompile(`\.(md|mdx|markdown)$`)
	result = re.ReplaceAllString(result, "")
	result = strings.TrimSpace(result)
	if result == "" {
		result = "unnamed"
	}
	return result
}

// formatNumber formats a number with comma separators.
func formatNumber(n int) string {
	numStr := strconv.Itoa(n)
	result := ""
	commaIdx := len(numStr) % 3
	for i, c := range numStr {
		if i > 0 && i%3 == commaIdx && commaIdx != 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// relativePath makes a path relative to root, or returns it unchanged on error.
func relativePath(path, root string) string {
	if path == "" || root == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

// sortIntsDesc sorts a slice of integers in descending order.
func sortIntsDesc(nums []int) []int {
	sort.Slice(nums, func(i, j int) bool {
		return nums[i] > nums[j]
	})
	return nums
}
