package extract

import (
	"os"
	"path/filepath"
	"testing"
)

// Tests targeting uncovered code paths in language extractors.
// Each test uses inline source snippets to exercise specific branches.

// --- C: cExtractFunctionDeclNode (0%) ---

func TestExtractCFunctionDeclaration(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "decl.c")
	os.WriteFile(f, []byte(`
#include <stdio.h>

// Forward declarations (no body)
void process(int x);
int compute(double a, double b);

void process(int x) {
    printf("%d", x);
}
`), 0644)

	result := ExtractC(f)
	if result == nil {
		t.Fatal("ExtractC returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("Expected nodes from C file with declarations")
	}
}

// --- C++: cppExtractBaseClass (0%), cppExtractMethodDecl (0%), cppExtractTopDecl (0%) ---

func TestExtractCppBaseClassClause(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "base.cpp")
	os.WriteFile(f, []byte(`
class Base {
public:
    virtual void run() = 0;
};

class Middle : public Base {
public:
    void run() override {}
};

class Child : public Middle, public Base {
public:
    void run() override {}
};
`), 0644)

	result := ExtractCpp(f)
	if result == nil {
		t.Fatal("ExtractCpp returned nil")
	}
	if len(result.Nodes) < 2 {
		t.Errorf("Expected >= 2 nodes from C++ with base classes, got %d", len(result.Nodes))
	}
}

func TestExtractCppMethodDeclaration(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "decl.cpp")
	os.WriteFile(f, []byte(`
class Service {
public:
    void start();
    void stop();
    int status() const;
};

void standalone();
int compute(double x);
`), 0644)

	result := ExtractCpp(f)
	if result == nil {
		t.Fatal("ExtractCpp returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("Expected nodes from C++ declarations")
	}
}

func TestExtractCppQualifiedName(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "qualified.cpp")
	os.WriteFile(f, []byte(`
namespace outer {
namespace inner {

class Widget {
public:
    void draw();
};

} // namespace inner
} // namespace outer

void outer::inner::Widget::draw() {
    // out-of-line definition with qualified name
}
`), 0644)

	result := ExtractCpp(f)
	if result == nil {
		t.Fatal("ExtractCpp returned nil")
	}
}

// --- C#: csExtractBaseList (34.5%) ---

func TestExtractCSharpBaseList(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "base.cs")
	os.WriteFile(f, []byte(`
using System;

interface IProcessor {
    void Process();
}

interface ILogger {
    void Log(string msg);
}

abstract class BaseService : IProcessor, ILogger {
    public abstract void Process();
    public void Log(string msg) {}
}

class UserService : BaseService {
    public override void Process() {
        Log("processing");
    }
}

struct Point : IComparable {
    public int X;
    public int Y;
    public int CompareTo(object obj) { return 0; }
}
`), 0644)

	result := ExtractCSharp(f)
	if result == nil {
		t.Fatal("ExtractCSharp returned nil")
	}
	hasInherits := false
	for _, e := range result.Edges {
		if e.Relation == "inherits" {
			hasInherits = true
			break
		}
	}
	if !hasInherits {
		t.Error("Expected inherits edges from C# base list")
	}
}

// --- Dart: dartExtractExportSpec (0%), dartExtractConstructorDecl (0%),
//          dartExtractTopVar (0%), collectCascadeCalls (0%) ---

func TestExtractDartExportAndTopVar(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "exports.dart")
	os.WriteFile(f, []byte(`
export 'src/widget.dart';
export 'src/button.dart';

const String appName = 'MyApp';
final int maxRetries = 3;
var globalState = {};

class Config {
  final String name;
  Config(this.name);
}
`), 0644)

	result := ExtractDart(f)
	if result == nil {
		t.Fatal("ExtractDart returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("Expected nodes from Dart exports and top-level vars")
	}
}

func TestExtractDartConstructorDecl(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "ctor.dart")
	os.WriteFile(f, []byte(`
class Point {
  final double x;
  final double y;

  Point(this.x, this.y);
  Point.origin() : x = 0, y = 0;
  Point.fromJson(Map<String, dynamic> json)
      : x = json['x'],
        y = json['y'];

  factory Point.zero() => Point(0, 0);

  double distanceTo(Point other) {
    return 0.0;
  }
}
`), 0644)

	result := ExtractDart(f)
	if result == nil {
		t.Fatal("ExtractDart returned nil")
	}
}

func TestExtractDartCascadeCalls(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "cascade.dart")
	os.WriteFile(f, []byte(`
class Builder {
  String name = '';
  int count = 0;

  void setName(String n) { name = n; }
  void setCount(int c) { count = c; }
  void build() {}
}

void main() {
  Builder()
    ..setName('test')
    ..setCount(5)
    ..build();
}
`), 0644)

	result := ExtractDart(f)
	if result == nil {
		t.Fatal("ExtractDart returned nil")
	}
}

// --- Kotlin: kotlinGetCallName (30.8%) ---

func TestExtractKotlinCallNames(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "calls.kt")
	os.WriteFile(f, []byte(`
package com.example

import kotlin.math.sqrt

class Calculator {
    fun add(a: Int, b: Int): Int = a + b
    fun subtract(a: Int, b: Int): Int = a - b
}

fun process(items: List<String>) {
    val calc = Calculator()
    calc.add(1, 2)
    calc.subtract(3, 1)
    items.forEach { println(it) }
    items.map { it.uppercase() }
    sqrt(16.0)
    listOf(1, 2, 3).filter { it > 1 }
}

fun main() {
    process(listOf("a", "b"))
}
`), 0644)

	result := ExtractKotlin(f)
	if result == nil {
		t.Fatal("ExtractKotlin returned nil")
	}
	hasCalls := false
	for _, e := range result.Edges {
		if e.Relation == "calls" {
			hasCalls = true
			break
		}
	}
	if !hasCalls {
		t.Error("Expected call edges from Kotlin file")
	}
}

// --- Lua: luaFindFuncBody (37.5%) ---

func TestExtractLuaFunctionVariants(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "funcs.lua")
	os.WriteFile(f, []byte(`
local utils = require("utils")

-- Local function
local function helper(x)
    return x * 2
end

-- Method-style function
function MyClass:method(a, b)
    return self.helper(a) + b
end

-- Module function
function MyModule.init()
    helper(1)
end

-- Anonymous function assignment
local process = function(data)
    return data
end

-- Nested function
function outer()
    local function inner()
        return 42
    end
    return inner()
end
`), 0644)

	result := ExtractLua(f)
	if result == nil {
		t.Fatal("ExtractLua returned nil")
	}
	if len(result.Nodes) < 3 {
		t.Errorf("Expected >= 3 nodes from Lua, got %d", len(result.Nodes))
	}
}

// --- Python: pyExtractImportStmt (40.9%) ---

func TestExtractPythonImportVariants(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "imports.py")
	os.WriteFile(f, []byte(`
import os
import sys
import os.path
import json as j
from pathlib import Path
from collections import OrderedDict, defaultdict
from typing import List, Dict, Optional
from . import utils
from ..core import base

class Worker:
    def run(self):
        path = Path(".")
        data = j.loads("{}")
        return os.path.exists(path)
`), 0644)

	result := ExtractPython(f)
	if result == nil {
		t.Fatal("ExtractPython returned nil")
	}
	importCount := 0
	for _, e := range result.Edges {
		if e.Relation == "imports" || e.Relation == "imports_from" {
			importCount++
		}
	}
	if importCount < 3 {
		t.Errorf("Expected >= 3 import edges, got %d", importCount)
	}
}

// --- Scala: scalaExtractTopFunc (0%) ---

func TestExtractScalaTopLevelFunction(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "top.scala")
	os.WriteFile(f, []byte(`
import scala.collection.mutable

object Utils {
  def helper(): Int = 42
}

def topLevelFunction(x: Int): Int = {
  val result = x * 2
  Utils.helper()
  result
}

def anotherTopLevel(): String = "hello"

class Service {
  def process(): Unit = {
    topLevelFunction(5)
  }
}
`), 0644)

	result := ExtractScala(f)
	if result == nil {
		t.Fatal("ExtractScala returned nil")
	}
	if len(result.Nodes) == 0 {
		t.Error("Expected nodes from Scala with top-level functions")
	}
}

// --- Julia: juliaGetFuncNameFromSignature (50%) ---

func TestExtractJuliaFunctionVariants(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "funcs.jl")
	os.WriteFile(f, []byte(`
module MyMod

using LinearAlgebra
import Base: show

abstract type Shape end

struct Circle <: Shape
    radius::Float64
end

function area(c::Circle)
    return pi * c.radius^2
end

# Short function definition
perimeter(c::Circle) = 2 * pi * c.radius

# Multi-method
function show(io::IO, c::Circle)
    print(io, "Circle($(c.radius))")
end

# Generic function
function process(items)
    map(area, items)
end

end # module
`), 0644)

	result := ExtractJulia(f)
	if result == nil {
		t.Fatal("ExtractJulia returned nil")
	}
	if len(result.Nodes) < 3 {
		t.Errorf("Expected >= 3 nodes from Julia, got %d", len(result.Nodes))
	}
}

// --- Ruby: rubyCollectCalls (52.9%), rubyExtractMethod/TopMethod (66.7%) ---

func TestExtractRubyMethodsAndCalls(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "methods.rb")
	os.WriteFile(f, []byte(`
require 'json'
require_relative 'helpers'

module Services
  class Processor
    def initialize(config)
      @config = config
    end

    def self.create(opts = {})
      new(opts)
    end

    def process(data)
      validate(data)
      transform(data)
      save(data)
    end

    private

    def validate(data)
      raise "invalid" unless data
    end

    def transform(data)
      data.map { |item| item.upcase }
    end

    def save(data)
      File.write("out.json", JSON.generate(data))
    end
  end
end

def standalone_helper(x)
  puts x.to_s
  x.length
end
`), 0644)

	result := ExtractRuby(f)
	if result == nil {
		t.Fatal("ExtractRuby returned nil")
	}
	if len(result.Nodes) < 3 {
		t.Errorf("Expected >= 3 nodes from Ruby, got %d", len(result.Nodes))
	}
}

// --- JavaScript: jsStringContent (57.1%) ---

func TestExtractJavaScriptImportVariants(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "imports.js")
	os.WriteFile(f, []byte(`
import React from 'react';
import { useState, useEffect } from "react";
import * as utils from './utils';

const API_URL = 'https://api.example.com';

class App extends React.Component {
  constructor(props) {
    super(props);
    this.state = {};
  }

  render() {
    return null;
  }
}

function fetchData(url) {
  return fetch(url);
}

const processItems = (items) => {
  return items.filter(x => x.active).map(x => x.name);
};

export default App;
`), 0644)

	result := ExtractJavaScript(f)
	if result == nil {
		t.Fatal("ExtractJavaScript returned nil")
	}
	hasImport := false
	for _, e := range result.Edges {
		if e.Relation == "imports" {
			hasImport = true
			break
		}
	}
	if !hasImport {
		t.Error("Expected import edges from JavaScript file")
	}
}

// --- Swift: swiftExtractImportNew (56.5%), swiftExtractTypeRefsForInheritance (68.8%) ---

func TestExtractSwiftInheritanceAndImports(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "inherit.swift")
	os.WriteFile(f, []byte(`
import Foundation
import UIKit

protocol Drawable {
    func draw()
}

protocol Resizable {
    func resize(to: CGSize)
}

class Shape: Drawable {
    func draw() {}
}

class Rectangle: Shape, Resizable {
    var width: Double
    var height: Double

    init(width: Double, height: Double) {
        self.width = width
        self.height = height
    }

    func resize(to: CGSize) {}
    func area() -> Double { return width * height }
}

enum Color: String {
    case red
    case blue
    case green
}

func createShape() -> Shape {
    return Rectangle(width: 10, height: 20)
}
`), 0644)

	result := ExtractSwift(f)
	if result == nil {
		t.Fatal("ExtractSwift returned nil")
	}
	hasInherits := false
	for _, e := range result.Edges {
		if e.Relation == "inherits" {
			hasInherits = true
			break
		}
	}
	if !hasInherits {
		t.Error("Expected inherits edges from Swift file")
	}
}

// --- PowerShell: psExtractImportFromCommand (71.4%), psExtractImportFromText (69.2%) ---

func TestExtractPowerShellImportVariants(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "imports.ps1")
	os.WriteFile(f, []byte(`
Import-Module ActiveDirectory
Import-Module -Name SqlServer

. .\helpers.ps1
. "$PSScriptRoot\utils.ps1"

using module MyModule
using namespace System.IO

function Get-UserInfo {
    param([string]$Name)
    Get-ADUser -Filter "Name -eq '$Name'"
}

class Logger {
    [void] Log([string]$msg) {
        Write-Host $msg
    }
}

function Process-Data {
    param($Data)
    $result = Get-UserInfo -Name "test"
    return $result
}
`), 0644)

	result := ExtractPowerShell(f)
	if result == nil {
		t.Fatal("ExtractPowerShell returned nil")
	}
}

// --- R: rExtractLibraryOrCall (73.9%), rExtractFunctionDefWithBody (78.9%) ---

func TestExtractRFunctionVariants(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "funcs.R")
	os.WriteFile(f, []byte(`
library(ggplot2)
library(dplyr)
require(tidyr)

source("helpers.R")

compute <- function(x, y) {
  result <- x + y
  return(result)
}

transform <- function(data) {
  data %>%
    filter(value > 0) %>%
    mutate(scaled = value * 2)
}

process <- function(items) {
  sapply(items, compute, y = 1)
  transform(items)
}
`), 0644)

	result := ExtractR(f)
	if result == nil {
		t.Fatal("ExtractR returned nil")
	}
	if len(result.Nodes) < 2 {
		t.Errorf("Expected >= 2 nodes from R, got %d", len(result.Nodes))
	}
}

// --- Haskell: hsExtractFunctionBody (71.4%), hsExtractDeclarationsWithBody (76%) ---

func TestExtractHaskellDeclarations(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "decls.hs")
	os.WriteFile(f, []byte(`
module MyLib where

import Data.List (sort, nub)
import qualified Data.Map as Map

data Shape = Circle Double | Rectangle Double Double

class Measurable a where
  area :: a -> Double
  perimeter :: a -> Double

instance Measurable Shape where
  area (Circle r) = pi * r * r
  area (Rectangle w h) = w * h
  perimeter (Circle r) = 2 * pi * r
  perimeter (Rectangle w h) = 2 * (w + h)

compute :: Int -> Int -> Int
compute x y = x + y

transform :: [Int] -> [Int]
transform = map (* 2) . filter (> 0)

process :: [Int] -> Int
process items = sum (transform items)
`), 0644)

	result := ExtractHaskell(f)
	if result == nil {
		t.Fatal("ExtractHaskell returned nil")
	}
	if len(result.Nodes) < 3 {
		t.Errorf("Expected >= 3 nodes from Haskell, got %d", len(result.Nodes))
	}
}

// --- Elixir: elixirGetDefName (70%) ---

func TestExtractElixirDefVariants(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "defs.ex")
	os.WriteFile(f, []byte(`
defmodule MyApp.Worker do
  use GenServer

  defstruct [:name, :status]

  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts)
  end

  def init(opts) do
    {:ok, opts}
  end

  defp do_work(state) do
    process(state)
    transform(state)
  end

  defmacro my_macro(expr) do
    quote do
      unquote(expr)
    end
  end

  def process(data), do: data
  def transform(data), do: data
end
`), 0644)

	result := ExtractElixir(f)
	if result == nil {
		t.Fatal("ExtractElixir returned nil")
	}
	if len(result.Nodes) < 3 {
		t.Errorf("Expected >= 3 nodes from Elixir, got %d", len(result.Nodes))
	}
}

// --- Elm: elmGetCallName (57.1%), elmExtractValueDeclarationWithBody (77.8%) ---

func TestExtractElmValueDeclarations(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "Values.elm")
	os.WriteFile(f, []byte(`
module Values exposing (..)

import Html exposing (text)
import String

type Msg = Increment | Decrement

type alias Model = { count : Int, name : String }

init : Model
init = { count = 0, name = "" }

update : Msg -> Model -> Model
update msg model =
    case msg of
        Increment -> { model | count = model.count + 1 }
        Decrement -> { model | count = model.count - 1 }

view : Model -> Html.Html Msg
view model =
    text (String.fromInt model.count)

helper : Int -> Int
helper x =
    x * 2
`), 0644)

	result := ExtractElm(f)
	if result == nil {
		t.Fatal("ExtractElm returned nil")
	}
	if len(result.Nodes) < 2 {
		t.Errorf("Expected >= 2 nodes from Elm, got %d", len(result.Nodes))
	}
}

// --- Zig: zigExtractFuncDeclWithBody (71.4%) ---

func TestExtractZigFunctionVariants(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "funcs.zig")
	os.WriteFile(f, []byte(`
const std = @import("std");
const mem = @import("std").mem;

const Config = struct {
    name: []const u8,
    count: usize,

    pub fn init(name: []const u8) Config {
        return Config{ .name = name, .count = 0 };
    }

    pub fn process(self: *Config) void {
        self.count += 1;
    }
};

pub fn createConfig(name: []const u8) Config {
    return Config.init(name);
}

fn helper(x: i32) i32 {
    return x * 2;
}

pub fn main() !void {
    var config = createConfig("test");
    config.process();
    const result = helper(42);
    _ = result;
}
`), 0644)

	result := ExtractZig(f)
	if result == nil {
		t.Fatal("ExtractZig returned nil")
	}
	if len(result.Nodes) < 3 {
		t.Errorf("Expected >= 3 nodes from Zig, got %d", len(result.Nodes))
	}
}
