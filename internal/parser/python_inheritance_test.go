package parser

import "testing"

func TestPythonParser_Inheritance(t *testing.T) {
	source := []byte(`
class Animal:
    def speak(self):
        pass

class Dog(Animal):
    def bark(self):
        pass

class GuideDog(Dog, Animal):
    def guide(self):
        pass
`)
	p := NewPythonParser()
	result, err := p.Parse("animals.py", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	classByName := map[string]ClassDef{}
	for _, cls := range result.Classes {
		classByName[cls.Name] = cls
	}

	// Dog extends Animal
	dog := classByName["Dog"]
	if len(dog.Bases) != 1 || dog.Bases[0] != "Animal" {
		t.Errorf("expected Dog bases [Animal], got %v", dog.Bases)
	}

	// GuideDog extends Dog and Animal (multiple inheritance)
	guide := classByName["GuideDog"]
	if len(guide.Bases) != 2 {
		t.Errorf("expected GuideDog to have 2 bases, got %d: %v", len(guide.Bases), guide.Bases)
	}
}

func TestPythonParser_ClassMethods(t *testing.T) {
	source := []byte(`
class MyClass:
    def method_a(self):
        pass

    def method_b(self):
        self.method_a()
`)
	p := NewPythonParser()
	result, err := p.Parse("cls.py", source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Methods should have Receiver = "MyClass"
	for _, fn := range result.Functions {
		if fn.Receiver != "MyClass" {
			t.Errorf("expected method %q to have receiver MyClass, got %q", fn.Name, fn.Receiver)
		}
	}
}
