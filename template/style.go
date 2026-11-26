package template

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/npillmayer/w3c-dom/style/cssom"
	"github.com/npillmayer/w3c-dom/styledtree"
	"golang.org/x/net/html"
)

// collectionStyles retains the pre-cascade source view, including declarations
// that lose the cascade. Shared sheets are read-only; each definition gets its
// own CSSOM, active nodes, property maps and defaults.
type collectionStyles struct {
	sheets []sourceSheet
	inline map[sourceNodeID][]cssom.SourceDeclaration
}

type sourceSheet struct {
	source     sourceRef
	inspection cssom.StylesheetInspection
}

type styledCollection struct {
	source    *templateCollection
	styles    *collectionStyles
	templates []*styledTemplate
	byID      map[string]*styledTemplate
}

type styledTemplate struct {
	active *activeTemplate
	root   *styledtree.StyNode
	nodes  map[*html.Node]*styledtree.StyNode
}

// styleTemplateCollection styles isolated definitions. Its input must be a
// successfully parsed collection. No partial styled collection escapes a failure.
// CSS values remain unsized until profile validation and typed adaptation.
func styleTemplateCollection(source *templateCollection, extra ...Stylesheet) (*styledCollection, error) {
	styles, err := collectStyles(source, extra...)
	if err != nil {
		return nil, err
	}
	result := &styledCollection{source: source, styles: styles, byID: make(map[string]*styledTemplate)}
	var diagnostics []diagnostic
	for _, definition := range source.templates {
		active := activateTemplate(source, definition)
		om := cssom.NewCSSOM(nil)
		registrationFailed := false
		for _, sheet := range styles.sheets {
			if err := om.AddStylesForScope(nil, sheet.inspection.Sheet, cssom.Author); err != nil {
				diagnostics = append(diagnostics, diagnostic{stage: stageStyling, code: codeStyleRegistration, source: sheet.source, message: "cannot register collection stylesheet", cause: err})
				registrationFailed = true
			}
		}
		if registrationFailed {
			continue
		}
		root, err := om.Style(active.document)
		if err != nil {
			diagnostics = append(diagnostics, diagnostic{stage: stageStyling, code: codeStyleFailure, source: source.sources[definition.page], message: "cannot style template", cause: err})
			continue
		}
		t := &styledTemplate{active: active, root: root.Payload, nodes: make(map[*html.Node]*styledtree.StyNode)}
		var index func(*styledtree.StyNode)
		index = func(n *styledtree.StyNode) {
			t.nodes[n.HTMLNode()] = n
			for _, child := range n.Children(true) {
				index(child.Payload)
			}
		}
		index(t.root)
		result.templates = append(result.templates, t)
		result.byID[definition.id] = t
	}
	if len(diagnostics) != 0 {
		return nil, orderedStyleError(diagnostics)
	}
	return result, nil
}

func collectStyles(c *templateCollection, extra ...Stylesheet) (*collectionStyles, error) {
	result := &collectionStyles{inline: make(map[sourceNodeID][]cssom.SourceDeclaration)}
	var diagnostics []diagnostic
	for _, node := range c.styles {
		// Applying a conditional or non-CSS sheet unconditionally would be wrong.
		for _, key := range []string{"type", "media", "scoped"} {
			value, exists := attribute(node, key)
			if !exists {
				continue
			}
			value = strings.Trim(value, htmlWhitespace)
			supported := false
			switch key {
			case "type":
				supported = value == "" || strings.EqualFold(value, "text/css")
			case "media":
				supported = value == "" || strings.EqualFold(value, "all")
			}
			if !supported {
				diagnostics = append(diagnostics, diagnostic{stage: stageCSS, code: codeStyleAttribute, source: c.sources[node], message: "unsupported style element attribute: " + key})
			}
		}
		var text strings.Builder
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode {
				text.WriteString(child.Data)
			}
		}
		inspection := cssom.InspectStylesheet(text.String())
		result.sheets = append(result.sheets, sourceSheet{source: c.sources[node], inspection: inspection})
		for _, d := range inspection.Diagnostics {
			diagnostics = append(diagnostics, cssDiagnostic(c.sources[node], d, false))
		}
	}
	for i, sheet := range extra {
		name := sheet.SourceName
		if name == "" {
			name = fmt.Sprintf("<stylesheet %d>", i+1)
		}
		// Additional CSS has no HTML node identity. Keep its ordering separately
		// so diagnostics follow collection diagnostics, then supplied sheet order.
		ref := sourceRef{name: name, stylesheet: i + 1}
		inspection := cssom.InspectStylesheet(sheet.CSS)
		result.sheets = append(result.sheets, sourceSheet{source: ref, inspection: inspection})
		for _, d := range inspection.Diagnostics {
			diagnostics = append(diagnostics, cssDiagnostic(ref, d, false))
		}
	}
	// Validate shared context once, followed by each definition's active elements.
	// Storage wrappers and ignored templates do not participate in the cascade.
	nodes := []*html.Node{c.html, c.body}
	for _, definition := range c.templates {
		nodes = append(nodes, definition.page)
		for _, frame := range definition.frames {
			nodes = append(nodes, frame.element)
		}
	}
	for _, node := range nodes {
		if value, exists := attribute(node, "style"); exists {
			declarations, ds := cssom.InspectInlineStyle(value)
			ref := c.sources[node]
			result.inline[ref.nodeID] = declarations
			for _, d := range ds {
				diagnostics = append(diagnostics, cssDiagnostic(ref, d, true))
			}
		}
	}
	if len(diagnostics) != 0 {
		return nil, orderedStyleError(diagnostics)
	}
	return result, nil
}

func cssDiagnostic(source sourceRef, d cssom.Diagnostic, inline bool) diagnostic {
	return diagnostic{stage: stageCSS, code: diagnosticCode(d.Code), source: source,
		css:     cssLocation{rule: d.Rule, declaration: d.Declaration, line: d.Line, column: d.Column, property: d.Property, inline: inline},
		message: "CSS source inspection failed", cause: d}
}

func orderedStyleError(ds []diagnostic) error {
	slices.SortStableFunc(ds, func(a, b diagnostic) int {
		if order := cmp.Compare(a.source.stylesheet, b.source.stylesheet); order != 0 {
			return order
		}
		return cmp.Compare(a.source.nodeID, b.source.nodeID)
	})
	return newLoadError(ds)
}
