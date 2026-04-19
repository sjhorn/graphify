package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sjhorn/graphify/pkg/extract"
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

	outDir := t.TempDir()
	ext := &extract.Extraction{
		Nodes: []extract.Node{{ID: "n1", Label: "Node1"}},
		Edges: []extract.Edge{},
	}
	SaveCached(tmpFile, ext, outDir)

	loaded, ok := LoadCached(tmpFile, outDir)
	if !ok {
		t.Fatal("LoadCached() returned false")
	}
	if len(loaded.Nodes) != 1 {
		t.Errorf("Loaded %d nodes; want 1", len(loaded.Nodes))
	}
	if loaded.Nodes[0].Label != "Node1" {
		t.Errorf("Loaded label = %q; want Node1", loaded.Nodes[0].Label)
	}
}

func TestCacheMissOnChange(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "sample.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	outDir := t.TempDir()
	ext := &extract.Extraction{
		Nodes: []extract.Node{},
		Edges: []extract.Edge{{Source: "a", Target: "b", Relation: "calls"}},
	}
	SaveCached(tmpFile, ext, outDir)

	// Modify the file
	os.WriteFile(tmpFile, []byte("completely different content"), 0644)

	_, ok := LoadCached(tmpFile, outDir)
	if ok {
		t.Error("LoadCached() should return false after file changed")
	}
}

func TestCachedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	f1 := filepath.Join(tmpDir, "file1.py")
	f2 := filepath.Join(tmpDir, "file2.py")
	os.WriteFile(f1, []byte("alpha"), 0644)
	os.WriteFile(f2, []byte("beta"), 0644)

	SaveCached(f1, &extract.Extraction{}, outDir)
	SaveCached(f2, &extract.Extraction{}, outDir)

	hashes := CachedFiles(outDir)
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

	outDir := t.TempDir()
	SaveCached(tmpFile, &extract.Extraction{}, outDir)

	cacheDir := CacheDir(outDir)
	files, _ := os.ReadDir(cacheDir)
	if len(files) == 0 {
		t.Fatal("Expected cache files")
	}

	ClearCache(outDir)

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

func TestFileHashNonexistent(t *testing.T) {
	hash := FileHash("/nonexistent/path/file.txt")
	if hash != "" {
		t.Errorf("FileHash(nonexistent) = %q; want empty", hash)
	}
}

func TestCacheDirPath(t *testing.T) {
	dir := CacheDir("/home/user/project/graphify-out")
	expected := filepath.Join("/home/user/project/graphify-out", "cache")
	if dir != expected {
		t.Errorf("CacheDir() = %q; want %q", dir, expected)
	}
}

func TestLoadCachedMissingFile(t *testing.T) {
	_, ok := LoadCached("/nonexistent/file.py", "/tmp/nonexistent")
	if ok {
		t.Error("LoadCached() should return false for missing cache")
	}
}

func TestCachedFilesEmptyDir(t *testing.T) {
	result := CachedFiles("/nonexistent/root")
	if len(result) != 0 {
		t.Errorf("CachedFiles() = %d; want 0 for nonexistent dir", len(result))
	}
}

func TestClearCacheNonexistentDir(t *testing.T) {
	// Should not panic
	ClearCache("/nonexistent/root")
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

// Tests moved from extract package

func TestCacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "test.py")
	os.WriteFile(srcFile, []byte("class Foo: pass"), 0644)

	outDir := t.TempDir()
	ext := &extract.Extraction{
		Nodes: []extract.Node{{ID: "n1", Label: "Foo", Type: "class", File: "test.py"}},
		Edges: []extract.Edge{},
	}
	if err := SaveCached(srcFile, ext, outDir); err != nil {
		t.Fatalf("SaveCached() error: %v", err)
	}

	loaded, ok := LoadCached(srcFile, outDir)
	if !ok {
		t.Fatal("LoadCached() = false; want true")
	}
	if len(loaded.Nodes) != 1 {
		t.Errorf("LoadCached() nodes = %d; want 1", len(loaded.Nodes))
	}
	if loaded.Nodes[0].Label != "Foo" {
		t.Errorf("LoadCached() label = %q; want Foo", loaded.Nodes[0].Label)
	}
}

func TestCacheMiss(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "test.py")
	os.WriteFile(srcFile, []byte("class Foo: pass"), 0644)

	_, ok := LoadCached(srcFile, filepath.Join(tmpDir, "nonexistent"))
	if ok {
		t.Error("LoadCached() = true; want false (no cache)")
	}
}

func TestFileHashConsistentForContent(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(f, []byte("hello"), 0644)

	h1 := FileHash(f)
	h2 := FileHash(f)
	if h1 != h2 {
		t.Errorf("FileHash() not consistent: %s vs %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("FileHash() length = %d; want 64", len(h1))
	}
}

func TestFileHashDifferentContent(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "a.txt")
	f2 := filepath.Join(tmpDir, "b.txt")
	os.WriteFile(f1, []byte("alpha"), 0644)
	os.WriteFile(f2, []byte("beta"), 0644)

	if FileHash(f1) == FileHash(f2) {
		t.Error("FileHash() same for different content")
	}
}
