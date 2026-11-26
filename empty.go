package pagelayout

import (
	"context"
	"fmt"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/pagelayout/content"
)

// Drain only a certified empty remainder to the current span boundary. Using
// the existing structural fitter preserves open-container/cut authority; no
// source byte coordinate is converted into a guessed continuation. The source
// registers and backend handles remain unchanged, and no physical page is made.
func (r *Pagination) drainEmptyMain(ctx context.Context, s State) (State, error) {
	if err := r.Validate(s); err != nil {
		return State{}, err
	}
	if err := r.checkFitCursors(s); err != nil {
		return State{}, err
	}
	if !s.mainRemaining() {
		return s, nil
	}
	head, ok := s.data.bank.First("main")
	if !ok {
		return s, nil
	}
	info, _ := head.Info()
	from, _ := r.context.plan.document.TreeBounds(info.Tree)
	if cursor, ok := s.data.cursors["main"]; ok {
		from = cursor.position
	}
	to := r.context.plan.spans[s.data.span-1].End
	if !r.prepared.EmptyRange(head, from, to) {
		return s, nil
	}
	f := r.fitters[head]
	if f == nil {
		var err error
		f, err = content.NewFitter(r.prepared, r.session, head)
		if err != nil {
			return State{}, err
		}
		r.fitters[head] = f
	}
	stop, err := f.BoundaryAt(to)
	if err != nil {
		return State{}, err
	}
	choices, err := content.Fit(ctx, r.prepared, r.session, f, content.FitRequest{Continuation: s.data.fits["main"].continuation, Stop: stop, MaxSteps: r.context.limits.LocalSteps}, emptyFitter{r.prepared})
	if err != nil {
		return State{}, pageExecutionError(DiagnosticLocation{Channel: "main", Box: head, Position: from}, err)
	}
	for _, choice := range choices {
		if !choice.Progress() || !choice.Complete() && !choice.StopReached() {
			continue
		}
		if choice.Fragment().Extent() != (layout.InlineExtent{}) {
			return State{}, content.ErrPreparationProtocol
		}
		hits, err := r.fragmentAnchors(s, head, choice.Fragment())
		if err != nil {
			return State{}, err
		}
		var active []BranchID
		for _, a := range r.context.plan.anchors {
			if a.Span == s.data.span && slices.Contains(hits, a.Source) {
				active = append(active, a.Target)
			}
		}
		next, err := r.advance(s, []headAdvance{{"main", head, choice.Complete(), choice.End().Logical()}}, active, false)
		if err != nil {
			return State{}, err
		}
		if !choice.Complete() {
			if next.data.fits == nil {
				next.data.fits = map[string]fitCursor{}
			}
			next.data.fits["main"] = fitCursor{choice.Continuation(), choice.End()}
		}
		if err := r.Validate(next); err != nil {
			return State{}, err
		}
		return next, nil
	}
	return State{}, fmt.Errorf("%w: certified empty span did not reach boundary", content.ErrPreparationProtocol)
}

type emptyFitter struct{ prepared *content.PreparedSnapshot }

func (emptyFitter) MeasureGroup(content.GroupHandle, content.InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	return layout.IntrinsicWidths{}, content.ErrPreparationProtocol
}
func (f emptyFitter) FitGroup(_ context.Context, _ content.GroupHandle, req content.GroupFitRequest, yield func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error) {
	input, ok := f.prepared.Input(req.Group)
	if !ok || input.Text != "" || len(input.Atomics) > 0 || req.Checkpoint != nil || req.From != (layout.ParagraphPosition{}) {
		return layout.FitFailed, content.ErrPreparationProtocol
	}
	var events []content.GroupEvent
	for _, e := range input.Events {
		if e.Affinity == content.AffinityStructural {
			events = append(events, e)
		}
	}
	if !yield(layout.ParagraphAlternative{Complete: true, Output: emptyOutput{events}, AllowsMarginCollapse: true}) {
		return layout.FitStopped, nil
	}
	return layout.FitExhausted, nil
}

type emptyOutput struct{ events []content.GroupEvent }

func (p emptyOutput) Clone() layout.FitPayload           { return emptyOutput{slices.Clone(p.events)} }
func (emptyOutput) LineCount() uint64                    { return 0 }
func (p emptyOutput) PlacedEvents() []content.GroupEvent { return slices.Clone(p.events) }
