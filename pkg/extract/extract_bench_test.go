package extract

import (
	"testing"
)

func BenchmarkExtractLua(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.lua"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractLua(fixture)
	}
}

func BenchmarkExtractR(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.R"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractR(fixture)
	}
}

func BenchmarkExtractHaskell(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.hs"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractHaskell(fixture)
	}
}

func BenchmarkExtractElm(b *testing.B) {
	fixture := "../../testdata/fixtures/Main.elm"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractElm(fixture)
	}
}

func BenchmarkExtractPython(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.py"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractPython(fixture)
	}
}

func BenchmarkExtractGo(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.go"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractGo(fixture)
	}
}

func BenchmarkExtractJavaScript(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.js"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractJavaScript(fixture)
	}
}

func BenchmarkExtractJava(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.java"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractJava(fixture)
	}
}

func BenchmarkExtractRuby(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.rb"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractRuby(fixture)
	}
}

func BenchmarkExtractRust(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.rs"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractRust(fixture)
	}
}

func BenchmarkExtractDart(b *testing.B) {
	fixture := "../../testdata/fixtures/sample.dart"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractDart(fixture)
	}
}
