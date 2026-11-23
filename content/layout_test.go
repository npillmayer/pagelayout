package content

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
)

type testLayouter struct {
	measure func(GroupHandle, InlineMeasureRequest) (layout.IntrinsicWidths, error)
	flow    func(GroupHandle, InlineLayoutRequest) (layout.InlineResult, error)
}

func (b testLayouter) MeasureGroup(h GroupHandle, r InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	return b.measure(h, r)
}
func (b testLayouter) LayoutGroup(h GroupHandle, r InlineLayoutRequest) (layout.InlineResult, error) {
	return b.flow(h, r)
}

func TestLayoutScopeAndSessionGuard(t *testing.T) {
	d := preparationDocument(t, "branches.html")
	m := &mockPreparer{}
	p, s := prepared(t, d, m)
	var calls []Box
	b := testLayouter{flow: func(h GroupHandle, r InlineLayoutRequest) (layout.InlineResult, error) {
		if h.(*mockGroupHandle).input.Group != r.Group {
			t.Fatal("wrong retained handle")
		}
		if !errors.Is(s.Close(), ErrSessionBusy) {
			t.Fatal("layout did not reserve session")
		}
		if _, err := Layout(p, s, d.Residual().Root, 20, testLayouter{}); !errors.Is(err, ErrSessionBusy) {
			t.Fatal("reentered layout", err)
		}
		calls = append(calls, r.Group)
		return layout.InlineResult{Extent: layout.InlineExtent{W: r.Width, H: 10}}, nil
	}}
	root := d.Branches("main")[0].Root
	before := cloneTrialTree(root.node())
	for _, w := range []float32{100, 200} {
		r, err := Layout(p, s, root, w, b)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Groups()) != 1 || r.Groups()[0].Group != calls[len(calls)-1] || r.Root() != root {
			t.Fatal("subtree coverage")
		}
		if r.Geometry()[root].Content.W != w {
			t.Fatal("containing width")
		}
	}
	if !reflect.DeepEqual(before, root.node()) || len(m.calls) != len(p.Groups()) {
		t.Fatal("layout changed source/preparation")
	}
	bank := d.Bank()
	for _, branch := range d.Branches("main") {
		item, next, ok := bank.Take("main")
		if !ok || item != branch.Root {
			t.Fatal("layout consumed bank")
		}
		bank = next
	}
}

func TestLayoutRejectsIncompleteOrForeignPreparation(t *testing.T) {
	d := preparationDocument(t, "nested.html")
	m := &mockPreparer{}
	p, s := prepared(t, d, m)
	other, otherSession := prepared(t, d, &mockPreparer{})
	foreign := preparationDocument(t, "nested.html")
	calls := 0
	b := testLayouter{
		measure: func(GroupHandle, InlineMeasureRequest) (layout.IntrinsicWidths, error) {
			calls++
			return layout.IntrinsicWidths{}, nil
		},
		flow: func(GroupHandle, InlineLayoutRequest) (layout.InlineResult, error) {
			calls++
			return layout.InlineResult{}, nil
		},
	}
	for _, tc := range []struct {
		p       *PreparedSnapshot
		s       *PreparationSession
		root    Box
		backend GroupLayouter
	}{
		{other, s, d.Residual().Root, b}, {p, otherSession, d.Residual().Root, b}, {p, s, foreign.Residual().Root, b},
		{p, s, Box{}, b}, {p, s, p.Groups()[0].Group, b}, {p, s, d.Residual().Root, nil},
	} {
		if r, err := Layout(tc.p, tc.s, tc.root, 100, tc.backend); r != nil || !errors.Is(err, ErrInvalidPreparation) || calls != 0 {
			t.Fatal(r, err)
		}
	}
	g := p.Groups()[0].Group
	index := s.state.index[g]
	delete(s.state.index, g)
	if r, err := Layout(p, s, d.Residual().Root, 100, b); r != nil || !errors.Is(err, ErrInvalidPreparation) || calls != 0 {
		t.Fatal("missing coverage reached services", r, err)
	}
	s.state.index[g] = index
	_ = s.Close()
	if r, err := Layout(p, s, d.Residual().Root, 100, b); r != nil || !errors.Is(err, ErrClosedSession) || calls != 0 {
		t.Fatal(r, err)
	}
}

func TestLayoutCallbackGuardAndCoreRejection(t *testing.T) {
	d := preparationDocument(t, "text-only.html")
	p, s := prepared(t, d, &mockPreparer{})
	g := p.Groups()[0]
	cause := errors.New("callback failure")
	calls := 0
	b := testLayouter{flow: func(GroupHandle, InlineLayoutRequest) (layout.InlineResult, error) {
		calls++
		return layout.InlineResult{}, cause
	}}
	i := &layoutAdapter{intrinsicAdapter: intrinsicAdapter{state: s.state, root: d.Residual().Root, backend: b}, backend: b, expected: map[Box]Box{g.Group: g.Owner}, started: make(map[Box]bool)}
	r := layout.InlineRequest{Group: g.Group.node(), OwnerID: g.Owner.id}
	for _, bad := range []layout.InlineRequest{{}, {Group: g.Group.node(), OwnerID: g.Owner.id + 100}, {Group: cloneTrialTree(g.Group.node()), OwnerID: g.Owner.id}} {
		if _, err := i.LayoutInline(bad); err == nil || calls != 0 {
			t.Fatal("invalid request reached backend", err)
		}
	}
	if _, err := i.LayoutInline(r); !errors.Is(err, cause) {
		t.Fatal(err)
	}
	if _, err := i.LayoutInline(r); !errors.Is(err, ErrPreparationProtocol) || calls != 1 {
		t.Fatal("failed attempt was repeated", err)
	}
	// A backend success can still fail core validation. Nothing is published.
	b.flow = func(GroupHandle, InlineLayoutRequest) (layout.InlineResult, error) {
		return layout.InlineResult{Extent: layout.InlineExtent{H: -1}}, nil
	}
	if result, err := Layout(p, s, d.Residual().Root, 100, b); result != nil || err == nil {
		t.Fatal(result, err)
	}
	b.flow = func(GroupHandle, InlineLayoutRequest) (layout.InlineResult, error) {
		return layout.InlineResult{Extent: layout.InlineExtent{H: 10}}, nil
	}
	if result, err := Layout(p, s, d.Residual().Root, 100, b); result == nil || err != nil {
		t.Fatal("retry poisoned", err)
	}
}

func cloneTrialTree(n *layout.LayoutNode) *layout.LayoutNode {
	if n == nil {
		return nil
	}
	c := *n
	if n.Style != nil {
		style := *n.Style
		c.Style = &style
	}
	c.Children = slices.Clone(n.Children)
	for i, child := range n.Children {
		c.Children[i] = cloneTrialTree(child)
	}
	return &c
}
