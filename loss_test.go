package pagelayout

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/pagelayout/content"
)

func TestScoringFixtures(t *testing.T) {
	var table struct {
		Units uint64 `json:"units_per_point"`
		Cases []struct {
			ID     string
			Frames []FrameAssessment
			Entry  struct {
				Main    bool `json:"main_remaining"`
				Pending []string
			}
			Pending   []string          `json:"pending_after"`
			Channels  map[string]string `json:"target_channels"`
			Finish    *int              `json:"main_finishes_in_frame"`
			Fragments []struct {
				Paragraph string
				Frame     int
				Lines     uint64
				Split     bool
			} `json:"paragraph_fragments"`
			Expect struct {
				Participating *[]int   `json:"participating_frames"`
				Exempt        *[]int   `json:"exempt_frames"`
				Area          *float64 `json:"area_denominator"`
				Underfill     uint64   `json:"underfill_units"`
				Carry         uint64   `json:"carry_units"`
				Split         uint64   `json:"split_units"`
				Missing       uint64   `json:"missing_main_units"`
				Total         uint64   `json:"total_units"`
			}
		}
	}
	raw, err := os.ReadFile("testdata/pagination/scoring-expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	if table.Units != UnitsPerPoint {
		t.Fatal("cost scale")
	}
	d, err := content.Load(strings.NewReader("<p>paragraph</p>"), content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	paragraph := d.Groups(d.Residual().Tree)[0].Group
	for _, tc := range table.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			a := PageAssessment{Frames: tc.Frames, MainRemaining: tc.Entry.Main}
			ids := map[string]BranchID{}
			target := func(name string) PendingTarget {
				id := ids[name]
				if id == 0 {
					id = BranchID(len(ids) + 1)
					ids[name] = id
				}
				return PendingTarget{id, tc.Channels[name]}
			}
			for _, name := range tc.Entry.Pending {
				a.PendingAtEntry = append(a.PendingAtEntry, target(name))
			}
			for _, name := range tc.Pending {
				a.PendingAfter = append(a.PendingAfter, target(name))
			}
			if tc.Finish != nil {
				a.MainFinishes = true
				a.MainFinishingFrame = *tc.Finish
			}
			for _, f := range tc.Fragments {
				a.Fragments = append(a.Fragments, ParagraphFragment{paragraph, f.Frame, f.Lines, f.Split})
			}
			result, err := AssessPage(DefaultLossPolicy(), a)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.Expect
			if result.Underfill.Info().Units != want.Underfill || result.Carry.Info().Units != want.Carry || result.Split.Info().Units != want.Split || result.MissingMain.Info().Units != want.Missing || result.Total.Info().Units != want.Total {
				t.Fatalf("score: %+v, want %+v", result, want)
			}
			if want.Participating != nil && !equalInts(result.Participating, *want.Participating) {
				t.Fatal("participation", result.Participating)
			}
			if want.Exempt != nil && !equalInts(result.Exempt, *want.Exempt) {
				t.Fatal("exempt", result.Exempt)
			}
			if want.Area != nil && result.AreaDenominator != *want.Area {
				t.Fatal("denominator", result.AreaDenominator)
			}
			if len(a.Frames) > 0 {
				a.Frames[0].Width = 999
				if result.Facts.Frames[0].Width == 999 {
					t.Fatal("facts aliased")
				}
			}
		})
	}
}
func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func TestLossArithmetic(t *testing.T) {
	sum := FiniteLoss(0)
	for i, value := range []float64{.5, 1.5, 2.5} {
		l, err := QuantizeLoss(value)
		if err != nil || l.Info().Units != []uint64{0, 2, 2}[i] {
			t.Fatal(l, err)
		}
		sum, err = sum.Add(l)
		if err != nil {
			t.Fatal(err)
		}
	}
	if sum.Info().Units != 4 {
		t.Fatal(sum)
	}
	max := FiniteLoss(math.MaxUint64)
	if max.Compare(BlockedLoss()) >= 0 {
		t.Fatal("finite/blocked ordering")
	}
	if l, err := max.Add(BlockedLoss()); err != nil || !l.Info().Blocked {
		t.Fatal(l, err)
	}
	if _, err := max.Add(FiniteLoss(1)); !errors.Is(err, ErrLossOverflow) {
		t.Fatal(err)
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1} {
		if _, err := QuantizeLoss(value); !errors.Is(err, ErrInvalidCost) {
			t.Fatal(value, err)
		}
	}
	if _, err := QuantizeLoss(math.Ldexp(1, 64)); !errors.Is(err, ErrLossOverflow) {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		value float64
		units uint64
	}{{math.Nextafter(.5, 0), 0}, {math.Nextafter(.5, 1), 1}, {math.Nextafter(1.5, 0), 1}, {math.Nextafter(1.5, 2), 2}} {
		l, err := QuantizeLoss(pair.value)
		if err != nil || l.Info().Units != pair.units {
			t.Fatal(pair, l, err)
		}
	}
	policy := DefaultLossPolicy()
	policy.Page = 2 * UnitsPerPoint
	lower, err := policy.MinimumFinalLoss(FiniteLoss(3 * UnitsPerPoint))
	if err != nil || lower.Info().Units != 5*UnitsPerPoint {
		t.Fatal(lower, err)
	}
	policy.Page = 0
	if _, err := AssessPage(policy, PageAssessment{}); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(max)
	if err != nil || !strings.Contains(string(encoded), "18446744073709551615") {
		t.Fatal(string(encoded), err)
	}
}
func TestScoringRejectsBadFactsAndChecksOverflow(t *testing.T) {
	base := PageAssessment{Frames: []FrameAssessment{{"main", 10, 10, 5}}, MainRemaining: true}
	for _, mutate := range []func(*PageAssessment){
		func(a *PageAssessment) { a.Frames[0].Occupied = 11 },
		func(a *PageAssessment) { a.Frames[0].Occupied = -1 },
		func(a *PageAssessment) { a.Frames[0].Width = math.NaN() },
		func(a *PageAssessment) { a.Frames[0].Height = 0 },
		func(a *PageAssessment) { a.MainFinishes = true; a.MainFinishingFrame = 2 },
	} {
		a := cloneAssessment(base)
		mutate(&a)
		result, err := AssessPage(DefaultLossPolicy(), a)
		if !errors.Is(err, ErrInvalidCost) || !reflect.DeepEqual(result, Score{}) {
			t.Fatal(result, err)
		}
	}
	p := LossPolicy{Page: math.MaxUint64, Carry: 1}
	if _, err := AssessPage(p, PageAssessment{PendingAfter: []PendingTarget{{1, "notes"}}}); !errors.Is(err, ErrLossOverflow) {
		t.Fatal(err)
	}
	p = LossPolicy{Page: 1, Carry: math.MaxUint64}
	if _, err := AssessPage(p, PageAssessment{PendingAfter: []PendingTarget{{1, "notes"}, {2, "notes"}}}); !errors.Is(err, ErrLossOverflow) {
		t.Fatal(err)
	}
	result, err := AssessPage(DefaultLossPolicy(), PageAssessment{PendingAfter: []PendingTarget{{1, "notes"}, {1, "notes"}}})
	if err != nil || result.Carry.Info().Units != UnitsPerPoint {
		t.Fatal("distinct targets", result, err)
	}
	// A large integer coefficient must not round through float64 before use.
	p = LossPolicy{Page: 1, Underfill: (1 << 53) + 1}
	result, err = AssessPage(p, PageAssessment{Frames: []FrameAssessment{{"main", 1, 1, 0}}, MainRemaining: true})
	if err != nil || result.Underfill.Info().Units != p.Underfill {
		t.Fatal("coefficient lost precision", result, err)
	}
}
