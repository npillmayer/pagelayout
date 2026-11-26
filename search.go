package pagelayout

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

// TemplateEligibility filters search edges without changing catalog order.
// It must be deterministic for the supplied immutable state/template, and must
// not reenter pagination or operate on its borrowed preparation session. Errors
// abort search; false excludes this template for this state only. PageAttempt
// itself remains an explicit one-template operation and does not use this filter.
type TemplateEligibility func(State, *template.Template) (bool, error)

// PaginationSearch selects one complete current pagespan. It neither advances
// the span nor delivers pages. Width truncation is heuristic; without it the
// result is optimal only under the configured eligibility and local page policy.
// Errors retain any fully completed incumbent as explicitly unselected.
func (r *Pagination) PaginationSearch(ctx context.Context, input State) (SearchReport, error) {
	fail := func(cause error) (SearchReport, error) {
		return finishSearch(SearchFailed, nil, false, paginationError(InvalidRequest, DiagnosticLocation{}, cause))
	}
	if ctx == nil {
		return fail(fmt.Errorf("context required"))
	}
	if err := r.Validate(input); err != nil {
		return finishSearch(SearchFailed, nil, false, err)
	}
	if r.busy {
		return fail(content.ErrSessionBusy)
	}
	r.busy = true
	defer func() { r.busy = false }()
	return r.paginationSearch(ctx, input)
}

// paginationSearch runs under the public search/document execution guard.
func (r *Pagination) paginationSearch(ctx context.Context, input State) (SearchReport, error) {
	fail := func(cause error) (SearchReport, error) {
		return finishSearch(SearchFailed, nil, false, paginationError(InvalidRequest, DiagnosticLocation{}, cause))
	}
	if err := ctx.Err(); err != nil {
		return finishSearch(SearchCancelled, nil, false, err)
	}
	var err error
	input, err = r.drainEmptyMain(ctx, input)
	if err != nil {
		return finishSearch(searchErrorOutcome(err), nil, false, err)
	}
	complete := input.SpanComplete()
	if !complete && nilFittingBackend(r.backend) {
		return fail(fmt.Errorf("fitting backend required"))
	}
	search := beamSearch[State]{
		templates: r.catalog.IDs(), width: r.context.beam,
		limits: r.context.limits, minimum: FiniteLoss(r.context.policy.Page),
		complete: State.SpanComplete,
		eligible: func(s State, id string) (bool, error) {
			if r.eligible == nil {
				return true, nil
			}
			t, _ := r.catalog.Lookup(id)
			return r.eligible(s, t)
		},
		attempt: func(ctx context.Context, s State, id string) (beamStep[State], error) {
			out, err := r.pageAttempt(ctx, s, id)
			if err != nil {
				return beamStep[State]{}, err
			}
			if out.Outcome() == NoFit {
				return beamStep[State]{}, nil
			}
			c := out.Candidate()
			if out.Outcome() != Constructed || c == nil {
				return beamStep[State]{}, fmt.Errorf("unexpected page outcome for unfinished span")
			}
			return beamStep[State]{state: c.successor, page: c, loss: c.score.Total, fit: true}, nil
		},
	}
	search.start(input)
	err = search.run(ctx)
	var sequence *Sequence
	if best := search.incumbent; best != nil {
		sequence = &Sequence{final: best.state, loss: best.rank.Loss}
		for p := best.tail; p != nil; p = p.previous {
			sequence.pages = append(sequence.pages, p.page)
		}
		slices.Reverse(sequence.pages)
	}
	if err == nil {
		err = ctx.Err()
	}
	report, err := finishSearch(search.outcome(err), sequence, search.stats.WidthPruned > 0, err)
	report.stats = search.stats
	return report, err
}

// Only the state parameter is abstract, so finite graph tests exercise the same
// scheduler without forging content cursors or doing paragraph layout. Production
// edges retain their Page candidates; output prefixes share immutable ancestors.
type beamStep[S any] struct {
	state S
	page  *Candidate
	loss  Loss
	fit   bool
}
type searchPath[S any] struct {
	state S
	tail  *searchPrefix
	rank  SequenceRank
}

// Prefix sharing retains only page output. Keeping whole ancestor search paths
// here would also retain every cumulative rank slice, quadratic in page depth.
type searchPrefix struct {
	previous *searchPrefix
	page     *Candidate
}
type beamSearch[S any] struct {
	templates []string
	width     int
	limits    WorkLimits
	minimum   Loss
	complete  func(S) bool
	eligible  func(S, string) (bool, error)
	attempt   func(context.Context, S, string) (beamStep[S], error)
	frontier  []*searchPath[S]
	incumbent *searchPath[S]
	stats     SearchStats
}

func (b *beamSearch[S]) start(s S) {
	initial := &searchPath[S]{state: s}
	if b.complete(s) {
		b.incumbent = initial
	} else {
		b.frontier = []*searchPath[S]{initial}
	}
}

func (b *beamSearch[S]) run(ctx context.Context) error {
	for len(b.frontier) > 0 {
		if err := b.round(ctx); err != nil {
			return err
		}
	}
	return ctx.Err()
}

// Every retained parent is extended through all eligible templates before any
// unfinished successors are pruned. Completing paths never take a beam slot.
func (b *beamSearch[S]) round(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.stats.Rounds >= b.limits.DepthRounds {
		return fmt.Errorf("%w: depth rounds %d", ErrLimit, b.limits.DepthRounds)
	}
	var next []*searchPath[S]
	for _, parent := range b.frontier {
		for ordinal, id := range b.templates {
			where := DiagnosticLocation{Template: id}
			if err := ctx.Err(); err != nil {
				return pageExecutionError(where, err)
			}
			if b.eligible != nil {
				ok, err := b.eligible(parent.state, id)
				if err != nil {
					return pageExecutionError(where, err)
				}
				if err = ctx.Err(); err != nil {
					return pageExecutionError(where, err)
				}
				if !ok {
					continue
				}
			}
			if b.stats.Attempts >= b.limits.SpanAttempts {
				return pageExecutionError(where, fmt.Errorf("%w: span attempts %d", ErrLimit, b.limits.SpanAttempts))
			}
			b.stats.Attempts++
			step, err := b.attempt(ctx, parent.state, id)
			if err != nil {
				return pageExecutionError(where, err)
			}
			if err = ctx.Err(); err != nil {
				return pageExecutionError(where, err)
			}
			if !step.fit {
				b.stats.NoFit++
				continue
			}
			if step.loss.blocked || step.loss.Compare(b.minimum) < 0 {
				return pageExecutionError(where, ErrInvalidCost)
			}
			loss, err := parent.rank.Loss.Add(step.loss)
			if err != nil {
				return pageExecutionError(where, err)
			}
			path := &searchPath[S]{state: step.state, tail: &searchPrefix{parent.tail, step.page},
				rank: SequenceRank{loss, append(slices.Clone(parent.rank.Templates), uint64(ordinal))}}
			if b.complete(step.state) {
				b.stats.Completions++
				if b.incumbent == nil || CompareSequences(path.rank, b.incumbent.rank) < 0 {
					b.incumbent = path
				}
			} else {
				next = append(next, path)
			}
		}
	}
	// Check bounds against the best completion of the whole round, so pruning
	// counts and the surviving frontier do not depend on when it was discovered.
	retained := next[:0]
	for _, path := range next {
		if err := ctx.Err(); err != nil {
			return err
		}
		if b.incumbent != nil {
			bound, err := path.rank.Loss.Add(b.minimum)
			if err != nil {
				return err
			}
			if bound.Compare(b.incumbent.rank.Loss) > 0 {
				b.stats.SafePruned++
				continue
			}
		}
		retained = append(retained, path)
	}
	slices.SortStableFunc(retained, func(a, c *searchPath[S]) int { return CompareSequences(a.rank, c.rank) })
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(retained) > b.width {
		b.stats.WidthPruned += uint64(len(retained) - b.width)
		retained = retained[:b.width]
	}
	// Release discarded branches rather than retaining them in slice capacity.
	clear(next[len(retained):])
	b.frontier = retained
	b.stats.Rounds++
	return nil
}

func (b *beamSearch[S]) outcome(err error) SearchOutcome {
	if err != nil {
		return searchErrorOutcome(err)
	}
	switch {
	case b.incumbent != nil:
		return SearchComplete
	case b.stats.WidthPruned > 0:
		return SearchHeuristicExhausted
	default:
		return SearchInfeasible
	}
}

func searchErrorOutcome(err error) SearchOutcome {
	switch {
	case errors.Is(err, ErrCancelled), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return SearchCancelled
	case errors.Is(err, ErrLimit):
		return SearchLimited
	default:
		return SearchFailed
	}
}
