package content

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
)

// ErrFitInterrupted means fitting stopped before all alternatives were enumerated.
// An interrupted enumeration cannot establish that no fitting alternative exists.
var ErrFitInterrupted = errors.New("content fitting interrupted before exhaustion")

// GroupFitRequest borrows only value views and an opaque checkpoint. Geometry
// is in CSS pixels; Height is a conservative ceiling before fragment edges and
// margins. Atomic interiors use a large ceiling and require a complete choice.
type GroupFitRequest struct {
	Group, Owner  Box
	Width, Height float32
	From          layout.ParagraphPosition
	Checkpoint    layout.FitPayload
	Used          layout.UsedValuesView
	Atomics       []layout.ParagraphAtomic
}

// GroupFitter executes retained preparation synchronously under the session's
// reservation. Honor the core prefix/payload protocol and a false yield. Never
// retain borrowed views/handles, release handles, or reenter the session.
type GroupFitter interface {
	GroupMeasurer
	FitGroup(context.Context, GroupHandle, GroupFitRequest, func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error)
}

// ParagraphOutput supplies the line count used to assess a frozen fragment.
// Zero lines is valid for empty material. The count describes this occurrence,
// not the entire source paragraph or its previously accepted prefix.
type ParagraphOutput interface {
	layout.FitPayload
	LineCount() uint64
}

// ParagraphEvents exposes annotations actually reached by the rendered prefix.
// Implement it for groups with owning-anchor events. Logical byte advancement
// alone is insufficient for silent discretionary material. Return copied events.
type ParagraphEvents interface{ PlacedEvents() []GroupEvent }

type fitIdentity struct {
	preparation PreparationID
	root        Box
}

// Fitter owns one immutable structural program and its preparation identity.
// It retains no session or backend between calls. Use the same fitter to resume;
// distinct fitters, including those for the same root, have distinct cursors.
type Fitter struct {
	id            *fitIdentity
	core          *layout.StructuralFitter
	relay         *fitRelay
	groups        map[Box]GroupInfo
	order         []Box
	boxes         map[layout.BoxID]Box
	before, after map[Box]layout.StructuralBoundary
}

// NewFitter checks the live pair and complete subtree preparation coverage,
// without executing any backend. The root must be a covered normal-flow block.
func NewFitter(p *PreparedSnapshot, s *PreparationSession, root Box) (*Fitter, error) {
	var out *Fitter
	err := s.withAccess(p, func(st *preparationState) error {
		if !p.Covers(root) || !layout.IsBlockLevel(root.node().Box) {
			return &PreparationError{Stage: StageAccess, Err: ErrInvalidPreparation}
		}
		f := &Fitter{id: &fitIdentity{p.ID(), root}, groups: map[Box]GroupInfo{}, boxes: map[layout.BoxID]Box{}}
		var walk func(*layout.LayoutNode)
		walk = func(n *layout.LayoutNode) {
			f.boxes[n.BoxID] = Box{root.owner, root.tree, n.BoxID}
			for _, c := range n.Children {
				walk(c)
			}
		}
		walk(root.node())
		f.before, f.after = map[Box]layout.StructuralBoundary{}, map[Box]layout.StructuralBoundary{}
		for _, b := range root.owner.trees[root.tree].provenance.Boundaries() {
			box := Box{root.owner, root.tree, b.Box}
			if b.Edge == layout.BoundaryBefore {
				f.before[box] = b
			} else {
				f.after[box] = b
			}
		}
		for g := range layout.InlineGroups(root.node()) {
			group, owner := f.boxes[g.Group.BoxID], f.boxes[g.Owner.BoxID]
			index, ok := st.index[group]
			if !ok || index < 0 || index >= len(st.owned) || st.owned[index].info.Owner != owner || nilInterface(st.owned[index].handle) {
				return fitFailure(root, ErrInvalidPreparation)
			}
			f.groups[group] = st.owned[index].info
			f.order = append(f.order, group)
		}
		f.relay = &fitRelay{fitter: f}
		var err error
		f.core, err = layout.NewStructuralFitterWithOptions(root.node(), f.relay, layout.StructuralFitOptions{Intrinsic: f.relay})
		if err != nil {
			return fitFailure(root, err)
		}
		out = f
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Root returns the source block covered by this fitter, or an invalid Box for a nil fitter.
func (f *Fitter) Root() Box {
	if f == nil || f.id == nil {
		return Box{}
	}
	return f.id.root
}

// FitContinuation is an immutable checkpoint qualified by fitter and preparation.
// The zero value requests the beginning of the fitted root; logical positions cannot
// replace a continuation. Resumption requires the matching live session.
type FitContinuation struct {
	id   *fitIdentity
	core layout.FitContinuation
}

// Valid reports whether this value contains an exact fitting checkpoint.
func (c FitContinuation) Valid() bool { return c.id != nil && c.core.Valid() }

// FitPosition is inspection/comparison only, never resumption authority.
type FitPosition struct {
	id      *fitIdentity
	core    layout.FitPosition
	logical LogicalPosition
}

// Logical returns the source-qualified inspection coordinate, including the
// backend's within-offset step. It is never resumption authority.
func (p FitPosition) Logical() LogicalPosition { return p.logical }

// Owns checks opaque continuation identity without executing a backend.
func (f *Fitter) Owns(c FitContinuation) bool {
	return f != nil && f.id != nil && c.id == f.id && c.Valid()
}

// FitPositionInfo describes a structural endpoint or paragraph position for inspection.
// Complete means the entire fitted root has been consumed.
type FitPositionInfo struct {
	Next      Box
	Paragraph layout.ParagraphPosition
	Complete  bool
}

// Info returns a copied description of the fitted endpoint.
func (p FitPosition) Info() FitPositionInfo {
	if p.id == nil {
		return FitPositionInfo{}
	}
	i := p.core.Info()
	next := Box{}
	if i.Next != 0 {
		next = Box{p.id.root.owner, p.id.root.tree, i.Next}
	}
	return FitPositionInfo{next, i.Paragraph, i.Complete}
}

// Compare orders positions from the same fitter. It returns false for different
// or invalid fitter identities; equal coordinates do not authorize cursor exchange.
func (p FitPosition) Compare(q FitPosition) (int, bool) {
	if p.id == nil || p.id != q.id {
		return 0, false
	}
	return p.core.Compare(q.core)
}

// FitBoundary is a structural stop qualified by one fitter. Its zero value imposes no stop.
type FitBoundary struct {
	id   *fitIdentity
	core layout.FitBoundary
}

// Boundary identifies the before/after edge of a box in this fitter. Foreign boxes
// and unsupported structural cuts return a PreparationError with the original cause.
func (f *Fitter) Boundary(box Box, edge layout.BoundaryEdge) (FitBoundary, error) {
	if f == nil || f.id == nil || f.boxes[box.id] != box || !box.Valid() {
		return FitBoundary{}, fitFailure(f.Root(), ErrInvalidPreparation)
	}
	b, err := f.core.Boundary(box.id, edge)
	if err != nil {
		return FitBoundary{}, fitFailure(box, err)
	}
	return FitBoundary{f.id, b}, nil
}

// BoundaryAt accepts structural source-plan positions. Text positions require
// paragraph coverage and cannot be turned into structural stop authority.
func (f *Fitter) BoundaryAt(p LogicalPosition) (FitBoundary, error) {
	if !p.Valid() {
		return FitBoundary{}, fitFailure(f.Root(), ErrInvalidPreparation)
	}
	if p.textPosition {
		return FitBoundary{}, fitFailure(p.box, layout.ErrUnsupportedFit)
	}
	return f.Boundary(p.box, p.edge)
}

// FitRequest supplies receiving dimensions in CSS pixels and an exact starting cursor.
// Zero Continuation starts at the root; zero Stop permits fitting to the root end.
// IncludeEmpty permits the core's nonprogressing alternative. MaxSteps zero imposes
// no core work cap. Dimensions must be finite and nonnegative.
type FitRequest struct {
	Width, Height float32
	Continuation  FitContinuation
	Stop          FitBoundary
	IncludeEmpty  bool
	MaxSteps      uint64
	// DeferVerticalBounds is for intermediate same-frame branch composition.
	// Call ComposeFragments and require Fits before publishing the result.
	DeferVerticalBounds bool
}

// Fit enumerates a complete candidate set transactionally. Interrupted/failed
// runs return nil, even if core discovered earlier alternatives. Prior cursors,
// source banks and frozen fragments remain valid. Serialize all session use.
func Fit(ctx context.Context, p *PreparedSnapshot, s *PreparationSession, f *Fitter, req FitRequest, backend GroupFitter) ([]FitAlternative, error) {
	var out []FitAlternative
	err := s.withAccess(p, func(st *preparationState) error {
		if f == nil || f.id == nil || f.id.preparation != p.ID() || !p.Covers(f.Root()) || nilInterface(backend) || ctx == nil {
			return &PreparationError{Stage: StageAccess, Err: ErrInvalidPreparation}
		}
		if req.Continuation.id != nil && req.Continuation.id != f.id || req.Stop.id != nil && req.Stop.id != f.id {
			return fitFailure(f.Root(), ErrInvalidPreparation)
		}
		f.relay.state, f.relay.backend = st, backend
		defer func() { f.relay.state, f.relay.backend = nil, nil }()
		var choices []layout.FitAlternative
		status, err := f.core.Enumerate(ctx, layout.FitRequest{Width: req.Width, Height: req.Height, Continuation: req.Continuation.core, Stop: req.Stop.core, IncludeEmpty: req.IncludeEmpty, MaxSteps: req.MaxSteps, DeferVerticalBounds: req.DeferVerticalBounds}, func(a layout.FitAlternative) bool { choices = append(choices, a); return true })
		if err != nil {
			return fitFailure(f.Root(), err)
		}
		if status != layout.FitExhausted {
			return fitFailure(f.Root(), ErrFitInterrupted)
		}
		for _, a := range choices {
			if err := ctx.Err(); err != nil {
				return fitFailure(f.Root(), err)
			}
			fragment, err := f.freezeFragment(a)
			if err != nil {
				return fitFailure(f.Root(), err)
			}
			logical, err := f.logicalPosition(a.End(), st)
			if err != nil {
				return fitFailure(f.Root(), err)
			}
			out = append(out, FitAlternative{fragment, FitContinuation{f.id, a.Continuation()}, FitPosition{f.id, a.End(), logical}, a.Progress(), a.Complete(), a.StopReached()})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (f *Fitter) logicalPosition(end layout.FitPosition, st *preparationState) (LogicalPosition, error) {
	root := f.Root()
	s := root.owner
	before, after := f.before, f.after
	i := end.Info()
	if i.Complete {
		return s.logical(root.tree, after[root], SourceRef{}), nil
	}
	box := f.boxes[i.Next]
	p := s.logical(root.tree, before[box], SourceRef{})
	if !p.Valid() {
		return LogicalPosition{}, ErrPreparationProtocol
	}
	if i.Paragraph != (layout.ParagraphPosition{}) {
		index, ok := st.prepared.index[box]
		if !ok {
			return LogicalPosition{}, ErrPreparationProtocol
		}
		input := st.prepared.inputs[index]
		if i.Paragraph.Offset > uint64(len(input.Text)) {
			return LogicalPosition{}, ErrPreparationProtocol
		}
		p.group, p.offset, p.textPosition, p.step = box, i.Paragraph.Offset, true, i.Paragraph.Step
		events := []PositionedEvent{{Position: p}}
		canonicalizeEventPositions([]GroupInput{input}, before, after, events)
		p = events[0].Position
	}
	return p, nil
}

func fitFailure(root Box, err error) error {
	var detail *PreparationError
	if errors.As(err, &detail) {
		return err
	}
	i, _ := root.Info()
	return &PreparationError{Stage: StageFitting, Owner: root, Source: i.Context, Err: err}
}

type fitRelay struct {
	fitter  *Fitter
	state   *preparationState
	backend GroupFitter
}

func (r *fitRelay) EnumerateParagraph(ctx context.Context, req layout.ParagraphFitRequest, yield func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error) {
	g, owner := r.fitter.boxes[req.Group], r.fitter.boxes[req.Owner]
	info, ok := r.fitter.groups[g]
	if !ok || info.Owner != owner || r.state == nil || r.backend == nil {
		return layout.FitFailed, ErrPreparationProtocol
	}
	var status layout.FitEnumeration
	err := r.state.useGroup(g, owner, func(h GroupHandle) error {
		var err error
		status, err = r.backend.FitGroup(ctx, h, GroupFitRequest{g, owner, req.Width, req.Height, req.From, req.Checkpoint, req.Used, req.Atomics}, yield)
		if err != nil {
			return preparationFailure(StageFitting, info, err)
		}
		return nil
	})
	if err != nil {
		return layout.FitFailed, err
	}
	return status, nil
}
func (r *fitRelay) MeasureInline(req layout.InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	if req.Group == nil || r.state == nil || r.backend == nil {
		return layout.IntrinsicWidths{}, ErrPreparationProtocol
	}
	g, owner := r.fitter.boxes[req.Group.BoxID], r.fitter.boxes[req.OwnerID]
	info, ok := r.fitter.groups[g]
	if !ok || info.Owner != owner {
		return layout.IntrinsicWidths{}, ErrPreparationProtocol
	}
	var out layout.IntrinsicWidths
	err := r.state.useGroup(g, owner, func(h GroupHandle) error {
		var err error
		out, err = r.backend.MeasureGroup(h, InlineMeasureRequest{g, owner, req.Edges, req.Atomics})
		if err != nil {
			return preparationFailure(StageMeasurement, info, err)
		}
		return nil
	})
	return out, err
}

// FragmentID identifies one paragraph occurrence, independently of its group's
// source identity. Alternatives and successive receivers have distinct IDs.
type FragmentID struct{ token *fragmentIdentity }
type fragmentIdentity struct{ marker byte }

// Valid reports whether this ID identifies a published paragraph occurrence.
func (id FragmentID) Valid() bool { return id.token != nil }

// FitParagraph describes one placed paragraph occurrence, including an atomic interior.
// From and To are backend logical positions; X, Y and Extent use receiver-relative
// CSS pixels. Output is frozen backend data, not a continuation.
type FitParagraph struct {
	ID                         FragmentID
	Group, Owner, ParentAtomic Box
	From, To                   layout.ParagraphPosition
	Complete                   bool
	X, Y                       float32 // receiving-rectangle coordinates, CSS pixels
	Extent                     layout.InlineExtent
	Output                     layout.FitPayload
}

// FitContainer describes one container occurrence with its used fragment edges and paint.
// First and Last control opening/closing edges. Geometry uses parent-content coordinates.
type FitContainer struct {
	Box         Box
	First, Last bool
	Geometry    layout.LayoutGeometry
	Edges       layout.BoxEdges
	Paint       Paint
}

// FitFragment is renderable selected material without continuation, fitter,
// session or backend. Geometry/paint are copied; paragraph payloads are cloned
// on inspection. It includes consumed atomic interiors, never suffix interiors.
type FitFragment struct {
	root       Box
	extent     layout.InlineExtent
	geometry   map[Box]layout.LayoutGeometry
	used       map[Box]layout.UsedValues
	containers []FitContainer
	paragraphs []FitParagraph
}

// Root returns the source branch root represented by this fragment.
func (f *FitFragment) Root() Box {
	if f == nil {
		return Box{}
	}
	return f.root
}

// Extent returns the standalone fitted allocation, before sibling composition.
// Use FrameComposition.Extent for the occupied extent of composed material.
func (f *FitFragment) Extent() layout.InlineExtent {
	if f == nil {
		return layout.InlineExtent{}
	}
	return f.extent
}

// Geometry returns a copied box-geometry map in CSS pixels. RelativeTo identifies
// the parent content origin; root geometry is relative to the receiver.
func (f *FitFragment) Geometry() map[Box]layout.LayoutGeometry {
	if f == nil {
		return nil
	}
	return maps.Clone(f.geometry)
}

// UsedValues returns a copied map of widths and edges resolved for this fragment.
func (f *FitFragment) UsedValues() map[Box]layout.UsedValues {
	if f == nil {
		return nil
	}
	return maps.Clone(f.used)
}

// Containers returns copied container occurrences with fragment-specific edges and paint.
func (f *FitFragment) Containers() []FitContainer {
	if f == nil {
		return nil
	}
	return slices.Clone(f.containers)
}

// Paragraphs returns copied paragraph occurrences, cloning each backend output payload.
func (f *FitFragment) Paragraphs() []FitParagraph {
	if f == nil {
		return nil
	}
	out := slices.Clone(f.paragraphs)
	for i := range out {
		out[i].Output = out[i].Output.Clone()
	}
	return out
}

// FitAlternative pairs a frozen fragment with its exact remainder and completion facts.
// Only alternatives returned by Fit are valid; inspect Progress before placing one.
type FitAlternative struct {
	fragment                 *FitFragment
	continuation             FitContinuation
	end                      FitPosition
	progress, complete, stop bool
}

// Fragment returns frozen output that can outlive the fitter and execution session.
func (a FitAlternative) Fragment() *FitFragment { return a.fragment }

// Continuation returns the exact checkpoint for resuming this alternative.
func (a FitAlternative) Continuation() FitContinuation { return a.continuation }

// End returns the fitted endpoint for inspection and comparison.
func (a FitAlternative) End() FitPosition { return a.end }

// Progress reports logical material advancement, including zero-height material.
func (a FitAlternative) Progress() bool { return a.progress }

// Complete reports exhaustion of the entire source root, not just the receiver.
func (a FitAlternative) Complete() bool { return a.complete }

// StopReached reports that fitting reached the requested structural stop.
func (a FitAlternative) StopReached() bool { return a.stop }

func (f *Fitter) freezeFragment(a layout.FitAlternative) (*FitFragment, error) {
	out := &FitFragment{root: f.Root(), extent: a.Extent(), geometry: map[Box]layout.LayoutGeometry{}, used: map[Box]layout.UsedValues{}}
	geometry := a.Geometry()
	for id, g := range geometry {
		out.geometry[f.boxes[id]] = g
	}
	for id, u := range a.UsedValues() {
		out.used[f.boxes[id]] = u
	}
	for _, c := range a.Containers() {
		box := f.boxes[c.Box]
		info, _ := box.Info()
		out.containers = append(out.containers, FitContainer{box, c.First, c.Last, c.Geometry, c.Edges, info.Paint})
	}
	paragraphs := map[Box]layout.ParagraphFragment{}
	add := func(p layout.ParagraphFragment) error {
		box := f.boxes[p.Group]
		info, ok := f.groups[box]
		if !ok || info.Owner != f.boxes[p.Owner] || nilInterface(p.Output) {
			return ErrPreparationProtocol
		}
		if _, exists := paragraphs[box]; exists {
			return fmt.Errorf("%w: duplicate group in one fragment", ErrPreparationProtocol)
		}
		paragraphs[box] = p
		return nil
	}
	for _, p := range a.Paragraphs() {
		if err := add(p); err != nil {
			return nil, err
		}
		for _, atomic := range p.Atomics() {
			for _, inner := range atomic.Paragraphs() {
				if err := add(inner); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, box := range f.order {
		p, ok := paragraphs[box]
		if !ok {
			continue
		}
		var x, y float32
		for id := p.Owner; id != 0; {
			g := geometry[id]
			x += g.Content.X
			y += g.Content.Y
			id = g.RelativeTo
		}
		out.paragraphs = append(out.paragraphs, FitParagraph{FragmentID{&fragmentIdentity{}}, box, f.boxes[p.Owner], f.groups[box].ParentAtomic, p.From, p.To, p.Complete, x, y, p.Extent, p.Output})
	}
	return out, nil
}
