package typography

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"weak"

	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/render"
	renderot "github.com/npillmayer/typography/render/adapter/opentype"
	"github.com/npillmayer/typography/style"
)

func realFragmentProofs(t testing.TB, f *Fragment) []*render.ParagraphProof {
	t.Helper()
	backend := renderot.New()
	var out []*render.ParagraphProof
	for _, id := range f.Groups() {
		g, _ := f.Group(id)
		paint := make([]style.PaintSpan, len(g.Input.Paint))
		for i, s := range g.Input.Paint {
			c := s.Color
			paint[i] = style.PaintSpan{Start: s.Start, End: s.End, Paint: style.Paint{Color: fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)}}
		}
		proof, err := render.BuildFragmentProof(render.FragmentInput{Fragment: g.Paragraph, Text: g.Input.Text, Paint: paint}, render.Services{Backend: backend, Identity: backend})
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, proof)
	}
	return out
}

type weakFit struct {
	preparation weakPreparation
	fitter      weak.Pointer[content.Fitter]
}
type retainedFit struct {
	manifest     *content.PreparedSnapshot
	continuation content.FitContinuation
	fragments    []*Fragment
	proofs       [][]*render.ParagraphProof
}

func retainRealFit(t *testing.T, manual bool) (weakFit, retainedFit) {
	t.Helper()
	b, _, _ := realBackend(t, "GentiumPlus-R.ttf")
	recipe := &lifetimeRecipe{}
	b.config.NewShaper = recipe.stream
	b.config.Hyphenator = recipe
	source := `<body style="font-size:20px;hyphens:none;color:#234567">Before <span style="display:inline-block;width:25px">in</span> after and more text.</body>`
	width, height := float32(90), float32(300)
	if manual {
		source = `<body style="font-size:20px;hyphens:manual;color:#234567">co&shy;operate</body>`
		width, height = 30, 300
	}
	d := load(t, source)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	f, err := content.NewFitter(p, s, d.Residual().Root)
	if err != nil {
		t.Fatal(err)
	}
	w := weakFit{fitter: weak.Make(f), preparation: weakPreparation{backend: weak.Make(b), session: weak.Make(s), recipe: weak.Make(recipe)}}
	for _, g := range p.Groups() {
		if err := s.WithGroup(p, g.Group, g.Owner, func(handle content.GroupHandle) error {
			h := handle.(*preparedGroup)
			w.preparation.groups = append(w.preparation.groups, weak.Make(h))
			w.preparation.paragraphs = append(w.preparation.paragraphs, weak.Make(h.paragraph))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	first := fitted(t, p, s, f, fitRequest(width, height, content.FitContinuation{}))
	for len(first) > 0 && first[0].Complete() {
		first = first[1:]
	}
	if len(first) == 0 {
		t.Fatal("real font did not produce a partial choice")
	}
	last := fitted(t, p, s, f, fitRequest(180, 300, first[0].Continuation()))[0]
	if !last.Complete() {
		t.Fatal("real-font continuation")
	}
	retained := retainedFit{manifest: p, continuation: first[0].Continuation(), fragments: []*Fragment{first[0].Fragment(), last.Fragment()}}
	for _, fragment := range retained.fragments {
		retained.proofs = append(retained.proofs, realFragmentProofs(t, fragment))
	}
	w.preparation.streams = append(w.preparation.streams, recipe.streams...)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return w, retained
}

func TestFitRealFontLifetime(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(fmt.Sprint(manual), func(t *testing.T) {
			w, retained := retainRealFit(t, manual)
			for range 5 {
				runtime.GC()
			}
			if w.fitter.Value() != nil || w.preparation.backend.Value() != nil || w.preparation.session.Value() != nil || w.preparation.recipe.Value() != nil {
				t.Fatal("retained working services through fragment or continuation")
			}
			for _, g := range w.preparation.groups {
				if g.Value() != nil {
					t.Fatal("retained group handle")
				}
			}
			for _, p := range w.preparation.paragraphs {
				if p.Value() != nil {
					t.Fatal("retained paragraph preparation")
				}
			}
			for _, s := range w.preparation.streams {
				if s.Value() != nil {
					t.Fatal("retained stream")
				}
			}
			for i, f := range retained.fragments {
				proofs := realFragmentProofs(t, f)
				if len(proofs) == 0 || len(proofs[0].Glyphs) == 0 || !reflect.DeepEqual(proofs, retained.proofs[i]) {
					t.Fatal("frozen rendering changed after collection")
				}
			}
			runtime.KeepAlive(retained)
		})
	}
}

func TestFitRealLigatureAnchorAndBidiCoverage(t *testing.T) {
	b, _, _ := realBackend(t, "GentiumPlus-R.ttf")
	d := load(t, `<body style="font-size:20px;hyphens:none">o<a data-flow-anchor href="#n">f</a>fice <span dir="rtl">123</span><aside id="n" style="flow-into:notes">Note</aside></body>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, err := content.NewFitter(p, s, d.Residual().Root)
	if err != nil {
		t.Fatal(err)
	}
	all := fitted(t, p, s, f, fitRequest(200, 100, content.FitContinuation{}))
	if len(all) == 0 || !all[0].Complete() {
		t.Fatal("ligature fit")
	}
	g, _ := all[0].Fragment().Group(all[0].Fragment().Groups()[0])
	if len(g.Events) != 1 || g.Events[0].Coverage.Source.Start != 1 || g.Events[0].Coverage.Source.End <= 2 {
		t.Fatal("anchor did not attach to whole ligature", g.Events)
	}
	for i := 1; i < len(g.Coverage); i++ {
		if g.Coverage[i].Coverage.Source.Start < g.Coverage[i-1].Coverage.Source.Start {
			t.Fatal("coverage in visual instead of logical order")
		}
	}
}
