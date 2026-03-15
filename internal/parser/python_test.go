package parser

import (
	"testing"
)

func TestPythonParser_Functions(t *testing.T) {
	source := []byte(`
import os
from pathlib import Path

def greet(name):
    print(f"hello {name}")

class UserService:
    def get_user(self, user_id):
        return self._fetch(user_id)

    def _fetch(self, uid):
        return None
`)
	p := NewPythonParser()
	result, err := p.Parse("app.py", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should find: greet, get_user, _fetch
	funcNames := map[string]bool{}
	for _, f := range result.Functions {
		funcNames[f.Name] = true
	}

	for _, expected := range []string{"greet", "get_user", "_fetch"} {
		if !funcNames[expected] {
			t.Errorf("expected function %q not found", expected)
		}
	}

	// greet should be exported, _fetch should not
	for _, f := range result.Functions {
		if f.Name == "greet" && !f.IsExport {
			t.Error("'greet' should be exported")
		}
		if f.Name == "_fetch" && f.IsExport {
			t.Error("'_fetch' should not be exported")
		}
	}

	// get_user and _fetch should have receiver "UserService"
	for _, f := range result.Functions {
		if f.Name == "get_user" && f.Receiver != "UserService" {
			t.Errorf("expected receiver 'UserService' for get_user, got %q", f.Receiver)
		}
	}

	// Should find UserService class
	if len(result.Classes) != 1 || result.Classes[0].Name != "UserService" {
		t.Errorf("expected class 'UserService', got %v", result.Classes)
	}

	// Should find imports
	if len(result.Imports) < 2 {
		t.Errorf("expected at least 2 imports, got %d", len(result.Imports))
	}
}

func TestPythonParser_TestFile(t *testing.T) {
	source := []byte(`def test_foo(): pass`)
	p := NewPythonParser()

	result, _ := p.Parse("test_app.py", source)
	if !result.IsTestFile {
		t.Error("test_app.py should be detected as test file")
	}
}
