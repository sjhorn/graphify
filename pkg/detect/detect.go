package detect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileType represents the classification of a file.
type FileType string

const (
	FileTypeCode     FileType = "code"
	FileTypeDocument FileType = "document"
	FileTypePaper    FileType = "paper"
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
)

// DetectResult contains the results of file detection.
type DetectResult struct {
	Files                map[FileType][]string `json:"files"`
	TotalFiles           int                   `json:"total_files"`
	TotalWords           int                   `json:"total_words"`
	NeedsGraph           bool                  `json:"needs_graph"`
	Warning              string                `json:"warning"`
	GraphifyignorePatterns int                 `json:"graphifyignore_patterns"`
}

// Corpus thresholds
const (
	CorpusWarnThreshold  = 50000   // words - below this, warn "you may not need a graph"
	CorpusUpperThreshold = 500000  // words - above this, warn about token cost
	FileCountUpper       = 200     // files - above this, warn about token cost
)

// CodeExtensions defines file extensions for code files.
var CodeExtensions = map[string]bool{
	".py": true, ".ts": true, ".js": true, ".jsx": true, ".tsx": true,
	".go": true, ".rs": true, ".java": true, ".cpp": true, ".cc": true,
	".cxx": true, ".c": true, ".h": true, ".hpp": true, ".rb": true,
	".swift": true, ".kt": true, ".kts": true, ".cs": true, ".scala": true,
	".php": true, ".lua": true, ".toc": true, ".zig": true, ".ps1": true,
	".ex": true, ".exs": true, ".m": true, ".mm": true, ".jl": true,
	".dart": true, ".vue": true, ".svelte": true,
}

// DocExtensions defines file extensions for document files.
var DocExtensions = map[string]bool{
	".md": true, ".txt": true, ".rst": true,
}

// PaperExtensions defines file extensions for paper files.
var PaperExtensions = map[string]bool{
	".pdf": true,
}

// ImageExtensions defines file extensions for image files.
var ImageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".svg": true,
}

// VideoExtensions defines file extensions for video and audio files.
var VideoExtensions = map[string]bool{
	".mp4": true, ".mov": true, ".webm": true, ".mkv": true,
	".avi": true, ".m4v": true, ".mp3": true, ".wav": true, ".m4a": true, ".ogg": true,
}

// OfficeExtensions defines file extensions for office documents.
var OfficeExtensions = map[string]bool{
	".docx": true, ".xlsx": true,
}

// AssetDirMarkers defines directory names that indicate asset directories.
var AssetDirMarkers = map[string]bool{
	".imageset": true, ".xcassets": true, ".appiconset": true,
	".colorset": true, ".launchimage": true,
}

// ClassifyFile classifies a file based on its extension and path.
func ClassifyFile(path string) *FileType {
	// Check for PDFs inside Xcode asset catalogs (vector icons, not papers)
	// Use both separators since paths may come from different sources
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if isAssetDirMarker(part) {
			return nil
		}
	}
	// Also check with backslash separator (Windows)
	parts = strings.Split(path, "\\")
	for _, part := range parts {
		if isAssetDirMarker(part) {
			return nil
		}
	}

	ext := strings.ToLower(filepath.Ext(path))

	if CodeExtensions[ext] {
		ft := FileTypeCode
		return &ft
	}
	if PaperExtensions[ext] {
		ft := FileTypePaper
		return &ft
	}
	if ImageExtensions[ext] {
		ft := FileTypeImage
		return &ft
	}
	if DocExtensions[ext] {
		// Check if it looks like a paper
		if looksLikePaper(path) {
			ft := FileTypePaper
			return &ft
		}
		ft := FileTypeDocument
		return &ft
	}
	if VideoExtensions[ext] {
		ft := FileTypeVideo
		return &ft
	}

	return nil
}

// isAssetDirMarker checks if a path component is an Xcode asset directory.
func isAssetDirMarker(part string) bool {
	// Check if the part ends with any of the markers
	for marker := range AssetDirMarkers {
		if strings.HasSuffix(part, marker) {
			return true
		}
	}
	return false
}

// CountWords counts words in a file.
func CountWords(path string) int {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return len(strings.Fields(string(content)))
}

// _isSensitive checks if a file likely contains secrets.
func isSensitive(path string) bool {
	name := filepath.Base(path)
	sensitivePatterns := []string{
		".env", ".envrc", "*.pem", "*.key", "*.p12", "*.pfx",
		"*.cert", "*.crt", "*.der", "*.p8", "*credential*", "*secret*",
		"*passwd*", "*password*", "*token*", "*private_key*",
		"id_rsa", "*id_dsa*", "*id_ecdsa*", "*id_ed25519*",
		".netrc", ".pgpass", ".htpasswd", "*aws_credentials*",
		"*gcloud_credentials*", "*service.account*",
	}

	for _, pattern := range sensitivePatterns {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
	}
	return false
}

// _looksLikePaper checks if a text file reads like an academic paper.
func looksLikePaper(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	// Only scan first 3000 chars for speed
	if len(content) > 3000 {
		content = content[:3000]
	}
	text := string(content)

	paperSignals := []string{
		"arxiv", "doi:", "abstract", "proceedings", "journal",
		"preprint", "\\cite{", "eq.", "equation", "we propose", "literature",
	}

	hits := 0
	for _, signal := range paperSignals {
		if strings.Contains(strings.ToLower(text), signal) {
			hits++
		}
	}

	// Check for numbered citations [1], [23]
	if strings.Contains(text, "[1]") || strings.Contains(text, "[23]") {
		hits++
	}

	return hits >= 3
}

// _loadGraphifyignore reads .graphifyignore patterns from root and ancestor directories.
func loadGraphifyignore(root string) []string {
	var patterns []string
	current, err := filepath.Abs(root)
	if err != nil {
		return patterns
	}

	for {
		ignoreFile := filepath.Join(current, ".graphifyignore")
		if _, err := os.Stat(ignoreFile); err == nil {
			content, err := os.ReadFile(ignoreFile)
			if err == nil {
				lines := strings.Split(string(content), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && !strings.HasPrefix(line, "#") {
						patterns = append(patterns, line)
					}
				}
			}
		}

		// Stop at git repo root
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			break
		}

		parent := filepath.Dir(current)
		if parent == current {
			break // filesystem root
		}
		current = parent
	}

	return patterns
}

// _isIgnored checks if a path matches any .graphifyignore pattern.
func isIgnored(path string, root string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)

	for _, pattern := range patterns {
		// Strip trailing slash from pattern
		p := strings.TrimSuffix(pattern, "/")

		// Match against filename
		matched, _ := filepath.Match(p, filepath.Base(path))
		if matched {
			return true
		}

		// Match against relative path
		matched, _ = filepath.Match(p, rel)
		if matched {
			return true
		}

		// Check if pattern matches any directory component
		if strings.Contains(rel, "/"+p+"/") || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}

	return false
}

// CollectFiles walks a directory tree and classifies all files.
func CollectFiles(root string) *DetectResult {
	files := map[FileType][]string{
		FileTypeCode:     {},
		FileTypeDocument: {},
		FileTypePaper:    {},
		FileTypeImage:    {},
		FileTypeVideo:    {},
	}

	var totalWords int
	var skipped []string

	patterns := loadGraphifyignore(root)

	// Walk directory tree
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip symlinks for now (can be added later)
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		if info.IsDir() {
			// Skip noise directories
			if isNoiseDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be ignored
		if isIgnored(path, root, patterns) {
			return nil
		}

		// Check for sensitive files
		if isSensitive(path) {
			skipped = append(skipped, path)
			return nil
		}

		// Classify file
		ft := ClassifyFile(path)
		if ft != nil {
			files[*ft] = append(files[*ft], path)

			// Count words (not for video files)
			if *ft != FileTypeVideo {
				totalWords += CountWords(path)
			}
		}

		return nil
	})

	if err != nil {
		// Return partial results on error
	}

	// Sort file lists for deterministic output
	for ft := range files {
		sort.Strings(files[ft])
	}

	totalFiles := 0
	for _, flist := range files {
		totalFiles += len(flist)
	}

	// Determine if graph is needed
	needsGraph := totalWords >= CorpusWarnThreshold

	// Generate warning
	var warning string
	if !needsGraph {
		warning = "Corpus is small, you may not need a graph."
	} else if totalWords >= CorpusUpperThreshold || totalFiles >= FileCountUpper {
		warning = "Large corpus: semantic extraction will be expensive."
	}

	return &DetectResult{
		Files:                files,
		TotalFiles:           totalFiles,
		TotalWords:           totalWords,
		NeedsGraph:           needsGraph,
		Warning:              warning,
		GraphifyignorePatterns: len(patterns),
	}
}

// isNoiseDir checks if a directory name indicates it should be skipped.
func isNoiseDir(name string) bool {
	noiseDirs := map[string]bool{
		"venv": true, ".venv": true, "env": true, ".env": true,
		"node_modules": true, "__pycache__": true, ".git": true,
		"dist": true, "build": true, "target": true, "out": true,
		"site-packages": true, "lib64": true,
		".pytest_cache": true, ".mypy_cache": true, ".ruff_cache": true,
		".tox": true, ".eggs": true,
	}
	return noiseDirs[name]
}
