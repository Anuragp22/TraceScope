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

// TestParseUnifiedDiff_DeletionOnlyHunk covers a hunk that only removes lines
// from the middle of an otherwise-unchanged function (no '+' lines). Such a
// change still alters the enclosing function, so it must produce a line range
// anchored at the deletion point — otherwise diff-to-function mapping misses
// the function entirely and its callers are never traversed.
func TestParseUnifiedDiff_DeletionOnlyHunk(t *testing.T) {
	diffData := []byte(`diff --git a/svc.go b/svc.go
index abc123..def456 100644
--- a/svc.go
+++ b/svc.go
@@ -10,7 +10,5 @@ func Handle() {
 	a()
 	b()
-	removed1()
-	removed2()
 	c()
 	d()
 }
`)
	files, err := ParseUnifiedDiff(diffData)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	cf := files[0]
	if len(cf.LineRanges) == 0 {
		t.Fatal("deletion-only hunk produced no line ranges; the enclosing function would never be flagged")
	}

	// The deletion sits at new-file line 12 (start 10 + two surviving context
	// lines a() and b()). Some range must cover it so it overlaps [Start,End]
	// of the enclosing function.
	const deletionLine = 12
	covered := false
	for _, r := range cf.LineRanges {
		if r.Start <= deletionLine && deletionLine <= r.End {
			covered = true
			break
		}
	}
	if !covered {
		t.Errorf("no line range covers the deletion point (line %d); got %+v", deletionLine, cf.LineRanges)
	}
}

// TestParseUnifiedDiff_DeletionAtFunctionStart covers a pure deletion at the
// very start of a hunk (no preceding context line to anchor against).
func TestParseUnifiedDiff_DeletionAtFunctionStart(t *testing.T) {
	diffData := []byte(`diff --git a/svc.go b/svc.go
index abc123..def456 100644
--- a/svc.go
+++ b/svc.go
@@ -5,4 +5,3 @@ func Handle() {
-	guard()
 	body1()
 	body2()
 }
`)
	files, err := ParseUnifiedDiff(diffData)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}
	if len(files) != 1 || len(files[0].LineRanges) == 0 {
		t.Fatalf("deletion at hunk start produced no line ranges: %+v", files)
	}
	const deletionLine = 5
	covered := false
	for _, r := range files[0].LineRanges {
		if r.Start <= deletionLine && deletionLine <= r.End {
			covered = true
			break
		}
	}
	if !covered {
		t.Errorf("no line range covers the deletion point (line %d); got %+v", deletionLine, files[0].LineRanges)
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
