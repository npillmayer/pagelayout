package pagelayout

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

func sequenceNames(s *Sequence) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s.pages))
	for i, p := range s.Pages() {
		out[i] = p.Template()
	}
	return out
}

// Exhaustively traverse the actual one-result-per-template API. This reference
// uses neither the beam scheduler nor its lower bound, and never merges states.
func exhaustivePages(t *testing.T, r *Pagination, initial State) *Sequence {
	t.Helper()
	var best *Sequence
	var bestOrdinals []uint64
	var visit func(State, []*Candidate, []uint64, uint64)
	visit = func(s State, pages []*Candidate, ordinals []uint64, cost uint64) {
		if len(pages) > 12 {
			t.Fatal("fixture did not terminate")
		}
		if s.SpanComplete() {
			if best == nil || cost < best.loss.units || cost == best.loss.units && slices.Compare(ordinals, bestOrdinals) < 0 {
				best = &Sequence{pages: slices.Clone(pages), final: s, loss: FiniteLoss(cost)}
				bestOrdinals = slices.Clone(ordinals)
			}
			return
		}
		for i, id := range r.catalog.IDs() {
			out, err := r.PageAttempt(t.Context(), s, id)
			if err != nil {
				t.Fatal(err)
			}
			if out.Outcome() == NoFit {
				continue
			}
			if out.Outcome() != Constructed {
				t.Fatal("unexpected completion")
			}
			c := out.Candidate()
			visit(c.Successor(), append(slices.Clone(pages), c), append(slices.Clone(ordinals), uint64(i)), cost+c.Score().Total.units)
		}
	}
	visit(initial, nil, nil, 0)
	return best
}

func TestPaginationSearchMatchesExhaustivePages(t *testing.T) {
	for _, fixture := range []string{"changed-width", "transitive.html", "cross-span-fifo.html"} {
		t.Run(fixture, func(t *testing.T) {
			policy := LossPolicy{Page: 1}
			r, _ := pageRun(t, `<body style="hyphens:none">ABCDEFGHIJ</body>`, [][][2]int{{{100, 20}}, {{50, 30}}}, &pageBackend{widthDependent: true}, &policy, nil)
			if fixture != "changed-width" {
				html, err := os.ReadFile("testdata/pagination/content/" + fixture)
				if err != nil {
					t.Fatal(err)
				}
				r = channelsRun(t, string(html), &pageBackend{whole: true}, nil, []channelFrame{{"main", 100, 100}})
				catalog, err := template.LoadFS(os.DirFS("testdata/pagination"), "templates/receivers.html")
				if err != nil {
					t.Fatal(err)
				}
				r.catalog = *catalog
			}
			initial := r.InitialState()
			before := initial.Info()
			ref := exhaustivePages(t, r, initial)
			got, err := r.PaginationSearch(t.Context(), initial)
			if err != nil || got.HeuristicTruncated() {
				t.Fatal(got, err)
			}
			if ref == nil {
				if got.Outcome() != SearchInfeasible || got.Selected() != nil {
					t.Fatal(got)
				}
				return
			}
			if got.Outcome() != SearchComplete || got.Incumbent() != nil || got.Selected().Loss() != ref.loss || !slices.Equal(sequenceNames(got.Selected()), sequenceNames(ref)) {
				t.Fatal(sequenceNames(got.Selected()), sequenceNames(ref), got)
			}
			if !reflect.DeepEqual(before, initial.Info()) || !got.Selected().FinalState().SpanComplete() {
				t.Fatal("search mutated input or failed to drain")
			}
			if fixture == "transitive.html" && (!slices.Equal(sequenceNames(got.Selected()), []string{"note-first", "figures-only"}) || got.Selected().Loss().units != 3_840_278 || got.Selected().FinalState().Info().Span != 1) {
				t.Fatal("span scope or scoring", got.Selected())
			}
			if err := r.Validate(got.Selected().FinalState()); err != nil {
				t.Fatal(err)
			}
			for i, c := range got.Selected().Pages() {
				if pageText(c.Page()) != pageText(ref.pages[i].Page()) || c.Page().Number() != initial.Info().NextPage+uint64(i) {
					t.Fatal("different frozen page")
				}
			}
			// Inspection returns independent handles, including writable zero values.
			names := sequenceNames(got.Selected())
			selected := got.Selected()
			pages := selected.Pages()
			*pages[0] = Candidate{}
			*selected = Sequence{}
			if !slices.Equal(names, sequenceNames(got.Selected())) {
				t.Fatal("sequence inspection alias")
			}
		})
	}
}

func TestPaginationSearchEligibilityAndReentry(t *testing.T) {
	b := &pageBackend{}
	base, s := pageRun(t, `<body style="hyphens:none">AB</body>`, [][][2]int{{{100, 10}}, {{100, 10}}}, b, &LossPolicy{Page: 1}, nil)
	var calls []string
	var r *Pagination
	filter := func(state State, tpl *template.Template) (bool, error) {
		calls = append(calls, tpl.ID())
		if out, err := r.PageAttempt(t.Context(), state, tpl.ID()); !errors.Is(err, content.ErrSessionBusy) || out.Candidate() != nil {
			t.Fatal("page reentry", err)
		}
		if out, err := r.PaginationSearch(t.Context(), state); !errors.Is(err, content.ErrSessionBusy) || out.Selected() != nil {
			t.Fatal("search reentry", err)
		}
		// Alternate template eligibility using the actual physical page number.
		eligible := tpl.ID() == "t0" && state.Info().NextPage%2 == 1 || tpl.ID() == "t1" && state.Info().NextPage%2 == 0
		*tpl = template.Template{} // the catalog must survive mutable handle inspection
		return eligible, nil
	}
	var err error
	r, err = NewPagination(PaginationConfig{Plan: base.context.plan, Catalog: &base.catalog, Prepared: base.prepared, Session: s, Fitter: b, Policy: &LossPolicy{Page: 1}, Eligible: filter})
	if err != nil {
		t.Fatal(err)
	}
	report, err := r.PaginationSearch(t.Context(), r.InitialState())
	if err != nil || !slices.Equal(sequenceNames(report.Selected()), []string{"t0", "t1"}) || !slices.Equal(calls, []string{"t0", "t1", "t0", "t1"}) || report.Stats().Attempts != 2 {
		t.Fatal(report, err, calls)
	}
	calls = nil
	if out, err := r.PageAttempt(t.Context(), r.InitialState(), "t1"); err != nil || out.Outcome() != Constructed || len(calls) != 0 {
		t.Fatal("explicit attempt filtered", err)
	}
	r.eligible = func(State, *template.Template) (bool, error) { return false, nil }
	if out, err := r.PaginationSearch(t.Context(), r.InitialState()); err != nil || out.Outcome() != SearchInfeasible || out.Stats().Attempts != 0 {
		t.Fatal(out, err)
	}
	boom := errors.New("eligibility failure")
	r.eligible = func(State, *template.Template) (bool, error) { return false, boom }
	out, err := r.PaginationSearch(t.Context(), r.InitialState())
	var detail *PaginationError
	if !errors.Is(err, boom) || out.Outcome() != SearchFailed || !errors.As(err, &detail) || detail.Location.Template != "t0" || out.Stats().Attempts != 0 {
		t.Fatal(out, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r.eligible = func(State, *template.Template) (bool, error) { cancel(); return false, nil }
	if out, err := r.PaginationSearch(ctx, r.InitialState()); !errors.Is(err, context.Canceled) || out.Outcome() != SearchCancelled || out.Stats().Attempts != 0 {
		t.Fatal(out, err)
	}
}

func TestPaginationSearchInterruptionsRetainOnlyCompleteIncumbents(t *testing.T) {
	for _, mode := range []string{"backend", "cancel", "attempt-limit", "depth-limit", "local-limit"} {
		t.Run(mode, func(t *testing.T) {
			b := &pageBackend{}
			policy := LossPolicy{Page: 1}
			r, _ := pageRun(t, `<body style="hyphens:none">ABCD</body>`, [][][2]int{{{100, 40}}, {{50, 20}}}, b, &policy, nil)
			initial := r.InitialState()
			before := initial.Info()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			boom := errors.New("later template failure")
			want := ErrExecution
			switch mode {
			case "backend", "cancel":
				b.before = func(req content.GroupFitRequest) error {
					if req.Width == 50 {
						if mode == "cancel" {
							cancel()
							return nil
						}
						return boom
					}
					return nil
				}
				if mode == "cancel" {
					want = ErrCancelled
				} else {
					want = boom
				}
			case "attempt-limit":
				r.context.limits.SpanAttempts = 1
				want = ErrLimit
			case "depth-limit":
				// No complete path in round one, so reaching the depth cap is explicit.
				r.catalog = *channelCatalog(t, []channelFrame{{"main", 100, 20}})
				r.context.limits.DepthRounds = 1
				want = ErrLimit
			case "local-limit":
				r.context.limits.LocalSteps = 1
				want = ErrLimit
			}
			out, err := r.PaginationSearch(ctx, initial)
			if !errors.Is(err, want) || out.Selected() != nil || !reflect.DeepEqual(before, initial.Info()) {
				t.Fatal(out, err)
			}
			if mode == "depth-limit" || mode == "local-limit" {
				if out.Incumbent() != nil {
					t.Fatal("incomplete incumbent")
				}
			} else {
				if out.Incumbent() == nil || !out.Incumbent().FinalState().SpanComplete() || !slices.Equal(sequenceNames(out.Incumbent()), []string{"t0"}) {
					t.Fatal("lost complete incumbent", out)
				}
				*out.Incumbent().Pages()[0] = Candidate{}
				if pageText(out.Incumbent().Pages()[0].Page()) != "ABCD" {
					t.Fatal("mutable incumbent")
				}
			}
			b.before = nil
			r.context.limits = DefaultWorkLimits()
			if retry, err := r.PaginationSearch(t.Context(), initial); err != nil || retry.Outcome() != SearchComplete {
				t.Fatal("retry failed", err)
			}
		})
	}
}

func TestPaginationSearchEmptyIdentityAndLifetime(t *testing.T) {
	b := &pageBackend{}
	r, s := pageRun(t, `<body style="hyphens:none">ABCD</body>`, [][][2]int{{{100, 20}}}, b, &LossPolicy{Page: 1}, nil)
	initial := r.InitialState()
	for _, bad := range []State{{}, func() State {
		other, _ := pageRun(t, `<body>X</body>`, [][][2]int{{{100, 20}}}, &pageBackend{}, nil, nil)
		return other.InitialState()
	}()} {
		if out, err := r.PaginationSearch(t.Context(), bad); !errors.Is(err, ErrInvalidRequest) || out.Selected() != nil || b.calls != 0 {
			t.Fatal(out, err)
		}
	}
	if out, err := r.PaginationSearch(nil, initial); !errors.Is(err, ErrInvalidRequest) || out.Outcome() != SearchFailed {
		t.Fatal(out, err)
	}
	old := attempt(t, r, initial, "t0")
	copy := *old.Successor().data
	copy.fits = nil
	if out, err := r.PaginationSearch(t.Context(), State{&copy}); !errors.Is(err, ErrInvalidRequest) || out.Selected() != nil {
		t.Fatal(out, err)
	}
	out, err := r.PaginationSearch(t.Context(), initial)
	if err != nil {
		t.Fatal(err)
	}
	before := b.calls
	r.eligible = func(State, *template.Template) (bool, error) {
		t.Fatal("filter called on complete input")
		return false, nil
	}
	r.backend = nil
	empty, err := r.PaginationSearch(t.Context(), out.Selected().FinalState())
	if err != nil || empty.Outcome() != SearchComplete || empty.Selected().Loss() != FiniteLoss(0) || len(empty.Selected().Pages()) != 0 || empty.Stats() != (SearchStats{}) || b.calls != before {
		t.Fatal(empty, err)
	}
	if missing, err := r.PaginationSearch(t.Context(), initial); !errors.Is(err, ErrInvalidRequest) || missing.Selected() != nil {
		t.Fatal(missing, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if failed, err := r.PaginationSearch(t.Context(), initial); !errors.Is(err, content.ErrClosedSession) || failed.Selected() != nil {
		t.Fatal(failed, err)
	}
	var text string
	for _, c := range out.Selected().Pages() {
		text += pageText(c.Page())
	}
	if text != "ABCD" {
		t.Fatal("frozen output invalid after close")
	}
	// Suppression leaves no main material, so the initial span is already complete.
	emptyRun, _ := pageRun(t, `<body style="display:none">hidden</body>`, [][][2]int{{{100, 20}}}, &pageBackend{}, nil, nil)
	emptyReport, err := emptyRun.PaginationSearch(t.Context(), emptyRun.InitialState())
	if err != nil || len(emptyReport.Selected().Pages()) != 0 {
		t.Fatal(emptyReport, err)
	}
}

func TestPaginationSearchReportsHeuristicTruncation(t *testing.T) {
	r, _ := pageRun(t, `<body style="hyphens:none">ABCD</body>`, [][][2]int{{{100, 10}}, {{100, 20}}}, &pageBackend{}, &LossPolicy{Page: 1}, nil)
	initial := r.InitialState()
	dead := attempt(t, r, initial, "t0").Successor().Info().Heads[0].Position
	r.eligible = func(s State, _ *template.Template) (bool, error) {
		for _, h := range s.Info().Heads {
			if h.Channel == "main" && h.HasContinuation {
				if order, ok := h.Position.Compare(dead); ok && order == 0 {
					return false, nil
				}
			}
		}
		return true, nil
	}
	r.context.beam = 1
	out, err := r.PaginationSearch(t.Context(), initial)
	if err != nil || out.Outcome() != SearchHeuristicExhausted || !out.HeuristicTruncated() || out.Selected() != nil || out.Stats().WidthPruned != 1 {
		t.Fatal(out, err)
	}
	// Widening is an explicit caller choice; the earlier result did not retry.
	r.context.beam = 32
	out, err = r.PaginationSearch(t.Context(), initial)
	if err != nil || out.Outcome() != SearchComplete || out.HeuristicTruncated() || !slices.Equal(sequenceNames(out.Selected()), []string{"t1", "t1"}) {
		t.Fatal(out, err)
	}
	r.eligible = nil
	r.context.beam = 1
	out, err = r.PaginationSearch(t.Context(), initial)
	if err != nil || out.Outcome() != SearchComplete || !out.HeuristicTruncated() || out.Selected() == nil || out.Stats().SafePruned == 0 {
		t.Fatal(out, err)
	}
}
