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

// PageFrame contains the compiled receiver and its ordered branch fragments.
// An unused receiver has no fragments and a zero occupied extent.
type PageFrame struct {
	Frame     template.FrameInfo
	fragments []*content.FitFragment
	extent    layout.InlineExtent
}

// Extent is the furthest allocated frame extent after composing all fragments.
func (f PageFrame) Extent() layout.InlineExtent { return f.extent }

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

// Fragments returns independent handles in register order. Geometry and paragraph
// origins are relative to this frame's ContentBox, including sibling placement.
func (f PageFrame) Fragments() []*content.FitFragment {
	out := make([]*content.FitFragment, len(f.fragments))
	for i, p := range f.fragments {
		copy := *p
		out[i] = &copy
	}
	return out
}
func (p *Page) Frames() []PageFrame {
	if p == nil {
		return nil
	}
	frames := slices.Clone(p.frames)
	for i := range frames {
		frames[i].fragments = frames[i].Fragments()
	}
	return frames
}

// PageAttempt enumerates complete arrangements across ordered flow chains,
// validates placement-dependent activation, and selects one by current-page
// loss, logical progress and stable enumeration order. Failures publish nothing.
func (r *Pagination) PageAttempt(ctx context.Context, input State, templateID string) (PageAttemptResult, error) {
	where := DiagnosticLocation{Template: templateID}
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
	if nilFittingBackend(r.backend) {
		return fail(InvalidRequest, fmt.Errorf("fitting backend required"))
	}
	for channel, cursor := range input.data.cursors {
		if !r.fitters[cursor.head].Owns(input.data.fits[channel].continuation) {
			return fail(InvalidRequest, fmt.Errorf("logical position without fitting checkpoint"))
		}
	}
	r.busy = true
	defer func() { r.busy = false }()
	run := newPageEnumeration(ctx, r, input, t)
	err := run.channels(0)
	if err == nil {
		err = run.budget.after(ctx)
	}
	if err != nil {
		return PageAttemptResult{}, pageExecutionError(where, err)
	}
	result, err := finishAttempt(input, true, run.best, nil)
	if err == nil && result.Outcome() == NoFit {
		result.obstructions = run.obstructions()
	}
	return result, err
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
