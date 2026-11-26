package pagelayout_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"weak"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	page "github.com/npillmayer/pagelayout"
	"github.com/npillmayer/pagelayout/content"
	prep "github.com/npillmayer/pagelayout/preparation/typography"
	"github.com/npillmayer/pagelayout/template"
	"github.com/npillmayer/typography/glyphing"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/render"
)

const proofFontSHA = "fa09788adcc651d77eba7d96f8cc3a600d6484c58e3b4ff79c2b2464dbfea6f8"

var errProofService = errors.New("proof fitting service failure")

type proofRecipe struct {
	factories, shapes, finishes, discoveries int
	streams                                  []weak.Pointer[proofStream]
}
type proofStream struct {
	contracts.StreamingShaper
	recipe   *proofRecipe
	finished bool
}

func (r *proofRecipe) stream() (contracts.StreamingShaper, error) {
	r.factories++
	s, err := glyphot.NewStream(32)
	if err != nil {
		return nil, err
	}
	p := &proofStream{StreamingShaper: s, recipe: r}
	r.streams = append(r.streams, weak.Make(p))
	return p, nil
}
func (s *proofStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	s.recipe.shapes++
	return s.StreamingShaper.Shape(ctx, p, r, o)
}
func (s *proofStream) Finish() error {
	if !s.finished {
		s.recipe.finishes++
		s.finished = true
	}
	return s.StreamingShaper.Finish()
}
func (r *proofRecipe) Discretionaries(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	r.discoveries++
	if word != "\u00ad" {
		return nil, errProofService
	}
	// The shaped right half ("operate") is a nonempty checkpoint carry;
	// the renderer uses its supported original-text paint correspondence.
	return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
}

type proofHandle struct{ inner content.GroupHandle }

func (h *proofHandle) Release() error { err := h.inner.Release(); h.inner = nil; return err }

type proofPreparer struct {
	inner   content.Preparer
	handles []weak.Pointer[proofHandle]
	calls   int
}

func (p *proofPreparer) Preflight(g []content.GroupInput) error { return p.inner.Preflight(g) }
func (p *proofPreparer) PrepareGroup(g content.GroupInput) (content.GroupPreparation, error) {
	p.calls++
	x, e := p.inner.PrepareGroup(g)
	if e != nil {
		return x, e
	}
	h := &proofHandle{x.Handle}
	p.handles = append(p.handles, weak.Make(h))
	x.Handle = h
	return x, nil
}

type proofFitter struct {
	inner          content.GroupFitter
	fail           bool
	calls, choices int
}

func (f *proofFitter) MeasureGroup(h content.GroupHandle, r content.InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	return f.inner.MeasureGroup(h.(*proofHandle).inner, r)
}
func (f *proofFitter) FitGroup(ctx context.Context, h content.GroupHandle, r content.GroupFitRequest, yield func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error) {
	f.calls++
	if f.fail {
		return layout.FitFailed, errProofService
	}
	return f.inner.FitGroup(ctx, h.(*proofHandle).inner, r, func(a layout.ParagraphAlternative) bool { f.choices++; return yield(a) })
}

type proofPageRecord struct {
	Number             uint64
	Template           string
	Span               int
	Score              page.Score
	Activated, Pending []page.BranchID
	Groups             []proofGroupRecord
}
type proofGroupRecord struct {
	Tree                content.TreeID
	Box                 layout.BoxID
	Source              content.SourceRef
	From, To            uint64
	Lines               int
	Atomic              bool
	X, Y, Width, Height float32
}
type proofDocumentRecord struct {
	Name             string
	FontSHA          string
	Pages            []proofPageRecord
	SpanLoss         []uint64
	BasePreparations int
}
type retainedShipProof struct {
	pages     []*page.Page
	proofs    [][]*render.ParagraphProof
	record    proofDocumentRecord
	collected map[string]func() bool
}

func watch[T any](m map[string]func() bool, name string, p *T) {
	w := weak.Make(p)
	m[name] = func() bool { return w.Value() == nil }
}

// Return output only from a separate stack frame. No result/state/session is
// intentionally retained: their source/checkpoint ownership is a different case.
func makeShipProof(t *testing.T, name string) retainedShipProof {
	t.Helper()
	ctx := t.Context()
	manual := name == "manual"
	recipe := &proofRecipe{}
	backend := pinnedProofBackend(t, recipe)
	loader := &proofPreparer{inner: backend}
	file, err := os.Open("testdata/pagination/content/proof-" + name + ".html")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := content.Load(file, content.LoadOptions{SourceName: "proof-" + name + ".html"})
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	prepared, session, err := content.Prepare(doc, loader)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	plan, err := page.Plan(doc)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := template.LoadFS(os.DirFS("testdata/pagination"), "templates/proof.html")
	if err != nil {
		t.Fatal(err)
	}
	adapter := &proofFitter{inner: prep.FittingAdapter()}
	opening := "narrow"
	if manual {
		opening = "manual"
	}
	// Force a changing-width main chain and separate transitive drain pages.
	// This fixture proves integration; unfiltered optimization is checked separately.
	eligible := func(s page.State, tpl *template.Template) (bool, error) {
		want := "wide"
		info := s.Info()
		if info.NextPage == 1 {
			want = opening
		} else if info.Span == 1 {
			main := false
			for _, h := range info.Heads {
				if h.Channel == "main" {
					main = true
					for _, span := range plan.Spans() {
						if span.ID == info.Span && h.HasContinuation {
							if c, ok := h.Position.Compare(span.End); ok && c >= 0 {
								main = false
							}
						}
					}
				}
			}
			if !main {
				want = "figures"
				for _, id := range info.Pending {
					for _, b := range plan.Branches() {
						if b.ID == id && b.Channel == "notes" {
							want = "notes"
						}
					}
				}
			}
		}
		return tpl.ID() == want, nil
	}
	run, err := page.NewPagination(page.PaginationConfig{Plan: plan, Catalog: catalog, Prepared: prepared, Session: session, Fitter: adapter, Eligible: eligible})
	if err != nil {
		t.Fatal(err)
	}
	retained := retainedShipProof{record: proofDocumentRecord{Name: name, FontSHA: proofFontSHA, BasePreparations: loader.calls}, collected: map[string]func() bool{}}
	watch(retained.collected, "paginator", run)
	watch(retained.collected, "session", session)
	watch(retained.collected, "preparer", loader)
	watch(retained.collected, "factory/hyphenator", recipe)
	watch(retained.collected, "backend", backend)
	watch(retained.collected, "fitting adapter", adapter)
	initial := run.InitialState()
	before := initial.Info()
	baseShapes, baseFactories := recipe.shapes, recipe.factories
	first, err := run.PageAttempt(ctx, initial, opening)
	if err != nil || first.Outcome() != page.Constructed {
		t.Fatal("first attempt", first, err)
	}
	prefix := first.Candidate().Page()
	prefixProof := pageProofs(t, prefix)
	prefixGeometry := proofGeometry(prefix)
	// An independently completed competing width, followed by a failed attempt.
	rejected, err := run.PageAttempt(ctx, initial, "wide")
	if err != nil || rejected.Outcome() != page.Constructed {
		t.Fatal("wide competitor", rejected, err)
	}
	rejectPage := rejected.Candidate().Page()
	watch(retained.collected, "rejected page handle", rejectPage)
	for _, frame := range rejectPage.Frames() {
		for _, part := range frame.Fragments() {
			bundle, e := prep.InspectFragment(part)
			if e != nil {
				t.Fatal(e)
			}
			for _, id := range bundle.Groups() {
				g, _ := bundle.Group(id)
				watch(retained.collected, fmt.Sprintf("rejected paragraph %v", id), g.Paragraph)
			}
		}
	}
	adapter.fail = true
	failed, err := run.PageAttempt(ctx, initial, "wide")
	if !errors.Is(err, errProofService) || failed.Candidate() != nil {
		t.Fatal("failure publication", err)
	}
	adapter.fail = false
	if !reflect.DeepEqual(prefixProof, pageProofs(t, prefix)) || !reflect.DeepEqual(prefixGeometry, proofGeometry(prefix)) || !reflect.DeepEqual(before, initial.Info()) {
		t.Fatal("competition changed accepted/source material")
	}
	result, err := run.Paginate(ctx, initial)
	if err != nil || !result.Complete() || len(result.Spans()) != 2 {
		t.Fatal(result, err)
	}
	if loader.calls != len(prepared.Groups()) || !manual && (recipe.shapes != baseShapes || recipe.factories != baseFactories) {
		t.Fatal("base preparation repeated", loader.calls, recipe.shapes, baseShapes)
	}
	// One authored opportunity lazily shapes its two replacement halves once.
	// Repeating the competing attempts/search must not encode base text again
	// or rediscover that accepted opportunity. Other fixtures may have more work.
	if manual && (recipe.discoveries != 1 || recipe.factories-baseFactories != 2 || recipe.shapes-baseShapes != 2) {
		t.Fatal("manual work was lost or repeated", recipe.discoveries, recipe.factories-baseFactories, recipe.shapes-baseShapes)
	}
	if recipe.finishes != recipe.factories {
		t.Fatal("unclosed shaping stream")
	}
	t.Logf("%s: groups=%d base factories/shapes=%d/%d after trials=%d/%d manual discoveries=%d fits=%d alternatives=%d", name, loader.calls, baseFactories, baseShapes, recipe.factories, recipe.shapes, recipe.discoveries, adapter.calls, adapter.choices)
	seen := map[content.FragmentID]bool{}
	ends := map[content.Box]paragraph.LogicalPosition{}
	replacements := map[uint64]int{}
	events := map[content.SourceRef]int{}
	atomicWidths := []float32{}
	for _, span := range result.Spans() {
		seq := span.Selected()
		retained.record.SpanLoss = append(retained.record.SpanLoss, seq.Loss().Info().Units)
		for _, candidate := range seq.Pages() {
			p := candidate.Page()
			rec := proofPageRecord{Number: p.Number(), Template: p.Template(), Span: candidate.Origin().Info().Span, Score: candidate.Score(), Activated: candidate.Successor().Info().Activated, Pending: candidate.Successor().Info().Pending}
			for _, frame := range p.Frames() {
				if frame.Extent().H > frame.Frame.ContentBox.H || frame.Extent().W > frame.Frame.ContentBox.W {
					t.Fatal("frame overflow")
				}
				for _, part := range frame.Fragments() {
					bundle, e := prep.InspectFragment(part)
					if e != nil {
						t.Fatal(e)
					}
					for _, id := range bundle.Groups() {
						g, _ := bundle.Group(id)
						info, _ := g.Input.Group.Info()
						if seen[id] {
							t.Fatal("duplicate occurrence")
						}
						seen[id] = true
						if prev, ok := ends[g.Input.Group]; ok {
							if c, ok := prev.Compare(g.Info.From); !ok || c != 0 {
								t.Fatal("gap or replay", prev, g.Info.From)
							}
						} else if g.Info.From.Offset != 0 {
							t.Fatal("lost prefix")
						}
						ends[g.Input.Group] = g.Info.To
						for _, c := range g.Coverage {
							if c.Source.Name != "proof-"+name+".html" || c.Source.NodeID == 0 {
								t.Fatal("lost source identity")
							}
							if c.Coverage.Kind == paragraph.ReplacementCoverage {
								replacements[p.Number()]++
							}
						}
						for _, e := range g.Events {
							if e.Event.Kind == content.SourceFlowAnchor {
								events[e.Event.Source]++
							}
						}
						if g.Input.ParentAtomic.Valid() {
							w := part.Geometry()[g.Input.ParentAtomic].Content.W
							atomicWidths = append(atomicWidths, w)
						}
						rec.Groups = append(rec.Groups, proofGroupRecord{info.Tree, info.ID, g.Input.Source, g.Info.From.Offset, g.Info.To.Offset, g.Info.Lines, g.Input.ParentAtomic.Valid(), g.X, g.Y, float32(g.Info.Width * 4 / 3), float32(g.Info.Height * 4 / 3)})
					}
				}
			}
			retained.record.Pages = append(retained.record.Pages, rec)
		}
		report, e := seq.Shipout(ctx, func(p *page.Page) error {
			retained.pages = append(retained.pages, p)
			retained.proofs = append(retained.proofs, pageProofs(t, p))
			return nil
		})
		if e != nil || !report.Complete() {
			t.Fatal(report, e)
		}
	}
	for _, g := range prepared.Groups() {
		input, _ := prepared.Input(g.Group)
		end, ok := ends[g.Group]
		if !ok || end.Offset != uint64(len(input.Text)) {
			t.Fatal("source not conserved", input.Text, end)
		}
	}
	for _, a := range plan.Anchors() {
		if events[a.Source] != 1 {
			t.Fatal("owning event replay/loss", a.Source, events)
		}
	}
	if !manual && !slices.Equal(atomicWidths, []float32{50, 80}) {
		t.Fatal("atomic suffix not rebound", atomicWidths)
	}
	if manual && (replacements[1] == 0 || replacements[2] == 0) {
		t.Fatal("manual replacement halves missing", replacements)
	}
	if !reflect.DeepEqual(prefixProof, retained.proofs[0]) {
		t.Fatal("search changed accepted prefix")
	}
	if !reflect.DeepEqual(prefixGeometry, proofGeometry(retained.pages[0])) {
		t.Fatal("search changed accepted atomic geometry")
	}
	// Independent facts behind the golden sequence/loss report: one changed-
	// width main continuation, then note and transitive figure draining before
	// the future main span. Carry is charged exactly on pages that defer targets.
	wantTemplates := []string{opening, "wide", "notes", "figures", "wide"}
	wantCarry := []uint64{0, page.UnitsPerPoint, page.UnitsPerPoint, 0, 0}
	if manual {
		wantCarry[0] = page.UnitsPerPoint
	}
	if len(retained.pages) != len(wantTemplates) {
		t.Fatal("unexpected page sequence")
	}
	for i, rec := range retained.record.Pages {
		wantSpan := 1
		if i == 4 {
			wantSpan = 2
		}
		if rec.Number != uint64(i+1) || rec.Template != wantTemplates[i] || rec.Span != wantSpan || rec.Score.Carry.Info().Units != wantCarry[i] || rec.Score.Page.Info().Units != page.UnitsPerPoint || rec.Score.MissingMain.Info().Units != 0 {
			t.Fatal("sequence or factual loss changed", rec)
		}
	}
	for _, proofs := range retained.proofs {
		for _, proof := range proofs {
			for _, glyph := range proof.Glyphs {
				if glyph.FaceSHA256 != proofFontSHA || glyph.Color != "#234567ff" && glyph.Color != "#9a3412ff" && glyph.Color != "#166534ff" {
					t.Fatal("font/source paint changed", glyph.Color)
				}
			}
		}
	}
	for i, w := range loader.handles {
		retained.collected[fmt.Sprintf("group handle %d", i)] = func() bool { return w.Value() == nil }
	}
	for i, w := range recipe.streams {
		retained.collected[fmt.Sprintf("stream %d", i)] = func() bool { return w.Value() == nil }
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := run.Paginate(ctx, result.State()); !errors.Is(err, content.ErrClosedSession) {
		t.Fatal("closed execution resumed", err)
	}
	return retained
}

func proofGeometry(p *page.Page) []map[content.Box]layout.LayoutGeometry {
	var result []map[content.Box]layout.LayoutGeometry
	for _, frame := range p.Frames() {
		for _, part := range frame.Fragments() {
			result = append(result, part.Geometry())
		}
	}
	return result
}

func TestShipoutRealFontDocumentProof(t *testing.T) {
	var records []proofDocumentRecord
	var all []retainedShipProof
	for _, name := range []string{"atomics", "manual"} {
		t.Run(name, func(t *testing.T) {
			retained := makeShipProof(t, name)
			for range 5 {
				runtime.GC()
			}
			for name, collected := range retained.collected {
				if !collected() {
					t.Error("output retained", name)
				}
			}
			for i, p := range retained.pages {
				if !reflect.DeepEqual(retained.proofs[i], pageProofs(t, p)) {
					t.Fatal("frozen output changed after collection")
				}
			}
			records = append(records, retained.record)
			all = append(all, retained)
			runtime.KeepAlive(retained)
		})
	}
	if t.Failed() {
		return
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if os.Getenv("PAGELAYOUT_UPDATE_SHIP_PROOF") == "1" {
		if err := os.WriteFile("testdata/pagination/proof-expected.json", data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile("testdata/pagination/proof-expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatalf("proof results differ; observed:\n%s", data)
	}
	if path := os.Getenv("PAGELAYOUT_SHIP_PROOF"); path != "" {
		if err := writeShipProof(path, all); err != nil {
			t.Fatal(err)
		}
	}
}

func pinnedProofBackend(t *testing.T, recipe *proofRecipe) *prep.Backend {
	t.Helper()
	ctx := t.Context()
	data, err := os.ReadFile("../opentype/testdata/fonts/GentiumPlus-R.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != proofFontSHA {
		t.Fatal("font fixture changed")
	}
	services := glyphot.Services{}
	face, err := services.Load(ctx, "../opentype/testdata/fonts/GentiumPlus-R.ttf", nil)
	if err != nil {
		t.Fatal(err)
	}
	mapper, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{Default: face.Resource.ID(), Size: 15 * dimen.BP, Faces: []glyphing.FaceEntry{face}}, services)
	if err != nil {
		t.Fatal(err)
	}
	backend, err := prep.New(prep.Config{Resolver: mapper, NewShaper: recipe.stream, Hyphenator: recipe})
	if err != nil {
		t.Fatal(err)
	}

	return backend
}

func TestShipoutOversizedRealAtomicIsInfeasible(t *testing.T) {
	recipe := &proofRecipe{}
	backend := pinnedProofBackend(t, recipe)
	doc, err := content.Load(strings.NewReader(`<body style="font-size:20px;hyphens:none"><span style="display:inline-block;width:300px">Too wide.</span></body>`), content.LoadOptions{})
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
	catalog, err := template.LoadFS(os.DirFS("testdata/pagination"), "templates/proof.html")
	if err != nil {
		t.Fatal(err)
	}
	run, err := page.NewPagination(page.PaginationConfig{Plan: plan, Catalog: catalog, Prepared: prepared, Session: session, Fitter: prep.FittingAdapter()})
	if err != nil {
		t.Fatal(err)
	}
	initial := run.InitialState()
	before := initial.Info()
	result, err := run.Paginate(t.Context(), initial)
	if err != nil || result.Complete() || len(result.Spans()) != 0 || result.LastSearch().Outcome() != page.SearchInfeasible || result.LastSearch().Stats().Attempts != uint64(len(catalog.IDs())) || result.State().Info().NextPage != 1 || !reflect.DeepEqual(initial.Info(), before) {
		t.Fatal("oversized atomic created endless/partial pages", result, err)
	}
}
