package template

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// sourceNodeID is a preorder identity scoped to one parsed collection. Paths
// describe the repaired HTML tree, not byte offsets in the input. Element path
// indices count siblings of the same name/namespace, ignoring formatting text.
type sourceNodeID uint64

type sourceRef struct {
	name       string
	templateID string
	nodeID     sourceNodeID
	path       string
	stylesheet int // Additional sheet order (one-based); zero for HTML sources.
}

type frameID uint64

// templateCollection is an unsized source model, not a compiled catalog.
// It owns the parse tree and its indices. Treat every node, attribute and slice
// as read-only after publication; later styling must clone active documents.
type templateCollection struct {
	document  *html.Node
	html      *html.Node
	head      *html.Node
	body      *html.Node
	styles    []*html.Node // Direct head styles, unparsed and in source order.
	templates []*templateSource
	byID      map[string]*templateSource
	sources   map[*html.Node]sourceRef
}

type templateSource struct {
	id      string
	element *html.Node // Storage template, excluded from future active ancestry.
	page    *html.Node
	frames  []frameSource
}

type frameSource struct {
	id      frameID // One-based source order, scoped to this template.
	element *html.Node
}

// parseTemplateCollection parses HTML and validates only the structural profile.
// CSS text and style attributes remain uninterpreted. On failure it returns no
// partial collection; diagnostics are ordered by source node, then check order.
func parseTemplateCollection(r io.Reader, sourceName string) (*templateCollection, error) {
	input := sourceRef{name: sourceName}
	if r == nil {
		return nil, newLoadError([]diagnostic{{
			stage: stageParse, code: codeNilInput, source: input,
			message: "an HTML reader is required",
		}})
	}
	document, err := html.Parse(r)
	if err != nil {
		return nil, newLoadError([]diagnostic{{
			stage: stageParse, code: codeParseFailure, source: input,
			message: "cannot parse collection HTML", cause: err,
		}})
	}
	c := &templateCollection{
		document: document,
		byID:     make(map[string]*templateSource),
		sources:  make(map[*html.Node]sourceRef),
	}
	c.html = elementChild(document, atom.Html)
	c.head = elementChild(c.html, atom.Head)
	c.body = elementChild(c.html, atom.Body)
	c.indexSources(sourceName)

	v := sourceValidator{collection: c}
	if v.validateDocument() {
		v.discoverTemplates()
	}
	if len(v.diagnostics) != 0 {
		slices.SortStableFunc(v.diagnostics, func(a, b diagnostic) int {
			return cmp.Compare(a.source.nodeID, b.source.nodeID)
		})
		return nil, newLoadError(v.diagnostics)
	}
	return c, nil
}

func (c *templateCollection) indexSources(name string) {
	var next sourceNodeID
	var visit func(*html.Node, string, string)
	visit = func(n *html.Node, path, templateID string) {
		next++
		if isPageTemplate(n) {
			templateID, _ = attribute(n, "id")
		}
		c.sources[n] = sourceRef{name: name, templateID: templateID, nodeID: next, path: path}
		counts := make(map[string]int)
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			label := pathLabel(child)
			counts[label]++
			childPath := fmt.Sprintf("%s/%s[%d]", strings.TrimSuffix(path, "/"), label, counts[label])
			visit(child, childPath, templateID)
		}
	}
	visit(c.document, "/", "")
}

func pathLabel(n *html.Node) string {
	switch n.Type {
	case html.ElementNode:
		if n.Namespace != "" {
			return n.Namespace + ":" + n.Data
		}
		return n.Data
	case html.TextNode:
		return "text()"
	case html.CommentNode:
		return "comment()"
	case html.DoctypeNode:
		return "doctype()"
	default:
		return "node()"
	}
}

func isElement(n *html.Node, tag atom.Atom) bool {
	return n != nil && n.Type == html.ElementNode && n.Namespace == "" && n.DataAtom == tag
}

func elementChild(n *html.Node, tag atom.Atom) *html.Node {
	if n != nil {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if isElement(child, tag) {
				return child
			}
		}
	}
	return nil
}

// attribute follows source order and uses the first unqualified attribute.
// Parsing/HTML recovery, rather than this profile, determines the input tree.
func attribute(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func isPageTemplate(n *html.Node) bool {
	if !isElement(n, atom.Template) {
		return false
	}
	kind, _ := attribute(n, "data-type")
	return kind == "page"
}

// HTML's ASCII whitespace is deliberately narrower than Unicode whitespace.
// For example, a nonbreaking space is content, not frame indentation.
const htmlWhitespace = " \t\n\f\r"

func isFormatting(n *html.Node) bool {
	return n.Type == html.CommentNode ||
		n.Type == html.TextNode && strings.Trim(n.Data, htmlWhitespace) == ""
}

func validID(id string) bool {
	return id != "" && !strings.ContainsAny(id, htmlWhitespace)
}
