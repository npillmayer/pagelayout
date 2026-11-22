package template

import (
	"slices"

	"golang.org/x/net/html"
)

// activeTemplate is a private working document for one definition's cascade.
// Every node and attribute slice is owned by this instance. The source model
// stays unchanged, and source references still name the original collection.
type activeTemplate struct {
	id                         string
	document, html, body, page *html.Node
	frames                     []frameSource
	sources                    map[*html.Node]sourceRef
}

func activateTemplate(c *templateCollection, definition *templateSource) *activeTemplate {
	a := &activeTemplate{id: definition.id, sources: make(map[*html.Node]sourceRef)}
	clones := make(map[*html.Node]*html.Node)
	shallow := func(source *html.Node) *html.Node {
		n := &html.Node{Type: source.Type, DataAtom: source.DataAtom, Data: source.Data, Namespace: source.Namespace, Attr: slices.Clone(source.Attr)}
		a.sources[n] = c.sources[source]
		clones[source] = n
		return n
	}
	var clone func(*html.Node) *html.Node
	clone = func(source *html.Node) *html.Node {
		n := shallow(source)
		for child := source.FirstChild; child != nil; child = child.NextSibling {
			n.AppendChild(clone(child))
		}
		return n
	}
	a.document = shallow(c.document)
	a.html = shallow(c.html)
	a.body = shallow(c.body)
	a.page = clone(definition.page)
	a.document.AppendChild(a.html)
	a.html.AppendChild(a.body)
	a.body.AppendChild(a.page)
	for _, frame := range definition.frames {
		a.frames = append(a.frames, frameSource{id: frame.id, element: clones[frame.element]})
	}
	return a
}
