package content

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
)

type groupMeasureFunc func(GroupHandle, InlineMeasureRequest) (layout.IntrinsicWidths, error)

func (f groupMeasureFunc) MeasureGroup(h GroupHandle, r InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	return f(h, r)
}

func TestMeasurePreparedPrivateTrees(t *testing.T) {
	d := preparationDocument(t, "nested.html")
	backend := &mockPreparer{}
	p, session := prepared(t, d, backend)
	before := p.Groups()
	var calls []Box
	var fail bool
	cause := errors.New("measurement service failure")
	measure := groupMeasureFunc(func(handle GroupHandle, r InlineMeasureRequest) (layout.IntrinsicWidths, error) {
		h := handle.(*mockGroupHandle)
		if h.input.Group != r.Group || h.input.Owner != r.Owner {
			t.Fatal("unqualified callback")
		}
		if !errors.Is(session.Close(), ErrSessionBusy) || !errors.Is(session.WithGroup(p, r.Group, r.Owner, func(GroupHandle) error { return nil }), ErrSessionBusy) {
			t.Fatal("measurement did not reserve the session")
		}
		calls = append(calls, r.Group)
		if fail && r.Group == before[0].Group {
			return layout.IntrinsicWidths{MinContent: 1000, MaxContent: 1000}, cause
		}
		w := layout.IntrinsicWidths{MinContent: 10, MaxContent: 20}
		for _, a := range h.input.Occurrences {
			info, _ := a.Box.Info()
			atom, ok := r.Atomics.Lookup(info.ID)
			if !ok {
				t.Fatal("missing nested contribution")
			}
			w.MinContent += atom.BorderWidths.MinContent
			w.MaxContent += atom.BorderWidths.MaxContent
		}
		return w, nil
	})
	first, err := MeasureIntrinsic(p, session, d.Residual().Root, layout.ResolveContext{}, measure)
	if err != nil || first != (layout.IntrinsicWidths{MinContent: 30, MaxContent: 60}) || !reflect.DeepEqual(calls, []Box{before[2].Group, before[1].Group, before[0].Group}) {
		t.Fatal(first, calls, err)
	}
	fail = true
	got, err := MeasureIntrinsic(p, session, d.Residual().Root, layout.ResolveContext{}, measure)
	var detail *PreparationError
	if got != (layout.IntrinsicWidths{}) || !errors.Is(err, cause) || !errors.As(err, &detail) || detail.Group != before[0].Group || detail.Source != before[0].Source || detail.Stage != StageMeasurement {
		t.Fatal("failure context/publication", got, err)
	}
	fail = false
	got, err = MeasureIntrinsic(p, session, d.Residual().Root, layout.ResolveContext{}, measure)
	if err != nil || got != first || !reflect.DeepEqual(p.Groups(), before) || len(backend.calls) != 3 || len(backend.released) != 0 {
		t.Fatal("retry/preparation/source mutation", got, err)
	}
}

func TestMeasurePreparationIdentityAndCoverage(t *testing.T) {
	d := preparationDocument(t, "nested.html")
	b := &mockPreparer{}
	p, s := prepared(t, d, b)
	other, otherSession := prepared(t, d, &mockPreparer{})
	foreignDoc := preparationDocument(t, "nested.html")
	foreign, _ := prepared(t, foreignDoc, &mockPreparer{})
	calls := 0
	m := groupMeasureFunc(func(GroupHandle, InlineMeasureRequest) (layout.IntrinsicWidths, error) {
		calls++
		return layout.IntrinsicWidths{}, nil
	})
	for _, tc := range []struct {
		p    *PreparedSnapshot
		s    *PreparationSession
		root Box
		m    GroupMeasurer
	}{
		{other, s, d.Residual().Root, m}, {p, otherSession, d.Residual().Root, m},
		{foreign, s, foreignDoc.Residual().Root, m}, {p, s, foreignDoc.Residual().Root, m},
		{p, s, Box{}, m}, {p, s, p.Groups()[0].Group, m}, {p, s, d.Residual().Root, nil},
	} {
		got, err := MeasureIntrinsic(tc.p, tc.s, tc.root, layout.ResolveContext{}, tc.m)
		if got != (layout.IntrinsicWidths{}) || !errors.Is(err, ErrInvalidPreparation) || calls != 0 {
			t.Fatal("foreign pair/root or absent backend", got, err)
		}
	}
	// A missing group must never reprepare on demand.
	missing := p.Groups()[2].Group
	index := s.state.index[missing]
	delete(s.state.index, missing)
	if got, err := MeasureIntrinsic(p, s, d.Residual().Root, layout.ResolveContext{}, m); got != (layout.IntrinsicWidths{}) || !errors.Is(err, ErrInvalidPreparation) || calls != 0 || len(b.calls) != 3 {
		t.Fatal("missing coverage was prepared", got, err)
	}
	s.state.index[missing] = index
	_ = s.Close()
	if _, err := MeasureIntrinsic(p, s, d.Residual().Root, layout.ResolveContext{}, m); !errors.Is(err, ErrClosedSession) || calls != 0 {
		t.Fatal("closed session executed", err)
	}
	// Definite roots skip callbacks, but must still validate the pair/lifecycle.
	fixed := loaded(t, `<body style="width:20px">fixed`, LoadOptions{})
	fp, fs := prepared(t, fixed, &mockPreparer{})
	if got, err := MeasureIntrinsic(fp, fs, fixed.Residual().Root, layout.ResolveContext{}, m); err != nil || got != (layout.IntrinsicWidths{MinContent: 20, MaxContent: 20}) || calls != 0 {
		t.Fatal(got, err)
	}
	_ = fs.Close()
	if _, err := MeasureIntrinsic(fp, fs, fixed.Residual().Root, layout.ResolveContext{}, m); !errors.Is(err, ErrClosedSession) {
		t.Fatal(err)
	}
}
