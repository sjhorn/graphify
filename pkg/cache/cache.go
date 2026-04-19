package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// FileHash returns SHA256 hash of file contents.
// For .md files, only the body (after YAML frontmatter) is hashed.
func FileHash(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// For markdown files, only hash the body
	if strings.HasSuffix(strings.ToLower(path), ".md") {
		content = bodyContent(content)
	}

	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// bodyContent strips YAML frontmatter from Markdown content.
func bodyContent(content []byte) []byte {
	text := string(content)
	if strings.HasPrefix(text, "---") {
		// Find end of frontmatter (search for closing ---)
		rest := text[4:] // Skip first "---"
		endIdx := strings.Index(rest, "\n---")
		if endIdx != -1 {
			// Return content after the closing "---"
			return []byte(rest[endIdx+4:])
		}
	}
	return content
}

// CacheDir returns the cache directory path.
func CacheDir(root string) string {
	return filepath.Join(root, "graphify-out", "cache")
}

// LoadCached returns cached extraction result if hash matches.
func LoadCached(path string, root string) map[string]interface{} {
	hash := FileHash(path)
	cachePath := filepath.Join(CacheDir(root), hash+".json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	return result
}

// SaveCached saves extraction result to cache.
func SaveCached(path string, result map[string]interface{}, root string) {
	hash := FileHash(path)
	cachePath := filepath.Join(CacheDir(root), hash+".json")

	// Ensure cache directory exists
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return
	}

	data, _ := json.Marshal(result)
	os.WriteFile(cachePath, data, 0644)
}

// CachedFiles returns set of file hashes that have valid cache entries.
func CachedFiles(root string) map[string]bool {
	cacheDir := CacheDir(root)
	result := make(map[string]bool)

	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return result
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			hash := strings.TrimSuffix(file.Name(), ".json")
			result[hash] = true
		}
	}

	return result
}

// ClearCache removes all cache files.
func ClearCache(root string) {
	cacheDir := CacheDir(root)

	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			os.Remove(filepath.Join(cacheDir, file.Name()))
		}
	}
}
