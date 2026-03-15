package parser

import "testing"

func TestGoParser_EmbeddedStruct(t *testing.T) {
	source := []byte(`package models

type Base struct {
	ID int
}

type User struct {
	Base
	Name string
}
`)
	p := NewGoParser()
	result, err := p.Parse("models.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(result.Classes) != 2 {
		t.Fatalf("expected 2 structs, got %d", len(result.Classes))
	}

	// User should have Base as a base
	var user ClassDef
	for _, cls := range result.Classes {
		if cls.Name == "User" {
			user = cls
		}
	}
	if len(user.Bases) != 1 || user.Bases[0] != "Base" {
		t.Errorf("expected User to embed Base, got bases: %v", user.Bases)
	}
}

func TestGoParser_EmbeddedInterface(t *testing.T) {
	source := []byte(`package io

type Reader interface {
	Read(p []byte) (n int, err error)
}

type ReadCloser interface {
	Reader
	Close() error
}
`)
	p := NewGoParser()
	result, err := p.Parse("io.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var rc ClassDef
	for _, cls := range result.Classes {
		if cls.Name == "ReadCloser" {
			rc = cls
		}
	}
	if len(rc.Bases) != 1 || rc.Bases[0] != "Reader" {
		t.Errorf("expected ReadCloser to embed Reader, got bases: %v", rc.Bases)
	}
}

func TestGoParser_PointerEmbedding(t *testing.T) {
	source := []byte(`package models

type Logger struct{}

type Service struct {
	*Logger
	Name string
}
`)
	p := NewGoParser()
	result, err := p.Parse("svc.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var svc ClassDef
	for _, cls := range result.Classes {
		if cls.Name == "Service" {
			svc = cls
		}
	}
	if len(svc.Bases) != 1 || svc.Bases[0] != "Logger" {
		t.Errorf("expected Service to embed *Logger, got bases: %v", svc.Bases)
	}
}

func TestGoParser_ContentHash(t *testing.T) {
	source := []byte(`package main
func hello() {}
`)
	p := NewGoParser()
	result, err := p.Parse("test.go", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	// ContentHash is set by the registry, not the parser directly
	// Just verify the field exists and doesn't panic
	if result.ContentHash != "" {
		t.Error("ContentHash should be empty when set by parser alone (registry sets it)")
	}
}
