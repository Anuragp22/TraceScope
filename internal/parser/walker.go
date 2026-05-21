package parser

import (
	"os"
	"path/filepath"
	"strings"
)

// Language represents a supported programming language.
type Language string

const (
	LangGo         Language = "go"
	LangJavaScript Language = "javascript"
	LangTypeScript Language = "typescript"
	LangPython     Language = "python"
)

// skipDirs are directories to skip during walking.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"__pycache__":  true,
	"dist":         true,
	"build":        true,
	".next":        true,
}

// allowDotDirs are dot-prefixed directories that should NOT be skipped.
var allowDotDirs = map[string]bool{
	".github": true,
}

// extensionToLanguage maps file extensions to languages.
var extensionToLanguage = map[string]Language{
	".go":  LangGo,
	".js":  LangJavaScript,
	".jsx": LangJavaScript,
	".ts":  LangTypeScript,
	".tsx": LangTypeScript,
	".py":  LangPython,
}

// WalkDirectory walks a directory tree and returns files grouped by language.
func WalkDirectory(root string) (map[Language][]string, error) {
	result := make(map[Language][]string)

	// WalkDir (not Walk) does not follow symlinks. The explicit symlink/
	// irregular skip below additionally guards against Windows directory
	// junctions, which filepath.Walk follows into infinite recursion (Go #17540).
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't read
		}

		// Never descend into symlinks or reparse points (Windows junctions).
		if d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if skipDirs[name] {
				return filepath.SkipDir
			}
			// Skip dot-dirs unless in the allow list
			if strings.HasPrefix(name, ".") && name != "." && !allowDotDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if lang, ok := extensionToLanguage[ext]; ok && !shouldSkipSourceFile(path) {
			// Still include test files — useful for test detection
			result[lang] = append(result[lang], path)
		}

		return nil
	})

	return result, err
}

func shouldSkipSourceFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(base, ".min.js") || strings.HasSuffix(base, ".min.jsx")
}
