package parser

type NodeKind string // represents the tag name like "esi:include"

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// represents one attribute on a tag
type Attribute struct {
	Name       string
	Value      string
	NameRange  Range
	ValueRange Range
}

// represents one esi element
type Node struct {
	Kind       NodeKind
	Range      Range
	OpenRange  Range // just the opening tag <esi:choose>
	CloseRange Range
	Attrs      []Attribute
	Children   []*Node
	Parent     *Node // nil if top level
}

type Document struct {
	URI    string
	Source string
	Nodes  []*Node // top level nodes only
	All    []*Node // every node, flat list — for easy searching
	Errors []Diagnostic
}

type Diagnostic struct {
	Range    Range
	Message  string
	Severity int // 1= error, 2=warning, 3=info
}

// bascially does a character count
// assuming, each line contains 10k character
// total number of characters in tha range is lines * 10k + diff(start char, end char)
func rangeSize(r Range) int {
	lines := r.End.Line - r.Start.Line
	chars := r.End.Character - r.Start.Character

	return lines*10000 + chars
}

// simple helper function to see if p is in r
func RangeContains(r Range, p Position) bool {
	// before start line, or after end line
	if p.Line < r.Start.Line || p.Line > r.End.Line {
		return false
	}
	// on the start line — must be at or after start character
	if p.Line == r.Start.Line && p.Character < r.Start.Character {
		return false
	}
	// on the end line — must be at or before end character
	if p.Line == r.End.Line && p.Character > r.End.Character {
		return false
	}
	return true
}

func (d *Document) NodeAt(pos Position) *Node {
	// node of interest
	var noi *Node
	for _, node := range d.All {
		// if the position is in the range
		if RangeContains(node.Range, pos) {
			// as noi is already having the pos and node is also having the pos
			// i.e, if node is in noi, then the rangesize of node must be less than noi
			if noi == nil || rangeSize(node.Range) < rangeSize(noi.Range) {
				noi = node
			}
		}
	}
	return noi
}

func (d *Document) AttrAt(pos Position) (*Node, *Attribute) {
	// node of interest
	noi := d.NodeAt(pos)
	if noi == nil {
		return nil, nil
	}

	// pos can only be on one attr
	for i := range noi.Attrs {
		attr := &noi.Attrs[i] // pointer into the actual slice
		if RangeContains(attr.NameRange, pos) || RangeContains(attr.ValueRange, pos) {
			return noi, attr
		}
	}
	return noi, nil // on the node but not on any attribute
}
