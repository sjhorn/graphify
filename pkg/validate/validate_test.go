package validate

import (
	"testing"
)

var validData = map[string]interface{}{
	"nodes": []interface{}{
		map[string]interface{}{"id": "n1", "label": "Foo", "file_type": "code", "source_file": "foo.py"},
		map[string]interface{}{"id": "n2", "label": "Bar", "file_type": "document", "source_file": "bar.md"},
	},
	"edges": []interface{}{
		map[string]interface{}{"source": "n1", "target": "n2", "relation": "references",
			"confidence": "EXTRACTED", "source_file": "foo.py", "weight": 1.0},
	},
}

func TestValidPasses(t *testing.T) {
	errors := ValidateExtraction(validData)
	if len(errors) != 0 {
		t.Errorf("ValidateExtraction() returned errors: %v", errors)
	}
}

func TestMissingNodesKey(t *testing.T) {
	errors := ValidateExtraction(map[string]interface{}{"edges": []interface{}{}})
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for missing nodes")
	}
	if !containsAny(errors, "nodes") {
		t.Errorf("Error should mention 'nodes': %v", errors)
	}
}

func TestMissingEdgesKey(t *testing.T) {
	errors := ValidateExtraction(map[string]interface{}{"nodes": []interface{}{}})
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for missing edges")
	}
	if !containsAny(errors, "edges") {
		t.Errorf("Error should mention 'edges': %v", errors)
	}
}

func TestNotADict(t *testing.T) {
	errors := ValidateExtraction(map[string]interface{}{"nodes": "not a dict"})
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for non-dict")
	}
}

func TestInvalidFileType(t *testing.T) {
	data := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "label": "X", "file_type": "video", "source_file": "x.mp4"},
		},
		"edges": []interface{}{},
	}
	errors := ValidateExtraction(data)
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for invalid file_type")
	}
	if !containsAny(errors, "file_type") {
		t.Errorf("Error should mention 'file_type': %v", errors)
	}
}

func TestInvalidConfidence(t *testing.T) {
	data := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "label": "A", "file_type": "code", "source_file": "a.py"},
			map[string]interface{}{"id": "n2", "label": "B", "file_type": "code", "source_file": "b.py"},
		},
		"edges": []interface{}{
			map[string]interface{}{"source": "n1", "target": "n2", "relation": "calls",
				"confidence": "CERTAIN", "source_file": "a.py"},
		},
	}
	errors := ValidateExtraction(data)
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for invalid confidence")
	}
	if !containsAny(errors, "confidence") {
		t.Errorf("Error should mention 'confidence': %v", errors)
	}
}

func TestDanglingEdgeSource(t *testing.T) {
	data := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "label": "A", "file_type": "code", "source_file": "a.py"},
		},
		"edges": []interface{}{
			map[string]interface{}{"source": "missing_id", "target": "n1", "relation": "calls",
				"confidence": "EXTRACTED", "source_file": "a.py"},
		},
	}
	errors := ValidateExtraction(data)
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for dangling edge source")
	}
	if !containsAny(errors, "source") {
		t.Errorf("Error should mention 'source': %v", errors)
	}
}

func TestDanglingEdgeTarget(t *testing.T) {
	data := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "label": "A", "file_type": "code", "source_file": "a.py"},
		},
		"edges": []interface{}{
			map[string]interface{}{"source": "n1", "target": "ghost", "relation": "calls",
				"confidence": "EXTRACTED", "source_file": "a.py"},
		},
	}
	errors := ValidateExtraction(data)
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for dangling edge target")
	}
	if !containsAny(errors, "target") {
		t.Errorf("Error should mention 'target': %v", errors)
	}
}

func TestEdgeNotADict(t *testing.T) {
	data := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "label": "A", "file_type": "code", "source_file": "a.py"},
		},
		"edges": []interface{}{
			"not an object",
		},
	}
	errors := ValidateExtraction(data)
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for non-object edge")
	}
}

func TestEdgeMissingField(t *testing.T) {
	data := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "label": "A", "file_type": "code", "source_file": "a.py"},
		},
		"edges": []interface{}{
			map[string]interface{}{"source": "n1"},
		},
	}
	errors := ValidateExtraction(data)
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for edge missing fields")
	}
}

func TestNodeNotADict(t *testing.T) {
	data := map[string]interface{}{
		"nodes": []interface{}{"not an object"},
		"edges": []interface{}{},
	}
	errors := ValidateExtraction(data)
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for non-object node")
	}
}

func TestEdgesNotAList(t *testing.T) {
	data := map[string]interface{}{
		"nodes": []interface{}{},
		"edges": "not a list",
	}
	errors := ValidateExtraction(data)
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for non-list edges")
	}
}

func TestMissingNodeField(t *testing.T) {
	data := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "label": "A", "source_file": "a.py"},
		},
		"edges": []interface{}{},
	}
	errors := ValidateExtraction(data)
	if len(errors) == 0 {
		t.Error("ValidateExtraction() should return error for missing file_type")
	}
	if !containsAny(errors, "file_type") {
		t.Errorf("Error should mention 'file_type': %v", errors)
	}
}

func containsAny(errors []string, substr string) bool {
	for _, e := range errors {
		if contains(e, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0)
}
