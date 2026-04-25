package analyzer

import (
	"slices"

	"github.com/vikranthBala/esi-lsp/internal/parser"
)

type tagMeta struct {
	requiredAttrs []string
	allowedAttrs  []string
	validParents  []parser.NodeKind // empty means "any parent or no parent is fine"
}

var tagRules = map[parser.NodeKind]tagMeta{
	"esi:include": {
		requiredAttrs: []string{"src"},
		allowedAttrs:  []string{"src", "alt", "onerror", "maxwait", "ttl"},
		validParents:  []parser.NodeKind{},
	},
	"esi:remove": {
		requiredAttrs: []string{},
		allowedAttrs:  []string{},
		validParents:  []parser.NodeKind{},
	},
	"esi:comment": {
		requiredAttrs: []string{"text"},
		allowedAttrs:  []string{"text"},
		validParents:  []parser.NodeKind{},
	},
	"esi:vars": {
		requiredAttrs: []string{},
		allowedAttrs:  []string{"name"},
		validParents:  []parser.NodeKind{},
	},
	"esi:assign": {
		requiredAttrs: []string{"name", "value"},
		allowedAttrs:  []string{"name", "value"},
		validParents:  []parser.NodeKind{},
	},
	"esi:eval": {
		requiredAttrs: []string{},
		allowedAttrs:  []string{"src", "maxwait"},
		validParents:  []parser.NodeKind{},
	},
	"esi:choose": {
		requiredAttrs: []string{},
		allowedAttrs:  []string{},
		validParents:  []parser.NodeKind{},
	},
	"esi:when": {
		requiredAttrs: []string{"test"},
		allowedAttrs:  []string{"test"},
		validParents:  []parser.NodeKind{"esi:choose"},
	},
	"esi:otherwise": {
		requiredAttrs: []string{},
		allowedAttrs:  []string{},
		validParents:  []parser.NodeKind{"esi:choose"},
	},
	"esi:try": {
		requiredAttrs: []string{},
		allowedAttrs:  []string{},
		validParents:  []parser.NodeKind{},
	},
	"esi:attempt": {
		requiredAttrs: []string{},
		allowedAttrs:  []string{},
		validParents:  []parser.NodeKind{"esi:try"},
	},
	"esi:except": {
		requiredAttrs: []string{},
		allowedAttrs:  []string{},
		validParents:  []parser.NodeKind{"esi:try"},
	},
	"esi:inline": {
		requiredAttrs: []string{"name", "fetchable"},
		allowedAttrs:  []string{"name", "fetchable"},
		validParents:  []parser.NodeKind{},
	},
	"esi:function": {
		requiredAttrs: []string{"name"},
		allowedAttrs:  []string{"name"},
		validParents:  []parser.NodeKind{},
	},
	"esi:text": {
		requiredAttrs: []string{},
		allowedAttrs:  []string{},
		validParents:  []parser.NodeKind{},
	},
}

func Analyze(doc *parser.Document) []parser.Diagnostic {
	// loop over all the nodes in the document

	result := make([]parser.Diagnostic, 0)
	for _, node := range doc.All {
		result = append(result, checkRequiredAttrs(node)...)
		result = append(result, checkUnknownAttrs(node)...)
	}
	return result
}

func checkRequiredAttrs(node *parser.Node) []parser.Diagnostic {
	nodeAttrs, known := tagRules[node.Kind]
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
				Message:  "missing required attribute",
				Severity: 1,
			})
		}
	}
	return result
}

func checkUnknownAttrs(node *parser.Node) []parser.Diagnostic {
	nodeAttrs, known := tagRules[node.Kind]
	if !known {
		return nil
	}

	result := make([]parser.Diagnostic, 0)

	// first check required attributes
	for _, attr := range node.Attrs {
		ok := slices.Contains(nodeAttrs.allowedAttrs, attr.Name)
		if !ok {
			result = append(result, parser.Diagnostic{
				Range:    node.OpenRange,
				Message:  "unknown attribute",
				Severity: 1,
			})
		}
	}
	return result
}
