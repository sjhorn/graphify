package analyze

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sjhorn/graphify/pkg/graph"
)

// DesignPattern represents a detected design pattern in the codebase.
type DesignPattern struct {
	Name         string
	AnchorNode   string
	AnchorFile   string
	Evidence     []string
	Participants int
}

// isProjectClass returns true if the node appears to be a project-defined class
// (not an external framework type).
func isProjectClass(g *graph.Graph, nodeID string) bool {
	node := g.GetNode(nodeID)
	if node == nil || node.Type != "class" {
		return false
	}
	return !isExternalType(g, nodeID)
}

// DetectPatterns scans the graph for common design patterns.
func DetectPatterns(g *graph.Graph) []DesignPattern {
	var patterns []DesignPattern

	patterns = append(patterns, detectCommand(g)...)
	patterns = append(patterns, detectBuilderFactory(g)...)
	patterns = append(patterns, detectObserver(g)...)
	patterns = append(patterns, detectStrategy(g)...)

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Participants > patterns[j].Participants
	})

	return patterns
}

// methodsOf returns the labels of nodes connected by outgoing "method" edges.
func methodsOf(g *graph.Graph, nodeID string) []string {
	var methods []string
	for _, edge := range g.Edges() {
		if edge.Source == nodeID && edge.Relation == "method" {
			tgt := g.GetNode(edge.Target)
			if tgt != nil {
				methods = append(methods, tgt.Label)
			}
		}
	}
	return methods
}

// inheritors returns node IDs connected by incoming "inherits" edges to nodeID.
func inheritors(g *graph.Graph, nodeID string) []string {
	var result []string
	for _, edge := range g.Edges() {
		if edge.Target == nodeID && edge.Relation == "inherits" {
			result = append(result, edge.Source)
		}
	}
	return result
}

// detectCommand looks for the Command pattern:
// A class with execute/run/perform/handle method + 3+ inheritors that also have that method.
// Also detects the pattern when the base class lacks the method edge but inheritors share one.
func detectCommand(g *graph.Graph) []DesignPattern {
	// "perform" excluded — too generic (matches Flutter's performLayout/performResize)
	commandVerbs := []string{"execute", "run", "handle"}
	var patterns []DesignPattern

	for _, node := range g.Nodes() {
		if !isProjectClass(g, node.ID) {
			continue
		}

		inhs := inheritors(g, node.ID)
		if len(inhs) < 3 {
			continue
		}

		// Check if base class has the method directly
		methods := methodsOf(g, node.ID)
		matchedVerb := matchesAny(methods, commandVerbs)

		// If base doesn't have it, check if inheritors share a common command method
		if matchedVerb == "" {
			verbCounts := make(map[string]int)
			for _, inhID := range inhs {
				inhMethods := methodsOf(g, inhID)
				for _, v := range commandVerbs {
					if matchesAny(inhMethods, []string{v}) != "" {
						verbCounts[v]++
					}
				}
			}
			for v, count := range verbCounts {
				if count >= 3 {
					matchedVerb = v
					break
				}
			}
		}

		if matchedVerb == "" {
			continue
		}

		// Count inheritors with the method
		count := 0
		var participantNames []string
		for _, inhID := range inhs {
			inhMethods := methodsOf(g, inhID)
			if matchesAny(inhMethods, []string{matchedVerb}) != "" {
				count++
				inhNode := g.GetNode(inhID)
				if inhNode != nil {
					participantNames = append(participantNames, inhNode.Label)
				}
			}
		}
		if count < 3 {
			continue
		}

		evidence := []string{
			count_str(len(inhs)) + " subclasses share " + matchedVerb + "() method",
			strings.Join(truncateNames(participantNames, 5), ", ") + " inherit and override it",
		}
		patterns = append(patterns, DesignPattern{
			Name:         "Command",
			AnchorNode:   node.Label,
			AnchorFile:   node.File,
			Evidence:     evidence,
			Participants: count + 1,
		})
	}
	return patterns
}

// detectBuilderFactory looks for Builder/Factory pattern:
// A class with create*/build*/make* methods + 2+ inheritors.
// Also detects when inheritors share factory methods even if base doesn't have them.
func detectBuilderFactory(g *graph.Graph) []DesignPattern {
	prefixes := []string{"create", "build", "make"}
	var patterns []DesignPattern

	for _, node := range g.Nodes() {
		if !isProjectClass(g, node.ID) {
			continue
		}

		inhs := inheritors(g, node.ID)
		if len(inhs) < 2 {
			continue
		}

		// Check base class methods first
		methods := methodsOf(g, node.ID)
		matched := matchesPrefix(methods, prefixes)

		// If base doesn't have factory methods, check inheritors
		if len(matched) == 0 {
			methodCounts := make(map[string]int)
			for _, inhID := range inhs {
				inhMethods := methodsOf(g, inhID)
				inhMatched := matchesPrefix(inhMethods, prefixes)
				for _, m := range inhMatched {
					methodCounts[strings.ToLower(m)]++
				}
			}
			// Find methods shared by 2+ inheritors
			for m, count := range methodCounts {
				if count >= 2 {
					matched = append(matched, m)
				}
			}
		}

		if len(matched) == 0 {
			continue
		}

		var inhNames []string
		for _, inhID := range inhs {
			inhNode := g.GetNode(inhID)
			if inhNode != nil {
				inhNames = append(inhNames, inhNode.Label)
			}
		}

		evidence := []string{
			"factory methods: " + strings.Join(matched, ", "),
			"subclasses: " + strings.Join(truncateNames(inhNames, 5), ", "),
		}
		patterns = append(patterns, DesignPattern{
			Name:         "Builder/Factory",
			AnchorNode:   node.Label,
			AnchorFile:   node.File,
			Evidence:     evidence,
			Participants: len(inhs) + 1,
		})
	}
	return patterns
}

// detectObserver looks for Observer pattern:
// A class with 2+ methods from {addListener, removeListener, notify, subscribe, unsubscribe, addReaction}.
func detectObserver(g *graph.Graph) []DesignPattern {
	observerPrefixes := []string{
		"addlistener", "removelistener", "notify",
		"addreaction", "subscribe", "unsubscribe",
		"addobserver", "removeobserver",
		"on", "emit",
	}
	var patterns []DesignPattern

	for _, node := range g.Nodes() {
		if !isProjectClass(g, node.ID) {
			continue
		}
		methods := methodsOf(g, node.ID)
		matched := matchesPrefix(methods, observerPrefixes)
		if len(matched) < 2 {
			continue
		}

		evidence := []string{
			"observer methods: " + strings.Join(matched, ", "),
		}
		patterns = append(patterns, DesignPattern{
			Name:         "Observer",
			AnchorNode:   node.Label,
			AnchorFile:   node.File,
			Evidence:     evidence,
			Participants: 1 + len(matched),
		})
	}
	return patterns
}

// detectStrategy finds groups of 3+ classes that share the same method interface
// (≥3 shared methods, ≥60% overlap) without being in the same inheritance tree.
// This catches Tool/Strategy/Handler patterns like markdraw's 10 drawing tools.
func detectStrategy(g *graph.Graph) []DesignPattern {
	// Build method sets per class
	type classInfo struct {
		ID      string
		Label   string
		File    string
		Methods map[string]bool
	}
	var classes []classInfo

	for _, node := range g.Nodes() {
		if !isProjectClass(g, node.ID) {
			continue
		}
		methods := methodsOf(g, node.ID)
		if len(methods) < 3 {
			continue
		}
		methodSet := make(map[string]bool)
		for _, m := range methods {
			methodSet[strings.ToLower(normalizeMethodName(m))] = true
		}
		classes = append(classes, classInfo{node.ID, node.Label, node.File, methodSet})
	}

	// Find groups sharing ≥3 methods with ≥60% overlap
	// Use the most common shared method set as the group key
	type groupKey struct {
		methods string
	}
	groups := make(map[string][]string) // shared methods string -> class labels
	groupFiles := make(map[string]string)
	groupMethods := make(map[string][]string)

	for i := range classes {
		for j := i + 1; j < len(classes); j++ {
			a, b := classes[i], classes[j]
			// Skip if in same inheritance tree
			if g.HasEdge(a.ID, b.ID) || g.HasEdge(b.ID, a.ID) {
				continue
			}
			// Compute overlap
			shared := make([]string, 0)
			for m := range a.Methods {
				if b.Methods[m] {
					shared = append(shared, m)
				}
			}
			union := len(a.Methods) + len(b.Methods) - len(shared)
			if len(shared) < 3 || (union > 0 && len(shared)*100/union < 60) {
				continue
			}
			sort.Strings(shared)
			key := strings.Join(shared, ",")
			if groups[key] == nil {
				groupMethods[key] = shared
				groupFiles[key] = a.File
			}
			groups[key] = append(groups[key], a.Label, b.Label)
		}
	}

	// Deduplicate class names per group and filter to 3+
	var patterns []DesignPattern
	seen := make(map[string]bool) // avoid duplicate class listings

	// Sort by group size descending
	type sortedGroup struct {
		key     string
		classes []string
	}
	var sorted []sortedGroup
	for key, classList := range groups {
		unique := uniqueStrings(classList)
		if len(unique) >= 3 {
			sorted = append(sorted, sortedGroup{key, unique})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].classes) > len(sorted[j].classes)
	})

	for _, sg := range sorted {
		// Skip if most classes already reported in another group
		newCount := 0
		for _, c := range sg.classes {
			if !seen[c] {
				newCount++
			}
		}
		if newCount < 2 {
			continue
		}
		for _, c := range sg.classes {
			seen[c] = true
		}

		methods := groupMethods[sg.key]
		evidence := []string{
			fmt.Sprintf("%d classes share interface: %s", len(sg.classes), strings.Join(truncateNames(methods, 5), ", ")),
			"implementations: " + strings.Join(truncateNames(sg.classes, 5), ", "),
		}
		patterns = append(patterns, DesignPattern{
			Name:         "Strategy",
			AnchorNode:   sg.classes[0],
			AnchorFile:   groupFiles[sg.key],
			Evidence:     evidence,
			Participants: len(sg.classes),
		})
	}

	return patterns
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result
}

func count_str(n int) string {
	return fmt.Sprintf("%d", n)
}

func truncateNames(names []string, max int) []string {
	if len(names) <= max {
		return names
	}
	result := make([]string, max)
	copy(result, names[:max])
	result = append(result, fmt.Sprintf("(+%d more)", len(names)-max))
	return result
}

// normalizeMethodName strips common prefixes like "." and "()" suffixes.
func normalizeMethodName(name string) string {
	n := strings.TrimPrefix(name, ".")
	n = strings.TrimSuffix(n, "()")
	return n
}

// matchesAny returns the first verb that any method name starts with (case-insensitive).
func matchesAny(methods []string, verbs []string) string {
	for _, m := range methods {
		lower := strings.ToLower(normalizeMethodName(m))
		for _, v := range verbs {
			if lower == v || strings.HasPrefix(lower, v) {
				return v
			}
		}
	}
	return ""
}

// matchesPrefix returns all methods that start with any of the given prefixes.
func matchesPrefix(methods []string, prefixes []string) []string {
	var matched []string
	for _, m := range methods {
		lower := strings.ToLower(normalizeMethodName(m))
		for _, p := range prefixes {
			if strings.HasPrefix(lower, p) {
				matched = append(matched, m)
				break
			}
		}
	}
	return matched
}
