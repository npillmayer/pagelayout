package pagelayout

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/pagelayout/content"
)

func threePageSelection(t *testing.T) *Sequence {
	t.Helper()
	r, s := pageRun(t, `<body style="hyphens:none">ABC</body>`, [][][2]int{{{100, 10}}}, &pageBackend{}, &LossPolicy{Page: 1}, nil)
	out, err := r.PaginationSearch(t.Context(), r.InitialState())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Selected().Pages()) != 3 {
		t.Fatal("fixture")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Selected()
}

func TestShipoutFailurePreservesSelection(t *testing.T) {
	selected := threePageSelection(t)
	before := selected.FinalState().Info()
	boom := errors.New("partial external write")
	var calls []uint64
	var retained *Page
	out, err := selected.Shipout(t.Context(), func(p *Page) error {
		calls = append(calls, p.Number())
		if p.Number() == 2 {
			retained = p
			return boom
		}
		return nil
	})
	if !errors.Is(err, boom) || !errors.Is(err, ErrDelivery) || out.Complete() || out.FailedPage() != 2 || out.NextPage() != 2 || !reflect.DeepEqual(out.CompletedPages(), []uint64{1}) || !reflect.DeepEqual(calls, []uint64{1, 2}) {
		t.Fatal(out, err, calls)
	}
	var detail *DeliveryError
	if !errors.As(err, &detail) || detail.Page != 2 || pageText(retained) != "B" {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, selected.FinalState().Info()) || pageText(selected.Pages()[2].Page()) != "C" {
		t.Fatal("selection changed")
	}
	copy := out.CompletedPages()
	copy[0] = 999
	if out.CompletedPages()[0] != 1 {
		t.Fatal("inspection alias")
	}
	// A separate explicit call redelivers the whole span, with no hidden resume.
	calls = nil
	again, err := selected.Shipout(t.Context(), func(p *Page) error { calls = append(calls, p.Number()); return nil })
	if err != nil || !again.Complete() || again.FailedPage() != 0 || again.NextPage() != 0 || !reflect.DeepEqual(calls, []uint64{1, 2, 3}) {
		t.Fatal(again, err)
	}
}

func TestShipoutRejectsUnselectedAndInvalidInput(t *testing.T) {
	r, _ := pageRun(t, `<body style="hyphens:none">AB</body>`, [][][2]int{{{100, 20}}, {{100, 10}}}, &pageBackend{}, &LossPolicy{Page: 1}, nil)
	r.context.limits.SpanAttempts = 1
	interrupted, err := r.PaginationSearch(t.Context(), r.InitialState())
	if !errors.Is(err, ErrLimit) || interrupted.Incumbent() == nil {
		t.Fatal(interrupted, err)
	}
	for _, s := range []*Sequence{nil, {}, interrupted.Incumbent()} {
		if out, err := s.Shipout(t.Context(), func(*Page) error { t.Fatal("unselected delivered"); return nil }); !errors.Is(err, ErrInvalidRequest) || out.Complete() {
			t.Fatal(out, err)
		}
	}
	selected := threePageSelection(t)
	if _, err := selected.Shipout(nil, func(*Page) error { return nil }); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	if _, err := selected.Shipout(t.Context(), nil); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	// Copies exposed for inspection do not confer selection on an incumbent.
	*interrupted.Incumbent() = *selected
	if _, err := interrupted.Incumbent().Shipout(t.Context(), func(*Page) error { t.Fatal("mutated report"); return nil }); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
}

func TestShipoutCancellationAndHandleIsolation(t *testing.T) {
	for _, after := range []uint64{0, 1, 3} {
		t.Run(string(rune('0'+after)), func(t *testing.T) {
			selected := threePageSelection(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if after == 0 {
				cancel()
			}
			var calls uint64
			out, err := selected.Shipout(ctx, func(p *Page) error {
				calls++
				if p.Number() == after {
					cancel()
				}
				*p = Page{}
				return nil
			})
			if !errors.Is(err, context.Canceled) || out.FailedPage() != 0 || calls != after || len(out.CompletedPages()) != int(after) || out.Complete() != (after == 3) {
				t.Fatal(out, err, calls)
			}
			if pageText(selected.Pages()[0].Page()) != "A" {
				t.Fatal("mutable page handle")
			}
		})
	}
	selected := threePageSelection(t)
	var numbers []uint64
	out, err := selected.Shipout(t.Context(), func(p *Page) error { numbers = append(numbers, p.Number()); *selected = Sequence{}; return nil })
	if err != nil || !out.Complete() || len(numbers) != 3 {
		t.Fatal("callback changed active delivery", out, err)
	}
	r, _ := pageRun(t, `<body></body>`, [][][2]int{{{100, 10}}}, &pageBackend{}, nil, nil)
	r.backend = nil
	empty, err := r.PaginationSearch(t.Context(), r.InitialState())
	if err != nil {
		t.Fatal(err)
	}
	zero, err := empty.Selected().Shipout(t.Context(), func(*Page) error { t.Fatal("empty span delivered a page"); return nil })
	if err != nil || !zero.Complete() || len(zero.CompletedPages()) != 0 {
		t.Fatal(zero, err)
	}
}

func TestEmptySpanWorkLimitAndConservativePaint(t *testing.T) {
	r, _ := pageRun(t, `<body></body>`, [][][2]int{{{100, 20}}}, &pageBackend{}, nil, nil)
	r.context.limits.LocalSteps = 1
	initial := r.InitialState()
	before := initial.Info()
	out, err := r.Paginate(t.Context(), initial)
	if !errors.Is(err, ErrLimit) || out.Complete() || len(out.Spans()) != 0 || out.LastSearch().Outcome() != SearchLimited || !reflect.DeepEqual(initial.Info(), before) {
		t.Fatal(out, err)
	}
	for _, style := range []string{"padding-top:2px", "border:1px solid black", "margin-top:3px", "background:red", "width:20px"} {
		b := &pageBackend{}
		r, _ := pageRun(t, `<body style="hyphens:none;`+style+`"></body>`, [][][2]int{{{100, 20}}}, b, nil, nil)
		got, err := r.Paginate(t.Context(), r.InitialState())
		if err != nil || !got.Complete() || len(got.Spans()) != 1 || len(got.Spans()[0].Selected().Pages()) != 1 || b.calls == 0 {
			t.Fatal("paint/spacing skipped", style, got, err)
		}
	}
	// Dropped structural cursor authority cannot be recovered from coordinates.
	r, _ = pageRun(t, `<body style="hyphens:none"><p></p><p style="break-before:page">A</p></body>`, [][][2]int{{{100, 20}}}, &pageBackend{}, nil, nil)
	first, err := r.PaginationSearch(t.Context(), r.InitialState())
	if err != nil {
		t.Fatal(err)
	}
	d := *first.Selected().FinalState().data
	d.fits = nil
	if _, err := r.AdvanceSpan(State{&d}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	if err := r.session.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.PaginationSearch(t.Context(), first.Selected().FinalState()); !errors.Is(err, content.ErrClosedSession) {
		t.Fatal(err)
	}
}
