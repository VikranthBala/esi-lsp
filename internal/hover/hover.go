package hover

import (
	"fmt"
	"slices"
	"strings"

	"github.com/vikranthBala/esi-lsp/internal/analyzer"
	"github.com/vikranthBala/esi-lsp/internal/parser"
)

type HoverResult struct {
	Contents MarkupContent `json:"contents"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Hover returns the hover result for the given position, or nil if nothing to show
func Hover(doc *parser.Document, pos parser.Position) *HoverResult {
	if doc == nil {
		return nil
	}

	node, attr := doc.AttrAt(pos)
	if node == nil {
		return nil
	}

	// meta for the node
	meta, known := analyzer.TagRules[node.Kind]
	if !known {
		return nil
	}

	res := &HoverResult{
		Contents: MarkupContent{
			Kind: "markdown",
		},
	}
	if attr != nil {
		res.Contents.Value = attrHover(attr.Name, meta)
	} else {
		res.Contents.Value = tagHover(node, meta)
	}
	return res
}

// tagHover builds markdown for a whole tag
func tagHover(node *parser.Node, meta analyzer.TagMeta) string {
	builder := strings.Builder{}

	builder.WriteString("### " + string(node.Kind) + "\n\n")

	builder.WriteString(meta.Summary + "\n\n")
	builder.WriteString("**Attributes**\n")
	builder.WriteString("| Attribute | Required | Description |\n|-----------|----------|-------------|\n")
	for _, attr := range meta.AllowedAttrs {
		required := "✗"
		if slices.Contains(meta.RequiredAttrs, attr) {
			required = "✓"
		}
		fmt.Fprintf(&builder, "| %s | %s | %s |\n", attr, required, meta.AttrDocs[attr])
	}
	return builder.String()
}

// attrHover builds markdown for a single attribute
func attrHover(attrName string, meta analyzer.TagMeta) string {
	// return
	builder := strings.Builder{}
	required := "*(required)*"
	if !slices.Contains(meta.RequiredAttrs, attrName) {
		required = ""
	}
	line := fmt.Sprintf("**%s** %s", attrName, required)
	builder.WriteString(strings.TrimSpace(line) + "\n\n")
	builder.WriteString(meta.AttrDocs[attrName])
	return builder.String()
}
