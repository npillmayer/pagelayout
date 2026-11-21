package template

import (
	"fmt"
	"maps"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type sourceValidator struct {
	collection  *templateCollection
	diagnostics []diagnostic
	contextIDs  map[string]*html.Node
}

func (v *sourceValidator) report(stage loadStage, code diagnosticCode, n *html.Node, format string, args ...any) {
	v.diagnostics = append(v.diagnostics, diagnostic{
		stage: stage, code: code, source: v.collection.sources[n],
		message: fmt.Sprintf(format, args...),
	})
}

func (v *sourceValidator) validateDocument() bool {
	c := v.collection
	// The parser inserts omitted html/head/body tags for ordinary documents, but
	// a frameset document, for example, has no body and is outside this profile.
	if c.html == nil || c.head == nil || c.body == nil {
		v.report(stageStructure, codeDocumentStructure, c.document, "collection requires an HTML document with head and body")
		return false
	}
	// Only html/body attributes are copied into the future active documents.
	v.contextIDs = make(map[string]*html.Node)
	v.validateElementID(c.html, v.contextIDs)
	v.validateElementID(c.body, v.contextIDs)
	for n := c.head.FirstChild; n != nil; n = n.NextSibling {
		switch {
		case isFormatting(n), isElement(n, atom.Meta), isElement(n, atom.Title):
		case isElement(n, atom.Style):
			c.styles = append(c.styles, n)
		case isPageTemplate(n):
			// Discovery diagnoses marked templates outside the body at any depth.
		default:
			v.report(stageStructure, codeHeadContent, n, "head accepts only metadata (title/meta) and direct style elements")
		}
	}
	for n := c.body.FirstChild; n != nil; n = n.NextSibling {
		if isFormatting(n) || isElement(n, atom.Template) {
			continue
		}
		v.report(stageStructure, codeBodyContent, n, "body accepts only template elements and formatting whitespace/comments")
	}
	return true
}

func (v *sourceValidator) discoverTemplates() {
	c := v.collection
	seen := make(map[string]*html.Node)
	marked := 0
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if isPageTemplate(n) {
			marked++
			if n.Parent != c.body {
				v.report(stageDiscovery, codeTemplateLocation, n, "page templates must be direct body children")
			} else {
				v.discoverTemplate(n, seen)
			}
		}
		// Include otherwise ignored storage templates so a misplaced marked page
		// definition cannot silently disappear inside one of them.
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(c.document)
	if marked == 0 && len(v.diagnostics) == 0 {
		v.report(stageDiscovery, codeEmptyCollection, c.body, "collection contains no page templates")
	}
}

func (v *sourceValidator) discoverTemplate(n *html.Node, seen map[string]*html.Node) {
	before := len(v.diagnostics)
	id, exists := attribute(n, "id")
	switch {
	case !exists || id == "":
		v.report(stageDiscovery, codeMissingTemplateID, n, "page template requires a nonempty id")
	case !validID(id):
		v.report(stageDiscovery, codeInvalidTemplateID, n, "template id %q contains HTML whitespace", id)
	case seen[id] != nil:
		v.report(stageDiscovery, codeDuplicateTemplateID, n, "template id %q was already used at %s", id, v.collection.sources[seen[id]].path)
	default:
		seen[id] = n
	}
	t := v.validateTemplate(n)
	if t != nil && len(v.diagnostics) == before {
		t.id = id
		v.collection.templates = append(v.collection.templates, t)
		v.collection.byID[id] = t
	}
}

func (v *sourceValidator) validateTemplate(n *html.Node) *templateSource {
	var elements []*html.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if isFormatting(child) {
			continue
		}
		if child.Type == html.ElementNode {
			elements = append(elements, child)
		} else {
			v.report(stageStructure, codeTemplateContent, child, "template content must be one page DIV and formatting whitespace/comments")
		}
	}
	if len(elements) == 0 {
		v.report(stageStructure, codeMissingPage, n, "template requires one page DIV")
		return nil
	}
	if len(elements) != 1 {
		v.report(stageStructure, codeMultiplePages, n, "template requires exactly one page DIV, found %d element children", len(elements))
		return nil
	}
	page := elements[0]
	if !isElement(page, atom.Div) {
		v.report(stageStructure, codePageElement, page, "page root must be an HTML DIV")
		return nil
	}
	ids := maps.Clone(v.contextIDs)
	v.validateElementID(page, ids)
	t := &templateSource{element: n, page: page}
	for child := page.FirstChild; child != nil; child = child.NextSibling {
		if isFormatting(child) {
			continue
		}
		if child.Type != html.ElementNode {
			v.report(stageStructure, codePageContent, child, "page content must be frame DIVs and formatting whitespace/comments")
			continue
		}
		if !isElement(child, atom.Div) {
			v.report(stageStructure, codeFrameElement, child, "frame must be a direct HTML DIV child of the page")
			continue
		}
		v.validateElementID(child, ids)
		t.frames = append(t.frames, frameSource{id: frameID(len(t.frames) + 1), element: child})
		for content := child.FirstChild; content != nil; content = content.NextSibling {
			if !isFormatting(content) {
				v.report(stageStructure, codeFrameContent, content, "frame must be empty except for formatting whitespace/comments")
			}
		}
	}
	return t
}

// Validate IDs in the future active document, excluding storage template IDs and
// head elements. Each definition gets its own copy of the html/body ID context.
func (v *sourceValidator) validateElementID(n *html.Node, seen map[string]*html.Node) {
	id, exists := attribute(n, "id")
	if !exists {
		return
	}
	if !validID(id) {
		v.report(stageStructure, codeInvalidElementID, n, "element id must be nonempty and contain no HTML whitespace")
		return
	}
	if first := seen[id]; first != nil {
		v.report(stageStructure, codeDuplicateElementID, n, "element id %q would duplicate %s in the active document", id, v.collection.sources[first].path)
		return
	}
	seen[id] = n
}
