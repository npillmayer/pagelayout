package content

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/npillmayer/w3c-dom/style/cssom"
	"github.com/npillmayer/w3c-dom/styledtree"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type sourceSheet struct {
	source     sourceRef
	inspection cssom.StylesheetInspection
}

// Losing and nonmatching declarations remain available for the complete
// authored profile audit. Syntax inspection does not establish layout eligibility.
type sourceStyles struct {
	sheets []sourceSheet
	inline map[sourceID][]cssom.SourceDeclaration
}

type styledSource struct {
	source *sourceDocument
	styles *sourceStyles
	root   *styledtree.StyNode
	nodes  map[sourceID]*styledtree.StyNode
}

func styleSource(source *sourceDocument, extra []Stylesheet) (*styledSource, error) {
	styles, err := inspectStyles(source, extra)
	if err != nil {
		return nil, err
	}
	om := cssom.NewCSSOM(nil)
	for _, sheet := range styles.sheets {
		if err := om.AddStylesForScope(nil, sheet.inspection.Sheet, cssom.Author); err != nil {
			return nil, newLoadError([]diagnostic{{stage: "source-style", code: "css.registration", source: sheet.source,
				message: "cannot register content stylesheet", cause: err}})
		}
	}
	root, err := om.StyleWithOptions(source.document, cssom.StyleOptions{SkipSubtree: func(n *html.Node) bool {
		kind := source.byNode[n].kind
		return kind != sourceContent && kind != sourceContext
	}})
	if err != nil {
		return nil, newLoadError([]diagnostic{{stage: "source-style", code: "css.styling", source: source.byNode[source.html].ref,
			message: "cannot style content document", cause: err}})
	}
	result := &styledSource{source: source, styles: styles, root: root.Payload, nodes: make(map[sourceID]*styledtree.StyNode)}
	var index func(*styledtree.StyNode)
	index = func(n *styledtree.StyNode) {
		// The synthetic UA root also refers to the document, but is not source.
		if !n.Styles().IsDefaults() {
			result.nodes[source.byNode[n.HTMLNode()].ref.nodeID] = n
		}
		for _, child := range n.Children(true) {
			index(child.Payload)
		}
	}
	index(root.Payload)
	return result, nil
}

func inspectStyles(source *sourceDocument, extra []Stylesheet) (*sourceStyles, error) {
	result := &sourceStyles{inline: make(map[sourceID][]cssom.SourceDeclaration)}
	var ds []diagnostic
	for _, n := range source.nodes[1:] {
		if n.kind == sourceInert {
			continue
		}
		// Resource/style-element checks include head metadata, but never descend
		// across inert boundaries. Display suppression is a later classification.
		ds = append(ds, inspectSourceRequirements(n)...)
		if n.kind == sourceStylesheet {
			for _, key := range []string{"type", "media", "scoped"} {
				if value, exists := attribute(n.node, key); exists {
					value = strings.Trim(value, htmlWhitespace)
					allowed := key == "type" && (value == "" || strings.EqualFold(value, "text/css")) ||
						key == "media" && (value == "" || strings.EqualFold(value, "all"))
					if !allowed {
						ds = append(ds, diagnostic{stage: "css", code: "css.style-attribute", source: n.ref,
							message: "unsupported style element attribute: " + key})
					}
				}
			}
			var cssText strings.Builder
			for c := n.node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					cssText.WriteString(c.Data)
				}
			}
			inspection := cssom.InspectStylesheet(cssText.String())
			result.sheets = append(result.sheets, sourceSheet{source: n.ref, inspection: inspection})
			for _, d := range inspection.Diagnostics {
				ds = append(ds, cssDiagnostic(n.ref, d, false))
			}
		}
		if n.node.Type == html.ElementNode && (n.kind == sourceContent || n.kind == sourceContext) {
			if value, exists := attribute(n.node, "style"); exists {
				declarations, diagnostics := cssom.InspectInlineStyle(value)
				result.inline[n.ref.nodeID] = declarations
				for _, d := range diagnostics {
					ds = append(ds, cssDiagnostic(n.ref, d, true))
				}
			}
		}
	}
	for i, sheet := range extra {
		name := sheet.SourceName
		if name == "" {
			name = fmt.Sprintf("<stylesheet %d>", i+1)
		}
		ref := sourceRef{name: name, stylesheet: i + 1}
		if !utf8.ValidString(sheet.CSS) {
			ds = append(ds, diagnostic{stage: "input", code: "input.encoding", source: ref, message: "stylesheet text must be UTF-8"})
			continue
		}
		inspection := cssom.InspectStylesheet(sheet.CSS)
		result.sheets = append(result.sheets, sourceSheet{source: ref, inspection: inspection})
		for _, d := range inspection.Diagnostics {
			ds = append(ds, cssDiagnostic(ref, d, false))
		}
	}
	if len(ds) != 0 {
		return nil, newLoadError(ds)
	}
	return result, nil
}

func cssDiagnostic(ref sourceRef, d cssom.Diagnostic, inline bool) diagnostic {
	return diagnostic{stage: "css", code: d.Code, source: ref,
		css: cssLocation{rule: d.Rule, declaration: d.Declaration, line: d.Line, column: d.Column,
			property: d.Property, inline: inline},
		message: "CSS source inspection failed", cause: d}
}

// Check source requirements before styling. This is not the full content profile:
// projection validates box participation, paint effects and typography.
func inspectSourceRequirements(n *sourceNode) []diagnostic {
	if n.node.Type != html.ElementNode {
		return nil
	}
	failure := func(stage, code, message string) []diagnostic {
		return []diagnostic{{stage: stage, code: code, source: n.ref, message: message}}
	}
	if isElement(n.node, atom.Style) && n.kind != sourceStylesheet {
		return failure("css", "css.style-location", "content stylesheets must be direct children of head")
	}
	if isElement(n.node, atom.Link) {
		rel, _ := attribute(n.node, "rel")
		for _, token := range strings.FieldsFunc(rel, func(r rune) bool { return strings.ContainsRune(htmlWhitespace, r) }) {
			if strings.EqualFold(token, "stylesheet") {
				return failure("input", "input.resource", "linked stylesheets are unsupported; supply named stylesheet text instead")
			}
		}
	}
	if n.kind == sourceContent {
		switch n.node.DataAtom {
		case atom.Img, atom.Picture, atom.Audio, atom.Video, atom.Source, atom.Track, atom.Iframe, atom.Object, atom.Embed:
			return failure("input", "input.resource", "resource-bearing content is unsupported: <"+n.node.Data+">")
		}
	}
	return nil
}
