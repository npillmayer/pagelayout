package pagelayout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"slices"
	"testing"
)

type graphEdge struct {
	From, Template, To string
	Loss               uint64 `json:"loss_units"`
}
type graphExpectation struct {
	Outcome         string
	Sequence        []string
	Loss            *uint64    `json:"loss_units"`
	Heuristic       bool       `json:"heuristic_truncation"`
	SafePruned      *uint64    `json:"safe_pruned_unfinished"`
	Attempts        *uint64    `json:"attempt_calls"`
	Round1          [][]string `json:"round_1_unfinished_sequences"`
	Round2          [][]string `json:"round_2_unfinished_sequences"`
	Round1Incumbent []string   `json:"round_1_incumbent"`
	Incumbent       []string   `json:"incumbent_sequence"`
	IncumbentLoss   *uint64    `json:"incumbent_loss_units"`
	Error           string
	Exhaustive      *graphExpectation `json:"exhaustive_reference"`
}
type searchGraph struct {
	ID            string
	Templates     []string
	Width         int
	Initial       string
	Complete      []string `json:"complete_states"`
	Minimum       uint64   `json:"minimum_page_cost_units"`
	Edges         []graphEdge
	Expect        graphExpectation
	CompareWidths []int `json:"compare_widths"`
	Errors        []struct{ From, Template, Kind string }
	Interrupt     struct {
		After uint64 `json:"after_round"`
		Kind  string
	}
}

func searchGraphs(t *testing.T) []searchGraph {
	t.Helper()
	data, err := os.ReadFile("testdata/pagination/search-expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct{ Cases []searchGraph }
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	return table.Cases
}

func graphBeam(g searchGraph, width int) *beamSearch[string] {
	b := &beamSearch[string]{templates: g.Templates, width: width, minimum: FiniteLoss(g.Minimum), limits: DefaultWorkLimits(),
		complete: func(s string) bool { return slices.Contains(g.Complete, s) },
		attempt: func(_ context.Context, s, id string) (beamStep[string], error) {
			for _, e := range g.Errors {
				if e.From == s && e.Template == id {
					return beamStep[string]{}, errors.New(e.Kind)
				}
			}
			for _, e := range g.Edges {
				if e.From == s && e.Template == id {
					return beamStep[string]{state: e.To, loss: FiniteLoss(e.Loss), fit: true}, nil
				}
			}
			return beamStep[string]{}, nil
		},
	}
	b.start(g.Initial)
	return b
}

func graphSequence[S any](b *beamSearch[S], p *searchPath[S]) []string {
	if p == nil {
		return nil
	}
	out := make([]string, len(p.rank.Templates))
	for i, n := range p.rank.Templates {
		out[i] = b.templates[n]
	}
	return out
}

func TestBeamSearchAdoptedGraphs(t *testing.T) {
	for _, g := range searchGraphs(t) {
		t.Run(g.ID, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			b := graphBeam(g, g.Width)
			var err error
			for len(b.frontier) > 0 {
				if err = b.round(ctx); err != nil {
					break
				}
				var want [][]string
				if b.stats.Rounds == 1 {
					want = g.Expect.Round1
				} else if b.stats.Rounds == 2 {
					want = g.Expect.Round2
				}
				if want != nil {
					got := make([][]string, len(b.frontier))
					for i, p := range b.frontier {
						got[i] = graphSequence(b, p)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("frontier %v, want %v", got, want)
					}
				}
				if b.stats.Rounds == 1 && g.Expect.Round1Incumbent != nil && !slices.Equal(graphSequence(b, b.incumbent), g.Expect.Round1Incumbent) {
					t.Fatal("incumbent occupied beam slot")
				}
				if g.Interrupt.After == b.stats.Rounds {
					if g.Interrupt.Kind == "cancelled" {
						cancel()
					} else {
						b.limits.DepthRounds = g.Interrupt.After
					}
				}
			}
			if err == nil {
				err = ctx.Err()
			}
			want := map[string]SearchOutcome{"Complete": SearchComplete, "SearchExhausted": SearchHeuristicExhausted, "InfeasibleUnderPolicies": SearchInfeasible, "Error": SearchFailed}[g.Expect.Outcome]
			if g.Expect.Outcome == "Interrupted" {
				want = SearchLimited
				if g.Expect.Error == "cancelled" {
					want = SearchCancelled
				}
			}
			if b.outcome(err) != want {
				t.Fatalf("outcome %v want %v: %v", b.outcome(err), want, err)
			}
			if g.Expect.Outcome == "Complete" {
				if !slices.Equal(graphSequence(b, b.incumbent), g.Expect.Sequence) || b.incumbent.rank.Loss.units != *g.Expect.Loss {
					t.Fatal(graphSequence(b, b.incumbent), b.incumbent.rank)
				}
			}
			if g.Expect.IncumbentLoss != nil && (b.incumbent == nil || !slices.Equal(graphSequence(b, b.incumbent), g.Expect.Incumbent) || b.incumbent.rank.Loss.units != *g.Expect.IncumbentLoss) {
				t.Fatal("lost completed incumbent")
			}
			if (b.stats.WidthPruned > 0) != g.Expect.Heuristic {
				t.Fatal("wrong truncation", b.stats)
			}
			if g.Expect.SafePruned != nil && b.stats.SafePruned != *g.Expect.SafePruned {
				t.Fatal(b.stats)
			}
			if g.Expect.Attempts != nil && b.stats.Attempts != *g.Expect.Attempts {
				t.Fatal(b.stats)
			}
			if len(g.Errors) == 0 && g.Interrupt.After == 0 {
				seq, cost, ok := exhaustiveGraph(t, g)
				if b.stats.WidthPruned == 0 && (ok != (b.incumbent != nil) || ok && (!slices.Equal(seq, graphSequence(b, b.incumbent)) || cost != b.incumbent.rank.Loss.units)) {
					t.Fatal("exhaustive reference differs", seq, cost)
				}
				if ref := g.Expect.Exhaustive; ref != nil && (!ok || !slices.Equal(seq, ref.Sequence) || cost != *ref.Loss) {
					t.Fatal("exhaustive fixture", seq, cost)
				}
			}
			for _, width := range g.CompareWidths {
				other := graphBeam(g, width)
				if err := other.run(t.Context()); err != nil || !slices.Equal(graphSequence(other, other.incumbent), g.Expect.Sequence) {
					t.Fatal(width, err)
				}
			}
		})
	}
}

// Independent depth-first reference: no beam, lower-bound pruning or state
// merging. These checked-in graphs are finite; repeated states retain paths.
func exhaustiveGraph(t *testing.T, g searchGraph) ([]string, uint64, bool) {
	t.Helper()
	var best []uint64
	var bestLoss uint64
	found := false
	var visit func(string, []uint64, uint64)
	visit = func(s string, path []uint64, loss uint64) {
		if len(path) > 50 {
			t.Fatal("fixture is not finite")
		}
		if slices.Contains(g.Complete, s) {
			if !found || loss < bestLoss || loss == bestLoss && slices.Compare(path, best) < 0 {
				found = true
				best = slices.Clone(path)
				bestLoss = loss
			}
			return
		}
		for i, id := range g.Templates {
			for _, e := range g.Edges {
				if e.From == s && e.Template == id {
					visit(e.To, append(slices.Clone(path), uint64(i)), loss+e.Loss)
				}
			}
		}
	}
	visit(g.Initial, nil, 0)
	out := make([]string, len(best))
	for i, n := range best {
		out[i] = g.Templates[n]
	}
	return out, bestLoss, found
}

func TestBeamSearchLimitsArithmeticAndCancellation(t *testing.T) {
	g := searchGraph{Templates: []string{"Z", "A"}, Width: 32, Initial: "S", Complete: []string{"D"}, Minimum: 1, Edges: []graphEdge{{"S", "Z", "D", 5}, {"S", "A", "U", 1}, {"U", "Z", "D", 1}}}
	t.Run("attempt-cap-keeps-incumbent", func(t *testing.T) {
		b := graphBeam(g, 32)
		b.limits.SpanAttempts = 1
		err := b.run(t.Context())
		if !errors.Is(err, ErrLimit) || b.stats.Attempts != 1 || b.stats.Rounds != 0 || !slices.Equal(graphSequence(b, b.incumbent), []string{"Z"}) {
			t.Fatal(b.stats, err)
		}
	})
	t.Run("cancel-after-synchronous-call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		b := graphBeam(g, 32)
		original := b.attempt
		b.attempt = func(ctx context.Context, s, id string) (beamStep[string], error) {
			step, err := original(ctx, s, id)
			cancel()
			return step, err
		}
		if err := b.run(ctx); !errors.Is(err, context.Canceled) || b.incumbent != nil || b.stats.Attempts != 1 {
			t.Fatal(err, b.stats)
		}
	})
	for _, mode := range []string{"sum-overflow", "bound-overflow", "blocked", "below-minimum"} {
		t.Run(mode, func(t *testing.T) {
			b := graphBeam(g, 32)
			b.attempt = func(_ context.Context, s, id string) (beamStep[string], error) {
				if mode == "blocked" {
					return beamStep[string]{fit: true, loss: BlockedLoss()}, nil
				}
				if mode == "below-minimum" {
					return beamStep[string]{fit: true}, nil
				}
				if s == "S" && id == "Z" {
					return beamStep[string]{state: "U", fit: true, loss: FiniteLoss(math.MaxUint64)}, nil
				}
				if s == "S" && mode == "bound-overflow" {
					return beamStep[string]{state: "D", fit: true, loss: FiniteLoss(1)}, nil
				}
				if s == "U" {
					return beamStep[string]{state: "D", fit: true, loss: FiniteLoss(1)}, nil
				}
				return beamStep[string]{}, nil
			}
			err := b.run(t.Context())
			want := ErrLossOverflow
			if mode == "blocked" || mode == "below-minimum" {
				want = ErrInvalidCost
			}
			if !errors.Is(err, want) || b.outcome(err) != SearchFailed {
				t.Fatal(err)
			}
		})
	}
	// Catalog position, not spelling, resolves an equal-loss complete tie.
	g.Edges = []graphEdge{{"S", "Z", "D", 1}, {"S", "A", "D", 1}}
	b := graphBeam(g, 32)
	if err := b.run(t.Context()); err != nil || !slices.Equal(graphSequence(b, b.incumbent), []string{"Z"}) {
		t.Fatal(fmt.Sprint(err), graphSequence(b, b.incumbent))
	}
}
