package typography

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/fontresource"
	"github.com/npillmayer/typography/glyphing"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/render"
	renderot "github.com/npillmayer/typography/render/adapter/opentype"
	"github.com/npillmayer/typography/style"
)

// Only tests/examples choose a concrete backend. Fonts are checked-in sibling
// fixtures, not host-installed or downloaded resources.
type realCalls struct {
	factories, shapes, finishes, coverage atomic.Int64
	failFactory                           atomic.Int64
	sealed                                atomic.Bool
}

func (c *realCalls) counts() [4]int64 {
	return [4]int64{c.factories.Load(), c.shapes.Load(), c.finishes.Load(), c.coverage.Load()}
}

var errRealService = errors.New("injected real-font service failure")

type realStream struct {
	contracts.StreamingShaper
	calls    *realCalls
	finished atomic.Bool
}

func (s *realStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	if s.calls.sealed.Load() {
		return errRealService
	}
	s.calls.shapes.Add(1)
	return s.StreamingShaper.Shape(ctx, p, r, o)
}
func (s *realStream) Finish() error {
	if s.finished.CompareAndSwap(false, true) {
		s.calls.finishes.Add(1)
	}
	return s.StreamingShaper.Finish()
}

func realBackend(t testing.TB, name string) (*Backend, *realCalls, string) {
	t.Helper()
	data, err := os.ReadFile("../../../opentype/testdata/fonts/" + name)
	if err != nil {
		t.Fatal(err)
	}
	r, err := fontresource.Read(bytes.NewReader(data), fontresource.Origin{Provider: "sibling fixture", Name: name})
	if err != nil {
		t.Fatal(err)
	}
	clear(data) // Caller-owned input bytes may be released immediately.
	wantSHA := map[string]string{
		"GentiumPlus-R.ttf":            "fa09788adcc651d77eba7d96f8cc3a600d6484c58e3b4ff79c2b2464dbfea6f8",
		"ShaperTestLatin-NoEacute.ttf": "a31c0d92101751fcd0643803bfb004060db47bdb5d77018cdf659526ed92be9b",
	}[name]
	if wantSHA == "" || r.SHA256() != wantSHA {
		t.Fatalf("unexpected real-font fixture %s: %s", name, r.SHA256())
	}
	services := glyphot.Services{}
	entry, err := services.Load(context.Background(), "", r)
	if err != nil {
		t.Fatal(err)
	}
	calls := &realCalls{}
	mapper, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{Faces: []glyphing.FaceEntry{entry}, Default: r.ID(), Size: 15 * dimen.BP}, glyphing.CoverageFuncs{
		Check: services.CheckFace,
		Lookup: func(f contracts.FontFace, c rune) (uint32, error) {
			if calls.sealed.Load() {
				return 0, errRealService
			}
			calls.coverage.Add(1)
			return services.GlyphIndex(f, c)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(Config{Resolver: mapper, NewShaper: func() (contracts.StreamingShaper, error) {
		if calls.sealed.Load() {
			return nil, errRealService
		}
		if calls.factories.Add(1) == calls.failFactory.Load() {
			return nil, errRealService
		}
		s, err := glyphot.NewStream(32)
		if err != nil {
			return nil, err
		}
		return &realStream{StreamingShaper: s, calls: calls}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	return b, calls, r.SHA256()
}

func realProofs(t testing.TB, result *Result) []*render.ParagraphProof {
	t.Helper()
	backend := renderot.New()
	var out []*render.ParagraphProof
	for _, g := range result.Groups() {
		v, ok := result.Group(g.Group)
		if !ok {
			t.Fatal("missing completed paragraph")
		}
		paint := make([]style.PaintSpan, len(v.Input.Paint))
		for i, s := range v.Input.Paint {
			c := s.Color
			paint[i] = style.PaintSpan{Start: s.Start, End: s.End, Paint: style.Paint{Color: fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)}}
		}
		p, err := render.BuildParagraphProof(render.ParagraphInput{Frozen: v.Paragraph.Frozen, VList: v.Paragraph.VList, Text: v.Input.Text, Paint: paint}, render.Services{Backend: backend, Identity: backend})
		if err != nil {
			t.Fatal(err)
		}
		for _, glyph := range p.Glyphs {
			if glyph.GlyphID == 0 || glyph.SourceByte >= uint64(len(v.Input.Text)) {
				t.Fatal("invalid real glyph/source", glyph)
			}
		}
		out = append(out, p)
	}
	return out
}

func TestRealFontTwoWidths(t *testing.T) {
	b, calls, sha := realBackend(t, "GentiumPlus-R.ttf")
	d := fixture(t, "nested.html")
	if calls.counts() != ([4]int64{}) {
		t.Fatal("content loading invoked font services")
	}
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := calls.counts()
	if base[0] != 3 || base[1] == 0 || base[0] != base[2] {
		t.Fatal("real preparation/stream lifecycle", base)
	}
	manifest := p.Groups()
	for range 2 {
		if _, err := MeasureIntrinsic(p, s, d.Residual().Root, layout.ResolveContext{}); err != nil {
			t.Fatal(err)
		}
	}
	a, err := Layout(p, s, d.Residual().Root, LayoutRequest{120, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	// Reject one complete candidate, then successfully try another width using
	// the same preparation. Rejection does not mutate or consume anything.
	if _, err := Layout(p, s, d.Residual().Root, LayoutRequest{160, paragraph.FirstFit}); err != nil {
		t.Fatal(err)
	}
	z, err := Layout(p, s, d.Residual().Root, LayoutRequest{240, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	pa, pz := realProofs(t, a), realProofs(t, z)
	if len(pa[0].Lines) <= len(pz[0].Lines) || reflect.DeepEqual(a.Geometry(), z.Geometry()) {
		t.Fatal("real font did not reflow at two widths")
	}
	for _, proof := range pa {
		if len(proof.Glyphs) == 0 {
			t.Fatal("empty real proof")
		}
		for _, g := range proof.Glyphs {
			if g.FaceSHA256 != sha || g.Path == "" {
				t.Fatal("font identity/outline lost")
			}
		}
	}
	if calls.counts() != base || !reflect.DeepEqual(manifest, p.Groups()) {
		t.Fatal("measurement/trials repeated base work")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	calls.sealed.Store(true)
	if !reflect.DeepEqual(pa, realProofs(t, a)) || !reflect.DeepEqual(pz, realProofs(t, z)) || calls.counts() != base {
		t.Fatal("retained proof changed after Close")
	}
	if path := os.Getenv("PAGELAYOUT_PREP_PROOF"); path != "" {
		if err := writeRealProof(path, []*Result{a, z}, [][]*render.ParagraphProof{pa, pz}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRealFontBankVersionsAndNewPreparation(t *testing.T) {
	b, calls, _ := realBackend(t, "GentiumPlus-R.ttf")
	d := fixture(t, "branches.html")
	original := d.Bank()
	first, advanced, ok := original.Take("article")
	if !ok {
		t.Fatal("missing branch")
	}
	next, ok := advanced.First("article")
	if !ok || first == next {
		t.Fatal("derived bank")
	}
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	branches := d.Branches("article")
	ga, gb := d.Groups(branches[0].Tree)[0], d.Groups(branches[1].Tree)[0]
	if ga.Group == gb.Group {
		t.Fatal("branches must have separate group identities")
	}
	a, err := Layout(p, s, first, LayoutRequest{100, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	before := realProofs(t, a)
	// A late construction failure must leave an older prepared pair and banks usable.
	calls.failFactory.Store(calls.factories.Load() + int64(len(p.Groups())))
	bad, bs, err := content.Prepare(d, b)
	var detail *content.PreparationError
	if bad != nil || bs != nil || !errors.Is(err, errRealService) || !errors.As(err, &detail) || detail.Group != gb.Group {
		t.Fatal("partial preparation published", err)
	}
	calls.failFactory.Store(0)
	z, err := Layout(p, s, next, LayoutRequest{100, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := z.Group(ga.Group); ok {
		t.Fatal("branch group identities aliased")
	}
	// A different caller-selected font resource produces a new preparation of the
	// same boxes. The sibling derivative has different bytes/identity but preserves
	// these ASCII glyphs; a cache must not confuse it with its source font.
	alternate, _, alternateSHA := realBackend(t, "ShaperTestLatin-NoEacute.ttf")
	p2, s2, err := content.Prepare(d, alternate)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if p.ID() == p2.ID() || !reflect.DeepEqual(p.Groups(), p2.Groups()) {
		t.Fatal("new preparation identity/source")
	}
	if r, err := Layout(p2, s, first, LayoutRequest{100, paragraph.FirstFit}); r != nil || !errors.Is(err, content.ErrInvalidPreparation) {
		t.Fatal("mixed sessions accepted", err)
	}
	changed, err := Layout(p2, s2, first, LayoutRequest{100, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	proof := realProofs(t, changed)
	if proof[0].Glyphs[0].FaceSHA256 != alternateSHA || reflect.DeepEqual(before, proof) {
		t.Fatal("HTML-only font cache")
	}
	_ = s.Close()
	_ = s2.Close()
	if !reflect.DeepEqual(before, realProofs(t, a)) {
		t.Fatal("new font/preparation changed retained output")
	}
	for _, bank := range []struct {
		want  content.Box
		first func() (content.Box, bool)
	}{{first, func() (content.Box, bool) { return original.First("article") }}, {next, func() (content.Box, bool) { return advanced.First("article") }}, {first, func() (content.Box, bool) { return d.Bank().First("article") }}} {
		got, ok := bank.first()
		if !ok || got != bank.want {
			t.Fatal("bank version advanced implicitly")
		}
	}
}

func TestRealFontCollidingBranchesAndStyleChanges(t *testing.T) {
	// Identical DOM structure yields colliding tree-local IDs across loads. CSS
	// overrides change source style/policy without changing that local numbering.
	const source = `<html><head><style>body {font-size:20px;hyphens:none} section {flow-into:article}</style></head><body><section>co&shy;operate</section></body></html>`
	var oldGroup content.Box
	var oldPreparation *content.PreparedSnapshot
	var oldResult *Result
	var oldProof []*render.ParagraphProof
	var oldInput content.GroupInput
	for _, css := range []string{"", "body {font-size:30px;color:blue}", "body {hyphens:manual}"} {
		d, err := content.Load(strings.NewReader(source), content.LoadOptions{Stylesheets: []content.Stylesheet{{SourceName: "override.css", CSS: css}}})
		if err != nil {
			t.Fatal(err)
		}
		b, _, _ := realBackend(t, "GentiumPlus-R.ttf")
		markers := &lifetimeRecipe{}
		b.config.Hyphenator = markers
		p, s, err := content.Prepare(d, b)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		branch := d.Branches("article")[0]
		group := d.Groups(branch.Tree)[0].Group
		if _, err := MeasureIntrinsic(p, s, branch.Root, layout.ResolveContext{}); err != nil {
			t.Fatal(err)
		}
		r, err := Layout(p, s, branch.Root, LayoutRequest{120, paragraph.FirstFit})
		if err != nil {
			t.Fatal(err)
		}
		input, _ := p.Input(group)
		proof := realProofs(t, r)
		if oldResult == nil {
			oldGroup, oldPreparation, oldResult, oldProof, oldInput = group, p, r, proof, input
		} else {
			a, _ := oldGroup.Info()
			z, _ := group.Info()
			if a.ID != z.ID || a.Tree != z.Tree || oldGroup == group {
				t.Fatal("test must collide locally but retain snapshot qualification")
			}
			if p.ID() == oldPreparation.ID() || input.TextStyle == oldInput.TextStyle {
				t.Fatal("changed CSS reused preparation")
			}
			if _, ok := r.Group(oldGroup); ok || p.Covers(oldGroup) {
				t.Fatal("cross-snapshot group identity aliased")
			}
			if _, ok := oldResult.Group(group); ok || !reflect.DeepEqual(oldProof, realProofs(t, oldResult)) {
				t.Fatal("new style/policy changed earlier result")
			}
			if saved, _ := oldPreparation.Input(oldGroup); !reflect.DeepEqual(saved, oldInput) {
				t.Fatal("new style/policy changed earlier preparation metadata")
			}
			if strings.Contains(css, "font-size") && reflect.DeepEqual(proof, oldProof) {
				t.Fatal("source font size/paint override was ignored")
			}
			if strings.Contains(css, "manual") && markers.discoveries == 0 {
				t.Fatal("source policy override did not enable manual discovery")
			}
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
