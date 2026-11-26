package typography

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing/paragraph"
	typolayout "github.com/npillmayer/typography/layout"
)

// FitRequest supplies CSS-pixel receiver dimensions and explicit FirstFit.
// Continuations/stops belong to the reusable content.Fitter, not a width.
type FitRequest struct {
	content.FitRequest
	Breaker paragraph.Breaker
}

// FittingAdapter supplies the neutral page-fitting backend. It uses explicit
// FirstFit and the retained group's source hyphenation policy. It owns no session.
func FittingAdapter() content.GroupFitter { return fitInvocation{breaker: paragraph.FirstFit} }

// InspectFragment recovers typed rendering data from a neutral frozen fragment,
// for example a frame returned by pagelayout.PageAttempt. It needs no live session.
func InspectFragment(f *content.FitFragment) (*Fragment, error) {
	if f == nil {
		return nil, content.ErrPreparationProtocol
	}
	bundle := &Fragment{css: f, groups: map[content.FragmentID]FragmentGroup{}}
	for _, g := range f.Paragraphs() {
		payload, ok := g.Output.(*fitOutput)
		if !ok || payload == nil || payload.input.Group != g.Group || payload.input.Owner != g.Owner {
			return nil, content.ErrPreparationProtocol
		}
		bundle.groups[g.ID] = FragmentGroup{g.ID, payload.input, g.X, g.Y, payload.info, payload.fragment, payload.coverage, payload.events}
		bundle.order = append(bundle.order, g.ID)
	}
	return bundle, nil
}

// FragmentGroup is one occurrence of a source group. X/Y use receiver-relative
// CSS pixels; Paragraph uses local big points. Input retains source paint and
// processed/original correspondence. All returned slices are independent.
type FragmentGroup struct {
	ID        content.FragmentID
	Input     content.GroupInput
	X, Y      float32
	Info      paragraph.PrefixInfo
	Paragraph *typolayout.Fragment
	Coverage  []FragmentSource
	Events    []FragmentEvent
}

// Fragment is a frozen render bundle. It retains neither checkpoints nor working
// preparations, collectors or execution services. Keep borrowed font resources
// valid. Multiple bundles can contain different occurrences of the same group.
type Fragment struct {
	css    *content.FitFragment
	groups map[content.FragmentID]FragmentGroup
	order  []content.FragmentID
}

func (f *Fragment) Root() content.Box {
	if f == nil {
		return content.Box{}
	}
	return f.css.Root()
}
func (f *Fragment) Extent() layout.InlineExtent {
	if f == nil {
		return layout.InlineExtent{}
	}
	return f.css.Extent()
}
func (f *Fragment) Geometry() map[content.Box]layout.LayoutGeometry {
	if f == nil {
		return nil
	}
	return f.css.Geometry()
}
func (f *Fragment) UsedValues() map[content.Box]layout.UsedValues {
	if f == nil {
		return nil
	}
	return f.css.UsedValues()
}
func (f *Fragment) Containers() []content.FitContainer {
	if f == nil {
		return nil
	}
	return f.css.Containers()
}
func (f *Fragment) Groups() []content.FragmentID {
	if f == nil {
		return nil
	}
	return slices.Clone(f.order)
}
func (f *Fragment) Group(id content.FragmentID) (FragmentGroup, bool) {
	if f == nil {
		return FragmentGroup{}, false
	}
	g, ok := f.groups[id]
	if !ok {
		return FragmentGroup{}, false
	}
	g.Input = copyInput(g.Input)
	g.Coverage = slices.Clone(g.Coverage)
	g.Events = slices.Clone(g.Events)
	return g, true
}

// FitAlternative pairs one frozen bundle with its exact opaque remainder.
// Retain Fragment separately when resumption is no longer needed.
type FitAlternative struct {
	css      content.FitAlternative
	fragment *Fragment
}

func (a FitAlternative) Fragment() *Fragment                   { return a.fragment }
func (a FitAlternative) Continuation() content.FitContinuation { return a.css.Continuation() }
func (a FitAlternative) End() content.FitPosition              { return a.css.End() }
func (a FitAlternative) Progress() bool                        { return a.css.Progress() }
func (a FitAlternative) Complete() bool                        { return a.css.Complete() }
func (a FitAlternative) StopReached() bool                     { return a.css.StopReached() }

// Fit uses existing preparations to enumerate complete whole-line/block choices.
// Failed or interrupted calls return nil, never a partial bundle. Later trials,
// rejections and session closure cannot change accepted fragments. Only fitting
// needs the matching live preparation/session; rendering does not.
func Fit(ctx context.Context, p *content.PreparedSnapshot, s *content.PreparationSession, f *content.Fitter, r FitRequest) ([]FitAlternative, error) {
	fail := func(err error) ([]FitAlternative, error) {
		info, _ := f.Root().Info()
		return nil, &content.PreparationError{Stage: content.StageFitting, Owner: f.Root(), Source: info.Context, Err: err}
	}
	if _, err := cssToDU(r.Width, false); err != nil {
		return fail(err)
	}
	if _, err := cssToDU(r.Height, false); err != nil {
		return fail(err)
	}
	if r.Breaker != paragraph.FirstFit {
		return fail(unsupported("fragment fitting requires explicit FirstFit"))
	}
	choices, err := content.Fit(ctx, p, s, f, r.FitRequest, fitInvocation{breaker: r.Breaker})
	if err != nil {
		return nil, err
	}
	var out []FitAlternative
	for _, a := range choices {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		bundle, err := InspectFragment(a.Fragment())
		if err != nil {
			return fail(err)
		}
		out = append(out, FitAlternative{a, bundle})
	}
	return out, nil
}

type fitCheckpoint struct {
	group      content.Box
	position   layout.ParagraphPosition
	checkpoint *paragraph.TrialCheckpoint
	events     []eventKey
}

// All fields are private immutable snapshots; no checkpoint chain is retained.
func (c *fitCheckpoint) Clone() layout.FitPayload { return c }

type fitOutput struct {
	input    content.GroupInput
	info     paragraph.PrefixInfo
	fragment *typolayout.Fragment
	coverage []FragmentSource
	events   []FragmentEvent
}

func (o *fitOutput) Clone() layout.FitPayload { return o }
func (o *fitOutput) LineCount() uint64        { return uint64(o.info.Lines) }
func (o *fitOutput) PlacedEvents() []content.GroupEvent {
	events := make([]content.GroupEvent, len(o.events))
	for i, e := range o.events {
		events[i] = e.Event
	}
	return events
}

type fitInvocation struct {
	groupMeasurer
	breaker paragraph.Breaker
}

var _ content.GroupFitter = fitInvocation{}

func (i fitInvocation) FitGroup(ctx context.Context, handle content.GroupHandle, r content.GroupFitRequest, yield func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error) {
	h, ok := handle.(*preparedGroup)
	if !ok || h == nil || h.paragraph == nil || h.input.Group != r.Group || h.input.Owner != r.Owner {
		return layout.FitFailed, content.ErrInvalidPreparation
	}
	fail := func(err error) (layout.FitEnumeration, error) {
		return layout.FitFailed, groupError(h.input, content.StageFitting, err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	width, err := cssToDU(r.Width, false)
	if err != nil {
		return fail(err)
	}
	// A rounded-up target would turn an otherwise fitting fractional CSS width
	// into horizontal overflow. Quantize the receiving constraint inward.
	width = dimen.DU(math.Floor(float64(r.Width) * duPerCSSPixel))
	// Core uses MaxFloat32 for indivisible interior layout. Normal callers' finite
	// receiver heights were checked above; do not quantize a height upward before
	// typography's strict whole-line test.
	height := float64(r.Height) * .75
	if r.Height == math.MaxFloat32 {
		height = float64(dimen.Fil-1) / float64(dimen.BP)
	} else if _, err := cssToDU(r.Height, false); err != nil {
		return fail(err)
	}
	atoms, err := fitAtomicBoxes(h.input, r)
	if err != nil {
		return fail(err)
	}
	var checkpoint *paragraph.TrialCheckpoint
	var seen []eventKey
	if r.Checkpoint != nil {
		cp, ok := r.Checkpoint.(*fitCheckpoint)
		if !ok || cp == nil || cp.group != r.Group || cp.position != r.From || cp.checkpoint == nil {
			return fail(content.ErrPreparationProtocol)
		}
		checkpoint, seen = cp.checkpoint, cp.events
	} else if r.From != (layout.ParagraphPosition{}) {
		return fail(content.ErrPreparationProtocol)
	}
	trial, err := h.paragraph.Trial(paragraph.TrialRequest{Prefix: checkpoint, Layout: paragraph.LayoutRequest{
		Params:     paragraph.Params{Constraints: paragraph.Constraints{Width: width}, Breaker: i.breaker, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &h.lineLayout},
		Horizontal: &paragraph.HorizontalLayout{}, Atomics: atoms,
	}})
	if err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	prefixes, err := trial.FitPrefixes(height)
	if err != nil {
		return fail(err)
	}
	for _, info := range prefixes.Prefixes() {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if info.Status == paragraph.FitNoProgress {
			continue
		}
		if paragraphPoint(info.From) != r.From {
			return fail(content.ErrPreparationProtocol)
		}
		selected, err := prefixes.Select(info.Lines)
		if err != nil {
			return fail(err)
		}
		w, err := pointsToCSS(info.Width, false)
		if err != nil {
			return fail(err)
		}
		ht, err := pointsToCSS(info.Height, false)
		if err != nil {
			return fail(err)
		}
		placements, err := fitPlacements(h.input, r, selected)
		if err != nil {
			return fail(err)
		}
		coverage, err := mapFragmentSources(h.input, selected.Coverage())
		if err != nil {
			return fail(err)
		}
		events := mapFragmentEvents(h.input, info, selected.Coverage(), seen)
		end := paragraphPoint(info.To)
		a := layout.ParagraphAlternative{End: end, Complete: info.Status == paragraph.FitComplete, Extent: layout.InlineExtent{W: w, H: ht}, Atomics: placements,
			Output: &fitOutput{copyInput(h.input), info, selected.Fragment(), coverage, events}, AllowsMarginCollapse: len(selected.Fragment().Lines()) == 0 && ht == 0 && len(placements) == 0}
		if !a.Complete {
			accepted := slices.Clone(seen)
			for _, e := range events {
				key := eventKey{e.Event.Source, e.Event.Kind}
				if !slices.Contains(accepted, key) {
					accepted = append(accepted, key)
				}
			}
			a.Checkpoint = &fitCheckpoint{r.Group, end, selected.Checkpoint(), accepted}
		}
		if !yield(a) {
			return layout.FitStopped, nil
		}
	}
	return layout.FitExhausted, nil
}

func paragraphPoint(p paragraph.LogicalPosition) layout.ParagraphPosition {
	return layout.ParagraphPosition{Offset: p.Offset, Step: uint64(p.NextKnot)}
}

type fitAtomicView map[layout.BoxID]layout.RealizedAtomic

func (v fitAtomicView) Lookup(id layout.BoxID) (layout.RealizedAtomic, bool) {
	a, ok := v[id]
	return a, ok
}
func fitAtomicBoxes(g content.GroupInput, r content.GroupFitRequest) ([]paragraph.AtomicBox, error) {
	if nilService(r.Used) || len(r.Atomics) != len(g.Occurrences) {
		return nil, invalidMeasurement("missing fitting atomic values")
	}
	view := fitAtomicView{}
	for j, a := range r.Atomics {
		info, _ := g.Occurrences[j].Box.Info()
		if a.BoxID != info.ID {
			return nil, invalidMeasurement("atomic occurrence order/identity")
		}
		u, ok := r.Used.Lookup(info.ID)
		if !ok || u.Margin != a.Margin {
			return nil, invalidMeasurement("atomic margin association")
		}
		view[a.BoxID] = a.RealizedAtomic
	}
	return atomicBoxes(g, content.InlineLayoutRequest{Group: r.Group, Owner: r.Owner, Width: r.Width, Used: r.Used, Atomics: view})
}

func fitPlacements(g content.GroupInput, r content.GroupFitRequest, p *paragraph.FittedPrefix) ([]layout.AtomicPlacement, error) {
	var out []layout.AtomicPlacement
	for _, a := range p.Atomics() {
		index := int(a.Box.ID) - 1
		if index < 0 || index >= len(g.Occurrences) || r.Atomics[index].Consumed {
			return nil, fmt.Errorf("%w: foreign or repeated fitted atomic", content.ErrPreparationProtocol)
		}
		x, err := pointsToCSS(a.Placement.X, true)
		if err != nil {
			return nil, err
		}
		y, err := pointsToCSS(a.Placement.Y, true)
		if err != nil {
			return nil, err
		}
		if a.Placement.ID != contracts.AtomicID(g.Occurrences[index].ID) {
			return nil, content.ErrPreparationProtocol
		}
		out = append(out, layout.AtomicPlacement{BoxID: r.Atomics[index].BoxID, X: x, Y: y})
	}
	return out, nil
}
