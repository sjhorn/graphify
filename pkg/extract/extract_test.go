package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMakeIdStripsDotsAndUnderscores(t *testing.T) {
	if MakeId("_auth") != "auth" {
		t.Errorf("MakeId(\"_auth\") = %s; want \"auth\"", MakeId("_auth"))
	}
	if MakeId(".httpx._client") != "httpx_client" {
		t.Errorf("MakeId(\".httpx._client\") = %s; want \"httpx_client\"", MakeId(".httpx._client"))
	}
}

func TestMakeIdConsistent(t *testing.T) {
	if MakeId("foo", "Bar") != MakeId("foo", "Bar") {
		t.Error("MakeId should be consistent for same input")
	}
}

func TestMakeIdNoLeadingTrailingUnderscores(t *testing.T) {
	result := MakeId("__init__")
	if len(result) == 0 {
		t.Error("MakeId(\"__init__\") should not be empty")
	}
	if result[0] == '_' || result[len(result)-1] == '_' {
		t.Errorf("MakeId(\"__init__\") = %s; should not start/end with underscore", result)
	}
}

func TestExtractPythonFindsClass(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPython(filepath.Join(fixturesDir, "sample.py"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == "Transformer" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPython() labels = %v; want Transformer", labels)
	}
}

func TestExtractPythonFindsMethods(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPython(filepath.Join(fixturesDir, "sample.py"))
	labels := GetNodeLabels(result)
	found := false
	for _, label := range labels {
		if label == ".__init__()" || label == ".forward()" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExtractPython() labels = %v; want .__init__() or .forward()", labels)
	}
}

func TestExtractPythonNoDanglingEdges(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPython(filepath.Join(fixturesDir, "sample.py"))
	nodeIds := GetNodeIds(result)
	for _, edge := range result.Edges {
		found := false
		for _, id := range nodeIds {
			if id == edge.Source {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Dangling edge source: %s", edge.Source)
		}
	}
}

func TestStructuralEdgesAreExtracted(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPython(filepath.Join(fixturesDir, "sample.py"))
	structural := map[string]bool{
		"contains": true, "method": true, "inherits": true,
		"imports": true, "imports_from": true,
	}
	for _, edge := range result.Edges {
		if structural[edge.Relation] {
			if edge.Confidence != "EXTRACTED" {
				t.Errorf("Structural edge %s should be EXTRACTED, got %s", edge.Relation, edge.Confidence)
			}
		}
	}
}

func TestExtractMergesMultipleFiles(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	files := []string{filepath.Join(fixturesDir, "sample.py")}
	result := Extract(files, fixturesDir)
	if len(result.Nodes) == 0 {
		t.Error("Extract() should return nodes")
	}
}

func TestCollectFilesFromDir(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	files := CollectFiles(fixturesDir)
	if len(files) == 0 {
		t.Error("CollectFiles() should return files")
	}
}

func TestCollectFilesSkipsHidden(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	files := CollectFiles(fixturesDir)
	for _, f := range files {
		if filepath.Base(f)[0] == '.' {
			t.Errorf("CollectFiles() included hidden file: %s", f)
		}
	}
}

func TestSortNodes(t *testing.T) {
	nodes := []Node{
		{ID: "c", Label: "C"},
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B"},
	}
	SortNodes(nodes)
	if nodes[0].ID != "a" || nodes[1].ID != "b" || nodes[2].ID != "c" {
		t.Errorf("SortNodes() = %v; want sorted by ID", nodes)
	}
}

func TestInferNodeTypes(t *testing.T) {
	ext := &Extraction{
		Nodes: []Node{
			{ID: "n1", Label: "main.py", File: "main.py"},
			{ID: "n2", Label: ".process()"},
			{ID: "n3", Label: "helper()"},
			{ID: "n4", Label: "MyClass"},
			{ID: "n5", Label: "unknown"},
		},
		Edges: []Edge{
			{Source: "n4", Target: "n2", Relation: "method"},
		},
	}
	inferNodeTypes(ext)

	expected := map[string]string{
		"n1": "file",
		"n2": "method",
		"n3": "function",
		"n4": "class",
	}
	for _, n := range ext.Nodes {
		if exp, ok := expected[n.ID]; ok {
			if n.Type != exp {
				t.Errorf("inferNodeTypes() %s type = %q; want %q", n.ID, n.Type, exp)
			}
		}
	}
}

func TestExtractDispatcherGo(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	goFiles := CollectFiles(fixturesDir)
	var goFile string
	for _, f := range goFiles {
		if filepath.Ext(f) == ".go" {
			goFile = f
			break
		}
	}
	if goFile == "" {
		t.Skip("no .go fixture files found")
	}
	result := Extract([]string{goFile}, fixturesDir)
	if result == nil {
		t.Fatal("Extract() returned nil for .go file")
	}
}

func TestExtractPythonWithCallsAndTopFunctions(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	result := ExtractPython(filepath.Join(fixturesDir, "sample_calls.py"))
	if len(result.Nodes) == 0 {
		t.Fatal("ExtractPython(sample_calls.py) returned 0 nodes")
	}
	// Check top-level functions
	labels := GetNodeLabels(result)
	foundTopFunc := false
	for _, l := range labels {
		if l == "compute_score()" || l == "normalize()" || l == "run_analysis()" {
			foundTopFunc = true
			break
		}
	}
	if !foundTopFunc {
		t.Errorf("ExtractPython(sample_calls.py) should find top-level functions, got labels: %v", labels)
	}
	// Check calls edges exist
	hasCall := false
	for _, e := range result.Edges {
		if e.Relation == "calls" {
			hasCall = true
			break
		}
	}
	if !hasCall {
		t.Error("ExtractPython(sample_calls.py) should find call edges")
	}
}

func TestExtractPythonWithImports(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "imports_sample.py")
	os.WriteFile(pyFile, []byte(`import os
import sys
from pathlib import Path
from collections import OrderedDict

class MyProcessor:
    def process(self):
        path = Path(".")
        return os.listdir(path)

def helper():
    return sys.argv
`), 0644)

	result := ExtractPython(pyFile)
	if len(result.Nodes) == 0 {
		t.Fatal("ExtractPython(imports) returned 0 nodes")
	}
	// Check for import edges
	hasImport := false
	for _, e := range result.Edges {
		if e.Relation == "imports" || e.Relation == "imports_from" {
			hasImport = true
			break
		}
	}
	if !hasImport {
		t.Error("ExtractPython(imports) should find import edges")
	}
}

func TestExtractCppWithNamespaceAndInheritance(t *testing.T) {
	tmpDir := t.TempDir()
	cppFile := filepath.Join(tmpDir, "ns.cpp")
	os.WriteFile(cppFile, []byte(`#include <string>
#include <vector>

namespace mylib {

class Base {
public:
    virtual void process() = 0;
    void log(const std::string& msg) {}
};

class Derived : public Base {
public:
    void process() override {
        log("processing");
        helper();
    }
private:
    void helper() {}
};

void standalone_func() {
    Derived d;
    d.process();
}

} // namespace mylib
`), 0644)

	result := ExtractCpp(cppFile)
	if result == nil {
		t.Fatal("ExtractCpp() returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("ExtractCpp() returned 0 nodes for namespace/inheritance fixture")
	}
}

func TestExtractRustWithEnumsAndTraits(t *testing.T) {
	tmpDir := t.TempDir()
	rsFile := filepath.Join(tmpDir, "sample.rs")
	os.WriteFile(rsFile, []byte(`
mod utils;

pub trait Processor {
    fn process(&self) -> String;
}

pub enum Status {
    Active,
    Inactive,
    Pending,
}

pub struct Worker {
    name: String,
}

impl Processor for Worker {
    fn process(&self) -> String {
        format!("Worker: {}", self.name)
    }
}

fn main() {
    let w = Worker { name: "test".to_string() };
    w.process();
}
`), 0644)

	result := ExtractRust(rsFile)
	if result == nil {
		t.Fatal("ExtractRust() returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("ExtractRust() returned 0 nodes")
	}
}

func TestExtractDartWithConstructors(t *testing.T) {
	tmpDir := t.TempDir()
	dartFile := filepath.Join(tmpDir, "sample.dart")
	os.WriteFile(dartFile, []byte(`
import 'dart:core';
export 'other.dart';

class Widget {
  final String name;

  Widget(this.name);
  Widget.named({required this.name});

  void build() {
    render();
  }

  void render() {}
}

class Button extends Widget {
  Button(String label) : super(label);

  @override
  void build() {
    super.build();
  }
}

void topLevelFunction() {
  var w = Widget("test");
  w.build();
}
`), 0644)

	result := ExtractDart(dartFile)
	if result == nil {
		t.Fatal("ExtractDart() returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("ExtractDart() returned 0 nodes")
	}
}

func TestExtractJavaWithEnumAndInterfaces(t *testing.T) {
	tmpDir := t.TempDir()
	jFile := filepath.Join(tmpDir, "Sample.java")
	os.WriteFile(jFile, []byte(`package com.example;

import java.util.List;

interface Processable {
    void process();
}

interface Loggable {
    void log(String msg);
}

enum Status {
    ACTIVE,
    INACTIVE,
    PENDING
}

class BaseService implements Processable, Loggable {
    public void process() {}
    public void log(String msg) {}
}

class UserService extends BaseService {
    @Override
    public void process() {
        log("processing");
    }
}
`), 0644)

	result := ExtractJava(jFile)
	if result == nil {
		t.Fatal("ExtractJava() returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("ExtractJava() returned 0 nodes")
	}
}

func TestExtractPHPWithInterfaceAndTrait(t *testing.T) {
	tmpDir := t.TempDir()
	phpFile := filepath.Join(tmpDir, "sample.php")
	os.WriteFile(phpFile, []byte(`<?php

namespace App\Services;

interface Cacheable {
    public function cache(): void;
}

trait Loggable {
    public function log(string $msg): void {}
}

class UserService implements Cacheable {
    use Loggable;

    public function cache(): void {
        $this->log("caching");
    }
}
`), 0644)

	result := ExtractPHP(phpFile)
	if result == nil {
		t.Fatal("ExtractPHP() returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("ExtractPHP() returned 0 nodes")
	}
}

func TestExtractRubyWithModule(t *testing.T) {
	tmpDir := t.TempDir()
	rbFile := filepath.Join(tmpDir, "sample.rb")
	os.WriteFile(rbFile, []byte(`require 'json'

module Validators
  class EmailValidator
    def validate(email)
      email.include?("@")
    end
  end

  def self.helper
    puts "module method"
  end
end

class UserService
  def process
    validator = Validators::EmailValidator.new
    validator.validate("test@test.com")
  end
end
`), 0644)

	result := ExtractRuby(rbFile)
	if result == nil {
		t.Fatal("ExtractRuby() returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("ExtractRuby() returned 0 nodes")
	}
}

func TestExtractJavaScriptWithVarDecl(t *testing.T) {
	tmpDir := t.TempDir()
	jsFile := filepath.Join(tmpDir, "sample.js")
	os.WriteFile(jsFile, []byte(`
import { useState } from 'react';

class Component {
  render() {
    return null;
  }
}

const helper = () => {
  return 42;
};

function process(data) {
  return helper();
}

const result = process([1, 2, 3]);
`), 0644)

	result := ExtractJavaScript(jsFile)
	if result == nil {
		t.Fatal("ExtractJavaScript() returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("ExtractJavaScript() returned 0 nodes")
	}
}

func TestExtractPythonRichFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "rich.py")
	os.WriteFile(pyFile, []byte(`"""Module docstring."""

import os
import sys
from pathlib import Path
from collections import OrderedDict, defaultdict

class Base:
    def __init__(self):
        pass

class Child(Base):
    """Child class with decorator."""

    @staticmethod
    def static_method():
        return 42

    @property
    def name(self):
        return "child"

    def process(self, data):
        result = compute(data)
        self._helper(result)
        return result

    def _helper(self, x):
        return x * 2

def compute(data):
    return sum(data)

def transform(items):
    return [compute(x) for x in items]

if __name__ == "__main__":
    c = Child()
    c.process([1, 2, 3])
    transform([4, 5, 6])
`), 0644)

	result := ExtractPython(pyFile)
	if len(result.Nodes) < 5 {
		t.Errorf("ExtractPython(rich) nodes = %d; want >= 5", len(result.Nodes))
	}
}

func TestExtractCppWithInheritanceAndDecls(t *testing.T) {
	tmpDir := t.TempDir()
	cppFile := filepath.Join(tmpDir, "inherit.cpp")
	os.WriteFile(cppFile, []byte(`#include <string>

class Animal {
public:
    virtual std::string speak() = 0;
    virtual ~Animal() {}
};

class Dog : public Animal {
public:
    std::string speak() override { return "woof"; }
    void fetch();
};

class Cat : public Animal {
public:
    std::string speak() override { return "meow"; }
};

void Dog::fetch() {
    speak();
}

namespace zoo {
    class Keeper {
    public:
        void feed(Animal& a) {
            a.speak();
        }
    };

    void daily_routine() {
        Dog d;
        d.fetch();
    }
}
`), 0644)

	result := ExtractCpp(cppFile)
	if len(result.Nodes) < 3 {
		t.Errorf("ExtractCpp(inherit) nodes = %d; want >= 3", len(result.Nodes))
	}
}

func TestExtractMultipleLanguages(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	tests := []struct {
		file string
		ext  string
	}{
		{"sample.go", ".go"},
		{"sample.ts", ".ts"},
		{"sample.java", ".java"},
		{"sample.rb", ".rb"},
		{"sample.cs", ".cs"},
		{"sample.php", ".php"},
		{"sample.swift", ".swift"},
		{"sample.kt", ".kt"},
		{"sample.dart", ".dart"},
		{"sample.scala", ".scala"},
		{"sample.rs", ".rs"},
		{"sample.ex", ".ex"},
		{"sample.zig", ".zig"},
		{"sample.jl", ".jl"},
		{"sample.lua", ".lua"},
		{"sample.c", ".c"},
		{"sample.cpp", ".cpp"},
		{"sample.ps1", ".ps1"},
		{"sample.m", ".m"},
		{"sample.R", ".R"},
		{"sample.hs", ".hs"},
		{"Main.elm", ".elm"},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			path := filepath.Join(fixturesDir, tt.file)
			result := Extract([]string{path}, fixturesDir)
			if result == nil {
				t.Fatalf("Extract(%s) returned nil", tt.ext)
			}
			if len(result.Nodes) == 0 {
				t.Errorf("Extract(%s) returned 0 nodes", tt.ext)
			}
		})
	}
}

func TestExtractUnknownExtension(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.xyz")
	os.WriteFile(f, []byte("unknown format"), 0644)

	result := Extract([]string{f}, tmpDir)
	if len(result.Nodes) != 0 {
		t.Errorf("Extract(.xyz) nodes = %d; want 0", len(result.Nodes))
	}
}

func TestNoDanglingEdgesOnExtract(t *testing.T) {
	fixturesDir := "../../testdata/fixtures"
	files := []string{filepath.Join(fixturesDir, "sample.py")}
	result := Extract(files, fixturesDir)
	nodeIds := GetNodeIds(result)
	for _, edge := range result.Edges {
		found := false
		for _, id := range nodeIds {
			if id == edge.Source {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Dangling edge source: %s", edge.Source)
		}
	}
}
