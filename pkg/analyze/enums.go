package analyze

import (
	"sort"

	"github.com/sjhorn/graphify/pkg/graph"
)

// EnumSummary describes an enum and its cases.
type EnumSummary struct {
	Label     string
	File      string
	CaseCount int
	Cases     []string // first few case labels
}

// DetectEnums finds enum nodes and counts their cases via case_of edges.
// Returns enums sorted by case count descending, filtered to 3+ cases.
func DetectEnums(g *graph.Graph) []EnumSummary {
	var enums []EnumSummary

	for _, node := range g.Nodes() {
		if node.Type != "enum" {
			continue
		}

		var cases []string
		for _, edge := range g.Edges() {
			if edge.Source == node.ID && edge.Relation == "case_of" {
				caseNode := g.GetNode(edge.Target)
				if caseNode != nil {
					cases = append(cases, caseNode.Label)
				}
			}
		}

		if len(cases) < 3 {
			continue
		}

		sort.Strings(cases)
		enums = append(enums, EnumSummary{
			Label:     node.Label,
			File:      node.File,
			CaseCount: len(cases),
			Cases:     cases,
		})
	}

	sort.Slice(enums, func(i, j int) bool {
		return enums[i].CaseCount > enums[j].CaseCount
	})

	return enums
}
