package pagelayout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

type pageHandle struct{ input content.GroupInput }

func (*pageHandle) Release() error { return nil }

type pagePayload struct {
	text   string
	lines  uint64
	end    uint64
	events []content.GroupEvent
}

func (p pagePayload) Clone() layout.FitPayload           { p.events = slices.Clone(p.events); return p }
func (p pagePayload) LineCount() uint64                  { return p.lines }
func (p pagePayload) PlacedEvents() []content.GroupEvent { return slices.Clone(p.events) }

type pageBackend struct {
	prepared, calls int
	before          func(content.GroupFitRequest) error
	widthDependent  bool
	stopped         bool
	whole           bool
}

func (*pageBackend) Preflight([]content.GroupInput) error { return nil }
func (b *pageBackend) PrepareGroup(g content.GroupInput) (content.GroupPreparation, error) {
	b.prepared++
	return content.GroupPreparation{Group: g.Group, Owner: g.Owner, Handle: &pageHandle{g}}, nil
}
func (*pageBackend) MeasureGroup(content.GroupHandle, content.InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	return layout.IntrinsicWidths{MinContent: 10, MaxContent: 100}, nil
}
func (b *pageBackend) FitGroup(ctx context.Context, h content.GroupHandle, r content.GroupFitRequest, yield func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error) {
	b.calls++
	if b.before != nil {
		if err := b.before(r); err != nil {
			return layout.FitFailed, err
		}
	}
	if b.stopped {
		return layout.FitStopped, nil
	}
	g := h.(*pageHandle).input
	if r.Checkpoint != nil && r.Checkpoint.(pagePayload).end != r.From.Offset {
		return layout.FitFailed, errors.New("checkpoint mismatch")
	}
	start, per := int(r.From.Offset), 1
	if b.widthDependent {
		per = max(1, int(r.Width)/50)
	}
	if b.whole {
		per = max(1, len(g.Text))
	}
	lines := uint64(0)
	for end := start; ; {
		if end < len(g.Text) {
			end = min(len(g.Text), end+per)
			lines++
		}
		output := pagePayload{text: g.Text[start:end], lines: lines, end: uint64(end)}
		for _, e := range g.Events {
			matched := false
			switch e.Affinity {
			case content.AffinityVisible:
				matched = e.Start < uint64(end) && e.End > uint64(start)
			case content.AffinityFollowing:
				matched = uint64(end) > e.Start
			case content.AffinityPreceding:
				matched = uint64(end) >= e.Start && uint64(start) < e.Start
			case content.AffinityStructural:
				matched = g.Text == ""
			}
			if matched {
				output.events = append(output.events, e)
			}
		}
		a := layout.ParagraphAlternative{End: layout.ParagraphPosition{Offset: uint64(end)}, Complete: end == len(g.Text), Extent: layout.InlineExtent{H: float32(lines) * 10}, Output: output, AllowsMarginCollapse: lines == 0}
		if !a.Complete {
			a.Checkpoint = output
		}
		if !yield(a) {
			return layout.FitStopped, nil
		}
		if a.Complete {
			return layout.FitExhausted, nil
		}
	}
}

// The test backend treats each ASCII character as a line (or two at wide widths).
// Supplied break choices make expected page losses independent of font metrics.
func pageRun(t *testing.T, html string, shapes [][][2]int, b *pageBackend, policy *LossPolicy, limits *WorkLimits) (*Pagination, *content.PreparationSession) {
	t.Helper()
	doc, err := content.Load(strings.NewReader(html), content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(doc)
	if err != nil {
		t.Fatal(err)
	}
	p, s, err := content.Prepare(doc, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	var source strings.Builder
	source.WriteString(`<style>.page{width:1000px;height:1000px;background-color:#abcdef}.frame{position:fixed;flow-from:main;padding:2px;border:1px solid #123456}</style><body>`)
	for n, shape := range shapes {
		fmt.Fprintf(&source, `<template id="t%d" data-type="page"><div class="page">`, n)
		for i, sz := range shape {
			fmt.Fprintf(&source, `<div class="frame" style="left:%dpx;top:10px;width:%dpx;height:%dpx"></div>`, 10+i*250, sz[0], sz[1])
		}
		source.WriteString(`</div></template>`)
	}
	catalog, err := template.Load(strings.NewReader(source.String()), template.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewPagination(PaginationConfig{Plan: plan, Catalog: catalog, Prepared: p, Session: s, Fitter: b, Policy: policy, Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	return r, s
}
func attempt(t *testing.T, r *Pagination, s State, id string) *Candidate {
	t.Helper()
	a, err := r.PageAttempt(t.Context(), s, id)
	if err != nil || a.Outcome() != Constructed || a.Candidate() == nil {
		t.Fatalf("attempt %s: %v %v", id, a.Outcome(), err)
	}
	return a.Candidate()
}
func pageText(p *Page) string {
	var text string
	for _, f := range p.Frames() {
		for _, fragment := range f.Fragments() {
			for _, g := range fragment.Paragraphs() {
				text += g.Output.(pagePayload).text
			}
		}
	}
	return text
}

func TestPageAttemptChoosesShorterFirstFrame(t *testing.T) {
	b := &pageBackend{}
	r, s := pageRun(t, `<body style="hyphens:none">ABCD</body>`, [][][2]int{{{100, 30}, {100, 20}}}, b, nil, nil)
	initial := r.InitialState()
	got := attempt(t, r, initial, "t0")
	frames := got.Page().Frames()
	if text := pageText(got.Page()); text != "ABCD" {
		t.Fatal(text)
	}
	for _, f := range frames {
		if f.Fragments()[0].Paragraphs()[0].Output.(pagePayload).lines != 2 {
			t.Fatal("greedy frame choice", frames)
		}
	}
	if got.Score().Total.Info().Units != 1_066_667 || got.Score().Split.Info().Units != 0 || !reflect.DeepEqual(got.Score().Exempt, []int{1}) || got.Score().AreaDenominator != 5000 {
		t.Fatal(got.Score())
	}
	if !got.Successor().SpanComplete() || got.Successor().Bank().Len() != 0 || initial.Bank().Len() != 1 || initial.Info().NextPage != 1 || got.Page().Number() != 1 || got.Successor().Info().NextPage != 2 {
		t.Fatal("state conservation")
	}
	if frames[0].Frame.ContentBox.X != 13 || frames[0].Frame.ContentBox.Y != 13 || got.Page().Info().Paint.Background.R != 0xab {
		t.Fatal("compiled geometry/paint")
	}
	first := frames[0].Fragments()[0].Paragraphs()[0]
	second := frames[1].Fragments()[0].Paragraphs()[0]
	if first.Group != second.Group || first.ID == second.ID || first.To != second.From {
		t.Fatal("fragment occurrence/checkpoint pairing")
	}
	frames[0].Frame.ContentBox.X = 999
	facts := got.Score()
	facts.Facts.Frames[0].Occupied = 999
	if got.Page().Frames()[0].Frame.ContentBox.X != 13 || got.Score().Facts.Frames[0].Occupied != 20 {
		t.Fatal("inspection alias")
	}
	oldPage := got.Page()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if pageText(oldPage) != "ABCD" || b.prepared != 1 {
		t.Fatal("retained rendering/preparation")
	}
}

func TestPageAttemptTwoTemplatesAndChangedWidthRemainder(t *testing.T) {
	policy := LossPolicy{Page: 1} // loss ties select further main progress
	b := &pageBackend{widthDependent: true}
	r, _ := pageRun(t, `<body style="hyphens:none">ABCDEFGHIJKL</body>`, [][][2]int{{{100, 20}, {50, 20}}, {{50, 20}}}, b, &policy, nil)
	s := r.InitialState()
	a := attempt(t, r, s, "t0")
	z := attempt(t, r, s, "t1")
	if pageText(a.Page()) != "ABCDEF" || pageText(z.Page()) != "AB" {
		t.Fatal(pageText(a.Page()), pageText(z.Page()))
	}
	if a.Origin().data != s.data || z.Origin().data != s.data || a.Successor().data == z.Successor().data {
		t.Fatal("competing state versions")
	}
	head, _ := s.Bank().First("main")
	for _, c := range []*Candidate{a, z} {
		kept, _ := c.Successor().Bank().First("main")
		if kept != head || !c.Successor().Info().Heads[0].HasContinuation {
			t.Fatal("partial head replaced")
		}
	}
	end := attempt(t, r, a.Successor(), "t0")
	if pageText(end.Page()) != "GHIJKL" || !end.Successor().SpanComplete() || pageText(z.Page()) != "AB" {
		t.Fatal("remainder corrupted")
	}
	if b.prepared != 1 {
		t.Fatal("prepared again")
	}
	complete, err := r.PageAttempt(t.Context(), end.Successor(), "t0")
	if err != nil || complete.Outcome() != SpanComplete || complete.Candidate() != nil {
		t.Fatal(complete, err)
	}
}

func TestPageAttemptStableArrangementTieAndFinalTail(t *testing.T) {
	policy := LossPolicy{Page: 1}
	r, _ := pageRun(t, `<body style="hyphens:none">ABCD</body>`, [][][2]int{{{100, 30}, {100, 30}, {100, 30}}}, &pageBackend{}, &policy, nil)
	c := attempt(t, r, r.InitialState(), "t0")
	f := c.Page().Frames()
	if f[0].Fragments()[0].Paragraphs()[0].Output.(pagePayload).text != "ABC" || f[1].Fragments()[0].Paragraphs()[0].Output.(pagePayload).text != "D" || len(f[2].Fragments()) != 0 {
		t.Fatal("unstable arrangement tie")
	}
	if !reflect.DeepEqual(c.Score().Exempt, []int{1, 2}) || c.Score().AreaDenominator != 9000 {
		t.Fatal(c.Score())
	}
}

func TestPageAttemptStopsInsideHeadAndResumesNextSpan(t *testing.T) {
	r, _ := pageRun(t, `<body style="hyphens:none"><section><p>AB</p><p style="break-before:page">CD</p></section></body>`, [][][2]int{{{100, 100}, {50, 100}}}, &pageBackend{}, nil, nil)
	initial := r.InitialState()
	first := attempt(t, r, initial, "t0")
	if pageText(first.Page()) != "AB" || !first.Successor().SpanComplete() || first.Successor().Bank().Len() != 1 {
		t.Fatal("span stop", first.Successor().Info())
	}
	p := first.Successor().Info().Heads[0].Position
	if order, ok := p.Compare(r.context.plan.spans[0].End); !ok || order != 0 {
		t.Fatal("wrong stop", p.Info(), r.context.plan.spans[0].End.Info())
	}
	advanced, err := r.advanceSpan(first.Successor())
	if err != nil {
		t.Fatal(err)
	}
	second := attempt(t, r, advanced, "t0")
	if pageText(second.Page()) != "CD" || !second.Successor().SpanComplete() || second.Successor().Bank().Len() != 0 {
		t.Fatal("crossed span")
	}
}

func TestPageAttemptEmptyAndNoFit(t *testing.T) {
	for _, tc := range []struct {
		html    string
		outcome PageOutcome
		text    string
	}{
		{`<body style="display:none">hidden</body>`, SpanComplete, ""},
		{`<body style="hyphens:none;break-before:page"></body>`, Constructed, ""},
		{`<body style="hyphens:none">A</body>`, NoFit, ""},
		{`<body style="hyphens:none"><p></p><p>A</p></body>`, Constructed, ""},
	} {
		r, _ := pageRun(t, tc.html, [][][2]int{{{100, 1}}}, &pageBackend{}, nil, nil)
		initial := r.InitialState()
		out, err := r.PageAttempt(t.Context(), initial, "t0")
		if err != nil || out.Outcome() != tc.outcome {
			t.Fatalf("%s: %v %v", tc.html, out.Outcome(), err)
		}
		if out.Candidate() != nil && (pageText(out.Candidate().Page()) != tc.text || !out.Candidate().placedMaterial) {
			t.Fatal("zero height progress")
		}
		if initial.Info().NextPage != 1 {
			t.Fatal("mutated input")
		}
	}
}

func TestPageAttemptErrorsAreTransactional(t *testing.T) {
	b := &pageBackend{}
	r, s := pageRun(t, `<body style="hyphens:none">ABCDEFGHIJ</body>`, [][][2]int{{{100, 30}, {100, 20}}}, b, nil, nil)
	initial := r.InitialState()
	old := attempt(t, r, initial, "t0")
	before, oldText := initial.Info(), pageText(old.Page())
	boom := errors.New("late fitting failure")
	calls := 0
	b.before = func(content.GroupFitRequest) error {
		calls++
		if _, err := r.PageAttempt(t.Context(), initial, "t0"); !errors.Is(err, content.ErrSessionBusy) {
			t.Fatal("reentrant attempt", err)
		}
		if err := s.Close(); !errors.Is(err, content.ErrSessionBusy) {
			t.Fatal("unreserved backend", err)
		}
		if calls == 3 {
			return boom
		}
		return nil
	}
	out, err := r.PageAttempt(t.Context(), initial, "t0")
	if out.Candidate() != nil || !errors.Is(err, boom) || !errors.Is(err, ErrExecution) {
		t.Fatal("published provisional best", out, err)
	}
	var detail *PaginationError
	if !errors.As(err, &detail) || detail.Location.Frame == 0 {
		t.Fatal("missing location", err)
	}
	b.before = nil
	if pageText(attempt(t, r, initial, "t0").Page()) != oldText || !reflect.DeepEqual(before, initial.Info()) {
		t.Fatal("failed retry")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if out, err := r.PageAttempt(ctx, initial, "t0"); out.Candidate() != nil || !errors.Is(err, ErrCancelled) {
		t.Fatal(err)
	}
	b.stopped = true
	if out, err := r.PageAttempt(t.Context(), initial, "t0"); out.Candidate() != nil || !errors.Is(err, content.ErrFitInterrupted) {
		t.Fatal(err)
	}
	b.stopped = false
	if out, err := r.PageAttempt(t.Context(), State{}, "t0"); out.Candidate() != nil || !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	if out, err := r.PageAttempt(t.Context(), initial, "missing"); out.Candidate() != nil || !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	s.Close()
	if out, err := r.PageAttempt(t.Context(), initial, "t0"); out.Candidate() != nil || !errors.Is(err, content.ErrClosedSession) {
		t.Fatal(err)
	}
	if pageText(old.Page()) != oldText {
		t.Fatal("old render data changed")
	}
}

func TestPageAttemptLimitsAndSecondaryProfile(t *testing.T) {
	limits := DefaultWorkLimits()
	limits.LocalSteps = 15
	r, _ := pageRun(t, `<body style="hyphens:none">ABCDEFGHIJ</body>`, [][][2]int{{{100, 30}, {100, 20}}}, &pageBackend{}, nil, &limits)
	if out, err := r.PageAttempt(t.Context(), r.InitialState(), "t0"); out.Candidate() != nil || !errors.Is(err, ErrLimit) {
		t.Fatal(out, err)
	}
	r, _ = pageRun(t, `<body style="hyphens:none"><p><a href="#n" data-flow-anchor>N</a></p><aside id="n" style="flow-into:notes">Note</aside></body>`, [][][2]int{{{100, 30}}}, &pageBackend{}, nil, nil)
	c := attempt(t, r, r.InitialState(), "t0")
	if len(c.Successor().Info().Pending) != 1 || c.Score().Carry.Info().Units != UnitsPerPoint {
		t.Fatal(c.Score())
	}
}

func TestPageAttemptShorterPagePrefixAndUnusedFrames(t *testing.T) {
	r, _ := pageRun(t, `<body style="hyphens:none"><p>AB</p><p>CDEF</p></body>`, [][][2]int{{{100, 30}}}, &pageBackend{}, nil, nil)
	c := attempt(t, r, r.InitialState(), "t0")
	if pageText(c.Page()) != "AB" || c.Score().Total.Info().Units != 1_111_111 || c.Successor().SpanComplete() {
		t.Fatal("greedy page prefix", pageText(c.Page()), c.Score())
	}
	r, _ = pageRun(t, `<body style="hyphens:none">AB</body>`, [][][2]int{{{100, 1}, {100, 20}, {100, 20}}}, &pageBackend{}, nil, nil)
	c = attempt(t, r, r.InitialState(), "t0")
	if len(c.Page().Frames()[0].Fragments()) != 0 || pageText(c.Page()) != "AB" || !reflect.DeepEqual(c.Score().Exempt, []int{1, 2}) {
		t.Fatal("unused receiver/final tail")
	}
	// An absent main receiver cannot produce a progressing page.
	catalog, err := template.LoadFS(os.DirFS("testdata/pagination"), "templates/receivers.html")
	if err != nil {
		t.Fatal(err)
	}
	r.catalog = *catalog
	if out, err := r.PageAttempt(t.Context(), r.InitialState(), "notes-only"); err != nil || out.Outcome() != NoFit || out.Candidate() != nil {
		t.Fatal(out, err)
	}
}

func TestPageAttemptRejectsForeignCheckpointAndCancellationAfterBackend(t *testing.T) {
	b := &pageBackend{}
	r, _ := pageRun(t, `<body style="hyphens:none">ABCDEF</body>`, [][][2]int{{{100, 20}}}, b, nil, nil)
	s := r.InitialState()
	old := attempt(t, r, s, "t0")
	other, _ := pageRun(t, `<body style="hyphens:none">ABCDEF</body>`, [][][2]int{{{100, 20}}}, &pageBackend{}, nil, nil)
	if out, err := other.PageAttempt(t.Context(), old.Successor(), "t0"); out.Candidate() != nil || !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	// The private state contract helper cannot manufacture execution authority.
	d := *old.Successor().data
	d.fits = nil
	if out, err := r.PageAttempt(t.Context(), State{&d}, "t0"); out.Candidate() != nil || !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	b.before = func(content.GroupFitRequest) error { cancel(); return nil }
	if out, err := r.PageAttempt(ctx, s, "t0"); out.Candidate() != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, ErrCancelled) {
		t.Fatal(err)
	}
	b.before = nil
	if pageText(attempt(t, r, old.Successor(), "t0").Page()) != "CD" {
		t.Fatal("failed call damaged old cursor")
	}
}

func TestPageAttemptAdoptedTwoTemplateFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/pagination/fitting-expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Cases []struct {
			ID    string
			Input struct {
				Templates string
				Material  []string `json:"material_lines"`
			}
			Expect struct {
				Attempts []struct {
					Template string
					Placed   []uint64 `json:"placed_per_frame"`
					Complete bool     `json:"span_complete"`
					Loss     uint64   `json:"loss_units"`
				}
			}
		}
	}
	if err = json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	for _, c := range table.Cases {
		if c.ID != "two-templates-same-start" {
			continue
		}
		r, _ := pageRun(t, `<body style="hyphens:none">`+strings.Join(c.Input.Material, "")+`</body>`, [][][2]int{{{100, 100}}}, &pageBackend{}, nil, nil)
		catalog, err := template.LoadFS(os.DirFS("testdata/pagination"), c.Input.Templates)
		if err != nil {
			t.Fatal(err)
		}
		r.catalog = *catalog
		initial := r.InitialState()
		var states []State
		for _, expected := range c.Expect.Attempts {
			candidate := attempt(t, r, initial, expected.Template)
			var placed []uint64
			for _, f := range candidate.Page().Frames() {
				var n uint64
				for _, fragment := range f.Fragments() {
					for _, p := range fragment.Paragraphs() {
						n += p.Output.(content.ParagraphOutput).LineCount()
					}
				}
				placed = append(placed, n)
			}
			if !reflect.DeepEqual(placed, expected.Placed) || candidate.Score().Total.Info().Units != expected.Loss || candidate.Successor().SpanComplete() != expected.Complete {
				t.Fatal(expected.Template, placed, candidate.Score())
			}
			states = append(states, candidate.Successor())
		}
		if len(states) != 2 || states[0].data == states[1].data || initial.Bank().Len() != 1 {
			t.Fatal("fixture state independence")
		}
		return
	}
	t.Fatal("missing fixture")
}

func TestPageAttemptSplitEndsAtMainMarker(t *testing.T) {
	r, _ := pageRun(t, `<body style="hyphens:none"><p>AB</p><p style="break-before:recto">CD</p></body>`, [][][2]int{{{100, 10}}}, &pageBackend{}, nil, nil)
	a := attempt(t, r, r.InitialState(), "t0")
	b := attempt(t, r, a.Successor(), "t0")
	if pageText(a.Page()) != "A" || pageText(b.Page()) != "B" || a.Successor().SpanComplete() || !b.Successor().SpanComplete() {
		t.Fatal("split/marker conservation")
	}
	if b.Score().Split.Info().Units != 10*UnitsPerPoint || b.Score().Underfill.Info().Units != 0 {
		t.Fatal(b.Score())
	}
}
