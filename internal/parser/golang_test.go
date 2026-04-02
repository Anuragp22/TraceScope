package parser

import (
	"testing"
)

func TestGoParser_Functions(t *testing.T) {
	source := []byte(`package main

import "fmt"

func hello() {
	fmt.Println("hello")
}

func Add(a, b int) int {
	return a + b
}
`)
	p := NewGoParser()
	result, err := p.Parse("test.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.Package != "main" {
		t.Errorf("expected package 'main', got %q", result.Package)
	}

	if len(result.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(result.Functions))
	}

	// hello is unexported
	if result.Functions[0].Name != "hello" {
		t.Errorf("expected 'hello', got %q", result.Functions[0].Name)
	}
	if result.Functions[0].IsExport {
		t.Error("'hello' should not be exported")
	}

	// Add is exported
	if result.Functions[1].Name != "Add" {
		t.Errorf("expected 'Add', got %q", result.Functions[1].Name)
	}
	if !result.Functions[1].IsExport {
		t.Error("'Add' should be exported")
	}

	// Should detect fmt.Println call
	foundCall := false
	for _, c := range result.Calls {
		if c.Name == "Println" && c.Receiver == "fmt" {
			foundCall = true
		}
	}
	if !foundCall {
		t.Error("expected to find fmt.Println call")
	}

	// Should detect "fmt" import
	if len(result.Imports) != 1 || result.Imports[0].Path != "fmt" {
		t.Errorf("expected 'fmt' import, got %v", result.Imports)
	}
}

func TestGoParser_Methods(t *testing.T) {
	source := []byte(`package models

type User struct {
	Name string
}

func (u *User) GetName() string {
	return u.Name
}
`)
	p := NewGoParser()
	result, err := p.Parse("models.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Classes) != 1 || result.Classes[0].Name != "User" {
		t.Errorf("expected struct 'User', got %v", result.Classes)
	}

	if len(result.Functions) != 1 {
		t.Fatalf("expected 1 method, got %d", len(result.Functions))
	}

	method := result.Functions[0]
	if method.Name != "GetName" {
		t.Errorf("expected 'GetName', got %q", method.Name)
	}
	if method.Receiver != "User" {
		t.Errorf("expected receiver 'User', got %q", method.Receiver)
	}
}

func TestGoParser_MethodCallReceiverType(t *testing.T) {
	source := []byte(`package models

type User struct{}

func (u User) GetName() string {
	return "alice"
}

func Render() {
	var user User
	user.GetName()
}
`)
	p := NewGoParser()
	result, err := p.Parse("models.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	found := false
	for _, call := range result.Calls {
		if call.Name == "GetName" && call.Receiver == "user" {
			found = true
			if call.ReceiverType != "User" {
				t.Fatalf("expected receiver type User, got %q", call.ReceiverType)
			}
			if call.ReceiverPackage != "models" {
				t.Fatalf("expected receiver package models, got %q", call.ReceiverPackage)
			}
		}
	}
	if !found {
		t.Fatal("expected user.GetName() call to be parsed")
	}
}

func TestGoParser_TestFile(t *testing.T) {
	source := []byte(`package main

func TestSomething() {}
`)
	p := NewGoParser()
	result, err := p.Parse("main_test.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !result.IsTestFile {
		t.Error("expected IsTestFile to be true")
	}
}

func TestGoParser_UnicodeExport(t *testing.T) {
	source := []byte(`package main

func Ñoño() {}
func ñoño() {}
`)
	p := NewGoParser()
	result, err := p.Parse("unicode.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(result.Functions))
	}
	if !result.Functions[0].IsExport {
		t.Error("'Ñoño' should be exported (uppercase unicode)")
	}
	if result.Functions[1].IsExport {
		t.Error("'ñoño' should not be exported (lowercase unicode)")
	}
}

func TestGoParser_ClosureCalls(t *testing.T) {
	source := []byte(`package main

import "fmt"

func outer() {
	fn := func() {
		fmt.Println("inside closure")
	}
	fn()
}
`)
	p := NewGoParser()
	result, err := p.Parse("closure.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	// Should detect Println call inside the closure
	found := false
	for _, c := range result.Calls {
		if c.Name == "Println" && c.Receiver == "fmt" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find fmt.Println call inside closure")
	}
}

func TestGoParser_Interface(t *testing.T) {
	source := []byte(`package io

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}
`)
	p := NewGoParser()
	result, err := p.Parse("io.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Classes) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(result.Classes))
	}
	if result.Classes[0].Kind != "interface" {
		t.Errorf("expected 'interface', got %q", result.Classes[0].Kind)
	}

	// Interface methods should be extracted as functions with receiver = interface name
	methodNames := map[string]string{} // name → receiver
	for _, f := range result.Functions {
		methodNames[f.Name] = f.Receiver
	}
	if recv, ok := methodNames["Read"]; !ok || recv != "Reader" {
		t.Errorf("expected Read method with receiver 'Reader', got receiver %q, found=%v", recv, ok)
	}
	if recv, ok := methodNames["Write"]; !ok || recv != "Writer" {
		t.Errorf("expected Write method with receiver 'Writer', got receiver %q, found=%v", recv, ok)
	}
}

func TestGoParser_InitFunction(t *testing.T) {
	source := []byte(`package main

func init() {
	setup()
}

func main() {}
`)
	p := NewGoParser()
	result, err := p.Parse("main.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundInit := false
	for _, f := range result.Functions {
		if f.Name == "init" {
			foundInit = true
			if !f.IsInit {
				t.Error("init() should have IsInit=true")
			}
			if f.IsExport {
				t.Error("init() should not be exported")
			}
		}
		if f.Name == "main" && f.IsInit {
			t.Error("main() should not have IsInit=true")
		}
	}
	if !foundInit {
		t.Error("expected to find init() function")
	}
}
