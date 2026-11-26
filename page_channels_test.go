package pagelayout

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

type eventTestBackend struct {
	*pageBackend
	mode string
}

type lineCountOnly pagePayload

func (p lineCountOnly) Clone() layout.FitPayload { return p }
func (p lineCountOnly) LineCount() uint64        { return p.lines }

func (b eventTestBackend) FitGroup(ctx context.Context, h content.GroupHandle, req content.GroupFitRequest, yield func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error) {
	return b.pageBackend.FitGroup(ctx, h, req, func(a layout.ParagraphAlternative) bool {
		p := a.Output.(pagePayload)
		if len(p.events) > 0 {
			switch b.mode {
			case "missing-protocol":
				p.events = nil
				a.Output = lineCountOnly(p)
			case "foreign-event":
				p.events[0].Source = content.SourceRef{}
				a.Output = p
			case "logical-only":
				// Model consumed source without rendered owning coverage.
				p.events, p.text = nil, ""
				a.Output = p
			}
		}
		return yield(a)
	})
}

func TestMultiChannelRenderedEventContract(t *testing.T) {
	for _, mode := range []string{"missing-protocol", "foreign-event", "logical-only"} {
		t.Run(mode, func(t *testing.T) {
			b := &pageBackend{whole: true}
			r := channelsRun(t, `<body style="hyphens:none"><a href="#n" data-flow-anchor>N</a><aside id="n" style="flow-into:notes">note</aside></body>`, b, nil, []channelFrame{{"main", 100, 20}, {"notes", 100, 20}})
			r.backend = eventTestBackend{b, mode}
			initial := r.InitialState()
			out, err := r.PageAttempt(t.Context(), initial, "t0")
			if mode != "logical-only" {
				if !errors.Is(err, content.ErrPreparationProtocol) || out.Candidate() != nil || len(initial.Info().Activated) != 0 {
					t.Fatal(out, err)
				}
				return
			}
			if err != nil || out.Outcome() != Constructed {
				t.Fatal(out, err)
			}
			c := out.Candidate()
			if len(c.Successor().Info().Activated) != 0 || flowText(c.Page(), "notes") != "" || c.Successor().SpanComplete() {
				t.Fatal("byte consumption activated unrendered owner", c.Successor().Info())
			}
		})
	}
}

type channelFrame struct {
	channel       string
	width, height int
}

func channelCatalog(t *testing.T, definitions ...[]channelFrame) *template.Catalog {
	t.Helper()
	var html strings.Builder
	html.WriteString(`<style>.page{width:1000px;height:2000px}.frame{position:fixed;left:0}</style><body>`)
	for i, frames := range definitions {
		fmt.Fprintf(&html, `<template data-type="page" id="t%d"><div class="page">`, i)
		y := 0
		for _, f := range frames {
			fmt.Fprintf(&html, `<div class="frame" style="flow-from:%s;top:%dpx;width:%dpx;height:%dpx"></div>`, f.channel, y, f.width, f.height)
			y += f.height + 10
		}
		html.WriteString(`</div></template>`)
	}
	c, err := template.Load(strings.NewReader(html.String()), template.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func channelsRun(t *testing.T, html string, b *pageBackend, policy *LossPolicy, definitions ...[]channelFrame) *Pagination {
	t.Helper()
	r, _ := pageRun(t, html, [][][2]int{{{100, 100}}}, b, policy, nil)
	r.catalog = *channelCatalog(t, definitions...)
	return r
}
func flowText(p *Page, channel string) string {
	var text string
	for _, f := range p.Frames() {
		if f.Frame.Flow == channel {
			for _, part := range f.Fragments() {
				for _, g := range part.Paragraphs() {
					text += g.Output.(pagePayload).text
				}
			}
		}
	}
	return text
}

func TestMultiChannelAdoptedTransitiveSequence(t *testing.T) {
	html, err := os.ReadFile("testdata/pagination/content/transitive.html")
	if err != nil {
		t.Fatal(err)
	}
	b := &pageBackend{whole: true}
	r := channelsRun(t, string(html), b, nil, []channelFrame{{"main", 100, 100}})
	catalog, err := template.LoadFS(os.DirFS("testdata/pagination"), "templates/receivers.html")
	if err != nil {
		t.Fatal(err)
	}
	r.catalog = *catalog
	initial := r.InitialState()
	future := branchNamed(t, r.context.plan, "later")
	b.before = func(req content.GroupFitRequest) error {
		info, _ := req.Group.Info()
		if info.Tree == future.Material.Tree {
			t.Fatal("future material fitted")
		}
		return nil
	}
	first := attempt(t, r, initial, "note-first")
	if flowText(first.Page(), "notes") != "Note F." || first.Score().Total.Info().Units != 2_000_000 || len(first.Successor().Info().Pending) != 1 || first.Successor().SpanComplete() {
		t.Fatal(first.Score(), first.Successor().Info(), pageText(first.Page()))
	}
	if !reflect.DeepEqual(first.Score().Participating, []int{1}) {
		t.Fatal("new notes incorrectly participate", first.Score())
	}
	second := attempt(t, r, first.Successor(), "figures-only")
	if !second.Successor().SpanComplete() || second.Score().Total.Info().Units != 1_840_278 || second.Score().MissingMain.Info().Units != 0 {
		t.Fatal(second.Score(), second.Successor().Info())
	}
	next, err := r.advanceSpan(second.Successor())
	if err != nil {
		t.Fatal(err)
	}
	b.before = nil
	third := attempt(t, r, next, "note-first")
	if !third.Successor().SpanComplete() || third.Successor().Bank().Len() != 0 || third.Score().Total.Info().Units != 1_000_000 || flowText(third.Page(), "notes") != "Later note." {
		t.Fatal(third.Score(), third.Successor().Info())
	}
	if initial.Bank().Len() != 3 || len(initial.Info().Activated) != 0 {
		t.Fatal("mutated input")
	}
}

func TestMultiChannelLaterChannelActivatesEarlierChannel(t *testing.T) {
	html := `<body style="hyphens:none"><p><a href="#n" data-flow-anchor>N</a></p><figure id="f" style="flow-into:figures">F</figure><aside id="n" style="flow-into:notes"><a href="#f" data-flow-anchor>n</a></aside></body>`
	r := channelsRun(t, html, &pageBackend{whole: true}, nil, []channelFrame{{"figures", 100, 20}, {"notes", 100, 20}, {"main", 100, 20}})
	if !reflect.DeepEqual(r.context.channels, []string{"main", "figures", "notes"}) {
		t.Fatal(r.context.channels)
	}
	c := attempt(t, r, r.InitialState(), "t0")
	if pageText(c.Page()) != "FnN" || !c.Successor().SpanComplete() || len(c.Successor().Info().Activated) != 2 || c.Score().Carry.Info().Units != 0 {
		t.Fatal(pageText(c.Page()), c.Successor().Info())
	}
}

func TestMultiChannelFIFOAndMultipleBranchesInFrame(t *testing.T) {
	html := `<body style="hyphens:none"><a href="#b" data-flow-anchor>B</a>X<a href="#a" data-flow-anchor>A</a><aside id="a" style="flow-into:notes">a</aside><aside id="b" style="flow-into:notes">b</aside></body>`
	policy := LossPolicy{Page: 1, Carry: UnitsPerPoint}
	r := channelsRun(t, html, &pageBackend{}, &policy, []channelFrame{{"notes", 100, 50}, {"main", 100, 10}}, []channelFrame{{"notes", 100, 50}, {"main", 100, 20}})
	a := branchNamed(t, r.context.plan, "a")
	b := branchNamed(t, r.context.plan, "b")
	first := attempt(t, r, r.InitialState(), "t0")
	if flowText(first.Page(), "main") != "B" || flowText(first.Page(), "notes") != "" || !reflect.DeepEqual(first.Successor().Info().Pending, []BranchID{b.ID}) {
		t.Fatal(first.Successor().Info(), pageText(first.Page()))
	}
	head, _ := first.Successor().Bank().First("notes")
	if head != a.Material.Root {
		t.Fatal("skipped unactivated head")
	}
	last := attempt(t, r, first.Successor(), "t1")
	if flowText(last.Page(), "notes") != "ab" || !last.Successor().SpanComplete() {
		t.Fatal(pageText(last.Page()), last.Successor().Info())
	}
	parts := last.Page().Frames()[0].Fragments()
	if len(parts) != 2 || parts[0].Root() != a.Material.Root || parts[1].Root() != b.Material.Root || parts[1].Paragraphs()[0].Y != 10 {
		t.Fatal("branch packing/order", parts)
	}
	*parts[0] = content.FitFragment{}
	if flowText(last.Page(), "notes") != "ab" {
		t.Fatal("mutable frame inspection")
	}
}

func TestMultiChannelCrossSpanFIFOObstruction(t *testing.T) {
	html, err := os.ReadFile("testdata/pagination/content/cross-span-fifo.html")
	if err != nil {
		t.Fatal(err)
	}
	r := channelsRun(t, string(html), &pageBackend{whole: true}, nil, []channelFrame{{"notes", 100, 100}, {"main", 100, 100}}, []channelFrame{{"notes", 100, 100}})
	first := attempt(t, r, r.InitialState(), "t0")
	if first.Successor().SpanComplete() || flowText(first.Page(), "notes") != "" || len(first.Successor().Info().Pending) != 1 {
		t.Fatal(first.Successor().Info())
	}
	out, err := r.PageAttempt(t.Context(), first.Successor(), "t1")
	if err != nil || out.Outcome() != NoFit || out.Candidate() != nil {
		t.Fatal(out, err)
	}
	obs := out.Obstructions()
	if len(obs) != 1 || obs[0].Reason != FutureSpanHead || len(obs[0].PendingBehind) != 1 {
		t.Fatal(obs)
	}
	obs[0].PendingBehind[0] = 999
	if out.Obstructions()[0].PendingBehind[0] == 999 {
		t.Fatal("obstruction alias")
	}
}

func TestMultiChannelPartialCarryAndEntryParticipation(t *testing.T) {
	policy := DefaultLossPolicy()
	policy.OneLineSplit = 0
	html := `<body style="hyphens:none"><a href="#n" data-flow-anchor>N</a><aside id="n" style="flow-into:notes">abcd</aside></body>`
	r := channelsRun(t, html, &pageBackend{}, &policy, []channelFrame{{"notes", 100, 20}, {"main", 100, 10}}, []channelFrame{{"notes", 100, 100}})
	first := attempt(t, r, r.InitialState(), "t0")
	if flowText(first.Page(), "notes") != "ab" || first.Score().Carry.Info().Units != UnitsPerPoint || !reflect.DeepEqual(first.Score().Participating, []int{1}) || first.Successor().SpanComplete() {
		t.Fatal(first.Score())
	}
	root, _ := first.Successor().Bank().First("notes")
	last := attempt(t, r, first.Successor(), "t1")
	if flowText(last.Page(), "notes") != "cd" || !last.Successor().SpanComplete() || last.Score().Total.Info().Units != 1_640_000 || last.Score().MissingMain.Info().Units != 0 {
		t.Fatal(last.Score())
	}
	if last.Page().Frames()[0].Fragments()[0].Root() != root {
		t.Fatal("partial root replaced")
	}
}

func TestMultiChannelSplitOwnerAndOrdinaryReferences(t *testing.T) {
	policy := LossPolicy{Page: 1, Carry: 100}
	html := `<body style="hyphens:none"><a href="#n">o</a><a href="#n" data-flow-anchor>AB</a><a href="#n">r</a><aside id="n" style="flow-into:notes">note</aside></body>`
	b := &pageBackend{}
	r := channelsRun(t, html, b, &policy, []channelFrame{{"main", 100, 10}, {"notes", 100, 1}}, []channelFrame{{"notes", 100, 40}})
	state := r.InitialState()
	for i, ch := range []string{"o", "A", "B", "r"} {
		b.before = func(req content.GroupFitRequest) error {
			info, _ := req.Group.Info()
			if i == 0 && info.Tree != r.context.plan.branches[0].Material.Tree {
				t.Fatal("unreached owner target fitted")
			}
			return nil
		}
		c := attempt(t, r, state, "t0")
		state = c.Successor()
		if flowText(c.Page(), "main") != ch {
			t.Fatal(pageText(c.Page()))
		}
		want := uint64(100)
		if i == 0 {
			want = 0
		}
		if c.Score().Carry.Info().Units != want || len(state.Info().Pending) != int(want/100) {
			t.Fatal(c.Score(), state.Info())
		}
	}
	b.before = nil
	last := attempt(t, r, state, "t1")
	if !last.Successor().SpanComplete() {
		t.Fatal(last.Successor().Info())
	}
}

func TestMultiChannelEmptyOwnerAffinities(t *testing.T) {
	for _, body := range []string{`<p><a href="#n" data-flow-anchor></a>B</p>`, `<p>B<a href="#n" data-flow-anchor></a></p>`, `<p><a href="#n" data-flow-anchor></a></p>`, `<div><a href="#n" data-flow-anchor></a><p>B</p></div>`, `<div><p>B</p><a href="#n" data-flow-anchor></a></div>`} {
		t.Run(body, func(t *testing.T) {
			r := channelsRun(t, `<body style="hyphens:none">`+body+`<aside id="n" style="flow-into:notes">N</aside></body>`, &pageBackend{}, nil, []channelFrame{{"notes", 100, 20}, {"main", 100, 20}})
			c := attempt(t, r, r.InitialState(), "t0")
			if flowText(c.Page(), "notes") != "N" || !c.Successor().SpanComplete() {
				t.Fatal(pageText(c.Page()), c.Successor().Info())
			}
		})
	}
}

func TestMultiChannelSecondaryOnlyWhileMainRemains(t *testing.T) {
	policy := DefaultLossPolicy()
	policy.OneLineSplit = 0
	r := channelsRun(t, `<body style="hyphens:none"><a href="#n" data-flow-anchor>N</a>ABC<aside id="n" style="flow-into:notes">x</aside></body>`, &pageBackend{}, &policy, []channelFrame{{"main", 100, 10}}, []channelFrame{{"notes", 100, 10}})
	first := attempt(t, r, r.InitialState(), "t0")
	second := attempt(t, r, first.Successor(), "t1")
	if second.Score().MissingMain.Info().Units != UnitsPerPoint || second.Score().Carry.Info().Units != 0 || second.Successor().SpanComplete() || !reflect.DeepEqual(first.Successor().Info().Heads[0], second.Successor().Info().Heads[0]) {
		t.Fatal(second.Score(), second.Successor().Info())
	}
}

func TestMultiChannelLateFailureRollsBackActivations(t *testing.T) {
	b := &pageBackend{}
	r := channelsRun(t, `<body style="hyphens:none"><a href="#n" data-flow-anchor>N</a><aside id="n" style="flow-into:notes">abcd</aside></body>`, b, nil, []channelFrame{{"notes", 100, 30}, {"notes", 80, 30}, {"main", 100, 10}})
	initial := r.InitialState()
	old := attempt(t, r, initial, "t0")
	saved := old.Successor().Info()
	text := pageText(old.Page())
	boom := errors.New("late secondary failure")
	calls := 0
	b.before = func(req content.GroupFitRequest) error {
		info, _ := req.Group.Info()
		if info.Tree != r.context.plan.branches[0].Material.Tree {
			calls++
			if calls == 3 {
				return boom
			}
		}
		return nil
	}
	out, err := r.PageAttempt(t.Context(), initial, "t0")
	if !errors.Is(err, boom) || out.Candidate() != nil || len(initial.Info().Activated) != 0 || !reflect.DeepEqual(saved, old.Successor().Info()) || pageText(old.Page()) != text {
		t.Fatal("partial publication", out, err)
	}
	var detail *PaginationError
	if !errors.As(err, &detail) || detail.Location.Channel != "notes" {
		t.Fatal(err)
	}
	b.before = nil
	if pageText(attempt(t, r, initial, "t0").Page()) != text {
		t.Fatal("retry changed result")
	}
}

func TestMultiChannelBranchMarginsUseCoreCollapse(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		bottom, top, height, y int
	}{
		{"positive", 10, 20, 40, 30},
		{"signed", 20, -5, 35, 25},
		{"oversized-trailing-margin", 100, -100, 20, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			html := fmt.Sprintf(`<body style="hyphens:none"><a href="#a" data-flow-anchor>A</a><a href="#b" data-flow-anchor>B</a><aside id="a" style="flow-into:notes;margin-bottom:%dpx">a</aside><aside id="b" style="flow-into:notes;margin-top:%dpx">b</aside></body>`, tc.bottom, tc.top)
			r := channelsRun(t, html, &pageBackend{whole: true}, nil, []channelFrame{{"notes", 100, tc.height}, {"main", 100, 10}})
			c := attempt(t, r, r.InitialState(), "t0")
			parts := c.Page().Frames()[0].Fragments()
			if len(parts) != 2 || !c.Successor().SpanComplete() || parts[1].Paragraphs()[0].Y != float32(tc.y) || c.Page().Frames()[0].Extent().H != float32(tc.height) {
				t.Fatal("margin placement", parts, c.Score())
			}
			a, _ := parts[0].Root().Info()
			b, _ := parts[1].Root().Info()
			if a.Tree == b.Tree || parts[0].Root() == parts[1].Root() {
				t.Fatal("branch identities were merged")
			}
		})
	}
}

func TestMultiChannelSeveralHeadsThenPartialAndDifferentWidth(t *testing.T) {
	policy := LossPolicy{Page: 1}
	html := `<body style="hyphens:none"><a href="#a" data-flow-anchor>A</a><a href="#b" data-flow-anchor>B</a><a href="#c" data-flow-anchor>C</a><aside id="a" style="flow-into:notes">aa</aside><aside id="b" style="flow-into:notes">bbbbbb</aside><aside id="c" style="flow-into:notes">cc</aside></body>`
	r := channelsRun(t, html, &pageBackend{widthDependent: true}, &policy, []channelFrame{{"main", 100, 20}, {"notes", 100, 20}}, []channelFrame{{"notes", 50, 60}})
	first := attempt(t, r, r.InitialState(), "t0")
	if flowText(first.Page(), "notes") != "aabb" {
		t.Fatal(pageText(first.Page()))
	}
	root, _ := first.Successor().Bank().First("notes")
	if root != branchNamed(t, r.context.plan, "b").Material.Root || len(first.Successor().Info().Pending) != 2 {
		t.Fatal(first.Successor().Info())
	}
	second := attempt(t, r, first.Successor(), "t1")
	if flowText(second.Page(), "notes") != "bbbbcc" || !second.Successor().SpanComplete() {
		t.Fatal(pageText(second.Page()), second.Successor().Info())
	}
	parts := second.Page().Frames()[0].Fragments()
	if len(parts) != 2 || parts[0].Root() != root || parts[1].Paragraphs()[0].Y != 40 {
		t.Fatal("remainder/packing")
	}
}
