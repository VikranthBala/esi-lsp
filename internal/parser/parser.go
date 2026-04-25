package parser

import (
	"sort"
	"strings"
)

var knownTags = map[string]NodeKind{
	"include":   "esi:include",
	"remove":    "esi:remove",
	"comment":   "esi:comment",
	"vars":      "esi:vars",
	"assign":    "esi:assign",
	"eval":      "esi:eval",
	"choose":    "esi:choose",
	"when":      "esi:when",
	"otherwise": "esi:otherwise",
	"try":       "esi:try",
	"attempt":   "esi:attempt",
	"except":    "esi:except",
	"inline":    "esi:inline",
	"function":  "esi:function",
	"text":      "esi:text",
}

type parser struct {
	uri       string
	source    string
	lineIndex []int
	doc       *Document
}

func buildLineIndex(src string) []int {
	ind := []int{0}
	for i, char := range src {
		if char == '\n' {
			ind = append(ind, i+1)
		}
	}
	return ind
}

func ParseDocument(uri, source string) *Document {
	p := &parser{
		uri:       uri,
		source:    source,
		lineIndex: buildLineIndex(source),
		doc:       &Document{URI: uri},
	}
	p.parse()
	return p.doc
}

func (p *parser) offsetToPosition(offset int) Position {
	// find first line that starts AFTER offset
	line := sort.Search(len(p.lineIndex), func(i int) bool {
		return p.lineIndex[i] > offset
	}) - 1 // step back — that's the line containing offset

	// guard: offset before any newline, or sort.Search returns 0
	if line < 0 {
		line = 0
	}

	return Position{Line: line, Character: offset - p.lineIndex[line]}
}

func (p *parser) offsetToRange(startOffset, endOffset int) Range {
	return Range{
		Start: p.offsetToPosition(startOffset),
		End:   p.offsetToPosition(endOffset),
	}
}

// returns the fragment string and the end offset in source
func (p *parser) extractFragment(startOffset int) (string, int) {
	depth := 0
	i := startOffset

	for i < len(p.source) {
		if p.source[i] != '<' {
			i++
			continue
		}

		// find the end of this tag
		relEnd := strings.IndexByte(p.source[i:], '>')
		if relEnd < 0 {
			// no closing '>' — malformed, add parse error and bail
			p.doc.Errors = append(p.doc.Errors, ParseError{
				Range:    p.offsetToRange(i, i+1),
				Message:  "unclosed tag",
				Severity: 1,
			})
			return p.source[startOffset:i], i
		}
		tagEnd := i + relEnd + 1  // absolute offset, past the '>'
		tag := p.source[i:tagEnd] // the raw tag text e.g. "<esi:include src=... />"

		if strings.HasPrefix(tag, "</esi:") {
			// closing tag
			depth--
			if depth == 0 {
				return p.source[startOffset:tagEnd], tagEnd
			}
		} else if strings.HasPrefix(tag, "<esi:") {
			// opening tag — is it self closing?
			selfClosing := strings.HasSuffix(tag, "/>")
			if selfClosing {
				if depth == 0 {
					return p.source[startOffset:tagEnd], tagEnd
				}
				// self-closing inside a block — don't increment depth
			} else {
				depth++
			}
		}

		i = tagEnd
	}

	// reached end of source without closing — unclosed tag error
	p.doc.Errors = append(p.doc.Errors, ParseError{
		Range:    p.offsetToRange(startOffset, len(p.source)),
		Message:  "unclosed ESI tag",
		Severity: 1,
	})
	return p.source[startOffset:], len(p.source)
}
