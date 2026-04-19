package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sjhorn/graphify/pkg/analyze"
	"github.com/sjhorn/graphify/pkg/cluster"
	"github.com/sjhorn/graphify/pkg/graph"
)

func makeInputs() (*graph.Graph, map[int][]string, map[int]float64, map[int]string, *analyze.Analysis) {
	fixturesDir := "../../testdata/fixtures"
	data, _ := os.ReadFile(filepath.Join(fixturesDir, "extraction.json"))
	var extraction map[string]interface{}
	json.Unmarshal(data, &extraction)
	g := graph.BuildFromJSON(extraction)
	communities := cluster.Cluster(g)
	scores := cluster.ScoreAll(g, communities.Communities)
	labels := make(map[int]string)
	for cid := range communities.Communities {
		labels[cid] = "Community " + string(rune('0'+cid))
	}
	detection := analyze.DetectResultInfo{TotalFiles: 4, TotalWords: 62400}
	analysis := analyze.Analyze(g, communities.Communities, detection)
	return g, communities.Communities, scores, labels, analysis
}

func TestReportContainsHeader(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if report == "" {
		t.Error("Generate() returned empty report")
	}
}

func TestReportContainsProjectStructure(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "## Project Structure") {
		t.Error("Report should contain Project Structure section")
	}
}

func TestReportContainsGodNodes(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "## God Nodes") {
		t.Error("Report should contain God Nodes section")
	}
}

func TestReportContainsSurprisingConnections(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "## Surprising Connections") {
		t.Error("Report should contain Surprising Connections section")
	}
}

func TestReportContainsCommunities(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "## Communities") {
		t.Error("Report should contain Communities section")
	}
}

func TestReportContainsAmbiguousSection(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "## Ambiguous Edges") {
		t.Error("Report should contain Ambiguous Edges section")
	}
}

func TestReportShowsTokenCost(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "Token cost") {
		t.Error("Report should contain Token cost")
	}
}

func TestReportShowsCohesionScores(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "Cohesion:") {
		t.Error("Report should contain Cohesion score")
	}
}

func TestReportContainsExecutiveSummary(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "## Executive Summary") {
		t.Error("Report should contain Executive Summary section")
	}
}

func TestReportHasDivider(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 4, TotalWords: 62400, NeedsGraph: true}
	tokens := TokenInfo{Input: 1200, Output: 300}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "\n---\n") {
		t.Error("Report should contain a divider between above/below the fold")
	}
}
