package template

import (
	"fmt"
	"image/color"
	"math"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/w3c-dom/dimen"
	"github.com/npillmayer/w3c-dom/style"
	"github.com/npillmayer/w3c-dom/style/css"
	"golang.org/x/net/html"
)

// validatedCollection is detached input for geometry compilation. It
// owns every slice and FixedStyle; it contains no DOM, CSSOM or styled nodes.
// It is not a catalog of resolved rectangles and remains an internal stage.
type validatedCollection struct {
	templates []*validatedTemplate
	byID      map[string]*validatedTemplate
}

type validatedTemplate struct {
	id     string
	page   pageInput
	frames []frameInput // Source order, including repeated flow names.
}

type pageInput struct {
	source        sourceRef
	width, height float32 // CSS pixels, not DU.
	paint         boxPaint
}

type frameInput struct {
	id          frameID
	source      sourceRef
	constraints layout.ComputedStyle
	paint       boxPaint
	flow        string // Decoded case-sensitive name; never empty.
}

type borderPaint struct {
	style string
	color color.NRGBA
}

type boxPaint struct {
	background color.NRGBA
	borders    [4]borderPaint // Top, right, bottom, left; widths are in constraints.
}

// validateTemplateCollection is the private TPL-03 boundary. Input must have
// passed TPL-01/02. Reject source profile failures first to avoid interpreting a
// partially supported cascade. All independent effective failures are collected.
func validateTemplateCollection(c *styledCollection) (*validatedCollection, error) {
	if err := validateProfileSources(c); err != nil {
		return nil, err
	}
	out := &validatedCollection{byID: make(map[string]*validatedTemplate)}
	var ds []diagnostic
	for _, t := range c.templates {
		v := boxAdapter{template: t}
		// Context nodes are styling ancestry, not geometry/paint boxes. They must
		// not suppress receivers or introduce another positioning context.
		for _, n := range []*html.Node{t.active.html, t.active.body} {
			v.checkBox(n, false)
			v.flow(n, false)
		}
		page := t.active.page
		v.checkBox(page, false)
		pstyle, pheight, paint := v.box(page, false)
		v.flow(page, false)
		result := &validatedTemplate{id: t.active.id, page: pageInput{
			source: t.active.sources[page], width: pstyle.Width.Value, height: pheight.Value, paint: paint,
		}}
		for _, f := range t.active.frames {
			v.checkBox(f.element, true)
			s, _, paint := v.box(f.element, true)
			result.frames = append(result.frames, frameInput{id: f.id, source: t.active.sources[f.element], constraints: s, paint: paint, flow: v.flow(f.element, true)})
		}
		ds = append(ds, v.diagnostics...)
		out.templates = append(out.templates, result)
		out.byID[result.id] = result
	}
	if len(ds) > 0 {
		return nil, orderedStyleError(ds)
	}
	return out, nil
}

type boxAdapter struct {
	template    *styledTemplate
	diagnostics []diagnostic
}

func (v *boxAdapter) fail(n *html.Node, code diagnosticCode, key, message string, cause error) {
	v.diagnostics = append(v.diagnostics, diagnostic{stage: stageProfile, code: code, source: v.template.active.sources[n], css: cssLocation{property: key}, message: message, cause: cause})
}

func (v *boxAdapter) checkBox(n *html.Node, frame bool) {
	node := v.template.nodes[n]
	d, err := css.GetDisplay(node)
	if err != nil || !blockFlow(d) {
		v.fail(n, codeProfileBox, "display", "a block/flow box is required", err)
	}
	for _, key := range []string{"position", "float", "visibility", "box-sizing", "writing-mode", "direction"} {
		word, err := css.GetBoxKeyword(node, key)
		ok := profileKeyword(key, word)
		if key == "position" {
			if frame {
				ok = word == "fixed"
			} else {
				ok = word == "static"
			}
		}
		if err != nil || !ok {
			v.fail(n, codeProfileBox, key, "unsupported effective "+key+": "+word, err)
		}
	}
}

func (v *boxAdapter) length(n *html.Node, key string, allowAuto bool) (layout.Length, bool) {
	l, err := css.GetBoxLength(v.template.nodes[n], key)
	if err == nil {
		if allowAuto && l.Kind == style.KeywordLength && l.Keyword == "auto" {
			return layout.Length{Kind: layout.LenAuto}, true
		}
		if l.Kind == style.AbsoluteLength {
			px, err := cssPixels(l.Absolute)
			if err == nil {
				return layout.Length{Kind: layout.LenPx, Value: px}, true
			}
			v.fail(n, codeProfileSizing, key, "cannot convert absolute length", err)
			return layout.Length{}, false
		}
	}
	v.fail(n, codeProfileSizing, key, "a supported absolute length"+autoSuffix(allowAuto)+" is required", err)
	return layout.Length{}, false
}

func autoSuffix(allowed bool) string {
	if allowed {
		return " or auto"
	}
	return ""
}

func cssPixels(d dimen.DU) (float32, error) {
	px := float32(float64(d) / float64(dimen.PX))
	if math.IsInf(float64(px), 0) || math.IsNaN(float64(px)) {
		return 0, fmt.Errorf("nonfinite CSS pixel value")
	}
	return px, nil
}

func (v *boxAdapter) box(n *html.Node, frame bool) (layout.ComputedStyle, layout.Length, boxPaint) {
	s := layout.DefaultComputedStyle()
	fixed := layout.DefaultFixedStyle()
	w, wok := v.length(n, "width", frame)
	h, hok := v.length(n, "height", frame)
	s.Width = w
	fixed.Height = h
	var paint boxPaint
	node := v.template.nodes[n]
	bg, err := css.GetColor(node, "background-color")
	if err != nil {
		v.fail(n, codeProfileValue, "background-color", "cannot resolve solid background", err)
	} else {
		paint.background = bg
	}
	for _, key := range paintResets {
		value, err := css.GetProperty(node, key)
		if err != nil || value != style.InitialProperty(key) {
			v.fail(n, codeProfileValue, key, "unsupported effective paint component", err)
		}
	}
	padding := []*layout.Length{&s.Padding.Top, &s.Padding.Right, &s.Padding.Bottom, &s.Padding.Left}
	borders := []*layout.Length{&s.Border.Top, &s.Border.Right, &s.Border.Bottom, &s.Border.Left}
	insets := []*layout.Length{&fixed.Insets.Top, &fixed.Insets.Right, &fixed.Insets.Bottom, &fixed.Insets.Left}
	insetOK := [4]bool{}
	for i, side := range physicalSides {
		margin, ok := v.length(n, "margin-"+side, false)
		if ok && margin.Value != 0 {
			v.fail(n, codeProfileBox, "margin-"+side, "template boxes require zero margins", nil)
		}
		p, ok := v.length(n, "padding-"+side, false)
		if ok && (p.Value < 0 || !frame && p.Value != 0) {
			v.fail(n, codeProfileBox, "padding-"+side, "padding must be nonnegative; page padding must be zero", nil)
		}
		*padding[i] = p
		b, err := css.GetBorder(node, side)
		if err != nil {
			v.fail(n, codeProfileValue, "border-"+side, "cannot resolve effective border", err)
		} else {
			if b.Style != "none" && b.Style != "hidden" && b.Style != "solid" {
				v.fail(n, codeProfileValue, "border-"+side+"-style", "unsupported border style", nil)
			}
			px, err := cssPixels(b.Width)
			if err != nil || px < 0 || !frame && px != 0 {
				v.fail(n, codeProfileBox, "border-"+side+"-width", "border must be nonnegative; page effective border must be zero", err)
			}
			*borders[i] = layout.Length{Kind: layout.LenPx, Value: px}
			paint.borders[i] = borderPaint{style: b.Style, color: b.Color}
		}
		*insets[i], insetOK[i] = v.length(n, side, true)
		if !frame && insetOK[i] && insets[i].Kind != layout.LenAuto {
			v.fail(n, codeProfileBox, side, "page insets must be auto; the page supplies the viewport", nil)
		}
	}
	if frame {
		s.Position = layout.PositionFixed
		s.Fixed = &fixed
		if wok && insetOK[3] && insetOK[1] {
			v.axis(n, "width", w, fixed.Insets.Left, fixed.Insets.Right)
		}
		if hok && insetOK[0] && insetOK[2] {
			v.axis(n, "height", h, fixed.Insets.Top, fixed.Insets.Bottom)
		}
	} else {
		if wok && w.Value <= 0 {
			v.fail(n, codeProfileSizing, "width", "page width must be positive", nil)
		}
		if hok && h.Value <= 0 {
			v.fail(n, codeProfileSizing, "height", "page height must be positive", nil)
		}
	}
	return s, h, paint
}

func (v *boxAdapter) axis(n *html.Node, key string, size, start, end layout.Length) {
	if start.Kind == layout.LenAuto && end.Kind == layout.LenAuto {
		v.fail(n, codeProfileSizing, key, "at least one definite inset per axis is required", nil)
	} else if size.Kind == layout.LenAuto && (start.Kind == layout.LenAuto || end.Kind == layout.LenAuto) {
		v.fail(n, codeProfileSizing, key, "auto size requires opposing definite insets; content sizing is unavailable", nil)
	} else if size.Kind == layout.LenPx && size.Value <= 0 {
		v.fail(n, codeProfileSizing, key, "explicit content size must be positive", nil)
	}
}

func (v *boxAdapter) flow(n *html.Node, frame bool) string {
	node := v.template.nodes[n]
	name, err := css.GetFlowName(node, "flow-from")
	if err != nil || frame && name == "" || !frame && name != "" {
		v.fail(n, codeProfileFlow, "flow-from", "only receiving frames must have a non-none flow-from", err)
	}
	into, err := css.GetFlowName(node, "flow-into")
	if err != nil || into != "" {
		v.fail(n, codeProfileFlow, "flow-into", "template boxes cannot supply content flows", err)
	}
	return name
}
