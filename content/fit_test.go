package content

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
)

type neutralFitPayload struct {
	end  uint64
	text []byte
}

func (p *neutralFitPayload) Clone() layout.FitPayload {
	c := *p
	c.text = slices.Clone(p.text)
	return &c
}

type neutralFitter struct {
	before         func(GroupFitRequest) error
	stopped        bool
	repeatComplete bool
}

func (neutralFitter) MeasureGroup(GroupHandle, InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	return layout.IntrinsicWidths{MinContent: 1, MaxContent: 20}, nil
}
func (b neutralFitter) FitGroup(_ context.Context, h GroupHandle, r GroupFitRequest, yield func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error) {
	if b.before != nil {
		if err := b.before(r); err != nil {
			return layout.FitFailed, err
		}
	}
	if b.stopped {
		return layout.FitStopped, nil
	}
	input := h.(*mockGroupHandle).input
	if input.Group != r.Group || input.Owner != r.Owner {
		return layout.FitFailed, ErrPreparationProtocol
	}
	if r.Checkpoint != nil {
		p := r.Checkpoint.(*neutralFitPayload)
		if p.end != r.From.Offset {
			return layout.FitFailed, ErrPreparationProtocol
		}
		p.end = 999
	}
	start := int(r.From.Offset)
	for end := min(start+1, len(input.Text)); ; end++ {
		payload := &neutralFitPayload{uint64(end), []byte(input.Text[start:end])}
		a := layout.ParagraphAlternative{End: layout.ParagraphPosition{Offset: uint64(end)}, Complete: end == len(input.Text), Extent: layout.InlineExtent{H: float32(end-start) * 10}, Output: payload}
		var cp *neutralFitPayload
		if !a.Complete {
			cp = &neutralFitPayload{end: uint64(end)}
			a.Checkpoint = cp
		}
		ok := yield(a)
		if ok && a.Complete && b.repeatComplete {
			ok = yield(a)
		}
		for i := range payload.text {
			payload.text[i] = '!'
		}
		if cp != nil {
			cp.end = 999
		}
		if !ok {
			return layout.FitStopped, nil
		}
		if a.Complete {
			return layout.FitExhausted, nil
		}
	}
}

func TestFitScopeContinuationAndSessionReservation(t *testing.T) {
	d := loaded(t, `<body style="hyphens:none"><section style="flow-into:article"><p>AB</p><p>C</p></section></body>`, LoadOptions{})
	p, s := prepared(t, d, &mockPreparer{})
	root := d.Branches("article")[0].Root
	f, err := NewFitter(p, s, root)
	if err != nil {
		t.Fatal(err)
	}
	before := cloneTrialTree(root.node())
	backend := neutralFitter{before: func(r GroupFitRequest) error {
		if !errors.Is(s.Close(), ErrSessionBusy) {
			t.Fatal("unreserved session")
		}
		if _, err := Fit(t.Context(), p, s, f, FitRequest{}, neutralFitter{}); !errors.Is(err, ErrSessionBusy) {
			t.Fatal("reentered fit", err)
		}
		if _, err := NewFitter(p, s, root); !errors.Is(err, ErrSessionBusy) {
			t.Fatal("reentered constructor", err)
		}
		return nil
	}}
	choices, err := Fit(t.Context(), p, s, f, FitRequest{Width: 100, Height: 10}, backend)
	if err != nil || len(choices) != 1 {
		t.Fatal(choices, err)
	}
	first := choices[0]
	para := first.Fragment().Paragraphs()[0]
	if !first.Progress() || first.Complete() || string(para.Output.(*neutralFitPayload).text) != "A" {
		t.Fatal("prefix association")
	}
	suffix, err := Fit(t.Context(), p, s, f, FitRequest{Width: 80, Height: 20, Continuation: first.Continuation()}, backend)
	if err != nil || len(suffix) != 2 || !suffix[0].Complete() {
		t.Fatal(suffix, err)
	}
	second := suffix[0].Fragment().Paragraphs()[0]
	if second.Group != para.Group || second.ID == para.ID || string(second.Output.(*neutralFitPayload).text) != "B" {
		t.Fatal("repeated group occurrence")
	}
	if order, ok := first.End().Compare(suffix[0].End()); !ok || order >= 0 {
		t.Fatal("logical progress")
	}
	para.Output.(*neutralFitPayload).text[0] = '?'
	geometry := first.Fragment().Geometry()
	delete(geometry, root)
	if string(first.Fragment().Paragraphs()[0].Output.(*neutralFitPayload).text) != "A" || len(first.Fragment().Geometry()) == 0 || !reflect.DeepEqual(before, root.node()) {
		t.Fatal("mutable snapshot")
	}
	if item, ok := d.Bank().First("article"); !ok || item != root {
		t.Fatal("fit advanced bank")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if len(first.Fragment().Paragraphs()) != 1 {
		t.Fatal("closure changed render bundle")
	}
	if got, err := Fit(t.Context(), p, s, f, FitRequest{Width: 100, Height: 30}, backend); got != nil || !errors.Is(err, ErrClosedSession) {
		t.Fatal(got, err)
	}
}

func TestFitIdentityCoverageStopsAndAtomicPublication(t *testing.T) {
	d := loaded(t, `<body style="hyphens:none"><p>AB</p><p>C</p></body>`, LoadOptions{})
	p, s := prepared(t, d, &mockPreparer{})
	f, err := NewFitter(p, s, d.Residual().Root)
	if err != nil {
		t.Fatal(err)
	}
	other, otherSession := prepared(t, d, &mockPreparer{})
	foreign, _ := NewFitter(other, otherSession, d.Residual().Root)
	same, _ := NewFitter(p, s, d.Residual().Root)
	good, err := Fit(t.Context(), p, s, f, FitRequest{Width: 100, Height: 30}, neutralFitter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range []func() error{
		func() error {
			_, e := Fit(t.Context(), p, s, same, FitRequest{Continuation: good[0].Continuation()}, neutralFitter{})
			return e
		},
		func() error {
			_, e := Fit(t.Context(), other, otherSession, f, FitRequest{}, neutralFitter{})
			return e
		},
		func() error { _, e := Fit(t.Context(), p, s, foreign, FitRequest{}, neutralFitter{}); return e },
		func() error { _, e := Fit(t.Context(), other, s, f, FitRequest{}, neutralFitter{}); return e },
		func() error {
			_, e := Fit(t.Context(), other, otherSession, foreign, FitRequest{Continuation: good[0].Continuation()}, neutralFitter{})
			return e
		},
	} {
		if err := run(); !errors.Is(err, ErrInvalidPreparation) {
			t.Fatal("foreign accepted", err)
		}
	}
	info := p.Groups()[0]
	stop, err := f.Boundary(info.Owner, layout.BoundaryAfter)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := Fit(t.Context(), p, s, f, FitRequest{Width: 100, Height: 30, Stop: stop}, neutralFitter{})
	if err != nil || !stopped[0].StopReached() || stopped[0].Complete() {
		t.Fatal(stopped, err)
	}
	index := s.state.index[info.Group]
	delete(s.state.index, info.Group)
	if _, err := NewFitter(p, s, d.Residual().Root); !errors.Is(err, ErrInvalidPreparation) {
		t.Fatal("incomplete coverage", err)
	}
	s.state.index[info.Group] = index
	boom := errors.New("late fit failure")
	calls := 0
	bad := neutralFitter{repeatComplete: true, before: func(GroupFitRequest) error {
		calls++
		if calls == 3 {
			return boom
		}
		return nil
	}}
	if got, err := Fit(t.Context(), p, s, f, FitRequest{Width: 100, Height: 30}, bad); got != nil || !errors.Is(err, boom) {
		t.Fatal("partial publication", got, err, calls)
	}
	if f.relay.state != nil || f.relay.backend != nil {
		t.Fatal("retained execution services")
	}
	if got, err := Fit(t.Context(), p, s, f, FitRequest{Width: 100, Height: 30}, neutralFitter{}); err != nil || len(got) != len(good) {
		t.Fatal("retry", got, err)
	}
	for _, tc := range []struct {
		req  FitRequest
		ctx  context.Context
		b    neutralFitter
		want error
	}{
		{FitRequest{Width: 100, Height: 30, MaxSteps: 1}, t.Context(), neutralFitter{}, layout.ErrFitLimit},
		{FitRequest{Width: 100, Height: 30}, t.Context(), neutralFitter{stopped: true}, ErrFitInterrupted},
		{FitRequest{Width: -1}, t.Context(), neutralFitter{}, layout.ErrInvalidFit},
	} {
		if got, err := Fit(tc.ctx, p, s, f, tc.req, tc.b); got != nil || !errors.Is(err, tc.want) {
			t.Fatal(got, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := Fit(ctx, p, s, f, FitRequest{Width: 100, Height: 30}, neutralFitter{}); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
}
