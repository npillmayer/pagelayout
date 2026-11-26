package pagelayout

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/css-box-layout/registers"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

type pageEnumeration struct {
	ctx         context.Context
	r           *Pagination
	input       State
	t           *template.Template
	budget      workBudget
	backend     *pageFitter
	frames      []PageFrame
	chains      map[string][]int
	queues      map[string][]content.Box
	facts       PageAssessment
	steps       []headAdvance
	cursors     map[string]fitCursor
	hits        map[content.SourceRef]bool
	placed      map[BranchID]bool
	best        *Candidate
	bestRank    LocalRank
	enumeration uint64
}

func newPageEnumeration(ctx context.Context, r *Pagination, s State, t *template.Template) *pageEnumeration {
	x := &pageEnumeration{
		ctx: ctx, r: r, input: s, t: t,
		budget:  workBudget{limit: r.context.limits.LocalSteps},
		chains:  map[string][]int{},
		queues:  map[string][]content.Box{},
		cursors: maps.Clone(s.data.fits),
		hits:    map[content.SourceRef]bool{},
		placed:  map[BranchID]bool{},
	}
	if x.cursors == nil {
		x.cursors = map[string]fitCursor{}
	}
	x.backend = &pageFitter{backend: r.backend, budget: &x.budget, ctx: ctx}
	x.facts.MainRemaining = s.mainRemaining()
	x.facts.PendingAtEntry = x.pending(s)
	for i, f := range t.Frames() {
		x.frames = append(x.frames, PageFrame{Frame: f})
		x.chains[f.Flow] = append(x.chains[f.Flow], i)
		x.facts.Frames = append(x.facts.Frames, FrameAssessment{Channel: f.Flow, Width: float64(f.ContentBox.W), Height: float64(f.ContentBox.H)})
	}
	for _, channel := range r.context.channels {
		reg, _ := s.data.bank.Lookup(registers.RegisterID(channel))
		for box, rest, ok := reg.Take(); ok; box, rest, ok = rest.Take() {
			x.queues[channel] = append(x.queues[channel], box)
		}
	}
	return x
}

func (x *pageEnumeration) pending(s State) []PendingTarget {
	var out []PendingTarget
	for _, id := range s.Info().Pending {
		out = append(out, PendingTarget{id, x.r.context.plan.branches[int(id)-1].Channel})
	}
	return out
}

func (x *pageEnumeration) channels(index int) error {
	if err := x.budget.before(x.ctx); err != nil {
		return err
	}
	if index == len(x.r.context.channels) {
		return x.assess()
	}
	channel := x.r.context.channels[index]
	return x.frame(index, 0, 0, x.cursors[channel], channel == "main" && !x.facts.MainRemaining)
}

func (x *pageEnumeration) frame(channelIndex, frameIndex, headIndex int, cursor fitCursor, done bool) error {
	if err := x.budget.before(x.ctx); err != nil {
		return err
	}
	channel := x.r.context.channels[channelIndex]
	chain := x.chains[channel]
	if frameIndex == len(chain) || done {
		return x.channels(channelIndex + 1)
	}
	return x.fill(channelIndex, frameIndex, headIndex, cursor, nil)
}

// Branches may be placed speculatively within the current span. Final activation
// validation removes ineligible arrangements, so a later-visited channel can
// activate a target in an earlier-visited channel without changing FIFO order.
func (x *pageEnumeration) fill(ci, fi, hi int, cursor fitCursor, parts []*content.FitFragment) error {
	if err := x.budget.before(x.ctx); err != nil {
		return err
	}
	channel := x.r.context.channels[ci]
	frameIndex := x.chains[channel][fi]
	frame := x.frames[frameIndex].Frame
	queue := x.queues[channel]
	var head content.Box
	if hi < len(queue) {
		head = queue[hi]
	}
	where := DiagnosticLocation{Template: x.t.ID(), Frame: frame.ID, Channel: channel, Box: head, Position: cursor.end.Logical()}
	if head.Valid() && (channel == "main" || x.r.context.branches[head].Span == x.input.data.span && x.possibleActivation(ci)[x.r.context.branches[head].ID]) {
		f := x.r.fitters[head]
		if f == nil {
			var err error
			f, err = content.NewFitter(x.r.prepared, x.r.session, head)
			if err != nil {
				return pageExecutionError(where, err)
			}
			x.r.fitters[head] = f
		}
		var stop content.FitBoundary
		if channel == "main" {
			var err error
			stop, err = f.BoundaryAt(x.r.context.plan.spans[x.input.data.span-1].End)
			if err != nil {
				return pageExecutionError(where, err)
			}
		}
		if err := x.budget.before(x.ctx); err != nil {
			return pageExecutionError(where, err)
		}
		remaining := x.budget.limit - x.budget.used
		if remaining == 0 {
			return pageExecutionError(where, ErrLimit)
		}
		choices, err := content.Fit(x.ctx, x.r.prepared, x.r.session, f, content.FitRequest{Width: frame.ContentBox.W, Height: frame.ContentBox.H, Continuation: cursor.continuation, Stop: stop, MaxSteps: remaining, DeferVerticalBounds: true}, x.backend)
		if err != nil {
			return pageExecutionError(where, err)
		}
		if err = x.budget.after(x.ctx); err != nil {
			return pageExecutionError(where, err)
		}
		for _, choice := range choices {
			if !choice.Progress() {
				continue
			}
			fragment := choice.Fragment()
			hits, err := x.fragmentAnchors(head, fragment)
			if err != nil {
				return pageExecutionError(where, err)
			}
			oldHits, oldPlaced := x.hits, x.placed
			x.hits = maps.Clone(x.hits)
			for _, src := range hits {
				x.hits[src] = true
			}
			x.placed = maps.Clone(x.placed)
			x.placed[x.r.context.branches[head].ID] = true
			oldSteps, oldFacts := len(x.steps), len(x.facts.Fragments)
			for _, p := range fragment.Paragraphs() {
				output, ok := p.Output.(content.ParagraphOutput)
				if !ok {
					return pageExecutionError(where, fmt.Errorf("%w: paragraph line counts required", content.ErrPreparationProtocol))
				}
				x.facts.Fragments = append(x.facts.Fragments, ParagraphFragment{p.Group, frameIndex, output.LineCount(), !p.Complete || p.From != (layout.ParagraphPosition{})})
			}
			x.steps = append(x.steps, headAdvance{channel, head, choice.Complete(), choice.End().Logical()})
			oldCursor, hadCursor := x.cursors[channel]
			nextCursor := fitCursor{choice.Continuation(), choice.End()}
			if choice.Complete() {
				delete(x.cursors, channel)
			} else {
				x.cursors[channel] = nextCursor
			}
			nextParts := append(slices.Clone(parts), fragment)
			finished := channel == "main" && (choice.Complete() || choice.StopReached())
			if finished {
				x.facts.MainFinishes = true
				x.facts.MainFinishingFrame = frameIndex
			}
			if choice.Complete() && channel != "main" {
				// Extend within this receiver before offering the shorter prefix.
				err = x.fill(ci, fi, hi+1, fitCursor{}, nextParts)
			} else {
				err = x.endFrame(ci, fi, hi, nextCursor, finished, nextParts)
			}
			x.hits, x.placed = oldHits, oldPlaced
			x.steps = x.steps[:oldSteps]
			x.facts.Fragments = x.facts.Fragments[:oldFacts]
			if hadCursor {
				x.cursors[channel] = oldCursor
			} else {
				delete(x.cursors, channel)
			}
			if finished {
				x.facts.MainFinishes = false
			}
			if err != nil {
				return err
			}
		}
	}
	// Explicitly leave the rest of this frame unused. A future-span queue head
	// reaches this path without a backend call, and no later item is bypassed.
	return x.endFrame(ci, fi, hi, cursor, false, parts)
}

// Earlier channels have fixed placements; their unrendered owners cannot be
// reached by this arrangement. Later/current channels remain an upper bound,
// including ownership paths that run against channel traversal order.
func (x *pageEnumeration) possibleActivation(channelIndex int) map[BranchID]bool {
	possible := maps.Clone(x.input.data.activated)
	if possible == nil {
		possible = map[BranchID]bool{}
	}
	for changed := true; changed; {
		changed = false
		for _, a := range x.r.context.plan.anchors {
			owner := x.r.context.plan.branches[int(a.OwnerBranch)-1]
			unvisited := slices.Index(x.r.context.channels, owner.Channel) >= channelIndex
			if a.Span == x.input.data.span && !possible[a.Target] && (owner.Channel == "main" || possible[owner.ID]) && (x.hits[a.Source] || unvisited) {
				possible[a.Target] = true
				changed = true
			}
		}
	}
	return possible
}

func (x *pageEnumeration) endFrame(ci, fi, hi int, cursor fitCursor, done bool, parts []*content.FitFragment) error {
	if err := x.budget.before(x.ctx); err != nil {
		return err
	}
	index := x.chains[x.r.context.channels[ci]][fi]
	f := x.frames[index].Frame
	composed, err := content.ComposeFragments(f.ContentBox.W, f.ContentBox.H, parts)
	if err != nil {
		return pageExecutionError(DiagnosticLocation{Template: x.t.ID(), Frame: f.ID, Channel: f.Flow}, err)
	}
	if !composed.Fits {
		return nil
	}
	x.frames[index].fragments = composed.Fragments
	x.frames[index].extent = composed.Extent
	x.facts.Frames[index].Occupied = float64(composed.Extent.H)
	err = x.frame(ci, fi+1, hi, cursor, done)
	x.frames[index].fragments = nil
	x.frames[index].extent = layout.InlineExtent{}
	x.facts.Frames[index].Occupied = 0
	return err
}

func (x *pageEnumeration) assess() error {
	if len(x.steps) == 0 {
		return nil
	}
	active := maps.Clone(x.input.data.activated)
	if active == nil {
		active = map[BranchID]bool{}
	}
	for changed := true; changed; {
		changed = false
		for _, a := range x.r.context.plan.anchors {
			owner := x.r.context.plan.branches[int(a.OwnerBranch)-1]
			if a.Span == x.input.data.span && x.hits[a.Source] && !active[a.Target] && (owner.Channel == "main" || active[owner.ID]) {
				active[a.Target] = true
				changed = true
			}
		}
	}
	for id := range x.placed {
		if x.r.context.plan.branches[int(id)-1].Channel != "main" && !active[id] {
			return nil
		}
	}
	var activations []BranchID
	for _, b := range x.r.context.plan.branches {
		if active[b.ID] && !x.input.data.activated[b.ID] {
			activations = append(activations, b.ID)
		}
	}
	next, err := x.r.advance(x.input, x.steps, activations, true)
	if err != nil {
		return err
	}
	next.data.fits = maps.Clone(x.cursors)
	if err = x.r.Validate(next); err != nil {
		return err
	}
	x.facts.PendingAfter = x.pending(next)
	score, err := AssessPage(x.r.context.policy, x.facts)
	if err != nil {
		return err
	}
	rank := LocalRank{score.Total, stateProgress(next), x.enumeration}
	x.enumeration++
	preferred := x.best == nil
	if !preferred {
		comparison, e := CompareLocal(rank, x.bestRank)
		if e != nil {
			return e
		}
		preferred = comparison < 0
	}
	if preferred {
		frames := slices.Clone(x.frames)
		for i := range frames {
			frames[i].fragments = slices.Clone(frames[i].fragments)
		}
		x.best = &Candidate{placedMaterial: true, origin: x.input, successor: next, template: x.t.ID(), score: score, progress: rank.Progress, page: &Page{x.input.data.page, x.t.ID(), x.t.Page(), frames}}
		x.bestRank = rank
	}
	return nil
}

func (x *pageEnumeration) fragmentAnchors(head content.Box, f *content.FitFragment) ([]content.SourceRef, error) {
	branch := x.r.context.branches[head].ID
	var out []content.SourceRef
	for _, a := range x.r.context.plan.anchors {
		if a.OwnerBranch == branch && a.Span == x.input.data.span && !a.Position.IsText() && f.ContainsBoundary(a.Position) {
			out = append(out, a.Source)
		}
	}
	for _, p := range f.Paragraphs() {
		input, ok := x.r.prepared.Input(p.Group)
		if !ok {
			return nil, content.ErrPreparationProtocol
		}
		if len(input.Events) == 0 {
			continue
		}
		payload, ok := p.Output.(content.ParagraphEvents)
		if !ok {
			return nil, fmt.Errorf("%w: rendered anchor events required", content.ErrPreparationProtocol)
		}
		for _, e := range payload.PlacedEvents() {
			if !slices.Contains(input.Events, e) {
				return nil, fmt.Errorf("%w: foreign rendered event", content.ErrPreparationProtocol)
			}
			for _, a := range x.r.context.plan.anchors {
				if a.OwnerBranch == branch && a.Span == x.input.data.span && a.Source == e.Source && a.Position.IsText() {
					out = append(out, e.Source)
				}
			}
		}
	}
	return out, nil
}

func (x *pageEnumeration) obstructions() []QueueObstruction {
	var out []QueueObstruction
	for _, channel := range x.r.context.channels {
		queue := x.queues[channel]
		if len(queue) == 0 {
			continue
		}
		head := x.r.context.branches[queue[0]]
		o := QueueObstruction{Channel: channel, Head: head.ID}
		for _, box := range queue[1:] {
			b := x.r.context.branches[box]
			if x.input.data.activated[b.ID] {
				o.PendingBehind = append(o.PendingBehind, b.ID)
			}
		}
		switch {
		case channel == "main":
			if x.facts.MainRemaining && len(x.chains[channel]) == 0 {
				o.Reason = MissingReceiver
			}
		case head.Span > x.input.data.span:
			o.Reason = FutureSpanHead
		case !x.input.data.activated[head.ID]:
			o.Reason = AwaitingOwner
		case len(x.chains[channel]) == 0:
			o.Reason = MissingReceiver
		}
		if o.Reason != "" {
			out = append(out, o)
		}
	}
	return out
}
