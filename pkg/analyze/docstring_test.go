package analyze

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractLeadingDocComment(t *testing.T) {
	lines := []string{
		"/// This is a doc comment.",
		"/// It spans two lines.",
		"",
		"class Foo {",
	}
	result := extractLeadingDocComment(lines)
	if result != "This is a doc comment. It spans two lines." {
		t.Errorf("extractLeadingDocComment() = %q", result)
	}
}

func TestExtractLeadingDocCommentEmpty(t *testing.T) {
	lines := []string{
		"class Foo {",
		"  int x;",
	}
	result := extractLeadingDocComment(lines)
	if result != "" {
		t.Errorf("extractLeadingDocComment() = %q; want empty", result)
	}
}

func TestExtractLeadingDocCommentBlankLeading(t *testing.T) {
	lines := []string{
		"",
		"",
		"/// After blank lines.",
		"class Foo {",
	}
	result := extractLeadingDocComment(lines)
	if result != "After blank lines." {
		t.Errorf("extractLeadingDocComment() = %q", result)
	}
}

func TestExtractClassDocComment(t *testing.T) {
	lines := []string{
		"import 'dart:async';",
		"",
		"/// A widget that renders text.",
		"/// Supports multiple styles.",
		"class TextWidget extends Widget {",
		"  void build() {}",
		"}",
	}
	result := extractClassDocComment(lines, "TextWidget")
	if result != "A widget that renders text. Supports multiple styles." {
		t.Errorf("extractClassDocComment() = %q", result)
	}
}

func TestExtractClassDocCommentNotFound(t *testing.T) {
	lines := []string{
		"class OtherWidget {",
		"}",
	}
	result := extractClassDocComment(lines, "Missing")
	if result != "" {
		t.Errorf("extractClassDocComment() = %q; want empty", result)
	}
}

func TestExtractFileDocstring(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "widget.dart")
	os.WriteFile(path, []byte("/// A top-level doc.\n/// Second line.\nclass Widget {}\n"), 0644)

	result := extractFileDocstring(path, "Widget")
	if result != "A top-level doc. Second line." {
		t.Errorf("extractFileDocstring() = %q", result)
	}
}

func TestExtractFileDocstringNoFile(t *testing.T) {
	result := extractFileDocstring("/nonexistent/path.dart", "Foo")
	if result != "" {
		t.Errorf("extractFileDocstring() = %q; want empty for missing file", result)
	}
}

func TestExtractFileDocstringClassFallback(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "widget.dart")
	os.WriteFile(path, []byte("import 'dart:io';\n\n/// A widget doc.\nclass MyWidget {}\n"), 0644)

	result := extractFileDocstring(path, "MyWidget")
	if result != "A widget doc." {
		t.Errorf("extractFileDocstring() = %q", result)
	}
}
