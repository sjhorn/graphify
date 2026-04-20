package csharp

import "testing"

func TestGetLanguage(t *testing.T) {
	lang := GetLanguage()
	if lang == nil {
		t.Fatal("GetLanguage() returned nil")
	}
}
