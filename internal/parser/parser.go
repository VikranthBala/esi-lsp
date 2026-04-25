package parser

import "sort"

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
