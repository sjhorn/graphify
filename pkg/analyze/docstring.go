package analyze

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhorn/graphify/pkg/graph"
)

// ExtractDocstrings reads source files for each god node and extracts the
// leading docstring (/// comments at the top of the file, or immediately
// above the class declaration). When the god node's file is a test file,
// also looks for the "contains" edge source file as fallback.
func ExtractDocstrings(details []GodNodeDetail, rootDir string, g *graph.Graph) {
	for i := range details {
		file := details[i].GodNode.File
		if file == "" {
			continue
		}

		// If the file looks like a test, try to find the actual source file
		// via the "contains" edge (file node → class node)
		sourceFile := file
		if strings.Contains(file, "test") || strings.Contains(file, "benchmark") ||
			strings.Contains(file, "example") {
			// Look for a contains edge from a non-test file
			for _, edge := range g.Edges() {
				if edge.Target == details[i].GodNode.ID && edge.Relation == "contains" {
					containerNode := g.GetNode(edge.Source)
					if containerNode != nil && containerNode.File != "" &&
						!strings.Contains(containerNode.File, "test") {
						sourceFile = containerNode.File
						break
					}
				}
			}
		}

		absPath := filepath.Join(rootDir, sourceFile)
		doc := extractFileDocstring(absPath, details[i].Label)
		if doc != "" {
			details[i].Docstring = doc
		}
	}
}

// extractFileDocstring reads a source file and extracts the most relevant
// docstring: first tries the file-level doc comment (top of file), then
// falls back to the comment immediately above the class declaration.
func extractFileDocstring(path string, classLabel string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
		if len(allLines) > 300 { // don't read huge files
			break
		}
	}

	// Strategy 1: file-level docstring (consecutive /// lines at start of file)
	fileDoc := extractLeadingDocComment(allLines)
	if fileDoc != "" {
		return fileDoc
	}

	// Strategy 2: docstring above the class declaration
	classDoc := extractClassDocComment(allLines, classLabel)
	if classDoc != "" {
		return classDoc
	}

	return ""
}

// extractLeadingDocComment extracts consecutive /// lines from the start of a file.
func extractLeadingDocComment(lines []string) string {
	var docLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "///") {
			// Strip the /// prefix and one optional space
			content := strings.TrimPrefix(trimmed, "///")
			if strings.HasPrefix(content, " ") {
				content = content[1:]
			}
			docLines = append(docLines, content)
		} else if trimmed == "" && len(docLines) > 0 {
			// Allow blank lines within the doc block only if we've started
			continue
		} else if len(docLines) > 0 {
			// Non-doc line after doc block — we're done
			break
		} else if trimmed == "" {
			// Skip leading blank lines
			continue
		} else {
			// Non-doc first line
			break
		}
	}

	if len(docLines) == 0 {
		return ""
	}

	// Trim trailing empty lines
	for len(docLines) > 0 && strings.TrimSpace(docLines[len(docLines)-1]) == "" {
		docLines = docLines[:len(docLines)-1]
	}

	// Cap at ~8 lines to keep the report concise
	if len(docLines) > 8 {
		docLines = docLines[:8]
	}

	return strings.Join(docLines, " ")
}

// extractClassDocComment finds the class declaration and extracts the /// block above it.
func extractClassDocComment(lines []string, classLabel string) string {
	// Clean the label (remove leading _ for private classes, handle .method() style)
	searchLabel := classLabel

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for "class ClassName" or "abstract class ClassName"
		if !strings.Contains(trimmed, "class "+searchLabel) &&
			!strings.Contains(trimmed, "mixin "+searchLabel) {
			continue
		}

		// Found the class — collect /// lines above it
		var docLines []string
		for j := i - 1; j >= 0; j-- {
			above := strings.TrimSpace(lines[j])
			if strings.HasPrefix(above, "///") {
				content := strings.TrimPrefix(above, "///")
				if strings.HasPrefix(content, " ") {
					content = content[1:]
				}
				docLines = append([]string{content}, docLines...)
			} else if above == "" {
				// skip blank lines between doc and class
				continue
			} else {
				break
			}
		}

		if len(docLines) > 0 {
			if len(docLines) > 8 {
				docLines = docLines[:8]
			}
			return strings.Join(docLines, " ")
		}
		break
	}

	return ""
}
