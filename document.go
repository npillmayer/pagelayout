package pagelayout

import (
	"context"
	"fmt"
	"slices"

	"github.com/npillmayer/pagelayout/content"
)

// DocumentComplete reports terminal span exhaustion and complete transitive
// draining. It performs no execution and remains inspectable after session close.
func (s State) DocumentComplete() bool {
	return s.Valid() && s.data.span == len(s.data.context.plan.spans) && s.SpanComplete()
}

// AdvanceSpan derives the next span's entry from an exactly drained state.
// Numbering, bank roots and opaque main continuation are preserved. Terminal,
// foreign, incomplete, closed-session and reentrant requests fail unchanged.
func (r *Pagination) AdvanceSpan(input State) (State, error) {
	if err := r.Validate(input); err != nil {
		return State{}, err
	}
	if r.busy {
		return State{}, paginationError(InvalidRequest, DiagnosticLocation{}, content.ErrSessionBusy)
	}
	if err := r.checkFitCursors(input); err != nil {
		return State{}, err
	}
	return r.advanceSpan(input)
}

func (r *Pagination) checkFitCursors(s State) error {
	for channel, cursor := range s.data.cursors {
		if !r.fitters[cursor.head].Owns(s.data.fits[channel].continuation) {
			return paginationError(InvalidRequest, DiagnosticLocation{Channel: channel, Box: cursor.head, Position: cursor.position}, fmt.Errorf("logical position without fitting checkpoint"))
		}
	}
	return nil
}

// DocumentResult retains only selected spans, plus the last search report.
// An incomplete result's State is the last selected boundary or the entry to
// the unsuccessful span, suitable for explicit retry while the session lives.
// LastSearch may contain a completed but unselected incumbent; Spans never does.
type DocumentResult struct {
	spans    []SearchReport
	state    State
	last     SearchReport
	complete bool
}

// Complete reports that terminal draining succeeded for the entire document.
func (d DocumentResult) Complete() bool { return d.complete }

// State returns the exact completion or retry state. Invalid requests can return
// a zero state; execution failures never replace it with a speculative successor.
func (d DocumentResult) State() State { return d.state }

// Spans returns independent reports for selections made by this invocation.
func (d DocumentResult) Spans() []SearchReport { return slices.Clone(d.spans) }

// LastSearch returns the last attempted span search, or zero if none ran.
// Interruption between searches can leave this at the preceding successful span.
func (d DocumentResult) LastSearch() SearchReport { return d.last }

// Paginate selects successive complete spans from an explicit starting state.
// Each selection is atomic and never reconsidered for a later span's layout or
// parity. Previously selected spans survive later failure or interruption. No
// pages are delivered here. Work limits restart for each new pagespan search.
func (r *Pagination) Paginate(ctx context.Context, input State) (DocumentResult, error) {
	if ctx == nil {
		return DocumentResult{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("context required"))
	}
	if err := r.Validate(input); err != nil {
		return DocumentResult{}, err
	}
	if r.busy {
		return DocumentResult{}, paginationError(InvalidRequest, DiagnosticLocation{}, content.ErrSessionBusy)
	}
	if err := r.checkFitCursors(input); err != nil {
		return DocumentResult{}, err
	}
	r.busy = true
	defer func() { r.busy = false }()
	out := DocumentResult{state: input}
	for {
		if err := ctx.Err(); err != nil {
			return out, executionError(err)
		}
		if out.state.DocumentComplete() {
			out.complete = true
			return out, nil
		}
		// Resuming a selected boundary never selects or delivers it again.
		if out.state.SpanComplete() {
			next, err := r.advanceSpan(out.state)
			if err != nil {
				return out, err
			}
			out.state = next
			continue
		}
		report, err := r.paginationSearch(ctx, out.state)
		out.last = report
		if err != nil || report.Outcome() != SearchComplete {
			return out, err
		}
		out.spans = append(out.spans, report)
		out.state = report.selected.final
		if out.state.DocumentComplete() {
			out.complete = true
			return out, nil
		}
	}
}
