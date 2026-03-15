package diff

import (
	"testing"
)

func TestParseUnifiedDiff(t *testing.T) {
	diffData := []byte(`diff --git a/main.go b/main.go
index abc123..def456 100644
--- a/main.go
+++ b/main.go
@@ -5,7 +5,8 @@ package main
 func main() {
-	fmt.Println("old")
+	fmt.Println("new")
+	fmt.Println("added")
 }
`)
	files, err := ParseUnifiedDiff(diffData)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 changed file, got %d", len(files))
	}

	cf := files[0]
	if cf.Path != "main.go" {
		t.Errorf("expected path 'main.go', got %q", cf.Path)
	}
	if cf.IsNew || cf.IsDeleted {
		t.Error("file should not be marked as new or deleted")
	}
	if len(cf.LineRanges) == 0 {
		t.Error("expected at least one changed line range")
	}
}

func TestParseUnifiedDiff_NewFile(t *testing.T) {
	diffData := []byte(`diff --git a/new.go b/new.go
new file mode 100644
index 0000000..abc123
--- /dev/null
+++ b/new.go
@@ -0,0 +1,5 @@
+package main
+
+func newFunc() {
+	return
+}
`)
	files, err := ParseUnifiedDiff(diffData)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !files[0].IsNew {
		t.Error("file should be marked as new")
	}
}

func TestParseUnifiedDiff_DeletedFile(t *testing.T) {
	diffData := []byte(`diff --git a/old.go b/old.go
deleted file mode 100644
index abc123..0000000
--- a/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package main
-
-func old() {}
`)
	files, err := ParseUnifiedDiff(diffData)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !files[0].IsDeleted {
		t.Error("file should be marked as deleted")
	}
}
