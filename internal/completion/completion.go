package completion

import (
	"fmt"
	"slices"
	"strings"

	"github.com/vikranthBala/esi-lsp/internal/analyzer"
	"github.com/vikranthBala/esi-lsp/internal/parser"
)

const (
	kindSnippet  = 15 // for tag completions
	kindField    = 5  // for attribute completions
	kindVariable = 6  // for ESI variable completions
)

// snippets defines the insert text for each tag kind.
var snippets = map[parser.NodeKind]string{
	"esi:include":   `<esi:include src="$1" />`,
	"esi:remove":    "<esi:remove>$1</esi:remove>",
	"esi:comment":   `<esi:comment text="$1" />`,
	"esi:vars":      "<esi:vars>$1</esi:vars>",
	"esi:assign":    `<esi:assign name="$1" value="$2" />`,
	"esi:eval":      `<esi:eval src="$1" />`,
	"esi:choose":    "<esi:choose>\n  <esi:when test=\"$1\">$2</esi:when>\n  <esi:otherwise>$3</esi:otherwise>\n</esi:choose>",
	"esi:when":      `<esi:when test="$1">$2</esi:when>`,
	"esi:otherwise": "<esi:otherwise>$1</esi:otherwise>",
	"esi:try":       "<esi:try>\n  <esi:attempt>$1</esi:attempt>\n  <esi:except>$2</esi:except>\n</esi:try>",
	"esi:attempt":   "<esi:attempt>$1</esi:attempt>",
	"esi:except":    "<esi:except>$1</esi:except>",
	"esi:inline":    `<esi:inline name="$1" fetchable="$2">$3</esi:inline>`,
	"esi:function":  `<esi:function name="$1">$2</esi:function>`,
	"esi:text":      "<esi:text>$1</esi:text>",
}

type Item struct {
	Label            string `json:"label"`
	Kind             int    `json:"kind"`
	Detail           string `json:"detail"`
	Documentation    string `json:"documentation"`
	InsertText       string `json:"insertText"`
	InsertTextFormat int    `json:"insertTextFormat"` // 1=plain, 2=snippet
}

// Given the cursor position, get the text on the current line up to the cursor
func lineUpToCursor(source string, pos parser.Position) string {
	lines := strings.Split(source, "\n")
	if pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	if pos.Character > len(line) {
		return line
	}
	return line[:pos.Character]
}

func tagCompletions() []Item {
	items := make([]Item, 0, len(analyzer.TagRules))
	for kind, meta := range analyzer.TagRules {
		snippet, ok := snippets[kind]
		if !ok {
			snippet = fmt.Sprintf("<%s>$1</%s>", kind, kind)
		}
		items = append(items, Item{
			Label:            string(kind),
			Kind:             kindSnippet,
			Detail:           meta.Summary,
			InsertText:       snippet,
			InsertTextFormat: 2,
		})
	}
	return items
}

func attrCompletions(kind parser.NodeKind) []Item {
	meta, ok := analyzer.TagRules[kind]
	if !ok {
		return nil
	}
	items := make([]Item, 0, len(meta.AllowedAttrs))
	for _, attr := range meta.AllowedAttrs {
		doc := meta.AttrDocs[attr]
		detail := "optional"
		if slices.Contains(meta.RequiredAttrs, attr) {
			detail = "required"
		}
		items = append(items, Item{
			Label:            attr,
			Kind:             kindField,
			Detail:           detail,
			Documentation:    doc,
			InsertText:       fmt.Sprintf(`%s="$1"`, attr),
			InsertTextFormat: 2,
		})
	}
	return items
}

func varCompletions() []Item {
	vars := []struct {
		name    string
		snippet string
		doc     string
	}{
		{"HTTP_COOKIE", `$(HTTP_COOKIE{$1})`, "Cookie value by name"},
		{"QUERY_STRING", `$(QUERY_STRING{$1})`, "Query string parameter by name"},
		{"HTTP_HOST", `$(HTTP_HOST)`, "Request Host header"},
		{"REQUEST_PATH", `$(REQUEST_PATH)`, "Request path"},
		{"REQUEST_METHOD", `$(REQUEST_METHOD)`, "HTTP method"},
		{"HTTP_ACCEPT_LANGUAGE", `$(HTTP_ACCEPT_LANGUAGE)`, "Accept-Language header"},
		{"GEO{country_code}", `$(GEO{country_code})`, "Akamai GeoIP country code"},
		{"GEO{region_code}", `$(GEO{region_code})`, "Akamai GeoIP region code"},
		{"USER_AGENT", `$(USER_AGENT)`, "User-Agent string"},
	}
	items := make([]Item, 0, len(vars))
	for _, v := range vars {
		items = append(items, Item{
			Label:            "$(" + v.name + ")",
			Kind:             kindVariable,
			Detail:           v.doc,
			InsertText:       v.snippet,
			InsertTextFormat: 2,
		})
	}
	return items
}

func Complete(doc *parser.Document, pos parser.Position) []Item {
	if doc == nil {
		return nil
	}

	lineText := lineUpToCursor(doc.Source, pos)

	// case 1 — tag completion
	if strings.HasSuffix(lineText, "<") ||
		strings.HasSuffix(lineText, "<esi") ||
		strings.HasSuffix(lineText, "<esi:") {
		return tagCompletions()
	}

	// case 2 — attribute completion
	ind := strings.LastIndex(lineText, "<esi:")
	if ind != -1 {
		// TODO: filter out already-typed attributes from suggestions
		tagKind := strings.SplitN(lineText[ind:], " ", 2)[0]
		return attrCompletions(parser.NodeKind(tagKind))
	}

	// case 3 — variable completion inside an attribute value
	_, attr := doc.AttrAt(pos)
	if attr != nil && parser.RangeContains(attr.ValueRange, pos) {
		return varCompletions()
	}

	return nil
}
