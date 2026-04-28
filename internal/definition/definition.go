package definition

import "github.com/vikranthBala/esi-lsp/internal/parser"

type Location struct {
	URI   string       `json:"uri"`
	Range parser.Range `json:"range"`
}

func attrValue(node *parser.Node, name string) string {
	for _, attr := range node.Attrs {
		if attr.Name == name {
			return attr.Value
		}
	}
	return ""
}

// Definition returns the location of the declaration for the symbol at pos
// TODO: this only supports only one use case, this is fine for mvp
// update the behavior once the lsp matures
func Definition(doc *parser.Document, pos parser.Position) *Location {
	if doc == nil {
		return nil
	}
	node := doc.NodeAt(pos)
	if node == nil || node.Kind != "esi:include" {
		return nil
	}
	v := attrValue(node, "src")
	if v == "" {
		return nil
	}
	for _, n := range doc.All {
		if n.Kind == "esi:inline" {
			if attrValue(n, "name") == v {
				return &Location{
					URI:   doc.URI,
					Range: n.OpenRange,
				}
			}
		}
	}
	return nil
}
