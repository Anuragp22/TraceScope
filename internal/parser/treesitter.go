package parser

import (
	"context"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
)

// parseWithTreeSitter creates a parser, sets the language, and parses with a 30s timeout.
// Returns the tree (caller must defer tree.Close()).
func parseWithTreeSitter(lang *sitter.Language, source []byte) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	defer parser.Close()

	parser.SetLanguage(lang)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return parser.ParseCtx(ctx, nil, source)
}
