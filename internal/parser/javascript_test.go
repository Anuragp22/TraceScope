package parser

import (
	"testing"
)

func TestJSParser_Functions(t *testing.T) {
	source := []byte(`
function greet(name) {
  console.log("hello " + name);
}

const add = (a, b) => a + b;

class Calculator {
  multiply(a, b) {
    return a * b;
  }
}
`)
	p := NewJavaScriptParser()
	result, err := p.Parse("app.js", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should find: greet, add, multiply
	funcNames := map[string]bool{}
	for _, f := range result.Functions {
		funcNames[f.Name] = true
	}

	for _, expected := range []string{"greet", "add", "multiply"} {
		if !funcNames[expected] {
			t.Errorf("expected function %q not found", expected)
		}
	}

	// Should find Calculator class
	if len(result.Classes) != 1 || result.Classes[0].Name != "Calculator" {
		t.Errorf("expected class 'Calculator', got %v", result.Classes)
	}

	// Should find console.log call
	foundCall := false
	for _, c := range result.Calls {
		if c.Name == "log" && c.Receiver == "console" {
			foundCall = true
		}
	}
	if !foundCall {
		t.Error("expected to find console.log call")
	}
}

func TestJSParser_Imports(t *testing.T) {
	source := []byte(`
import { useState, useEffect as effect } from 'react';
import axios from 'axios';
import * as api from './client';
const fs = require('fs');
`)
	p := NewJavaScriptParser()
	result, err := p.Parse("app.js", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Imports) < 3 {
		t.Fatalf("expected at least 3 imports, got %d", len(result.Imports))
	}

	type importKey struct {
		path   string
		alias  string
		symbol string
	}
	imports := map[importKey]bool{}
	for _, imp := range result.Imports {
		imports[importKey{path: imp.Path, alias: imp.Alias, symbol: imp.Symbol}] = true
	}
	for _, expected := range []importKey{
		{path: "react", alias: "useState", symbol: "useState"},
		{path: "react", alias: "effect", symbol: "useEffect"},
		{path: "axios", alias: "axios", symbol: "default"},
		{path: "./client", alias: "api", symbol: "*"},
		{path: "fs", alias: "fs", symbol: "*"},
	} {
		if !imports[expected] {
			t.Errorf("expected import %+v not found", expected)
		}
	}
}

func TestJSParser_TestFile(t *testing.T) {
	source := []byte(`function test() {}`)
	p := NewJavaScriptParser()

	result, _ := p.Parse("app.test.js", source)
	if !result.IsTestFile {
		t.Error("app.test.js should be detected as test file")
	}

	result, _ = p.Parse("app.spec.js", source)
	if !result.IsTestFile {
		t.Error("app.spec.js should be detected as test file")
	}
}

func TestJSParser_DefaultExport(t *testing.T) {
	source := []byte(`
export default function() {
  console.log("default");
}
`)
	p := NewJavaScriptParser()
	result, err := p.Parse("default.js", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should detect the anonymous default export as a function named "default"
	found := false
	for _, f := range result.Functions {
		if f.Name == "default" && f.IsExport {
			found = true
		}
	}
	if !found {
		t.Error("expected to find exported default function")
	}
}

func TestJSParser_NamedDefaultExportAlias(t *testing.T) {
	source := []byte(`
export default function run() {
  return 1;
}
`)
	p := NewJavaScriptParser()
	result, err := p.Parse("default.js", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	names := map[string]bool{}
	for _, fn := range result.Functions {
		names[fn.Name] = true
	}
	for _, expected := range []string{"run", "default"} {
		if !names[expected] {
			t.Fatalf("expected function %q not found in %+v", expected, result.Functions)
		}
	}
}

func TestJSParser_TrimQuotesBacktick(t *testing.T) {
	s := trimQuotes("`hello`")
	if s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}
}
