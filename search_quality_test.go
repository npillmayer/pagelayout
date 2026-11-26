package pagelayout

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"weak"
)

// All instrumentation is test-only. Repeated vectors are observations, never
// evidence that the opaque continuations or future costs are interchangeable.
type qualityObservation struct {
	Case                                                                                  string
	Width                                                                                 int // zero denotes the independent exhaustive reference
	Attempts, LocalAlternatives, PeakFrontier, PeakPending, PeakRetained, RepeatedVectors uint64
	SafeCuts, WidthCuts                                                                   uint64
	Complete                                                                              bool
	Loss                                                                                  uint64
	Sequence                                                                              []string
}

func measureQuality[S any](t *testing.T, name string, b *beamSearch[S], key func(S) string, alternatives *uint64) qualityObservation {
	t.Helper()
	o := qualityObservation{Case: name, Width: b.width}
	attempt := b.attempt
	seen := map[string]bool{}
	pending := uint64(0)
	b.attempt = func(ctx context.Context, s S, id string) (beamStep[S], error) {
		step, err := attempt(ctx, s, id)
		if err == nil && step.fit && !b.complete(step.state) {
			pending++
			k := key(step.state)
			if seen[k] {
				o.RepeatedVectors++
			}
			seen[k] = true
		}
		// Parents stay retained through the round. Count unfinished successors
		// and the best completed path separately, excluding transient local cuts
		// and shared ancestor output prefixes (this is not a heap-byte estimate).
		live := uint64(len(b.frontier)) + pending
		if b.incumbent != nil || err == nil && step.fit && b.complete(step.state) {
			live++
		}
		o.PeakRetained = max(o.PeakRetained, live)
		return step, err
	}
	for len(b.frontier) > 0 {
		o.PeakFrontier = max(o.PeakFrontier, uint64(len(b.frontier)))
		clear(seen)
		pending = 0
		if err := b.round(t.Context()); err != nil {
			t.Fatal(err)
		}
		o.PeakPending = max(o.PeakPending, pending)
	}
	o.Attempts = b.stats.Attempts
	o.SafeCuts = b.stats.SafePruned
	o.WidthCuts = b.stats.WidthPruned
	o.LocalAlternatives = *alternatives
	if b.incumbent != nil {
		o.Complete = true
		o.Loss = b.incumbent.rank.Loss.units
		o.Sequence = graphSequence(b, b.incumbent)
	}
	return o
}

// Independent depth-first enumeration. No beam, bound pruning, memoization or
// state merging. Every template has the same one-local-winner meaning as search.
func referenceQuality[S any](t *testing.T, name string, initial S, templates []string, complete func(S) bool, attempt func(context.Context, S, string) (beamStep[S], error), alternatives *uint64) qualityObservation {
	t.Helper()
	o := qualityObservation{Case: name}
	var best []uint64
	var visit func(S, []uint64, uint64)
	visit = func(s S, path []uint64, loss uint64) {
		if len(path) > 12 {
			t.Fatal("quality fixture is not finite")
		}
		if complete(s) {
			if !o.Complete || loss < o.Loss || loss == o.Loss && slices.Compare(path, best) < 0 {
				o.Complete = true
				o.Loss = loss
				best = slices.Clone(path)
			}
			return
		}
		for i, id := range templates {
			o.Attempts++
			step, err := attempt(t.Context(), s, id)
			if err != nil {
				t.Fatal(err)
			}
			if step.fit {
				visit(step.state, append(slices.Clone(path), uint64(i)), loss+step.loss.units)
			}
		}
	}
	visit(initial, nil, 0)
	for _, i := range best {
		o.Sequence = append(o.Sequence, templates[i])
	}
	o.LocalAlternatives = *alternatives
	return o
}

func qualityGraphs() []searchGraph {
	late := searchGraph{ID: "late-cheap-completion", Initial: "S", Complete: []string{"D"}, Minimum: 1}
	for i := range 40 {
		tpl := fmt.Sprintf("t%02d", i)
		state := fmt.Sprintf("u%02d", i)
		late.Templates = append(late.Templates, tpl)
		cost := uint64(9)
		if i == 39 {
			cost = 1
		}
		late.Edges = append(late.Edges, graphEdge{"S", tpl, state, 1}, graphEdge{state, "t00", "D", cost})
	}
	// All routes converge in source-position terms but remain distinct paths.
	diamond := searchGraph{ID: "repeated-vectors", Initial: "S", Complete: []string{"D"}, Minimum: 1, Templates: []string{"a", "b", "c"}}
	for i, s := range []string{"S", "U", "V"} {
		next := []string{"U", "V", "D"}[i]
		for _, id := range diamond.Templates {
			diamond.Edges = append(diamond.Edges, graphEdge{s, id, next, 1})
		}
	}
	// A completion safely removes expensive unfinished paths without consuming beam.
	bounded := searchGraph{ID: "strict-bound", Initial: "S", Complete: []string{"D"}, Minimum: 1, Templates: []string{"a", "b", "c"}, Edges: []graphEdge{{"S", "a", "D", 3}, {"S", "b", "U", 3}, {"S", "c", "V", 2}, {"U", "a", "D", 1}, {"V", "a", "D", 1}}}
	return []searchGraph{late, diamond, bounded}
}

func TestSearchQualityWidthsAndExhaustiveReference(t *testing.T) {
	var observations []qualityObservation
	for _, g := range qualityGraphs() {
		var none uint64
		ref := referenceQuality(t, g.ID, g.Initial, g.Templates, graphBeam(g, 128).complete, graphBeam(g, 128).attempt, &none)
		for _, width := range []int{8, 32, 128} {
			b := graphBeam(g, width)
			o := measureQuality(t, g.ID, b, func(s string) string { return s }, &none)
			if !o.Complete || o.Loss < ref.Loss || o.WidthCuts == 0 && (o.Loss != ref.Loss || !slices.Equal(o.Sequence, ref.Sequence)) {
				t.Fatal("search/reference disagreement", o, ref)
			}
			if g.ID == "late-cheap-completion" {
				want := uint64(10)
				if width == 128 {
					want = 2
				}
				if o.Loss != want {
					t.Fatal("quality degradation not recorded", o)
				}
			}
			observations = append(observations, o)
		}
		observations = append(observations, ref)
	}
	// Real core/page enumeration, six catalog entries with identical geometry.
	// Repeated source breakpoints must not merge their independently owned cursors.
	makeRun := func() *Pagination {
		frames := make([][][2]int, 6)
		for i := range frames {
			frames[i] = [][2]int{{100, 20}}
		}
		r, _ := pageRun(t, `<body style="hyphens:none">ABCDE</body>`, frames, &pageBackend{}, &LossPolicy{Page: 1}, nil)
		return r
	}
	measured := func(r *Pagination, count *uint64) func(context.Context, State, string) (beamStep[State], error) {
		return func(ctx context.Context, s State, id string) (beamStep[State], error) {
			tpl, _ := r.catalog.Lookup(id)
			x := newPageEnumeration(ctx, r, s, tpl)
			err := x.channels(0)
			*count += x.enumeration
			if err != nil || x.best == nil {
				return beamStep[State]{}, err
			}
			return beamStep[State]{state: x.best.successor, page: x.best, loss: x.best.score.Total, fit: true}, nil
		}
	}
	r := makeRun()
	var alternatives uint64
	ref := referenceQuality(t, "actual-pages", r.InitialState(), r.catalog.IDs(), State.SpanComplete, measured(r, &alternatives), &alternatives)
	for _, width := range []int{8, 32, 128} {
		r := makeRun()
		alternatives = 0
		b := &beamSearch[State]{templates: r.catalog.IDs(), width: width, limits: DefaultWorkLimits(), minimum: FiniteLoss(1), complete: State.SpanComplete, attempt: measured(r, &alternatives)}
		b.start(r.InitialState())
		o := measureQuality(t, "actual-pages", b, func(s State) string {
			h := s.Info().Heads[0]
			p := h.Position.Info()
			return fmt.Sprintf("%d:%d:%d:%d:%d", s.Info().Span, h.Branch, p.Order, p.Offset, p.Step)
		}, &alternatives)
		if !o.Complete || o.Loss != ref.Loss || !slices.Equal(o.Sequence, ref.Sequence) || o.RepeatedVectors == 0 || o.LocalAlternatives == 0 {
			t.Fatal(o, ref)
		}
		// Cross-check the measured scheduler against the complete public search API.
		r.context.beam = width
		public, err := r.PaginationSearch(t.Context(), r.InitialState())
		if err != nil || public.Stats() != b.stats || !slices.Equal(sequenceNames(public.Selected()), o.Sequence) {
			t.Fatal(public, err)
		}
		observations = append(observations, o)
	}
	observations = append(observations, ref)
	data, err := json.MarshalIndent(observations, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if os.Getenv("PAGELAYOUT_UPDATE_SEARCH_QUALITY") == "1" {
		if err := os.WriteFile("testdata/pagination/search-quality.json", data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile("testdata/pagination/search-quality.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(data) {
		t.Fatalf("work/quality observations changed:\n%s", data)
	}
}

// Keep selected output, not a Sequence/State, while releasing the scheduler,
// complete and unfinished paths, local enumerations and losing page payloads.
func retainedSearchOutput(t *testing.T) ([]*Page, []func() bool) {
	r, session := pageRun(t, `<body style="hyphens:none">ABCDE</body>`, [][][2]int{{{100, 20}}, {{100, 10}}}, &pageBackend{}, &LossPolicy{Page: 1}, nil)
	defer session.Close()
	var checks []func() bool
	watch := func(p *searchPath[State]) {
		w := weak.Make(p)
		checks = append(checks, func() bool { return w.Value() == nil })
	}
	b := &beamSearch[State]{templates: r.catalog.IDs(), width: 1, limits: DefaultWorkLimits(), minimum: FiniteLoss(1), complete: State.SpanComplete}
	b.attempt = func(ctx context.Context, s State, id string) (beamStep[State], error) {
		tpl, _ := r.catalog.Lookup(id)
		x := newPageEnumeration(ctx, r, s, tpl)
		w := weak.Make(x)
		checks = append(checks, func() bool { return w.Value() == nil })
		if err := x.channels(0); err != nil {
			return beamStep[State]{}, err
		}
		if x.best == nil {
			return beamStep[State]{}, nil
		}
		candidate := weak.Make(x.best)
		checks = append(checks, func() bool { return candidate.Value() == nil })
		return beamStep[State]{state: x.best.successor, page: x.best, loss: x.best.score.Total, fit: true}, nil
	}
	b.start(r.InitialState())
	for len(b.frontier) > 0 {
		for _, p := range b.frontier {
			watch(p)
		}
		if err := b.round(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if b.incumbent == nil || b.stats.WidthPruned == 0 {
		t.Fatal("fixture did not reject branches")
	}
	watch(b.incumbent)
	var out []*Page
	for p := b.incumbent.tail; p != nil; p = p.previous {
		out = append(out, p.page.Page())
	}
	slices.Reverse(out)
	w := weak.Make(b)
	checks = append(checks, func() bool { return w.Value() == nil })
	return out, checks
}
func TestSelectedPagesReleaseSearchWorkspaces(t *testing.T) {
	pages, checks := retainedSearchOutput(t)
	before := []string{}
	for _, p := range pages {
		before = append(before, pageText(p))
	}
	for range 5 {
		runtime.GC()
	}
	for _, collected := range checks {
		if !collected() {
			t.Fatal("retained search path or local enumeration")
		}
	}
	after := []string{}
	for _, p := range pages {
		after = append(after, pageText(p))
	}
	if !reflect.DeepEqual(before, after) || len(pages) == 0 {
		t.Fatal("lost output")
	}
	runtime.KeepAlive(pages)
}
