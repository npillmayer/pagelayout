package content

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestComposeFragmentsPreservesSnapshotsAndBounds(t *testing.T) {
	d := loaded(t, `<body style="hyphens:none"><aside style="flow-into:notes;margin-bottom:10px;margin-right:10px">A</aside><aside style="flow-into:notes;margin-top:20px;margin-right:10px">B</aside></body>`, LoadOptions{})
	p, s := prepared(t, d, &mockPreparer{})
	var parts []*FitFragment
	for _, b := range d.Branches("notes") {
		f, err := NewFitter(p, s, b.Root)
		if err != nil {
			t.Fatal(err)
		}
		choices, err := Fit(t.Context(), p, s, f, FitRequest{Width: 100, Height: 40, DeferVerticalBounds: true}, neutralFitter{})
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, choices[0].Fragment())
	}
	before := parts[1].Geometry()
	a, err := ComposeFragments(100, 40, parts)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Fits || a.Extent.W != 100 || a.Extent.H != 40 || a.Fragments[1].Paragraphs()[0].Y != 30 || parts[1].Paragraphs()[0].Y != 20 || !reflect.DeepEqual(before, parts[1].Geometry()) {
		t.Fatal("composed geometry changed input")
	}
	short, err := ComposeFragments(100, 39, parts)
	if err != nil || short.Fits {
		t.Fatal(short, err)
	}
	if a.Fragments[1].Paragraphs()[0].ID != parts[1].Paragraphs()[0].ID {
		t.Fatal("recomposition lost occurrence identity")
	}
	for _, bad := range []struct {
		w, h  float32
		parts []*FitFragment
	}{{float32(math.NaN()), 40, parts}, {100, -1, parts}, {100, 40, []*FitFragment{nil}}, {100, 40, []*FitFragment{parts[0], parts[0]}}} {
		if out, err := ComposeFragments(bad.w, bad.h, bad.parts); !errors.Is(err, ErrPreparationProtocol) || len(out.Fragments) != 0 {
			t.Fatal(out, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := ComposeFragments(100, 40, a.Fragments)
	if err != nil || !again.Fits || !reflect.DeepEqual(again.Fragments[1].Geometry(), a.Fragments[1].Geometry()) {
		t.Fatal("closed-session recompose", err)
	}
}
