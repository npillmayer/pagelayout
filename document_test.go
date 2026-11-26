package pagelayout

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

func transitiveRun(t *testing.T) (*Pagination, *pageBackend) {
	t.Helper()
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
	return r, b
}

func TestPaginateTwoSpansAndExactTransition(t *testing.T) {
	r, b := transitiveRun(t)
	initial := r.InitialState()
	before := initial.Info()
	root, _ := initial.Bank().First("main")
	var seen []int
	r.eligible = func(s State, tpl *template.Template) (bool, error) {
		seen = append(seen, s.Info().Span)
		if s.Info().Span == 2 {
			if head, _ := s.Bank().First("main"); head != root || s.Info().NextPage != 3 {
				t.Fatal("reset state at boundary", s.Info())
			}
		}
		return true, nil
	}
	got, err := r.Paginate(t.Context(), initial)
	if err != nil || !got.Complete() || !got.State().DocumentComplete() || len(got.Spans()) != 2 {
		t.Fatal(got, err)
	}
	spans := got.Spans()
	first, last := spans[0].Selected(), spans[1].Selected()
	if !slices.Equal(sequenceNames(first), []string{"note-first", "figures-only"}) || !slices.Equal(sequenceNames(last), []string{"note-first"}) || first.Loss().units != 3_840_278 || last.Loss().units != 1_000_000 {
		t.Fatal(sequenceNames(first), sequenceNames(last))
	}
	if first.FinalState().Info().NextPage != 3 || first.FinalState().DocumentComplete() || last.FinalState().Info().NextPage != 4 {
		t.Fatal("numbering")
	}
	if last.Pages()[0].Page().Number() != 3 || flowText(last.Pages()[0].Page(), "notes") != "Later note." {
		t.Fatal("future material lost")
	}
	if !reflect.DeepEqual(initial.Info(), before) {
		t.Fatal("input changed")
	}
	for i, span := range seen {
		if span == 2 {
			for _, next := range seen[i:] {
				if next != 2 {
					t.Fatal("revisited earlier span")
				}
			}
			break
		}
	}
	start, err := r.AdvanceSpan(first.FinalState())
	if err != nil || start.Info().Span != 2 || start.Info().NextPage != 3 || !reflect.DeepEqual(start.data.fits["main"], first.FinalState().data.fits["main"]) {
		t.Fatal(start.Info(), err)
	}
	if _, err := r.AdvanceSpan(initial); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("advanced undrained span", err)
	}
	if _, err := r.AdvanceSpan(last.FinalState()); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("advanced terminal span", err)
	}
	// Starting at the selected boundary skips it, preserving numbering and output.
	seen = nil
	resumed, err := r.Paginate(t.Context(), first.FinalState())
	if err != nil || !resumed.Complete() || len(resumed.Spans()) != 1 || resumed.Spans()[0].Selected().Pages()[0].Page().Number() != 3 {
		t.Fatal(resumed, err)
	}
	for _, span := range seen {
		if span != 2 {
			t.Fatal("reselected a span")
		}
	}
	calls := b.calls
	again, err := r.Paginate(t.Context(), got.State())
	if err != nil || !again.Complete() || len(again.Spans()) != 0 || calls != b.calls {
		t.Fatal(again, err)
	}
	spans[0] = SearchReport{}
	if got.Spans()[0].Selected() == nil {
		t.Fatal("mutable report slice")
	}
}

func TestPaginateEmptySpansAndStructuralOwners(t *testing.T) {
	fixture, err := os.ReadFile("testdata/pagination/content/empty.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, html := range []string{string(fixture), `<body></body>`, `<body style="display:none">hidden</body>`, `<body style="break-before:page"><!--comment--></body>`} {
		t.Run(html, func(t *testing.T) {
			b := &pageBackend{}
			r, _ := pageRun(t, html, [][][2]int{{{100, 10}}}, b, nil, nil)
			r.backend = nil
			r.eligible = func(State, *template.Template) (bool, error) {
				t.Fatal("empty source called eligibility")
				return false, nil
			}
			initial := r.InitialState()
			before := initial.Info()
			got, err := r.Paginate(t.Context(), initial)
			if err != nil || !got.Complete() || got.State().Info().NextPage != 1 || b.calls != 0 {
				t.Fatal(got, err)
			}
			for _, span := range got.Spans() {
				if len(span.Selected().Pages()) != 0 || span.Selected().Loss().units != 0 {
					t.Fatal("blank page")
				}
			}
			if !reflect.DeepEqual(before, initial.Info()) {
				t.Fatal("input mutated")
			}
		})
	}
	for _, html := range []string{
		`<body style="hyphens:none"><p></p><p style="break-before:page">A</p></body>`,
		`<body style="hyphens:none"><p style="break-before:page">A</p></body>`,
		`<body style="hyphens:none"><p></p><p style="break-before:page"></p><p style="break-before:page">A</p></body>`,
	} {
		r, _ := pageRun(t, html, [][][2]int{{{100, 20}}}, &pageBackend{}, nil, nil)
		got, err := r.Paginate(t.Context(), r.InitialState())
		if err != nil {
			t.Fatal(err)
		}
		var pages []*Candidate
		for _, span := range got.Spans() {
			pages = append(pages, span.Selected().Pages()...)
		}
		if !got.Complete() || len(pages) != 1 || pages[0].Page().Number() != 1 || pageText(pages[0].Page()) != "A" {
			t.Fatal("leading blank", got)
		}
	}
	// With no main receiver, an empty structural owner still activates its note
	// before terminal draining. It cannot create a marker-only main page.
	r := channelsRun(t, `<body style="hyphens:none"><p><a href="#n" data-flow-anchor></a></p><aside id="n" style="flow-into:notes">N</aside></body>`, &pageBackend{}, nil, []channelFrame{{"notes", 100, 10}})
	got, err := r.Paginate(t.Context(), r.InitialState())
	if err != nil || !got.Complete() || len(got.Spans()) != 1 || len(got.Spans()[0].Selected().Pages()) != 1 {
		t.Fatal("structural owner", got, err)
	}
	c := got.Spans()[0].Selected().Pages()[0]
	if flowText(c.Page(), "notes") != "N" || c.Score().MissingMain.units != 0 || c.Page().Number() != 1 {
		t.Fatal(c)
	}
}

func TestPaginateLaterIncumbentRemainsUnselected(t *testing.T) {
	r, _ := pageRun(t, `<body style="hyphens:none"><p>A</p><p style="break-before:page">B</p></body>`, [][][2]int{{{100, 20}}, {{100, 20}}}, &pageBackend{}, &LossPolicy{Page: 1}, nil)
	boom := errors.New("second span eligibility failed after a completion")
	ids := r.catalog.IDs()
	r.eligible = func(s State, tpl *template.Template) (bool, error) {
		if s.Info().Span == 2 && tpl.ID() == ids[1] {
			return false, boom
		}
		return true, nil
	}
	got, err := r.Paginate(t.Context(), r.InitialState())
	if !errors.Is(err, boom) || got.Complete() || len(got.Spans()) != 1 || got.State().Info().Span != 2 || got.LastSearch().Selected() != nil || got.LastSearch().Incumbent() == nil {
		t.Fatal(got, err)
	}
	if _, err := got.LastSearch().Incumbent().Shipout(t.Context(), func(*Page) error { t.Fatal("delivered incumbent"); return nil }); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	var pages []uint64
	for _, span := range got.Spans() {
		if _, err := span.Selected().Shipout(t.Context(), func(p *Page) error { pages = append(pages, p.Number()); return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(pages, []uint64{1}) {
		t.Fatal("delivered incomplete document", pages)
	}
}

func TestPaginateLaterFailurePreservesSelection(t *testing.T) {
	for _, mode := range []string{"backend", "infeasible", "cancel", "limit"} {
		t.Run(mode, func(t *testing.T) {
			r, b := transitiveRun(t)
			initial := r.InitialState()
			boom := errors.New("second span service failure")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			r.eligible = func(s State, _ *template.Template) (bool, error) {
				if s.Info().Span == 2 {
					switch mode {
					case "backend":
						b.before = func(content.GroupFitRequest) error { return boom }
					case "infeasible":
						return false, nil
					case "cancel":
						cancel()
					case "limit":
						return false, ErrLimit
					}
				}
				return true, nil
			}
			got, err := r.Paginate(ctx, initial)
			if got.Complete() || len(got.Spans()) != 1 || got.State().Info().Span != 2 || got.State().Info().NextPage != 3 {
				t.Fatal(got, err)
			}
			want := map[string]error{"backend": boom, "cancel": context.Canceled, "limit": ErrLimit}[mode]
			if mode == "infeasible" {
				if err != nil || got.LastSearch().Outcome() != SearchInfeasible {
					t.Fatal(err)
				}
			} else if !errors.Is(err, want) {
				t.Fatal(err)
			}
			if got.LastSearch().Selected() != nil || got.Spans()[0].Selected().FinalState().DocumentComplete() {
				t.Fatal("partial publication")
			}
			selected := got.Spans()[0].Selected()
			text := pageText(selected.Pages()[0].Page())
			b.before = nil
			r.eligible = nil
			retry, err := r.Paginate(t.Context(), got.State())
			if err != nil || !retry.Complete() || len(retry.Spans()) != 1 || retry.Spans()[0].Selected().Pages()[0].Page().Number() != 3 || pageText(selected.Pages()[0].Page()) != text {
				t.Fatal("retry", retry, err)
			}
		})
	}
}

func TestDocumentExecutionGuardsAndClose(t *testing.T) {
	r, _ := transitiveRun(t)
	initial := r.InitialState()
	r.eligible = func(s State, _ *template.Template) (bool, error) {
		if _, err := r.AdvanceSpan(s); !errors.Is(err, content.ErrSessionBusy) {
			t.Fatal("transition reentry", err)
		}
		if _, err := r.Paginate(t.Context(), s); !errors.Is(err, content.ErrSessionBusy) {
			t.Fatal("document reentry", err)
		}
		return true, nil
	}
	got, err := r.Paginate(t.Context(), initial)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Paginate(nil, initial); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	if _, err := r.Paginate(t.Context(), State{}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	foreign, _ := transitiveRun(t)
	if _, err := foreign.AdvanceSpan(got.Spans()[0].Selected().FinalState()); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	if err := r.session.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AdvanceSpan(got.Spans()[0].Selected().FinalState()); !errors.Is(err, content.ErrClosedSession) {
		t.Fatal(err)
	}
	if _, err := r.Paginate(t.Context(), initial); !errors.Is(err, content.ErrClosedSession) {
		t.Fatal(err)
	}
	for _, span := range got.Spans() {
		delivered, err := span.Selected().Shipout(t.Context(), func(p *Page) error {
			if pageText(p) == "" {
				t.Fatal("lost output")
			}
			return nil
		})
		if err != nil || !delivered.Complete() {
			t.Fatal(delivered, err)
		}
	}
}
