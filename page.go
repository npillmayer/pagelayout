package pagelayout

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

// Page is frozen rendering data, independent of execution and continuation
// lifetimes. Frame rectangles and paint are copied from the compiled catalog.
// Fragment coordinates are relative to the frame's ContentBox origin.
type Page struct {
	number   uint64
	template string
	info     template.PageInfo
	frames   []PageFrame
}

type PageFrame struct {
	Frame    template.FrameInfo
	Fragment *content.FitFragment // nil for an unused receiving frame
}

func (p *Page) Number() uint64 {
	if p == nil {
		return 0
	}
	return p.number
}
func (p *Page) Template() string {
	if p == nil {
		return ""
	}
	return p.template
}
func (p *Page) Info() template.PageInfo {
	if p == nil {
		return template.PageInfo{}
	}
	return p.info
}
func (p *Page) Frames() []PageFrame {
	if p == nil {
		return nil
	}
	frames := slices.Clone(p.frames)
	for i := range frames {
		if frames[i].Fragment != nil {
			fragment := *frames[i].Fragment
			frames[i].Fragment = &fragment
		}
	}
	return frames
}

// PageAttempt enumerates all supported arrangements for one template and
// selects one using current-page loss, logical progress, then enumeration order.
// It currently admits main-only documents. Secondary ownership/placement is
// diagnosed as unsupported rather than silently consuming or ignoring targets.
// Failure publishes neither a provisional page nor a successor.
func (r *Pagination) PageAttempt(ctx context.Context, input State, templateID string) (PageAttemptResult, error) {
	where := DiagnosticLocation{Template: templateID, Channel: "main"}
	fail := func(kind ErrorKind, cause error) (PageAttemptResult, error) {
		return PageAttemptResult{}, paginationError(kind, where, cause)
	}
	if ctx == nil {
		return fail(InvalidRequest, fmt.Errorf("context required"))
	}
	if err := r.Validate(input); err != nil {
		return PageAttemptResult{}, err
	}
	if r.busy {
		return fail(InvalidRequest, content.ErrSessionBusy)
	}
	t, ok := r.catalog.Lookup(templateID)
	if !ok {
		return fail(InvalidRequest, fmt.Errorf("unknown template %q", templateID))
	}
	if err := ctx.Err(); err != nil {
		return fail(Cancelled, err)
	}
	if input.SpanComplete() {
		return finishAttempt(input, true, nil, nil)
	}
	for _, b := range r.context.plan.branches {
		if b.Channel != "main" {
			return fail(Unsupported, fmt.Errorf("secondary channel %q requires multi-channel page construction", b.Channel))
		}
	}
	if nilFittingBackend(r.backend) {
		return fail(InvalidRequest, fmt.Errorf("fitting backend required"))
	}
	r.busy = true
	defer func() { r.busy = false }()
	head, ok := input.data.bank.First("main")
	if !ok {
		return fail(InvalidRequest, fmt.Errorf("unfinished span has no main material"))
	}
	where.Box = head
	where.Position = input.data.cursors["main"].position
	f := r.fitters[head]
	if f == nil {
		var err error
		f, err = content.NewFitter(r.prepared, r.session, head)
		if err != nil {
			return PageAttemptResult{}, pageExecutionError(where, err)
		}
		r.fitters[head] = f
	}
	initial := input.data.fits["main"]
	if _, has := input.data.cursors["main"]; has && !f.Owns(initial.continuation) {
		return fail(InvalidRequest, fmt.Errorf("logical position without fitting checkpoint"))
	}
	stop, err := f.BoundaryAt(r.context.plan.spans[input.data.span-1].End)
	if err != nil {
		return PageAttemptResult{}, pageExecutionError(where, err)
	}
	frames := t.Frames()
	chain := []int{}
	placements := make([]PageFrame, len(frames))
	facts := PageAssessment{MainRemaining: true}
	for i, frame := range frames {
		placements[i].Frame = frame
		facts.Frames = append(facts.Frames, FrameAssessment{Channel: frame.Flow, Width: float64(frame.ContentBox.W), Height: float64(frame.ContentBox.H)})
		if frame.Flow == "main" {
			chain = append(chain, i)
		}
	}
	budget := workBudget{limit: r.context.limits.LocalSteps}
	backend := &pageFitter{backend: r.backend, budget: &budget, ctx: ctx}
	var best *Candidate
	var bestRank LocalRank
	var enumeration uint64
	var explore func(int, fitCursor, bool, bool, bool) error
	// Each recursion selects a whole frame prefix. Empty placements are tried
	// last; no branch takes a locally longest-prefix shortcut.
	explore = func(index int, cursor fitCursor, progressed, finished, complete bool) error {
		if err := budget.before(ctx); err != nil {
			return err
		}
		if index == len(chain) || finished {
			if !progressed {
				return nil
			}
			position := cursor.end.Logical()
			next, err := r.advance(input, []headAdvance{{"main", head, complete, position}}, nil, true)
			if err != nil {
				return err
			}
			if !complete {
				if next.data.fits == nil {
					next.data.fits = map[string]fitCursor{}
				}
				next.data.fits["main"] = cursor
			}
			if err := r.Validate(next); err != nil {
				return err
			}
			score, err := AssessPage(r.context.policy, facts)
			if err != nil {
				return err
			}
			rank := LocalRank{score.Total, stateProgress(next), enumeration}
			enumeration++
			preferred := best == nil
			if !preferred {
				comparison, err := CompareLocal(rank, bestRank)
				if err != nil {
					return err
				}
				preferred = comparison < 0
			}
			if preferred {
				best = &Candidate{placedMaterial: true, origin: input, successor: next, template: templateID, score: score, progress: rank.Progress,
					page: &Page{input.data.page, templateID, t.Page(), slices.Clone(placements)}}
				bestRank = rank
			}
			return nil
		}
		frameIndex := chain[index]
		frame := frames[frameIndex]
		location := DiagnosticLocation{Template: templateID, Frame: frame.ID, Channel: "main", Box: head, Position: cursor.end.Logical()}
		if err := budget.before(ctx); err != nil {
			return pageExecutionError(location, err)
		}
		remaining := budget.limit - budget.used
		if remaining == 0 {
			return pageExecutionError(location, ErrLimit)
		}
		choices, err := content.Fit(ctx, r.prepared, r.session, f, content.FitRequest{
			Width: frame.ContentBox.W, Height: frame.ContentBox.H, Continuation: cursor.continuation, Stop: stop, MaxSteps: remaining,
		}, backend)
		if err != nil {
			return pageExecutionError(location, err)
		}
		if err := budget.after(ctx); err != nil {
			return pageExecutionError(location, err)
		}
		for _, choice := range choices {
			if !choice.Progress() {
				continue
			}
			fragment := choice.Fragment()
			oldCount := len(facts.Fragments)
			for _, p := range fragment.Paragraphs() {
				output, ok := p.Output.(content.ParagraphOutput)
				if !ok {
					return pageExecutionError(location, fmt.Errorf("%w: paragraph output must supply line counts", content.ErrPreparationProtocol))
				}
				facts.Fragments = append(facts.Fragments, ParagraphFragment{p.Group, frameIndex, output.LineCount(), !p.Complete || p.From != (layout.ParagraphPosition{})})
			}
			placements[frameIndex].Fragment = fragment
			facts.Frames[frameIndex].Occupied = float64(fragment.Extent().H)
			facts.MainFinishes = choice.StopReached() || choice.Complete()
			facts.MainFinishingFrame = frameIndex
			err = explore(index+1, fitCursor{choice.Continuation(), choice.End()}, true, facts.MainFinishes, choice.Complete())
			placements[frameIndex].Fragment = nil
			facts.Frames[frameIndex].Occupied = 0
			facts.Fragments = facts.Fragments[:oldCount]
			facts.MainFinishes = false
			if err != nil {
				return err
			}
		}
		return explore(index+1, cursor, progressed, false, false)
	}
	err = explore(0, initial, false, false, false)
	if err != nil {
		return PageAttemptResult{}, pageExecutionError(where, err)
	}
	if err := budget.after(ctx); err != nil {
		return PageAttemptResult{}, pageExecutionError(where, err)
	}
	return finishAttempt(input, true, best, nil)
}

func nilFittingBackend(b content.GroupFitter) bool {
	if b == nil {
		return true
	}
	v := reflect.ValueOf(b)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		return v.IsNil()
	}
	return false
}

func pageExecutionError(where DiagnosticLocation, err error) error {
	kind := ExecutionFailure
	var detail *PaginationError
	if errors.As(err, &detail) {
		kind = detail.Kind
		if detail.Location.Template != "" {
			return err
		}
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		kind = Cancelled
	case errors.Is(err, layout.ErrFitLimit), errors.Is(err, ErrLimit):
		kind = LimitReached
	case errors.Is(err, layout.ErrUnsupportedFit):
		kind = Unsupported
	}
	return paginationError(kind, where, err)
}

// Count orchestration work and synchronous backend units across the entire
// attempt. Each core call additionally caps its internal structural traversal.
type pageFitter struct {
	backend content.GroupFitter
	budget  *workBudget
	ctx     context.Context
}

func (b *pageFitter) MeasureGroup(h content.GroupHandle, r content.InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	if err := b.budget.before(b.ctx); err != nil {
		return layout.IntrinsicWidths{}, err
	}
	w, err := b.backend.MeasureGroup(h, r)
	if err != nil {
		return layout.IntrinsicWidths{}, err
	}
	return w, b.budget.after(b.ctx)
}
func (b *pageFitter) FitGroup(ctx context.Context, h content.GroupHandle, r content.GroupFitRequest, yield func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error) {
	if err := b.budget.before(ctx); err != nil {
		return layout.FitFailed, err
	}
	var stopped error
	status, err := b.backend.FitGroup(ctx, h, r, func(a layout.ParagraphAlternative) bool {
		if stopped != nil {
			return false
		}
		stopped = b.budget.before(ctx)
		if stopped != nil {
			return false
		}
		return yield(a)
	})
	if stopped != nil {
		return layout.FitFailed, stopped
	}
	if err != nil {
		return layout.FitFailed, err
	}
	if err := b.budget.after(ctx); err != nil {
		return layout.FitFailed, err
	}
	return status, nil
}
