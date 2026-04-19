package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileHashConsistent(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "sample.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	h1 := FileHash(tmpFile)
	h2 := FileHash(tmpFile)

	if h1 != h2 {
		t.Errorf("FileHash() = %s, then %s; want same hash", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("FileHash() length = %d; want 64", len(h1))
	}
}

func TestFileHashChanges(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "a.txt")
	f2 := filepath.Join(tmpDir, "b.txt")
	os.WriteFile(f1, []byte("content one"), 0644)
	os.WriteFile(f2, []byte("content two"), 0644)

	if FileHash(f1) == FileHash(f2) {
		t.Error("FileHash() should be different for different content")
	}
}

func TestCacheRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "sample.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	result := map[string]interface{}{
		"nodes": []map[string]interface{}{{"id": "n1", "label": "Node1"}},
		"edges": []map[string]interface{}{},
	}
	SaveCached(tmpFile, result, tmpDir)

	loaded := LoadCached(tmpFile, tmpDir)
	if loaded == nil {
		t.Fatal("LoadCached() returned nil")
	}

	nodes, ok := loaded["nodes"].([]interface{})
	if !ok {
		t.Fatal("Loaded nodes should be a slice")
	}
	if len(nodes) != 1 {
		t.Errorf("Loaded %d nodes; want 1", len(nodes))
	}
}

func TestCacheMissOnChange(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "sample.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	result := map[string]interface{}{
		"nodes": []map[string]interface{}{},
		"edges": []map[string]interface{}{{"source": "a", "target": "b"}},
	}
	SaveCached(tmpFile, result, tmpDir)

	// Modify the file
	os.WriteFile(tmpFile, []byte("completely different content"), 0644)

	loaded := LoadCached(tmpFile, tmpDir)
	if loaded != nil {
		t.Error("LoadCached() should return nil after file changed")
	}
}

func TestCachedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	cacheRoot := t.TempDir()

	f1 := filepath.Join(tmpDir, "file1.py")
	f2 := filepath.Join(tmpDir, "file2.py")
	os.WriteFile(f1, []byte("alpha"), 0644)
	os.WriteFile(f2, []byte("beta"), 0644)

	SaveCached(f1, map[string]interface{}{"nodes": []interface{}{}, "edges": []interface{}{}}, cacheRoot)
	SaveCached(f2, map[string]interface{}{"nodes": []interface{}{}, "edges": []interface{}{}}, cacheRoot)

	hashes := CachedFiles(cacheRoot)
	h1 := FileHash(f1)
	h2 := FileHash(f2)

	if _, ok := hashes[h1]; !ok {
		t.Errorf("CachedFiles() missing hash for f1: %s", h1)
	}
	if _, ok := hashes[h2]; !ok {
		t.Errorf("CachedFiles() missing hash for f2: %s", h2)
	}
}

func TestClearCache(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "sample.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	cacheRoot := t.TempDir()
	SaveCached(tmpFile, map[string]interface{}{"nodes": []interface{}{}, "edges": []interface{}{}}, cacheRoot)

	cacheDir := filepath.Join(cacheRoot, "graphify-out", "cache")
	files, _ := os.ReadDir(cacheDir)
	if len(files) == 0 {
		t.Fatal("Expected cache files")
	}

	ClearCache(cacheRoot)

	files, _ = os.ReadDir(cacheDir)
	if len(files) != 0 {
		t.Errorf("ClearCache() left %d files; want 0", len(files))
	}
}

func TestBodyContentStripsFrontmatter(t *testing.T) {
	content := []byte("---\ntitle: Test\n---\n\nActual body.")
	expected := []byte("\n\nActual body.")
	result := bodyContent(content)

	if string(result) != string(expected) {
		t.Errorf("bodyContent() = %q; want %q", result, expected)
	}
}

func TestBodyContentNoFrontmatter(t *testing.T) {
	content := []byte("No frontmatter here.")
	result := bodyContent(content)

	if string(result) != string(content) {
		t.Errorf("bodyContent() = %q; want %q", result, content)
	}
}

func TestMDFileHashedFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "doc.md")
	os.WriteFile(f, []byte("---\nreviewed: 2026-01-01\n---\n\n# Title\n\nBody text."), 0644)
	h1 := FileHash(f)

	os.WriteFile(f, []byte("---\nreviewed: 2026-04-09\n---\n\n# Title\n\nBody text."), 0644)
	h2 := FileHash(f)

	if h1 != h2 {
		t.Error("FileHash() should be same when only frontmatter changes")
	}
}

func TestMDFileBodyChangeDifferentHash(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "doc.md")
	os.WriteFile(f, []byte("---\nreviewed: 2026-01-01\n---\n\n# Title\n\nOriginal body."), 0644)
	h1 := FileHash(f)

	os.WriteFile(f, []byte("---\nreviewed: 2026-01-01\n---\n\n# Title\n\nChanged body."), 0644)
	h2 := FileHash(f)

	if h1 == h2 {
		t.Error("FileHash() should be different when body changes")
	}
}
