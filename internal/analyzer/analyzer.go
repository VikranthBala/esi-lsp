package analyzer

import (
	"fmt"
	"slices"

	"github.com/vikranthBala/esi-lsp/internal/parser"
)

type TagMeta struct {
	requiredAttrs []string
	allowedAttrs  []string
	validParents  []parser.NodeKind // empty means "any parent or no parent is fine"
	summary       string
	attrDocs      map[string]string
}

func Analyze(doc *parser.Document) []parser.Diagnostic {
	// loop over all the nodes in the document

	result := make([]parser.Diagnostic, 0)
	for _, node := range doc.All {
		result = append(result, checkRequiredAttrs(node)...)
		result = append(result, checkUnknownAttrs(node)...)
		result = append(result, checkNesting(node)...)
		result = append(result, checkChooseHasWhen(node)...)
	}
	return result
}

func checkRequiredAttrs(node *parser.Node) []parser.Diagnostic {
	nodeAttrs, known := TagRules[node.Kind]
	if !known {
		return nil
	}

	result := make([]parser.Diagnostic, 0)

	// first check required attributes
	for _, req := range nodeAttrs.requiredAttrs {
		found := false
		for _, attr := range node.Attrs {
			if attr.Name == req {
				found = true
			}
		}
		if !found {
			result = append(result, parser.Diagnostic{
				Range:    node.OpenRange,
				Message:  fmt.Sprintf("missing required attribute: %s", req),
				Severity: 1,
			})
		}
	}
	return result
}

func checkUnknownAttrs(node *parser.Node) []parser.Diagnostic {
	nodeAttrs, known := TagRules[node.Kind]
	if !known {
		return nil
	}

	result := make([]parser.Diagnostic, 0)

	// first check required attributes
	for _, attr := range node.Attrs {
		ok := slices.Contains(nodeAttrs.allowedAttrs, attr.Name)
		if !ok {
			result = append(result, parser.Diagnostic{
				Range:    attr.NameRange,
				Message:  fmt.Sprintf("unknown attribute: %s", attr.Name),
				Severity: 1,
			})
		}
	}
	return result
}

func checkNesting(node *parser.Node) []parser.Diagnostic {
	meta, known := TagRules[node.Kind]
	if !known || len(meta.validParents) == 0 {
		return nil
	}

	if node.Parent == nil {
		return []parser.Diagnostic{{
			Range:    node.OpenRange,
			Message:  fmt.Sprintf("%s must be inside %v", node.Kind, meta.validParents),
			Severity: 1,
		}}
	}

	if !slices.Contains(meta.validParents, node.Parent.Kind) {
		return []parser.Diagnostic{{
			Range:    node.OpenRange,
			Message:  fmt.Sprintf("%s must be inside %v, found inside %s", node.Kind, meta.validParents, node.Parent.Kind),
			Severity: 1,
		}}
	}

	return nil
}

func checkChooseHasWhen(node *parser.Node) []parser.Diagnostic {
	if node.Kind != "esi:choose" {
		return nil
	}
	for _, child := range node.Children {
		if child.Kind == "esi:when" {
			return nil
		}
	}
	return []parser.Diagnostic{{
		Range:    node.OpenRange,
		Message:  "esi:choose must contain at least one esi:when",
		Severity: 1,
	}}
}
