package parser

import (
	"encoding/xml"
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
		doc:       &Document{URI: uri, Source: source},
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
	line = max(line, 0)

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
			p.doc.Errors = append(p.doc.Errors, Diagnostic{
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
	p.doc.Errors = append(p.doc.Errors, Diagnostic{
		Range:    p.offsetToRange(startOffset, len(p.source)),
		Message:  "unclosed ESI tag",
		Severity: 1,
	})
	return p.source[startOffset:], len(p.source)
}

// findNext finds the next occurrence of substr in p.source starting at from.
// Returns the absolute offset, or -1 if not found.
func (p *parser) findNext(from int, substr string) int {
	idx := strings.Index(p.source[from:], substr)
	if idx < 0 {
		return -1
	}
	return from + idx
}

func (p *parser) buildAttrs(tagStart, tagEnd int, xmlAttrs []xml.Attr) []Attribute {
	attrs := make([]Attribute, 0, len(xmlAttrs))
	cursor := tagStart

	for _, xmlAttr := range xmlAttrs {
		name := xmlAttr.Name.Local
		value := xmlAttr.Value

		// find name= starting from cursor
		nameStart := p.findNext(cursor, name)
		if nameStart < 0 {
			continue
		}
		nameEnd := nameStart + len(name)

		// find =" just after the name, then skip past it
		eqInd := p.findNext(nameEnd, `="`)
		if eqInd < 0 {
			continue
		}
		valStart := eqInd + 2 // skip ="
		valEnd := valStart + len(value)

		attrs = append(attrs, Attribute{
			Name:       name,
			Value:      value,
			NameRange:  p.offsetToRange(nameStart, nameEnd),
			ValueRange: p.offsetToRange(valStart, valEnd),
		})

		// advance cursor past this attribute so next search starts here
		cursor = valEnd
	}

	return attrs
}

func (p *parser) tokenize(fragment string, baseOffset int) []*Node {
	dec := xml.NewDecoder(strings.NewReader(fragment))
	dec.Strict = false

	var stack []*Node
	var roots []*Node
	cursor := baseOffset

	for {
		tok, err := dec.Token()
		if err != nil {
			break // EOF or error — either way we stop
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space != "esi" {
				continue // ignore non-ESI tags inside fragment
			}

			kind, known := knownTags[t.Name.Local]
			if !known {
				// add a warning but still create a node
				kind = NodeKind("esi:" + t.Name.Local)
				p.doc.Errors = append(p.doc.Errors, Diagnostic{
					Range:    p.offsetToRange(cursor, cursor+10),
					Message:  "unknown ESI tag: esi:" + t.Name.Local,
					Severity: 2,
				})
			}

			// find this tag's position in source
			tagStart := p.findNext(cursor, "<esi:"+t.Name.Local)
			tagEnd := p.findNext(tagStart, ">") + 1

			node := &Node{
				Kind:      kind,
				OpenRange: p.offsetToRange(tagStart, tagEnd),
				Range:     p.offsetToRange(tagStart, tagEnd),
			}

			// build attrs — your turn to fill this in
			node.Attrs = p.buildAttrs(tagStart, tagEnd, t.Attr)

			// wire up parent/child relationship
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				node.Parent = parent
				parent.Children = append(parent.Children, node)
			}

			// add to flat list
			p.doc.All = append(p.doc.All, node)

			stack = append(stack, node)
			cursor = tagEnd

		case xml.EndElement:
			if t.Name.Space != "esi" {
				continue
			}
			if len(stack) == 0 {
				continue
			}

			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			// find closing tag position
			closeStart := p.findNext(cursor, "</esi:"+t.Name.Local)
			closeEnd := p.findNext(closeStart, ">") + 1

			node.CloseRange = p.offsetToRange(closeStart, closeEnd)
			node.Range = Range{
				Start: node.OpenRange.Start,
				End:   node.CloseRange.End,
			}

			cursor = closeEnd

			// if stack is empty, this is a root node
			if len(stack) == 0 {
				roots = append(roots, node)
			}
		}
	}

	return roots
}

/*
	1. Start at offset 0
	2. Find next "<esi:" from current offset
	3. If not found → done
	4. Extract a balanced fragment from that position
	5. Tokenize the fragment → get root nodes
	6. Add root nodes to doc.Nodes
	7. Advance offset past the fragment
	8. Go to step 2
*/

func (p *parser) parse() {
	offset := 0
	for {
		ind := p.findNext(offset, "<esi:")
		if ind < 0 {
			break
		}
		fragment, end := p.extractFragment(ind)
		nodes := p.tokenize(fragment, ind)
		p.doc.Nodes = append(p.doc.Nodes, nodes...)
		offset = end
	}
}
