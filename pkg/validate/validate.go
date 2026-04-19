package validate

import (
	"fmt"
)

var validFileTypes = map[string]bool{
	"code": true, "document": true, "paper": true, "image": true, "rationale": true,
}

var validConfidences = map[string]bool{
	"EXTRACTED": true, "INFERRED": true, "AMBIGUOUS": true,
}

var requiredNodeFields = []string{"id", "label", "file_type", "source_file"}
var requiredEdgeFields = []string{"source", "target", "relation", "confidence", "source_file"}

// ValidateExtraction validates an extraction map against the graphify schema.
// Returns a list of error strings - empty slice means valid.
func ValidateExtraction(data map[string]interface{}) []string {
	var errors []string

	// Nodes
	nodesVal, hasNodes := data["nodes"]
	if !hasNodes {
		errors = append(errors, "Missing required key 'nodes'")
	} else {
		nodes, ok := nodesVal.([]interface{})
		if !ok {
			errors = append(errors, "'nodes' must be a list")
		} else {
			for i, nodeVal := range nodes {
				node, ok := nodeVal.(map[string]interface{})
				if !ok {
					errors = append(errors, fmt.Sprintf("Node %d must be an object", i))
					continue
				}
				nodeID := "?"
				if idVal, ok := node["id"]; ok {
					if idStr, ok := idVal.(string); ok {
						nodeID = idStr
					}
				}
				for _, field := range requiredNodeFields {
					if _, exists := node[field]; !exists {
						errors = append(errors, fmt.Sprintf("Node %d (id=%q) missing required field '%s'", i, nodeID, field))
					}
				}
				if fileTypeVal, ok := node["file_type"]; ok {
					if fileType, ok := fileTypeVal.(string); ok && !validFileTypes[fileType] {
						errors = append(errors, fmt.Sprintf("Node %d (id=%q) has invalid file_type '%s'", i, nodeID, fileType))
					}
				}
			}
		}
	}

	// Edges
	edgesVal, hasEdges := data["edges"]
	if !hasEdges {
		errors = append(errors, "Missing required key 'edges'")
	} else {
		edges, ok := edgesVal.([]interface{})
		if !ok {
			errors = append(errors, "'edges' must be a list")
		} else {
			// Build set of node IDs
			nodeIDs := make(map[string]bool)
			if nodes, ok := nodesVal.([]interface{}); ok {
				for _, nodeVal := range nodes {
					if node, ok := nodeVal.(map[string]interface{}); ok {
						if idVal, ok := node["id"]; ok {
							if idStr, ok := idVal.(string); ok {
								nodeIDs[idStr] = true
							}
						}
					}
				}
			}

			for i, edgeVal := range edges {
				edge, ok := edgeVal.(map[string]interface{})
				if !ok {
					errors = append(errors, fmt.Sprintf("Edge %d must be an object", i))
					continue
				}
				for _, field := range requiredEdgeFields {
					if _, exists := edge[field]; !exists {
						errors = append(errors, fmt.Sprintf("Edge %d missing required field '%s'", i, field))
					}
				}
				if confVal, ok := edge["confidence"]; ok {
					if confidence, ok := confVal.(string); ok && !validConfidences[confidence] {
						errors = append(errors, fmt.Sprintf("Edge %d has invalid confidence '%s'", i, confidence))
					}
				}
				if srcVal, ok := edge["source"]; ok {
					if src, ok := srcVal.(string); ok && len(nodeIDs) > 0 && !nodeIDs[src] {
						errors = append(errors, fmt.Sprintf("Edge %d source '%s' does not match any node id", i, src))
					}
				}
				if tgtVal, ok := edge["target"]; ok {
					if tgt, ok := tgtVal.(string); ok && len(nodeIDs) > 0 && !nodeIDs[tgt] {
						errors = append(errors, fmt.Sprintf("Edge %d target '%s' does not match any node id", i, tgt))
					}
				}
			}
		}
	}

	return errors
}
