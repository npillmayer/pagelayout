package content

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type sourceID uint64

// These are parse-time categories. Display suppression and flow ownership need
// later effective-style/projection checks; content here means candidate material.
type sourceKind uint8

const (
	sourceContext sourceKind = iota // document and html/head/body context
	sourceContent
	sourceMetadata
	sourceInert
	sourceStylesheet
)

type sourceNode struct {
	ref      sourceRef
	parent   sourceID
	children []sourceID
	node     *html.Node
	kind     sourceKind
	text     string // Decoded text-node bytes, not HTML offsets or processed text.
}

// sourceDocument owns a complete recovered parse tree and its preorder ledger.
// Treat both as read-only. Slot zero is reserved for an absent source identity.
type sourceDocument struct {
	document, html, head, body *html.Node
	nodes                      []*sourceNode
	byNode                     map[*html.Node]*sourceNode
}

func parseSource(r io.Reader, name string) (*sourceDocument, error) {
	input := sourceRef{name: name}
	fail := func(code, message string, cause error) (*sourceDocument, error) {
		return nil, newLoadError([]diagnostic{{stage: "input", code: code, source: input, message: message, cause: cause}})
	}
	if r == nil {
		return fail("input.nil", "an HTML reader is required", nil)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fail("input.read", "cannot read content HTML", err)
	}
	if !utf8.Valid(data) {
		return fail("input.encoding", "content HTML must be UTF-8", nil)
	}
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return fail("input.parse", "cannot parse content HTML", err)
	}
	s := &sourceDocument{document: doc, nodes: []*sourceNode{nil}, byNode: make(map[*html.Node]*sourceNode)}
	s.html = elementChild(doc, atom.Html)
	s.head = elementChild(s.html, atom.Head)
	s.body = elementChild(s.html, atom.Body)
	if s.html == nil || s.head == nil || s.body == nil {
		return fail("input.document-structure", "content requires an HTML document with a body (framesets are unsupported)", nil)
	}
	s.indexSources(name)
	return s, nil
}

func (s *sourceDocument) indexSources(name string) {
	var visit func(*html.Node, sourceID, string, bool, sourceKind)
	visit = func(n *html.Node, parent sourceID, path string, inBody bool, inherited sourceKind) {
		inBody = inBody || n == s.body
		kind := sourceMetadata
		switch {
		case inherited == sourceInert:
			kind = sourceInert
		case inherited == sourceMetadata || inherited == sourceStylesheet:
			kind = sourceMetadata
		case n == s.document || n == s.html || n == s.head || n == s.body:
			kind = sourceContext
		case isElement(n, atom.Template):
			kind = sourceInert
		case isElement(n, atom.Style) && n.Parent == s.head:
			kind = sourceStylesheet
		case isMetadataElement(n):
			kind = sourceMetadata
		case inBody && (n.Type == html.ElementNode || n.Type == html.TextNode):
			kind = sourceContent
		}
		id := sourceID(len(s.nodes))
		record := &sourceNode{ref: sourceRef{name: name, nodeID: id, path: path}, parent: parent, node: n, kind: kind}
		if n.Type == html.TextNode {
			record.text = n.Data
		}
		s.nodes = append(s.nodes, record)
		s.byNode[n] = record
		counts := make(map[string]int)
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			label := pathLabel(child)
			counts[label]++
			childPath := fmt.Sprintf("%s/%s[%d]", strings.TrimSuffix(path, "/"), label, counts[label])
			record.children = append(record.children, sourceID(len(s.nodes)))
			visit(child, id, childPath, inBody, kind)
		}
	}
	visit(s.document, 0, "/", false, sourceContext)
}

func isMetadataElement(n *html.Node) bool {
	if n.Type != html.ElementNode || n.Namespace != "" {
		return false
	}
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Title, atom.Meta, atom.Link, atom.Base:
		return true
	}
	return false
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
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if isElement(c, tag) {
				return c
			}
		}
	}
	return nil
}

func attribute(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

const htmlWhitespace = " \t\n\f\r"
