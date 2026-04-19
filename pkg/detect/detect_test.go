package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyPython(t *testing.T) {
	result := ClassifyFile("foo.py")
	if result == nil || *result != FileTypeCode {
		t.Errorf("ClassifyFile(foo.py) = %v; want %v", result, FileTypeCode)
	}
}

func TestClassifyTypeScript(t *testing.T) {
	result := ClassifyFile("bar.ts")
	if result == nil || *result != FileTypeCode {
		t.Errorf("ClassifyFile(bar.ts) = %v; want %v", result, FileTypeCode)
	}
}

func TestClassifyMarkdown(t *testing.T) {
	result := ClassifyFile("README.md")
	if result == nil || *result != FileTypeDocument {
		t.Errorf("ClassifyFile(README.md) = %v; want %v", result, FileTypeDocument)
	}
}

func TestClassifyPDF(t *testing.T) {
	result := ClassifyFile("paper.pdf")
	if result == nil || *result != FileTypePaper {
		t.Errorf("ClassifyFile(paper.pdf) = %v; want %v", result, FileTypePaper)
	}
}

func TestClassifyPDFInXCassetsSkipped(t *testing.T) {
	// PDFs inside Xcode asset catalogs are vector icons, not papers
	assetPDF := "MyApp/Images.xcassets/icon.imageset/icon.pdf"
	result := ClassifyFile(assetPDF)
	if result != nil {
		t.Errorf("ClassifyFile(%s) = %v; want nil", assetPDF, result)
	}
}

func TestClassifyPDFInXCassetsRootSkipped(t *testing.T) {
	assetPDF := "Pods/HXPHPicker/Assets.xcassets/photo.pdf"
	result := ClassifyFile(assetPDF)
	if result != nil {
		t.Errorf("ClassifyFile(%s) = %v; want nil", assetPDF, result)
	}
}

func TestClassifyUnknownReturnsNone(t *testing.T) {
	result := ClassifyFile("archive.zip")
	if result != nil {
		t.Errorf("ClassifyFile(archive.zip) = %v; want nil", result)
	}
}

func TestClassifyImage(t *testing.T) {
	images := []string{"screenshot.png", "design.jpg", "diagram.webp"}
	for _, img := range images {
		result := ClassifyFile(img)
		if result == nil || *result != FileTypeImage {
			t.Errorf("ClassifyFile(%s) = %v; want %v", img, result, FileTypeImage)
		}
	}
}

func TestCountWordsSampleMd(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	words := CountWords(filepath.Join(fixturesDir, "sample.md"))
	if words <= 5 {
		t.Errorf("CountWords(sample.md) = %d; want > 5", words)
	}
}

func TestDetectFindsFixtures(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := CollectFiles(fixturesDir)

	if result.TotalFiles < 2 {
		t.Errorf("CollectFiles() total_files = %d; want >= 2", result.TotalFiles)
	}
	if _, ok := result.Files[FileTypeCode]; !ok {
		t.Error("CollectFiles() should have 'code' key")
	}
	if _, ok := result.Files[FileTypeDocument]; !ok {
		t.Error("CollectFiles() should have 'document' key")
	}
}

func TestDetectWarnsSmallCorpus(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := CollectFiles(fixturesDir)

	if result.NeedsGraph {
		t.Error("CollectFiles() needs_graph = true; want false for small corpus")
	}
	if result.Warning == "" {
		t.Error("CollectFiles() warning = ''; want warning for small corpus")
	}
}

func TestDetectSkipsDotfiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a hidden directory with a file
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "secret.py"), []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a normal file
	if err := os.WriteFile(filepath.Join(tmpDir, "normal.py"), []byte("y = 2"), 0644); err != nil {
		t.Fatal(err)
	}

	result := CollectFiles(tmpDir)
	codeFiles := result.Files[FileTypeCode]

	// Check that hidden file is excluded
	for _, f := range codeFiles {
		if filepath.Base(f) == "secret.py" {
			t.Errorf("CollectFiles() included hidden file: %s", f)
		}
	}

	// Check that normal file is included
	found := false
	for _, f := range codeFiles {
		if filepath.Base(f) == "normal.py" {
			found = true
			break
		}
	}
	if !found {
		t.Error("normal.py should be included")
	}
}

func TestClassifyMdPaperBySignals(t *testing.T) {
	tmpDir := t.TempDir()
	paper := filepath.Join(tmpDir, "paper.md")
	content := "# Abstract\n\nWe propose a new method. See [1] and [23].\nThis work was published in the Journal of AI. ArXiv preprint.\nSee Equation 3 for details. \\cite{vaswani2017}.\n"
	if err := os.WriteFile(paper, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := ClassifyFile(paper)
	if result == nil || *result != FileTypePaper {
		t.Errorf("ClassifyFile(paper.md with signals) = %v; want %v", result, FileTypePaper)
	}
}

func TestClassifyMdDocWithoutSignals(t *testing.T) {
	tmpDir := t.TempDir()
	doc := filepath.Join(tmpDir, "notes.md")
	content := "# My Notes\n\nHere are some notes about the project.\n"
	if err := os.WriteFile(doc, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := ClassifyFile(doc)
	if result == nil || *result != FileTypeDocument {
		t.Errorf("ClassifyFile(notes.md without signals) = %v; want %v", result, FileTypeDocument)
	}
}

func TestGraphifyignoreExcludesFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .graphifyignore
	ignoreContent := "vendor/\n*.generated.py\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".graphifyignore"), []byte(ignoreContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create vendor/lib.py
	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "lib.py"), []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create main.py
	if err := os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte("print('hi')"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create schema.generated.py
	if err := os.WriteFile(filepath.Join(tmpDir, "schema.generated.py"), []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	result := CollectFiles(tmpDir)
	fileList := result.Files[FileTypeCode]

	found := false
	for _, f := range fileList {
		if filepath.Base(f) == "main.py" {
			found = true
			break
		}
	}
	if !found {
		t.Error("main.py should be included")
	}

	// Check vendor is excluded
	for _, f := range fileList {
		if filepath.Base(f) == "lib.py" {
			t.Error("vendor/lib.py should be excluded")
		}
		if filepath.Base(f) == "schema.generated.py" {
			t.Error("schema.generated.py should be excluded")
		}
	}
}

func TestGraphifyignoreMissingIsFine(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	result := CollectFiles(tmpDir)
	if result.GraphifyignorePatterns != 0 {
		t.Errorf("CollectFiles() graphifyignore_patterns = %d; want 0", result.GraphifyignorePatterns)
	}
}

func TestGraphifyignoreCommentsIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	ignoreContent := "# this is a comment\n\nmain.py\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".graphifyignore"), []byte(ignoreContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "other.py"), []byte("x = 2"), 0644); err != nil {
		t.Fatal(err)
	}

	result := CollectFiles(tmpDir)
	codeFiles := result.Files[FileTypeCode]

	// main.py should be excluded (listed in ignore)
	// other.py should be included
	for _, f := range codeFiles {
		if filepath.Base(f) == "main.py" {
			t.Error("main.py should be excluded due to .graphifyignore")
		}
	}

	found := false
	for _, f := range codeFiles {
		if filepath.Base(f) == "other.py" {
			found = true
			break
		}
	}
	if !found {
		t.Error("other.py should be included")
	}
}

func TestDetectFollowsSymlinkedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real_lib")
	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "util.py"), []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(tmpDir, "linked_lib")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}

	result := CollectFiles(tmpDir)
	codeFiles := result.Files[FileTypeCode]

	found := false
	for _, f := range codeFiles {
		if filepath.Base(f) == "util.py" {
			found = true
			break
		}
	}
	if !found {
		t.Error("util.py should be found")
	}
}

func TestDetectHandlesCircularSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "a")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "main.py"), []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	loopLink := filepath.Join(subDir, "loop")
	if err := os.Symlink(tmpDir, loopLink); err != nil {
		t.Fatal(err)
	}

	result := CollectFiles(tmpDir)
	codeFiles := result.Files[FileTypeCode]

	found := false
	for _, f := range codeFiles {
		if filepath.Base(f) == "main.py" {
			found = true
			break
		}
	}
	if !found {
		t.Error("main.py should be found despite circular symlink")
	}
}

func TestClassifyVideoExtensions(t *testing.T) {
	videos := []string{"lecture.mp4", "podcast.mp3", "talk.mov", "recording.wav", "webinar.webm", "audio.m4a"}
	for _, video := range videos {
		result := ClassifyFile(video)
		if result == nil || *result != FileTypeVideo {
			t.Errorf("ClassifyFile(%s) = %v; want %v", video, result, FileTypeVideo)
		}
	}
}

func TestDetectIncludesVideoKey(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	result := CollectFiles(tmpDir)
	if _, ok := result.Files[FileTypeVideo]; !ok {
		t.Error("CollectFiles() should always include 'video' key")
	}
}

func TestDetectFindsVideoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "lecture.mp4"), []byte("fake video data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "notes.md"), []byte("# Notes\nSome content here."), 0644); err != nil {
		t.Fatal(err)
	}

	result := CollectFiles(tmpDir)
	videoFiles := result.Files[FileTypeVideo]

	if len(videoFiles) != 1 {
		t.Errorf("CollectFiles() video files = %d; want 1", len(videoFiles))
	}

	found := false
	for _, f := range videoFiles {
		if filepath.Base(f) == "lecture.mp4" {
			found = true
			break
		}
	}
	if !found {
		t.Error("lecture.mp4 should be found")
	}
}

func TestDetectVideoNotInWords(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "clip.mp4"), []byte("\x00\x00\x00\x00"), 0644); err != nil {
		t.Fatal(err)
	}

	result := CollectFiles(tmpDir)
	// Video files should not contribute to total_words
	if result.TotalWords != 0 {
		t.Errorf("CollectFiles() total_words = %d; want 0 (video has no readable text)", result.TotalWords)
	}
}
