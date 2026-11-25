package content

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/npillmayer/css-box-layout/registers"
)

type mockPreparer struct {
	preflights     int
	inputs         []GroupInput
	preflight      func([]GroupInput) error
	prepare        func(GroupInput, int) (GroupPreparation, error)
	calls          []Box
	released       []Box
	handles        []*mockGroupHandle
	releaseErrors  map[Box]error
	borrowedCloses int
}

func (m *mockPreparer) Preflight(in []GroupInput) error {
	m.preflights++
	m.inputs = in
	if m.preflight != nil {
		return m.preflight(in)
	}
	return nil
}
func (m *mockPreparer) PrepareGroup(g GroupInput) (GroupPreparation, error) {
	m.calls = append(m.calls, g.Group)
	if m.prepare != nil {
		return m.prepare(g, len(m.calls))
	}
	return m.result(g), nil
}
func (m *mockPreparer) result(g GroupInput) GroupPreparation {
	h := &mockGroupHandle{input: g, release: func() error {
		m.released = append(m.released, g.Group)
		return m.releaseErrors[g.Group]
	}}
	m.handles = append(m.handles, h)
	return GroupPreparation{Group: g.Group, Owner: g.Owner, Handle: h}
}
func (m *mockPreparer) Close() error { m.borrowedCloses++; return nil }

type mockGroupHandle struct {
	input          GroupInput
	release        func() error
	releases, runs int
}

func (h *mockGroupHandle) Release() error { h.releases++; return h.release() }

func preparationDocument(t *testing.T, name string) *Document {
	t.Helper()
	b, err := os.ReadFile("testdata/preparation/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return loaded(t, string(b), LoadOptions{SourceName: name})
}
func prepared(t *testing.T, d *Document, m Preparer) (*PreparedSnapshot, *PreparationSession) {
	t.Helper()
	p, s, err := Prepare(d, m)
	if err != nil || p == nil || s == nil {
		t.Fatal(p, s, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return p, s
}

var errMockPolicy = errors.New("mock unsupported hyphenation")

// The neutral orchestrator does not hard-code one backend's admission policy.
// This fake models the pinned typography profile, without fonts or measurement.
func mockPolicyPreflight(inputs []GroupInput) error {
	for _, g := range inputs {
		policy := g.TextStyle.Hyphens
		if len(g.Hyphens) > 0 {
			policy = g.Hyphens[0].Policy
		}
		if len(g.Hyphens) == 0 && policy != "none" && policy != "manual" {
			return &PreparationError{Stage: StagePreflight, Group: g.Group, Owner: g.Owner, Source: g.Source, Err: errMockPolicy}
		}
		for _, span := range g.Hyphens {
			if span.Policy != policy || span.Policy != "none" && span.Policy != "manual" {
				return &PreparationError{Stage: StagePreflight, Group: g.Group, Owner: g.Owner, Source: span.Source, Range: span.Range, HasRange: true, Err: errMockPolicy}
			}
		}
	}
	return nil
}

func TestPrepareAcceptance(t *testing.T) {
	b, err := os.ReadFile("testdata/preparation/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []preparationFixture
	if err := json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixtures {
		t.Run(tc.File, func(t *testing.T) {
			d := preparationDocument(t, tc.File)
			original := extractAllGroups(t, d)
			m := &mockPreparer{preflight: mockPolicyPreflight}
			p, s, err := Prepare(d, m)
			if m.preflights != 1 || len(m.calls) != tc.PrepareCalls {
				t.Fatal("wrong service counts", m.preflights, len(m.calls))
			}
			if tc.PrepareError != "" {
				var e *PreparationError
				if p != nil || s != nil || !errors.Is(err, errMockPolicy) || !errors.As(err, &e) || e.Stage != StagePreflight || !e.HasRange {
					t.Fatal("policy failure", err)
				}
				n, _ := d.NodeSource(e.Source.NodeID)
				if n.Parent != materialSource(t, d, tc.ErrorSource).Source.NodeID {
					t.Fatal("wrong policy source", e)
				}
				return
			}
			if err != nil || !p.ID().Valid() {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = s.Close() })
			for i, g := range original {
				input, ok := p.Input(g.Group)
				if !ok || !reflect.DeepEqual(input, *g) || m.calls[i] != g.Group || !p.Covers(g.Owner) {
					t.Fatal("ordered coverage", i)
				}
			}
			if !reflect.DeepEqual(original, extractAllGroups(t, d)) || p.Document().material != d.material {
				t.Fatal("changed source")
			}
			for _, channel := range d.Channels() {
				item, _, ok := d.Bank().Take(registers.RegisterID(channel))
				if !ok || item != d.Branches(channel)[0].Root || !p.Covers(item) {
					t.Fatal("bank changed")
				}
			}
		})
	}
}

func TestPrepareLateFailureAndRetry(t *testing.T) {
	d := preparationDocument(t, "branches.html")
	old, oldSession := prepared(t, d, &mockPreparer{})
	groups := old.Groups()
	original, cleanup1, cleanup2 := errors.New("late prepare failure"), errors.New("release one"), errors.New("release two")
	m := &mockPreparer{releaseErrors: map[Box]error{groups[2].Group: cleanup1, groups[0].Group: cleanup2}}
	m.prepare = func(g GroupInput, n int) (GroupPreparation, error) {
		if n == 4 {
			return GroupPreparation{}, original
		}
		return m.result(g), nil
	}
	p, s, err := Prepare(d, m)
	if p != nil || s != nil || !errors.Is(err, original) || !errors.Is(err, cleanup1) || !errors.Is(err, cleanup2) {
		t.Fatal("failure/cleanup causes", p, s, err)
	}
	if !slices.Equal(m.released, []Box{groups[2].Group, groups[1].Group, groups[0].Group}) {
		t.Fatal("cleanup order", m.released)
	}
	var e *PreparationError
	if !errors.As(err, &e) || e.Stage != StagePreparation || e.Group != groups[3].Group || e.Owner != groups[3].Owner || e.Source != groups[3].Source {
		t.Fatal("late failure identity", e)
	}
	if err := oldSession.WithGroup(old, groups[0].Group, groups[0].Owner, func(h GroupHandle) error { h.(*mockGroupHandle).runs++; return nil }); err != nil {
		t.Fatal("old session damaged", err)
	}
	m.prepare, m.releaseErrors = nil, nil
	retry, retrySession := prepared(t, d, m)
	if retry.ID() == old.ID() || !reflect.DeepEqual(old.Groups(), retry.Groups()) {
		t.Fatal("retry identity")
	}
	if err := retrySession.Close(); err != nil {
		t.Fatal(err)
	}
	for _, h := range m.handles {
		if h.releases != 1 {
			t.Fatal("handle leaked/released twice")
		}
	}
}

func TestPreparePreflightBeforeAnyGroup(t *testing.T) {
	d := loaded(t, `<body style="hyphens:none"><p>first</p><div style="flow-into:article">last <span style="hyphens:auto">bad</span></div>`, LoadOptions{})
	m := &mockPreparer{preflight: mockPolicyPreflight}
	p, s, err := Prepare(d, m)
	if p != nil || s != nil || !errors.Is(err, errMockPolicy) || len(m.calls) != 0 {
		t.Fatal("late policy invoked engine", err)
	}
	serviceFailure := errors.New("explicit service required")
	m.preflight = func([]GroupInput) error { return serviceFailure }
	p, s, err = Prepare(preparationDocument(t, "manual.html"), m)
	if p != nil || s != nil || !errors.Is(err, serviceFailure) || len(m.calls) != 0 {
		t.Fatal("service preflight", err)
	}
	// A late extraction failure occurs even before backend preflight.
	d = preparationDocument(t, "branches.html")
	tree := d.Branches("article")[1].Tree
	n := d.Groups(tree)[0].Group.node().Children[0]
	n.Text.Range.End++
	m = &mockPreparer{}
	p, s, err = Prepare(d, m)
	var located *PreparationError
	if p != nil || s != nil || m.preflights != 0 || !errors.As(err, &located) || located.Stage != StageExtraction || located.Source.NodeID != n.NodeID || !located.HasRange {
		t.Fatal("extraction failure", err)
	}
}

func TestPrepareReturnedIdentityAndHandles(t *testing.T) {
	d := preparationDocument(t, "branches.html")
	foreign := preparationDocument(t, "branches.html")
	for _, mode := range []string{"nil", "typed-nil", "foreign", "owner", "duplicate", "error-handle"} {
		t.Run(mode, func(t *testing.T) {
			m := &mockPreparer{}
			cause := errors.New("backend failure with handle")
			m.prepare = func(g GroupInput, n int) (GroupPreparation, error) {
				if n != 4 {
					return m.result(g), nil
				}
				if mode == "nil" {
					return GroupPreparation{Group: g.Group, Owner: g.Owner}, nil
				}
				if mode == "typed-nil" {
					var h *mockGroupHandle
					return GroupPreparation{Group: g.Group, Owner: g.Owner, Handle: h}, nil
				}
				r := m.result(g)
				switch mode {
				case "foreign":
					r.Group = foreign.Groups(foreign.Branches("article")[1].Tree)[0].Group
				case "owner":
					r.Owner = g.Group
				case "duplicate":
					r.Group = m.calls[0]
				case "error-handle":
					return r, cause
				}
				return r, nil
			}
			p, s, err := Prepare(d, m)
			if p != nil || s != nil || err == nil {
				t.Fatal("partial publication")
			}
			if mode == "error-handle" {
				if !errors.Is(err, cause) {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrPreparationProtocol) {
				t.Fatal(err)
			}
			want := slices.Clone(m.calls)
			if mode == "nil" || mode == "typed-nil" {
				want = want[:3]
			}
			slices.Reverse(want)
			if !slices.Equal(want, m.released) {
				t.Fatal("returned handle not cleaned", mode, m.released)
			}
		})
	}
}

func TestPreparationSessionIdentityLifecycle(t *testing.T) {
	d := preparationDocument(t, "nested.html")
	m := &mockPreparer{}
	p, s := prepared(t, d, m)
	p2, s2 := prepared(t, d, &mockPreparer{})
	foreign, _ := prepared(t, preparationDocument(t, "nested.html"), &mockPreparer{})
	g := p.Groups()[0]
	if p.ID() == p2.ID() || p.ID() == foreign.ID() || g.Group.id != foreign.Groups()[0].Group.id {
		t.Fatal("preparation identity")
	}
	callbackCalls := 0
	call := func(GroupHandle) error { callbackCalls++; return nil }
	for _, pair := range []struct {
		p            *PreparedSnapshot
		group, owner Box
	}{
		{p2, g.Group, g.Owner}, {foreign, g.Group, g.Owner}, {nil, g.Group, g.Owner},
		{p, foreign.Groups()[0].Group, foreign.Groups()[0].Owner}, {p, g.Group, p.Groups()[1].Owner}, {p, g.Owner, g.Owner}, {p, Box{}, g.Owner},
	} {
		if err := s.WithGroup(pair.p, pair.group, pair.owner, call); !errors.Is(err, ErrInvalidPreparation) {
			t.Fatal("identity accepted", err)
		}
	}
	if callbackCalls != 0 || p.Covers(foreign.Groups()[0].Group) {
		t.Fatal("foreign source reached backend")
	}
	failed := errors.New("trial failure")
	var frozen string
	err := s.WithGroup(p, g.Group, g.Owner, func(handle GroupHandle) error {
		h := handle.(*mockGroupHandle)
		h.runs++
		frozen = h.input.Text
		if !errors.Is(s.Close(), ErrSessionBusy) || !errors.Is(s.WithGroup(p, g.Group, g.Owner, call), ErrSessionBusy) {
			t.Fatal("reentrant use")
		}
		return failed
	})
	if !errors.Is(err, failed) {
		t.Fatal("execution cause")
	}
	if err = s.WithGroup(p, g.Group, g.Owner, call); err != nil {
		t.Fatal("session not reusable", err)
	}
	cleanup := errors.New("close failed")
	m.releaseErrors = map[Box]error{g.Group: cleanup}
	copyOfSession := *s
	err = s.Close()
	if !errors.Is(err, cleanup) || copyOfSession.Close() != err || s.Close() != err {
		t.Fatal("idempotent close", err)
	}
	if s.state.owned != nil || s.state.index != nil || s.state.prepared != nil || m.borrowedCloses != 0 {
		t.Fatal("session kept handles or closed borrowed services")
	}
	for _, h := range m.handles {
		if h.releases != 1 {
			t.Fatal("duplicate release")
		}
	}
	if !errors.Is(s.WithGroup(p, g.Group, g.Owner, call), ErrClosedSession) || callbackCalls != 1 {
		t.Fatal("closed session reached callback")
	}
	input, ok := p.Input(g.Group)
	if !ok || input.Text != frozen || !p.Covers(g.Owner) {
		t.Fatal("snapshot/frozen output invalidated")
	}
	if err := s2.WithGroup(p2, g.Group, g.Owner, call); err != nil {
		t.Fatal("other session damaged", err)
	}
}

func TestPreparationDetachedBackendAndInspection(t *testing.T) {
	d := loaded(t, `<body style="hyphens:none">a<span dir="rtl">ב<span style="display:inline-block">inside</span>ג</span>z`, LoadOptions{})
	want := extractAllGroups(t, d)
	mutate := func(g *GroupInput) {
		g.Text = "changed"
		g.Source.Path = "changed"
		if len(g.Atomics) > 0 {
			g.Atomics[0] = Box{}
		}
		if len(g.Styles) > 0 {
			g.Styles[0].Style.Family = "changed"
		}
		if len(g.Paint) > 0 {
			g.Paint[0].Color.A = 0
		}
		if len(g.Hyphens) > 0 {
			g.Hyphens[0].Policy = "changed"
		}
		if len(g.Isolates) > 0 {
			g.Isolates[0].End = 999
		}
		if len(g.Occurrences) > 0 {
			g.Occurrences[0].Box = Box{}
		}
		if len(g.TextSources) > 0 {
			g.TextSources[0].Range.End = 999
		}
	}
	m := &mockPreparer{preflight: func(in []GroupInput) error {
		for i := range in {
			mutate(&in[i])
		}
		return nil
	}}
	m.prepare = func(g GroupInput, n int) (GroupPreparation, error) {
		if !reflect.DeepEqual(g, *want[n-1]) {
			t.Fatal("preflight mutated group input")
		}
		r := m.result(g)
		mutate(&g)
		return r, nil
	}
	p, s := prepared(t, d, m)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 10 {
				for _, expected := range want {
					g, ok := p.Input(expected.Group)
					if !ok || !reflect.DeepEqual(g, *expected) {
						t.Error("backend/caller mutated manifest")
					}
					mutate(&g)
				}
				groups := p.Groups()
				if len(groups[0].Atomics) > 0 {
					groups[0].Atomics[0] = Box{}
				}
			}
		})
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if !reflect.DeepEqual(want, extractAllGroups(t, d)) {
		t.Fatal("source changed")
	}
}

func TestPrepareZeroAndEmpty(t *testing.T) {
	d := preparationDocument(t, "nested.html")
	var typedNil *mockPreparer
	for _, tc := range []struct {
		d *Document
		b Preparer
	}{{nil, &mockPreparer{}}, {&Document{}, &mockPreparer{}}, {d, nil}, {d, typedNil}} {
		p, s, err := Prepare(tc.d, tc.b)
		if p != nil || s != nil || !errors.Is(err, ErrInvalidPreparation) {
			t.Fatal("missing explicit input")
		}
	}
	m := &mockPreparer{}
	p, s := prepared(t, loaded(t, `<body style="display:none">hidden</body>`, LoadOptions{}), m)
	if !p.ID().Valid() || len(p.Groups()) != 0 || m.preflights != 1 || len(m.calls) != 0 {
		t.Fatal("empty document")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	var zero PreparedSnapshot
	if zero.ID().Valid() || zero.Document() != nil || len(zero.Groups()) != 0 || zero.Covers(Box{}) {
		t.Fatal("zero snapshot")
	}
	if _, ok := zero.Input(Box{}); ok {
		t.Fatal("zero input")
	}
	var session PreparationSession
	if session.Close() != nil || !errors.Is(session.WithGroup(p, Box{}, Box{}, nil), ErrClosedSession) {
		t.Fatal("zero session")
	}
}
