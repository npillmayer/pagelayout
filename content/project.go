package content

import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"slices"
	"strings"

	"github.com/npillmayer/css-box-layout/layout"
	boxtext "github.com/npillmayer/css-box-layout/text"
	"github.com/npillmayer/w3c-dom/dimen"
	"github.com/npillmayer/w3c-dom/style"
	"github.com/npillmayer/w3c-dom/style/css"
	"github.com/npillmayer/w3c-dom/style/display"
	"github.com/npillmayer/w3c-dom/style/textstyle"
	"github.com/npillmayer/w3c-dom/styledtree"
	"golang.org/x/net/html"
)

type participation uint8

const (
	contextNode participation = iota
	materialNode
	suppressedNode
	inertNode
	metadataNode
)

type borderPaint struct {
	style string
	color color.NRGBA
}
type boxPaint struct {
	background color.NRGBA
	borders    [4]borderPaint
}

// projectedSource is a detached source ledger entry, including omitted sources.
// The original ancestry is provenance, never a second layout parent relationship.
type projectedSource struct {
	ref                                   sourceRef
	parent                                sourceID
	children                              []sourceID
	kind                                  sourceKind
	participation                         participation
	nodeType                              html.NodeType
	name, namespace, text                 string
	attributes                            []html.Attribute
	typography                            textstyle.Style
	display                               layout.Display
	position, visibility, boxSizing, flow string
	breakBefore                           string
	constraints                           layout.ComputedStyle
	paint                                 boxPaint
	textRef                               boxtext.TextRef
}

// sourceProjection owns a detached, unpartitioned render projection. No source
// DOM/CSSOM, font objects, normalized boxes, used geometry or registers escape.
// A fully display-suppressed document has a nil root and a complete ledger.
type sourceProjection struct {
	root         *layout.RenderNode
	nodes        map[sourceID]*layout.RenderNode
	sources      []projectedSource // indexed by sourceID; slot zero remains reserved
	rootFontSize dimen.DU
}

func loadProjection(r io.Reader, opts LoadOptions) (*sourceProjection, error) {
	s, err := loadSource(r, opts)
	if err != nil {
		return nil, err
	}
	return projectSource(s)
}

// projectSource creates detached typed input with source styles and identities.
// Routing, partitioning and normalization consume this projection separately.
func projectSource(s *styledSource) (*sourceProjection, error) {
	if err := validateProfileSources(s); err != nil {
		return nil, err
	}
	if uint64(len(s.source.nodes)) > math.MaxUint32 {
		return nil, newLoadError([]diagnostic{{stage: "projection", code: "projection.identity", message: "too many sources for text source identities"}})
	}
	p := &sourceProjection{nodes: make(map[sourceID]*layout.RenderNode), sources: make([]projectedSource, len(s.source.nodes))}
	a := projectionAdapter{source: s, out: p, lengths: make(map[sourceID]map[string]layout.Length)}
	var selected []*styledtree.StyNode
	var ids []sourceID
	for _, n := range s.source.nodes[1:] {
		id := n.ref.nodeID
		r := &p.sources[id]
		*r = projectedSource{ref: n.ref, parent: n.parent, children: slices.Clone(n.children), kind: n.kind,
			nodeType: n.node.Type, name: n.node.Data, namespace: n.node.Namespace, attributes: slices.Clone(n.node.Attr), text: n.text}
		switch {
		case n.kind == sourceInert:
			r.participation = inertNode
			continue
		case n.kind == sourceMetadata || n.kind == sourceStylesheet || n.node == s.source.head:
			r.participation = metadataNode
			continue
		case n.node == s.source.document:
			continue
		case p.sources[n.parent].participation == suppressedNode:
			r.participation = suppressedNode
			continue
		}
		if n.node.Type == html.ElementNode {
			d, err := css.GetDisplay(s.nodes[id])
			if err != nil {
				a.fail(id, "profile", "profile.box", "display", "cannot resolve content display", err)
				continue
			}
			if d.Kind() == display.None {
				r.participation = suppressedNode
				continue
			}
			r.display, err = projectDisplay(d)
			if err != nil {
				a.fail(id, "profile", "profile.box", "display", "unsupported content participation", err)
			}
			if n.node == s.source.body && r.display.Outer != layout.OuterBlock {
				a.fail(id, "profile", "profile.box", "display", "body must be block flow or block flow-root", nil)
			}
			if !supportedElement(n.node) {
				a.fail(id, "profile", "profile.element", "", "unsupported content element <"+n.node.Data+">", nil)
			}
			if _, hidden := attribute(n.node, "hidden"); hidden {
				a.fail(id, "profile", "profile.element", "", "hidden attribute requires explicit display:none in this profile", nil)
			}
		}
		if n.node != s.source.html {
			r.participation = materialNode
		}
		selected = append(selected, s.nodes[id])
		ids = append(ids, id)
	}
	if len(a.diagnostics) > 0 {
		return nil, newLoadError(a.diagnostics)
	}
	styles, err := textstyle.Resolve(selected...)
	if err != nil {
		var located *textstyle.Error
		d := diagnostic{stage: "source-style", code: "text.value", message: "cannot resolve source text style", cause: err}
		if errors.As(err, &located) {
			if n := s.source.byNode[located.Node]; n != nil {
				d.source = n.ref
			}
			d.css = cssLocation{property: located.Property, value: located.Value}
		}
		return nil, newLoadError([]diagnostic{d})
	}
	for i, id := range ids {
		p.sources[id].typography = styles[i]
	}
	rootID := s.source.byNode[s.source.html].ref.nodeID
	p.rootFontSize = p.sources[rootID].typography.FontSize
	for _, id := range ids {
		r := &p.sources[id]
		a.checkTypography(id)
		if r.nodeType == html.ElementNode {
			a.element(id)
		}
	}
	if len(a.diagnostics) > 0 {
		return nil, newLoadError(a.diagnostics)
	}
	for _, id := range ids {
		r := &p.sources[id]
		if r.participation != materialNode {
			continue
		}
		descriptor := &html.Node{Type: r.nodeType, Data: r.name, Namespace: r.namespace}
		n := &layout.RenderNode{ID: layout.NodeID(id), HTML: descriptor}
		if r.nodeType == html.TextNode {
			descriptor.Data = r.text
			r.textRef = boxtext.TextRef{Source: boxtext.TextSourceID(id), Range: boxtext.TextRange{End: uint64(len(r.text))}}
			ref := r.textRef
			n.Text = &ref
		} else {
			descriptor.DataAtom = s.source.nodes[id].node.DataAtom
			n.Display = r.display
			constraints := r.constraints
			n.Style = &constraints
		}
		p.nodes[id] = n
		if parent := p.nodes[r.parent]; parent != nil {
			parent.ChildrenNodes = append(parent.ChildrenNodes, n)
		}
	}
	p.root = p.nodes[s.source.byNode[s.source.body].ref.nodeID]
	return p, nil
}

type projectionAdapter struct {
	source      *styledSource
	out         *sourceProjection
	lengths     map[sourceID]map[string]layout.Length
	diagnostics []diagnostic
}

func (a *projectionAdapter) fail(id sourceID, stage, code, key, message string, cause error) {
	value := ""
	if key != "" {
		if node := a.source.nodes[id]; node != nil {
			p, _ := css.GetProperty(node, key)
			value = string(p)
		}
	}
	a.diagnostics = append(a.diagnostics, diagnostic{stage: stage, code: code, source: a.out.sources[id].ref, css: cssLocation{property: key, value: value}, message: message, cause: cause})
}

var supportedElements = strings.Fields(`html body div p address article section aside main header footer nav blockquote
 h1 h2 h3 h4 h5 h6 hgroup figure figcaption pre
 a abbr b bdi cite code data del dfn em i ins kbd mark q s samp small span strong sub sup time u var`)

func supportedElement(n *html.Node) bool {
	return n.Namespace == "" && slices.Contains(supportedElements, n.Data)
}

func (a *projectionAdapter) checkTypography(id sourceID) {
	s := a.out.sources[id].typography
	for _, p := range []struct {
		key string
		ok  bool
	}{
		{"font-size", s.FontSize > 0 && s.FontSize < dimen.Fil},
		{"white-space", s.WhiteSpace == "normal"},
		{"line-height", s.LineHeight.Kind == textstyle.NormalLineHeight},
		{"unicode-bidi", s.UnicodeBidi == "normal" || s.UnicodeBidi == "isolate"},
	} {
		if !p.ok {
			a.fail(id, "profile", "profile.value", p.key, "unsupported effective text policy", nil)
			// HTML hints need not appear in the raw CSS property map (pre is
			// one example). Report the actual shared text-style snapshot.
			d := &a.diagnostics[len(a.diagnostics)-1]
			if p.key == "white-space" {
				d.css.value = s.WhiteSpace
			}
			if p.key == "unicode-bidi" {
				d.css.value = s.UnicodeBidi
			}
			d.message += ": " + d.css.value
		}
	}
}

func (a *projectionAdapter) element(id sourceID) {
	n := a.source.nodes[id]
	r := &a.out.sources[id]
	s := layout.DefaultComputedStyle()
	s.FontSizePx = float32(float64(r.typography.FontSize) / float64(dimen.PX))
	s.WhiteSpace = layout.WhiteSpaceNormal
	for _, key := range []string{"break-before", "position", "float", "visibility", "box-sizing", "writing-mode", "direction"} {
		word, err := css.GetBoxKeyword(n, key)
		if err != nil || !profileKeyword(key, word) {
			a.fail(id, "profile", "profile.value", key, "unsupported effective box policy", err)
		}
		switch key {
		case "break-before":
			r.breakBefore = word
			if word != "auto" && err == nil && profileKeyword(key, word) {
				r.breakBefore = "page"
			}
		case "position":
			r.position = word
		case "visibility":
			r.visibility = word
		case "box-sizing":
			r.boxSizing = word
		}
	}
	for _, key := range append([]string{"height", "min-width", "min-height", "max-width", "max-height", "top", "right", "bottom", "left", "word-spacing", "letter-spacing", "word-break", "overflow-wrap", "flow-from"}, paintResets...) {
		v, err := css.GetProperty(n, key)
		if err == nil {
			err = checkProfileValue(key, v)
		}
		if err != nil {
			a.fail(id, "profile", "profile.value", key, "unsupported effective property", err)
		}
	}
	var err error
	r.flow, err = css.GetFlowName(n, "flow-into")
	if err != nil {
		a.fail(id, "profile", "profile.value", "flow-into", "cannot resolve flow name", err)
	}
	a.lengths[id] = make(map[string]layout.Length)
	s.Width = a.length(id, "width")
	fonts := css.FontContext{RootFontSize: a.out.rootFontSize, FontSize: func(node *styledtree.StyNode) (dimen.DU, error) {
		if src := a.source.source.byNode[node.HTMLNode()]; src != nil {
			if size := a.out.sources[src.ref.nodeID].typography.FontSize; size > 0 {
				return size, nil
			}
		}
		return 0, fmt.Errorf("source font snapshot is missing")
	}}
	for i, side := range []struct {
		name                    string
		margin, padding, border *layout.Length
	}{
		{"top", &s.Margin.Top, &s.Padding.Top, &s.Border.Top}, {"right", &s.Margin.Right, &s.Padding.Right, &s.Border.Right},
		{"bottom", &s.Margin.Bottom, &s.Padding.Bottom, &s.Border.Bottom}, {"left", &s.Margin.Left, &s.Padding.Left, &s.Border.Left},
	} {
		*side.margin = a.length(id, "margin-"+side.name)
		*side.padding = a.length(id, "padding-"+side.name)
		b, e := css.GetBorderWithFontContext(n, side.name, fonts)
		if e != nil {
			a.fail(id, "projection", "projection.value", "border-"+side.name+"-width", "cannot resolve source border", e)
		} else {
			if b.Style != "solid" && b.Style != "none" && b.Style != "hidden" {
				a.fail(id, "profile", "profile.value", "border-"+side.name+"-style", "only solid borders are supported", nil)
			}
			*side.border = layout.Length{Kind: layout.LenPx, Value: float32(float64(b.Width) / float64(dimen.PX))}
			r.paint.borders[i] = borderPaint{style: b.Style, color: b.Color}
		}
		if r.display.Outer == layout.OuterInline && r.display.Inner == layout.InnerFlow {
			for _, edge := range []struct {
				key string
				l   layout.Length
			}{{"margin-" + side.name, *side.margin}, {"padding-" + side.name, *side.padding}, {"border-" + side.name + "-width", *side.border}} {
				if edge.l.Kind == layout.LenAuto || edge.l.Value != 0 {
					a.fail(id, "profile", "profile.inline-edge", edge.key, "ordinary inline boxes require zero effective edges", nil)
				}
			}
		}
	}
	r.paint.background, err = css.GetColor(n, "background-color")
	if err != nil {
		a.fail(id, "projection", "projection.value", "background-color", "cannot resolve background", err)
	}
	r.constraints = s
}

func (a *projectionAdapter) length(id sourceID, key string) layout.Length {
	n := a.source.nodes[id]
	if style.DeclarationKeyword(string(css.GetLocalProperty(n.Styles(), key))) == "inherit" {
		parent := a.out.sources[id].parent
		if a.out.sources[parent].nodeType == html.ElementNode {
			if l, ok := a.lengths[parent][key]; ok {
				a.lengths[id][key] = l
				return l
			}
			a.fail(id, "projection", "projection.context", key, "missing computed parent length", nil)
			return layout.Length{}
		}
	}
	l, err := css.GetBoxLength(n, key)
	if err == nil {
		v, _ := css.GetProperty(n, key)
		err = checkProfileValue(key, v)
	}
	var result layout.Length
	if err == nil {
		var value float64
		switch l.Kind {
		case style.AbsoluteLength:
			result.Kind = layout.LenPx
			value = float64(l.Absolute) / float64(dimen.PX)
		case style.PercentageLength:
			result.Kind = layout.LenPercent
			value = l.Magnitude / 100
		case style.RelativeLength:
			font := a.out.sources[id].typography.FontSize
			if l.Unit == "rem" {
				font = a.out.rootFontSize
			}
			result.Kind = layout.LenPx
			value = l.Magnitude * float64(font) / float64(dimen.PX)
		case style.KeywordLength:
			result.Kind = layout.LenAuto
		default:
			err = fmt.Errorf("unsupported length kind")
		}
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > math.MaxFloat32 || value != 0 && float32(value) == 0 {
			err = fmt.Errorf("length cannot be represented in layout units")
		}
		result.Value = float32(value)
	}
	if err != nil {
		a.fail(id, "projection", "projection.value", key, "cannot project source length", err)
		return layout.Length{}
	}
	a.lengths[id][key] = result
	return result
}
