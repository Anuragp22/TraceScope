package parser

import "testing"

func TestTSParser_Imports(t *testing.T) {
	source := []byte(`
import handler from './handler';
import { run, build as makeBuild } from './worker';
import * as metrics from './metrics';
`)

	p := NewTypeScriptParser()
	result, err := p.Parse("app.ts", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
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
		{path: "./handler", alias: "handler", symbol: "default"},
		{path: "./worker", alias: "run", symbol: "run"},
		{path: "./worker", alias: "makeBuild", symbol: "build"},
		{path: "./metrics", alias: "metrics", symbol: "*"},
	} {
		if !imports[expected] {
			t.Fatalf("expected import %+v not found in %+v", expected, result.Imports)
		}
	}
}

func TestTSParser_NamedDefaultExportAlias(t *testing.T) {
	source := []byte(`
export default function loadUser() {
  return "ok";
}
`)

	p := NewTypeScriptParser()
	result, err := p.Parse("loader.ts", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	names := map[string]bool{}
	for _, fn := range result.Functions {
		names[fn.Name] = true
	}
	for _, expected := range []string{"loadUser", "default"} {
		if !names[expected] {
			t.Fatalf("expected function %q not found in %+v", expected, result.Functions)
		}
	}
}
