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

func TestSafeCommunityName(t *testing.T) {
	tests := map[string]string{
		"Hello World":       "Hello World",
		"test/foo*bar":      "testfoobar",
		"file.md":           "file",
		"":                  "unnamed",
		"has\nnewlines":     "has newlines",
		"special[chars]#^":  "specialchars",
	}
	for input, expected := range tests {
		got := safeCommunityName(input)
		if got != expected {
			t.Errorf("safeCommunityName(%q) = %q; want %q", input, got, expected)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	tests := map[int]string{
		0:       "0",
		42:      "42",
		1000:    "1,000",
		1234567: "1,234,567",
	}
	for input, expected := range tests {
		got := formatNumber(input)
		if got != expected {
			t.Errorf("formatNumber(%d) = %q; want %q", input, got, expected)
		}
	}
}

func TestRelativePath(t *testing.T) {
	got := relativePath("/home/user/project/src/main.go", "/home/user/project")
	if got != "src/main.go" {
		t.Errorf("relativePath() = %q; want src/main.go", got)
	}
}

func TestRelativePathEmpty(t *testing.T) {
	got := relativePath("", "/root")
	if got != "" {
		t.Errorf("relativePath('', ...) = %q; want empty", got)
	}
	got = relativePath("/foo", "")
	if got != "/foo" {
		t.Errorf("relativePath(..., '') = %q; want /foo", got)
	}
}

func TestSortIntsDesc(t *testing.T) {
	nums := []int{3, 1, 4, 1, 5, 9, 2, 6}
	result := sortIntsDesc(nums)
	for i := 1; i < len(result); i++ {
		if result[i] > result[i-1] {
			t.Errorf("sortIntsDesc() not descending at index %d: %d > %d", i, result[i], result[i-1])
		}
	}
}

func TestReportWithWarning(t *testing.T) {
	g, communities, scores, labels, analysis := makeInputs()
	detection := DetectionResult{TotalFiles: 1, TotalWords: 10, Warning: "Too few files"}
	tokens := TokenInfo{Input: 100, Output: 50}
	report := Generate(g, communities, scores, labels, analysis, detection, tokens, "./project")
	if !strings.Contains(report, "Too few files") {
		t.Error("Report should contain warning when present")
	}
}

func TestReportNoSurprisingConnections(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	communities := map[int][]string{0: {"a"}}
	scores := map[int]float64{0: 1.0}
	labels := map[int]string{0: "Solo"}
	detection := analyze.DetectResultInfo{TotalFiles: 1}
	analysis := analyze.Analyze(g, communities, detection)
	det := DetectionResult{TotalFiles: 1}
	tokens := TokenInfo{}
	report := Generate(g, communities, scores, labels, analysis, det, tokens, ".")
	if !strings.Contains(report, "None detected") {
		t.Error("Report should say none detected when no surprising connections")
	}
}

func TestReportWithLayersAndPatterns(t *testing.T) {
	g := graph.NewGraph()
	// Create a graph that produces layers, patterns, enums, key files, and runtime deps
	g.AddNode("cmd", "Command", "class", "lib/src/commands/command.dart")
	g.AddNode("cmd_exec", "execute", "method", "lib/src/commands/command.dart")
	g.AddEdge("cmd", "cmd_exec", "method", "EXTRACTED", 1.0)
	for _, name := range []string{"EditCmd", "DeleteCmd", "CopyCmd"} {
		id := "cmd_" + name
		g.AddNode(id, name, "class", "lib/src/commands/"+name+".dart")
		g.AddEdge(id, "cmd", "inherits", "EXTRACTED", 1.0)
		execID := id + "_exec"
		g.AddNode(execID, "execute", "method", "lib/src/commands/"+name+".dart")
		g.AddEdge(id, execID, "method", "EXTRACTED", 1.0)
	}

	// Enum
	g.AddNode("color", "Color", "enum", "lib/src/models/colors.dart")
	for _, c := range []string{"Red", "Green", "Blue"} {
		id := "color_" + c
		g.AddNode(id, c, "enum_value", "lib/src/models/colors.dart")
		g.AddEdge("color", id, "case_of", "EXTRACTED", 1.0)
	}

	// Cross-file calls (runtime deps)
	g.AddNode("caller", "caller()", "function", "lib/src/widgets/caller.dart")
	g.AddEdge("caller", "cmd_exec", "calls", "EXTRACTED", 1.0)

	// Cross-directory imports (layers)
	g.AddEdge("caller", "cmd", "imports", "EXTRACTED", 1.0)

	communities := map[int][]string{
		0: {"cmd", "cmd_exec", "cmd_EditCmd", "cmd_DeleteCmd", "cmd_CopyCmd",
			"cmd_EditCmd_exec", "cmd_DeleteCmd_exec", "cmd_CopyCmd_exec"},
		1: {"color", "color_Red", "color_Green", "color_Blue"},
		2: {"caller"},
	}
	scores := cluster.ScoreAll(g, communities)
	labels := map[int]string{0: "Commands", 1: "Colors", 2: "UI"}

	detection := analyze.DetectResultInfo{TotalFiles: 5, TotalWords: 2000}
	analysis := analyze.Analyze(g, communities, detection)

	det := DetectionResult{TotalFiles: 5, TotalWords: 2000}
	tokens := TokenInfo{Input: 5000, Output: 1000}
	report := Generate(g, communities, scores, labels, analysis, det, tokens, ".")

	if !strings.Contains(report, "## Design Patterns") {
		t.Error("Report should contain Design Patterns section")
	}
	if !strings.Contains(report, "Enums") {
		t.Error("Report should contain Enums section")
	}
}

func TestReportWithLayerCycles(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "WidgetA", "class", "lib/src/widgets/widget_a.dart")
	g.AddNode("b", "ModelB", "class", "lib/src/models/model_b.dart")
	g.AddNode("c", "ServiceC", "class", "lib/src/services/service_c.dart")
	g.AddEdge("a", "b", "imports", "EXTRACTED", 1.0)
	g.AddEdge("b", "c", "calls", "EXTRACTED", 1.0)
	g.AddEdge("c", "a", "imports", "EXTRACTED", 1.0)

	communities := map[int][]string{0: {"a", "b", "c"}}
	scores := map[int]float64{0: 0.5}
	labels := map[int]string{0: "Core"}

	detection := analyze.DetectResultInfo{TotalFiles: 3}
	analysis := analyze.Analyze(g, communities, detection)

	det := DetectionResult{TotalFiles: 3}
	tokens := TokenInfo{}
	report := Generate(g, communities, scores, labels, analysis, det, tokens, ".")

	if !strings.Contains(report, "Architecture") {
		t.Error("Report should contain Architecture section with cycles")
	}
}

func TestReportSingletonCommunities(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode("a", "A", "class", "a.py")
	g.AddNode("b", "B", "class", "b.py")
	g.AddNode("c", "C", "class", "c.py")

	communities := map[int][]string{0: {"a", "b"}, 1: {"c"}}
	scores := map[int]float64{0: 0.5, 1: 1.0}
	labels := map[int]string{0: "Main", 1: "Solo"}

	detection := analyze.DetectResultInfo{TotalFiles: 3}
	analysis := analyze.Analyze(g, communities, detection)

	det := DetectionResult{TotalFiles: 3}
	tokens := TokenInfo{}
	report := Generate(g, communities, scores, labels, analysis, det, tokens, ".")

	if !strings.Contains(report, "isolated nodes omitted") {
		t.Error("Report should mention isolated/singleton nodes")
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
