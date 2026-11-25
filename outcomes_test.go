package pagelayout

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/pagelayout/content"
)

func testCandidate(t *testing.T, r *Pagination, from, to State) *Candidate {
	t.Helper()
	score, err := AssessPage(r.Policy(), PageAssessment{})
	if err != nil {
		t.Fatal(err)
	}
	return &Candidate{placedMaterial: true, origin: from, successor: to, template: r.catalog.IDs()[0], score: score, progress: stateProgress(to)}
}
func TestAttemptPublicationContracts(t *testing.T) {
	r, _, _ := testPagination(t)
	initial, next := firstSpanState(t, r)
	best := testCandidate(t, r, initial, next)
	t.Run("no-arrangement-fits", func(t *testing.T) {
		result, err := finishAttempt(initial, true, nil, nil)
		if err != nil || result.Outcome() != NoFit || result.Candidate() != nil {
			t.Fatal(result, err)
		}
	})
	t.Run("already-complete-span", func(t *testing.T) {
		result, err := finishAttempt(next, true, nil, nil)
		if err != nil || result.Outcome() != SpanComplete || result.Candidate() != nil {
			t.Fatal(result, err)
		}
	})
	t.Run("page-completes-span", func(t *testing.T) {
		result, err := finishAttempt(initial, true, best, nil)
		if err != nil || result.Outcome() != Constructed || !result.Candidate().Successor().SpanComplete() {
			t.Fatal(result, err)
		}
		saved := result.Candidate().Score()
		best.score.Page = FiniteLoss(999)
		best.progress[0].Channel = "changed"
		if !reflect.DeepEqual(saved, result.Candidate().Score()) || result.Candidate().Progress()[0].Channel != "main" {
			t.Fatal("publication borrowed scratch")
		}
		copy := result.Candidate().Progress()
		copy[0].Channel = "changed"
		if result.Candidate().Progress()[0].Channel != "main" {
			t.Fatal("inspection alias")
		}
	})
	best = testCandidate(t, r, initial, next)
	t.Run("boundary-metadata-is-not-placement", func(t *testing.T) {
		onlyMetadata := *best
		onlyMetadata.placedMaterial = false
		result, err := finishAttempt(initial, true, &onlyMetadata, nil)
		if !errors.Is(err, ErrInvalidRequest) || result.Candidate() != nil {
			t.Fatal("metadata treated as placement", result, err)
		}
	})
	t.Run("local-limit-after-feasible-alternative", func(t *testing.T) {
		result, err := finishAttempt(initial, false, best, nil)
		if !errors.Is(err, ErrLimit) || result.Candidate() != nil || result.Outcome() != 0 {
			t.Fatal(result, err)
		}
	})
	t.Run("late-channel-error", func(t *testing.T) {
		cause := errors.New("backend failure")
		result, err := finishAttempt(initial, true, best, cause)
		if !errors.Is(err, cause) || !errors.Is(err, ErrExecution) || result.Candidate() != nil {
			t.Fatal(result, err)
		}
		var typed *PaginationError
		if !errors.As(err, &typed) {
			t.Fatal("untyped failure")
		}
	})
	t.Run("foreign-candidate", func(t *testing.T) {
		bad := *best
		bad.origin = r.InitialState()
		result, err := finishAttempt(initial, true, &bad, nil)
		if !errors.Is(err, ErrInvalidRequest) || result.Candidate() != nil {
			t.Fatal(result, err)
		}
	})
	t.Run("metadata-does-not-make-page", func(t *testing.T) {
		data := *initial.data
		data.page++
		bad := *best
		bad.successor = State{&data}
		result, err := finishAttempt(initial, true, &bad, nil)
		if !errors.Is(err, ErrInvalidRequest) || result.Candidate() != nil {
			t.Fatal(result, err)
		}
	})
}
func TestSearchOutcomeContracts(t *testing.T) {
	r, _, _ := testPagination(t)
	initial, next := firstSpanState(t, r)
	candidate := testCandidate(t, r, initial, next)
	complete := &Sequence{pages: []*Candidate{candidate}, final: next, loss: candidate.score.Total}
	report, err := finishSearch(SearchComplete, complete, false, nil)
	if err != nil || report.Selected() == nil || report.Incumbent() != nil {
		t.Fatal(report, err)
	}
	complete.pages[0] = nil
	if report.Selected().Pages()[0] == nil {
		t.Fatal("borrowed sequence")
	}
	complete = &Sequence{pages: []*Candidate{candidate}, final: next, loss: candidate.score.Total}
	for _, tc := range []struct {
		out   SearchOutcome
		kind  ErrorKind
		cause error
	}{{SearchCancelled, Cancelled, context.Canceled}, {SearchLimited, LimitReached, ErrLimit}, {SearchFailed, ExecutionFailure, errors.New("failed")}} {
		err := paginationError(tc.kind, DiagnosticLocation{}, tc.cause)
		report, got := finishSearch(tc.out, complete, true, err)
		if !errors.Is(got, tc.cause) || report.Selected() != nil || report.Incumbent() == nil || !report.HeuristicTruncated() {
			t.Fatal(report, got)
		}
	}
	if _, err := finishSearch(SearchInfeasible, nil, true, nil); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("heuristic truncation called proof")
	}
	if r, err := finishSearch(SearchInfeasible, nil, false, nil); err != nil || r.Outcome() != SearchInfeasible {
		t.Fatal(r, err)
	}
	if r, err := finishSearch(SearchHeuristicExhausted, nil, true, nil); err != nil || r.Outcome() != SearchHeuristicExhausted {
		t.Fatal(r, err)
	}
	partial := *complete
	partial.final = initial
	if _, err := finishSearch(SearchLimited, &partial, false, ErrLimit); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("incomplete incumbent")
	}
}
func TestWorkLimitsAndCooperativeCancellation(t *testing.T) {
	b := workBudget{limit: 2}
	for range 2 {
		if err := b.before(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.before(context.Background()); !errors.Is(err, ErrLimit) || b.used != 2 {
		t.Fatal(b, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	b = workBudget{limit: 2}
	if err := b.before(ctx); err != nil {
		t.Fatal(err)
	}
	// A synchronous service returns before cancellation is observed.
	cancel()
	if err := b.after(ctx); !errors.Is(err, context.Canceled) || !errors.Is(err, ErrCancelled) {
		t.Fatal(err)
	}
	before := b.used
	if err := b.before(ctx); !errors.Is(err, ErrCancelled) || b.used != before {
		t.Fatal(err)
	}
	cause := errors.New("cause")
	location := DiagnosticLocation{Template: "T", Frame: 2, Channel: "notes"}
	err := paginationError(Unsupported, location, cause)
	var typed *PaginationError
	if !errors.Is(err, cause) || !errors.Is(err, ErrUnsupported) || !errors.As(err, &typed) || typed.Location != location {
		t.Fatal(err)
	}
}
func TestLocalAndSequenceComparators(t *testing.T) {
	rank := func(loss uint64, positions []uint64, enumeration uint64) LocalRank {
		channels := []string{"main", "notes", "figures"}
		r := LocalRank{Loss: FiniteLoss(loss), Enumeration: enumeration}
		for i, p := range positions {
			r.Progress = append(r.Progress, ChannelProgress{Channel: channels[i], Completed: p})
		}
		return r
	}
	short, long := rank(1160000, []uint64{6, 0, 0}, 0), rank(2040000, []uint64{8, 0, 0}, 1)
	if c, e := CompareLocal(short, long); e != nil || c >= 0 {
		t.Fatal("loss must beat progress", c, e)
	}
	order := []LocalRank{rank(1000000, []uint64{5, 4, 1}, 2), rank(1000000, []uint64{5, 4, 1}, 3), rank(1000000, []uint64{5, 3, 9}, 1), rank(1000000, []uint64{4, 9, 9}, 0)}
	for i := 1; i < len(order); i++ {
		if c, e := CompareLocal(order[i-1], order[i]); e != nil || c >= 0 {
			t.Fatal("main/secondary/enumeration ties", i, c, e)
		}
	}
	r, _, _ := testPagination(t)
	from, to := r.context.plan.document.TreeBounds(r.context.plan.document.Residual().Tree)
	a, b := rank(1, []uint64{0}, 0), rank(1, []uint64{0}, 0)
	a.Progress[0].Position = from
	b.Progress[0].Position = to
	if c, e := CompareLocal(a, b); e != nil || c <= 0 {
		t.Fatal("logical continuation tie", c, e)
	}
	other, _, _ := testPagination(t)
	foreign, _ := other.context.plan.document.TreeBounds(other.context.plan.document.Residual().Tree)
	b.Progress[0].Position = foreign
	if _, e := CompareLocal(a, b); !errors.Is(e, ErrInvalidRequest) {
		t.Fatal("foreign logical positions")
	}
	for _, pair := range [][2][]uint64{{{0, 2}, {1, 0}}, {{0, 1}, {0, 1, 0}}, {{0, 0}, {1}}} {
		if c := CompareSequences(SequenceRank{FiniteLoss(7), pair[0]}, SequenceRank{FiniteLoss(7), pair[1]}); c >= 0 {
			t.Fatal("lexicographic templates", pair, c)
		}
	}
	// Zero positions represent the head beginning, not a source-byte offset.
	a.Progress[0].Position = content.LogicalPosition{}
	if c, e := CompareLocal(a, rank(1, []uint64{1}, 0)); e != nil || c <= 0 {
		t.Fatal(c, e)
	}
}
