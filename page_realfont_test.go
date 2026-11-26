package pagelayout_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"weak"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	page "github.com/npillmayer/pagelayout"
	"github.com/npillmayer/pagelayout/content"
	prep "github.com/npillmayer/pagelayout/preparation/typography"
	"github.com/npillmayer/pagelayout/template"
	"github.com/npillmayer/typography/glyphing"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/render"
	renderot "github.com/npillmayer/typography/render/adapter/opentype"
	"github.com/npillmayer/typography/style"
)

type pageMarkers struct{}

func (pageMarkers) Discretionaries(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
	return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
}

func pageProofs(t *testing.T, p *page.Page) []*render.ParagraphProof {
	t.Helper()
	backend := renderot.New()
	var proofs []*render.ParagraphProof
	for _, f := range p.Frames() {
		for _, part := range f.Fragments() {
			fragment, err := prep.InspectFragment(part)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range fragment.Groups() {
				g, _ := fragment.Group(id)
				paints := make([]style.PaintSpan, len(g.Input.Paint))
				for i, s := range g.Input.Paint {
					c := s.Color
					paints[i] = style.PaintSpan{Start: s.Start, End: s.End, Paint: style.Paint{Color: fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)}}
				}
				proof, err := render.BuildFragmentProof(render.FragmentInput{Fragment: g.Paragraph, Text: g.Input.Text, Paint: paints}, render.Services{Backend: backend, Identity: backend})
				if err != nil {
					t.Fatal(err)
				}
				for _, glyph := range proof.Glyphs {
					if glyph.Path == "" || glyph.FaceSHA256 == "" {
						t.Fatal("missing frozen outline")
					}
				}
				proofs = append(proofs, proof)
			}
		}
	}
	return proofs
}

func retainedPageAttempt(t *testing.T, manual bool) ([]*page.Page, [][]*render.ParagraphProof, page.State, weak.Pointer[page.Pagination], weak.Pointer[content.PreparationSession]) {
	t.Helper()
	ctx := context.Background()
	services := glyphot.Services{}
	face, err := services.Load(ctx, "../opentype/testdata/fonts/GentiumPlus-R.ttf", nil)
	if err != nil {
		t.Fatal(err)
	}
	mapper, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{Default: face.Resource.ID(), Size: 15 * dimen.BP, Faces: []glyphing.FaceEntry{face}}, services)
	if err != nil {
		t.Fatal(err)
	}
	shapes := 0
	backend, err := prep.New(prep.Config{Resolver: mapper, NewShaper: func() (contracts.StreamingShaper, error) { shapes++; return glyphot.NewStream(32) }, Hyphenator: pageMarkers{}})
	if err != nil {
		t.Fatal(err)
	}
	policy := "none"
	text := strings.Repeat("A paragraph flows through frames with different widths. ", 3)
	text = `Before <span style="display:inline-block;width:25%">in</span> ` + text
	if manual {
		policy = "manual"
		text = strings.Repeat("co&shy;operate ", 14)
	}
	doc, err := content.Load(strings.NewReader(`<body style="font-size:20px;hyphens:`+policy+`;color:#234567">`+text+`</body>`), content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	prepared, session, err := content.Prepare(doc, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	baseShapes := shapes
	plan, err := page.Plan(doc)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := template.LoadFS(os.DirFS("testdata/pagination"), "templates/receivers.html")
	if err != nil {
		t.Fatal(err)
	}
	run, err := page.NewPagination(page.PaginationConfig{Plan: plan, Catalog: catalog, Prepared: prepared, Session: session, Fitter: prep.FittingAdapter()})
	if err != nil {
		t.Fatal(err)
	}
	wr, ws := weak.Make(run), weak.Make(session)
	initial := run.InitialState()
	wide, err := run.PageAttempt(ctx, initial, "wide")
	if err != nil {
		t.Fatal(err)
	}
	chain, err := run.PageAttempt(ctx, initial, "narrow-chain")
	if err != nil {
		t.Fatal(err)
	}
	if wide.Outcome() != page.Constructed || chain.Outcome() != page.Constructed || initial.Info().NextPage != 1 {
		t.Fatal("real template alternatives")
	}
	pages := []*page.Page{wide.Candidate().Page(), chain.Candidate().Page()}
	proofs := [][]*render.ParagraphProof{pageProofs(t, pages[0]), pageProofs(t, pages[1])}
	state := chain.Candidate().Successor()
	seen := map[content.FragmentID]bool{}
	var previousEnd uint64
	check := func(p *page.Page) {
		for _, f := range p.Frames() {
			for _, part := range f.Fragments() {
				for _, g := range part.Paragraphs() {
					if g.ParentAtomic.Valid() {
						continue
					}
					if seen[g.ID] || g.From.Offset != previousEnd {
						t.Fatal("replayed or missing source prefix", g.From, previousEnd)
					}
					seen[g.ID] = true
					previousEnd = g.To.Offset
				}
			}
		}
	}
	check(pages[1])
	for i := 0; !state.SpanComplete() && i < 20; i++ {
		out, err := run.PageAttempt(ctx, state, "wide")
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome() != page.Constructed {
			t.Fatal("stalled real continuation", out.Outcome())
		}
		p := out.Candidate().Page()
		check(p)
		pages = append(pages, p)
		proofs = append(proofs, pageProofs(t, p))
		state = out.Candidate().Successor()
	}
	input, _ := prepared.Input(prepared.Groups()[0].Group)
	if !state.SpanComplete() || previousEnd != uint64(len(input.Text)) || doc.Bank().Len() != 1 {
		t.Fatal("real conservation", state.Info(), previousEnd, len(input.Text))
	}
	if !manual && shapes != baseShapes {
		t.Fatal("base preparation repeated")
	}
	return pages, proofs, chain.Candidate().Successor(), wr, ws
}

func TestPageAttemptRealFontAndFrozenLifetime(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(fmt.Sprint(manual), func(t *testing.T) {
			pages, proofs, state, run, session := retainedPageAttempt(t, manual)
			for range 5 {
				runtime.GC()
			}
			if run.Value() != nil || session.Value() != nil {
				t.Fatal("page/state retained execution")
			}
			if !state.Valid() {
				t.Fatal("lost retained state")
			}
			for i, p := range pages {
				if !reflect.DeepEqual(proofs[i], pageProofs(t, p)) {
					t.Fatal("frozen proof changed")
				}
			}
		})
	}
}

func TestMultiChannelRealFontEventsAndFrozenRendering(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(fmt.Sprint(manual), func(t *testing.T) {
			ctx := t.Context()
			services := glyphot.Services{}
			face, err := services.Load(ctx, "../opentype/testdata/fonts/GentiumPlus-R.ttf", nil)
			if err != nil {
				t.Fatal(err)
			}
			mapper, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{Default: face.Resource.ID(), Size: 15 * dimen.BP, Faces: []glyphing.FaceEntry{face}}, services)
			if err != nil {
				t.Fatal(err)
			}
			backend, err := prep.New(prep.Config{Resolver: mapper, NewShaper: func() (contracts.StreamingShaper, error) { return glyphot.NewStream(32) }, Hyphenator: pageMarkers{}})
			if err != nil {
				t.Fatal(err)
			}
			policy := "none"
			main := `o<a href="#n" data-flow-anchor>f</a>fice`
			if manual {
				policy = "manual"
				main = `<a href="#n" data-flow-anchor>co&shy;operate</a>`
			}
			doc, err := content.Load(strings.NewReader(`<body style="font-size:20px;hyphens:`+policy+`;color:#234567"><p>`+main+`</p><figure id="f" style="flow-into:figures">Figure.</figure><aside id="n" style="flow-into:notes">Note <a href="#f" data-flow-anchor>F</a>.</aside></body>`), content.LoadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			prepared, session, err := content.Prepare(doc, backend)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			plan, err := page.Plan(doc)
			if err != nil {
				t.Fatal(err)
			}
			catalog, err := template.Load(strings.NewReader(`<style>.page{width:250px;height:400px}.frame{position:fixed;left:10px;width:180px;height:50px}.figures{flow-from:figures;top:10px}.notes{flow-from:notes;top:80px}.main{flow-from:main;top:150px;width:30px;height:32px}.tail{flow-from:main;top:210px;height:100px}</style><body><template id="proof" data-type="page"><div class="page"><div class="frame figures"></div><div class="frame notes"></div><div class="frame main"></div><div class="frame tail"></div></div></template></body>`), template.LoadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			weights := page.DefaultLossPolicy()
			weights.OneLineSplit = 0
			run, err := page.NewPagination(page.PaginationConfig{Plan: plan, Catalog: catalog, Prepared: prepared, Session: session, Fitter: prep.FittingAdapter(), Policy: &weights})
			if err != nil {
				t.Fatal(err)
			}
			initial := run.InitialState()
			out, err := run.PageAttempt(ctx, initial, "proof")
			if err != nil {
				t.Fatal(err)
			}
			if out.Outcome() != page.Constructed || !out.Candidate().Successor().SpanComplete() || len(out.Candidate().Successor().Info().Activated) != 2 {
				t.Fatal("rendered owning edges", out.Outcome())
			}
			frozen := out.Candidate().Page()
			proofs := pageProofs(t, frozen)
			if len(frozen.Frames()[0].Fragments()) != 1 || len(frozen.Frames()[1].Fragments()) != 1 {
				t.Fatal("earlier target frames unused")
			}
			if manual && len(frozen.Frames()[2].Fragments()) == 0 {
				t.Fatal("manual prefix was not split")
			}
			searched, err := run.PaginationSearch(ctx, initial)
			if err != nil || searched.Outcome() != page.SearchComplete || searched.HeuristicTruncated() || len(searched.Selected().Pages()) != 1 {
				t.Fatal("real-font span search", searched, err)
			}
			selectedPage := searched.Selected().Pages()[0].Page()
			if !reflect.DeepEqual(proofs, pageProofs(t, selectedPage)) {
				t.Fatal("search changed the local frozen result")
			}
			document, err := run.Paginate(ctx, initial)
			if err != nil || !document.Complete() || len(document.Spans()) != 1 || !document.State().DocumentComplete() {
				t.Fatal("real-font document pagination", document, err)
			}
			if err := session.Close(); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				runtime.GC()
			}
			if !reflect.DeepEqual(proofs, pageProofs(t, frozen)) || len(initial.Info().Activated) != 0 {
				t.Fatal("retained page/source changed")
			}
			if !reflect.DeepEqual(proofs, pageProofs(t, selectedPage)) {
				t.Fatal("selected search output changed after session close")
			}
			calls := 0
			delivered, err := document.Spans()[0].Selected().Shipout(ctx, func(p *page.Page) error {
				calls++
				if p.Number() != 1 || !reflect.DeepEqual(proofs, pageProofs(t, p)) {
					t.Fatal("delivered frozen output changed after session close")
				}
				return nil
			})
			if err != nil || !delivered.Complete() || calls != 1 || len(delivered.CompletedPages()) != 1 {
				t.Fatal("real-font delivery", delivered, err)
			}
		})
	}
}
