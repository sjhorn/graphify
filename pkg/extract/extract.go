package extract

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Node represents a node in the knowledge graph.
type Node struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Type     string `json:"type,omitempty"`
	File     string `json:"source_file"`
	Location string `json:"source_location"`
}

// Edge represents a relationship between nodes.
type Edge struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Relation   string  `json:"relation"`
	Confidence string  `json:"confidence"`
	Weight     float64 `json:"weight"`
}

// Extraction represents the result of extracting structured data from a file.
type Extraction struct {
	Nodes        []Node `json:"nodes"`
	Edges        []Edge `json:"edges"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// MakeId creates a stable node ID from name parts.
func MakeId(parts ...string) string {
	combined := strings.Join(parts, "_")
	// Remove dots and leading/trailing underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	cleaned := re.ReplaceAllString(combined, "_")
	cleaned = strings.Trim(cleaned, "_")
	return strings.ToLower(cleaned)
}

// ExtractPython is in extract_python.go (tree-sitter based).

// GetNodeLabels returns the labels of all nodes in an extraction.
func GetNodeLabels(extraction *Extraction) []string {
	labels := make([]string, len(extraction.Nodes))
	for i, node := range extraction.Nodes {
		labels[i] = node.Label
	}
	return labels
}

// GetNodeIds returns the IDs of all nodes in an extraction.
func GetNodeIds(extraction *Extraction) []string {
	ids := make([]string, len(extraction.Nodes))
	for i, node := range extraction.Nodes {
		ids[i] = node.ID
	}
	return ids
}

// Extract runs extraction on multiple files.
func Extract(files []string, root string) *Extraction {
	var allNodes []Node
	var allEdges []Edge

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		var result *Extraction
		switch ext {
		case ".py":
			result = ExtractPython(file)
		case ".go":
			result = ExtractGo(file)
		case ".js", ".ts", ".tsx", ".jsx":
			result = ExtractJavaScript(file)
		case ".java":
			result = ExtractJava(file)
		case ".rb":
			result = ExtractRuby(file)
		case ".cs":
			result = ExtractCSharp(file)
		case ".php":
			result = ExtractPHP(file)
		case ".swift":
			result = ExtractSwift(file)
		case ".kt", ".kts":
			result = ExtractKotlin(file)
		case ".dart":
			result = ExtractDart(file)
		case ".scala":
			result = ExtractScala(file)
		case ".rs":
			result = ExtractRust(file)
		case ".ex", ".exs":
			result = ExtractElixir(file)
		case ".zig":
			result = ExtractZig(file)
		case ".jl":
			result = ExtractJulia(file)
		case ".lua":
			result = ExtractLua(file)
		case ".c", ".h":
			result = ExtractC(file)
		case ".cpp", ".cc", ".cxx", ".hpp":
			result = ExtractCpp(file)
		case ".ps1":
			result = ExtractPowerShell(file)
		case ".m", ".mm":
			result = ExtractObjectiveC(file)
		case ".r", ".R":
			result = ExtractR(file)
		case ".hs":
			result = ExtractHaskell(file)
		case ".elm":
			result = ExtractElm(file)
		default:
			continue
		}
		allNodes = append(allNodes, result.Nodes...)
		allEdges = append(allEdges, result.Edges...)
	}

	result := &Extraction{
		Nodes: allNodes,
		Edges: allEdges,
	}
	inferNodeTypes(result)
	return result
}

// inferNodeTypes fills in missing Node.Type fields using edge relations and label patterns.
func inferNodeTypes(ext *Extraction) {
	// Build edge-based type hints: target of "method" edge → method, target of "contains" from file → class/function, etc.
	typeHints := make(map[string]string)
	for _, e := range ext.Edges {
		switch e.Relation {
		case "method":
			typeHints[e.Target] = "method"
		case "case_of":
			typeHints[e.Target] = "enum_value"
		case "inherits":
			if _, ok := typeHints[e.Target]; !ok {
				typeHints[e.Target] = "class"
			}
		case "imports":
			if _, ok := typeHints[e.Target]; !ok {
				typeHints[e.Target] = "module"
			}
		}
	}

	for i := range ext.Nodes {
		if ext.Nodes[i].Type != "" {
			continue
		}
		// Check edge hints
		if t, ok := typeHints[ext.Nodes[i].ID]; ok {
			ext.Nodes[i].Type = t
			continue
		}
		// Label-based fallback
		label := ext.Nodes[i].Label
		file := ext.Nodes[i].File
		if file != "" && label == filepath.Base(file) {
			ext.Nodes[i].Type = "file"
		} else if strings.HasPrefix(label, ".") && strings.HasSuffix(label, "()") {
			ext.Nodes[i].Type = "method"
		} else if strings.HasSuffix(label, "()") {
			ext.Nodes[i].Type = "function"
		} else if len(label) > 0 && label[0] >= 'A' && label[0] <= 'Z' {
			ext.Nodes[i].Type = "class"
		}
	}
}

// CollectFiles collects all source files from a directory.
func CollectFiles(root string) []string {
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		supportedExts := map[string]bool{
			".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
			".go": true, ".java": true, ".c": true, ".cpp": true, ".cc": true,
			".cxx": true, ".cs": true, ".rb": true, ".kt": true, ".kts": true,
			".scala": true, ".php": true, ".swift": true, ".dart": true,
			".rs": true, ".lua": true, ".zig": true, ".ps1": true,
			".ex": true, ".exs": true, ".m": true, ".mm": true, ".jl": true,
		}
		if supportedExts[ext] {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// SortNodes sorts nodes by ID for deterministic output.
func SortNodes(nodes []Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
}
