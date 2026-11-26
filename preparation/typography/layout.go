package typography

import (
	"math"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

// LayoutRequest sets the containing width in CSS pixels and explicit breaker.
// Select FirstFit explicitly; other breakers are outside the supported profile.
// Zero width is valid. The profile uses CSS normal line height, zero
// side skips and bottom-margin-edge alignment of completed atomic boxes.
type LayoutRequest struct {
	Width   float32
	Breaker paragraph.Breaker
}

// GroupResult retains copied source/paint and frozen paragraph output. Paragraph
// geometry uses big points relative to the owner's content origin. Frozen/VList
// values are immutable; their borrowed font resources must remain valid.
type GroupResult struct {
	Input     content.GroupInput
	Paragraph paragraph.Result
}

// Result is a completed independent subtree layout, without preparation/session handles.
// It owns CSS geometry and all paragraphs together. Accessors copy envelopes.
type Result struct {
	css    *content.LayoutResult
	groups map[content.Box]GroupResult
}

// Root returns the source root whose complete layout this result describes.
func (r *Result) Root() content.Box {
	if r == nil {
		return content.Box{}
	}
	return r.css.Root()
}

// Geometry returns a copied CSS-pixel map with parent-content RelativeTo coordinates.
func (r *Result) Geometry() map[content.Box]layout.LayoutGeometry {
	if r == nil {
		return nil
	}
	return r.css.Geometry()
}

// Groups returns source group descriptions in normalized preorder.
func (r *Result) Groups() []content.GroupInfo {
	if r == nil {
		return nil
	}
	return r.css.Groups()
}

// Group returns a copied source/paint description and shared immutable paragraph
// output for one source group. A missing or foreign group returns false.
func (r *Result) Group(b content.Box) (GroupResult, bool) {
	if r == nil {
		return GroupResult{}, false
	}
	g, ok := r.groups[b]
	if !ok {
		return GroupResult{}, false
	}
	g.Input = copyInput(g.Input)
	g.Paragraph.BreakPositions = slices.Clone(g.Paragraph.BreakPositions)
	g.Paragraph.BidiRuns = slices.Clone(g.Paragraph.BidiRuns)
	return g, true
}

// Layout produces a fresh complete subtree result using existing preparations.
// Calls sharing a session/services must be serialized. Later trials or Close do
// not invalidate accepted output. Failure returns nil, never partial geometry or
// paragraphs. No bank is advanced and no missing group is prepared implicitly.
func Layout(p *content.PreparedSnapshot, s *content.PreparationSession, root content.Box, r LayoutRequest) (*Result, error) {
	fail := func(err error) (*Result, error) {
		info, _ := root.Info()
		return nil, &content.PreparationError{Stage: content.StageLayout, Owner: root, Source: info.Context, Err: err}
	}
	if _, err := cssToDU(r.Width, false); err != nil {
		return fail(err)
	}
	if r.Breaker != paragraph.FirstFit {
		return fail(unsupported("whole-tree layout requires explicit FirstFit"))
	}
	i := &layoutInvocation{breaker: r.Breaker, groups: make(map[content.Box]GroupResult)}
	css, err := content.Layout(p, s, root, r.Width, i)
	if err != nil {
		return nil, err
	}
	if len(css.Groups()) != len(i.groups) {
		return nil, content.ErrPreparationProtocol
	}
	return &Result{css: css, groups: i.groups}, nil
}

type layoutInvocation struct {
	groupMeasurer
	breaker paragraph.Breaker
	groups  map[content.Box]GroupResult
}

var _ content.GroupLayouter = (*layoutInvocation)(nil)

func (i *layoutInvocation) LayoutGroup(handle content.GroupHandle, r content.InlineLayoutRequest) (layout.InlineResult, error) {
	h, ok := handle.(*preparedGroup)
	if !ok || h == nil || h.paragraph == nil || h.input.Group != r.Group || h.input.Owner != r.Owner {
		return layout.InlineResult{}, content.ErrInvalidPreparation
	}
	fail := func(err error) (layout.InlineResult, error) {
		return layout.InlineResult{}, groupError(h.input, content.StageLayout, err)
	}
	if nilService(r.Used) || nilService(r.Atomics) {
		return fail(invalidMeasurement("missing layout views"))
	}
	if _, ok := i.groups[r.Group]; ok {
		return fail(content.ErrPreparationProtocol)
	}
	width, err := cssToDU(r.Width, false)
	if err != nil {
		return fail(err)
	}
	atoms, err := atomicBoxes(h.input, r)
	if err != nil {
		return fail(err)
	}
	trial, err := h.paragraph.Trial(paragraph.TrialRequest{Layout: paragraph.LayoutRequest{
		Params: paragraph.Params{
			Constraints: paragraph.Constraints{Width: width}, Breaker: i.breaker,
			Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &h.lineLayout,
		},
		Horizontal: &paragraph.HorizontalLayout{}, Atomics: atoms,
	}})
	if err != nil {
		return fail(err)
	}
	result, err := trial.Assemble()
	if err != nil {
		return fail(err)
	}
	summary, err := inlineSummary(h.input, result)
	if err != nil {
		return fail(err)
	}
	i.groups[r.Group] = GroupResult{Input: copyInput(h.input), Paragraph: *result}
	return summary, nil
}

func atomicBoxes(g content.GroupInput, r content.InlineLayoutRequest) ([]paragraph.AtomicBox, error) {
	atoms := make([]paragraph.AtomicBox, 0, len(g.Occurrences))
	for _, source := range g.Occurrences {
		info, _ := source.Box.Info()
		a, ok := r.Atomics.Lookup(info.ID)
		if !ok || a.BoxID != info.ID {
			return nil, invalidMeasurement("missing or mismatched realized atomic")
		}
		u, ok := r.Used.Lookup(info.ID)
		if !ok || u.WidthPending || u.EdgesPending {
			return nil, invalidMeasurement("missing or provisional atomic values")
		}
		out := paragraph.AtomicBox{ID: contracts.AtomicID(source.ID), Alignment: paragraph.AlignBottomMarginEdge}
		for _, v := range []struct {
			px     float32
			signed bool
			dest   *dimen.DU
		}{
			{a.BorderWidth, false, &out.BorderWidth}, {a.BorderHeight, false, &out.BorderHeight},
			{u.Margin.Top, true, &out.Margin.Top}, {u.Margin.Right, true, &out.Margin.Right},
			{u.Margin.Bottom, true, &out.Margin.Bottom}, {u.Margin.Left, true, &out.Margin.Left},
		} {
			value, err := cssToDU(v.px, v.signed)
			if err != nil {
				return nil, err
			}
			*v.dest = value
		}
		atoms = append(atoms, out)
	}
	return atoms, nil
}

func pointsToCSS(points float64, signed bool) (float32, error) {
	if math.IsNaN(points) || math.IsInf(points, 0) || !signed && points < 0 || math.Abs(points)*float64(dimen.BP) >= float64(dimen.Fil) {
		return 0, invalidMeasurement("paragraph geometry outside finite domain")
	}
	px := float32(points * (4.0 / 3.0))
	if math.Abs(float64(px))*duPerCSSPixel >= float64(dimen.Fil) {
		return 0, invalidMeasurement("CSS geometry rounds outside finite domain")
	}
	return px, nil
}

func inlineSummary(g content.GroupInput, r *paragraph.Result) (layout.InlineResult, error) {
	var out layout.InlineResult
	var err error
	box := r.VList.Box()
	if out.Extent.W, err = pointsToCSS(box.Width, false); err != nil {
		return layout.InlineResult{}, err
	}
	if out.Extent.H, err = pointsToCSS(box.Extent, false); err != nil {
		return layout.InlineResult{}, err
	}
	pending := make(map[contracts.AtomicID]layout.BoxID)
	for _, a := range g.Occurrences {
		info, _ := a.Box.Info()
		pending[contracts.AtomicID(a.ID)] = info.ID
	}
	for _, p := range r.VList.AtomicPlacements() {
		id, ok := pending[p.ID]
		if !ok {
			return layout.InlineResult{}, invalidMeasurement("unknown or duplicate atomic placement")
		}
		x, err := pointsToCSS(p.X, true)
		if err != nil {
			return layout.InlineResult{}, err
		}
		y, err := pointsToCSS(p.Y, true)
		if err != nil {
			return layout.InlineResult{}, err
		}
		out.Atomics = append(out.Atomics, layout.AtomicPlacement{BoxID: id, X: x, Y: y})
		delete(pending, p.ID)
	}
	if len(pending) != 0 {
		return layout.InlineResult{}, invalidMeasurement("missing atomic placement")
	}
	out.AllowsMarginCollapse = len(r.VList.Lines()) == 0 && out.Extent.H == 0 && len(g.Occurrences) == 0
	return out, nil
}
