package typography

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/uax/bidi"
	domdimen "github.com/npillmayer/w3c-dom/dimen"
	"github.com/npillmayer/w3c-dom/style/textstyle"
)

func TestPhysicalStyleConversion(t *testing.T) {
	s := textstyle.Initial()
	s.FontSize = 20 * domdimen.PX
	s.Direction = textstyle.RightToLeft
	s.Language = "he"
	s.Hyphens = "auto"
	s.Color = "red"
	m, err := mapStyle(s)
	if err != nil || m.EmSize != 15*dimen.BP || m.Direction != bidi.RightToLeft || m.Language != "he" || m.Color != "" || m.Family != s.Family {
		t.Fatal(m, err)
	}
	for _, size := range []domdimen.DU{0, -1, domdimen.DU(dimen.Fil), math.MaxInt32} {
		s.FontSize = size
		if _, err := mapStyle(s); !errors.Is(err, khipu.ErrUnsupportedPolicy) {
			t.Fatal("unchecked finite font size", size, err)
		}
	}
	s = textstyle.Initial()
	s.FontSize = 1
	if m, err := mapStyle(s); err != nil || m.EmSize != 1 {
		t.Fatal("smallest positive unit", m, err)
	}
}

func TestOwnerStrutAndPaintBypass(t *testing.T) {
	for _, objectOnly := range []bool{false, true} {
		b, s := testBackend(t)
		html := `<body style="font-size:20px;hyphens:none"><p><span style="font-size:8px;color:red">a</span><span style="font-size:8px;color:blue">b</span></p>`
		if objectOnly {
			html = `<body style="font-size:20px;hyphens:none"><p><span style="font-size:8px;display:inline-block"></span></p>`
		}
		p, session, err := content.Prepare(load(t, html), b)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		if s.face.metrics.Load() != 0 {
			t.Fatal("preparation queried vertical metrics")
		}
		if !objectOnly && (len(s.shapes) != 1 || s.shapes[0] != (shapeCall{"ab", 6 * dimen.BP})) {
			t.Fatal("paint split shaping or font size changed", s.shapes)
		}
		group := p.Groups()[0]
		input, _ := p.Input(group.Group)
		if input.TextStyle.FontSize != 20*domdimen.PX || !objectOnly && (len(input.Paint) != 2 || input.Paint[0].Color == input.Paint[1].Color) {
			t.Fatal("owner/paint lost")
		}
		var result *paragraph.Result
		before := s.created
		err = session.WithGroup(p, group.Group, group.Owner, func(h content.GroupHandle) error {
			x := h.(*preparedGroup)
			req := paragraph.LayoutRequest{Params: paragraph.Params{Constraints: paragraph.Constraints{Width: 200 * dimen.BP}, Breaker: paragraph.FirstFit, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &x.lineLayout}, Horizontal: &paragraph.HorizontalLayout{}}
			if objectOnly {
				req.Atomics = []paragraph.AtomicBox{{ID: 1, Alignment: paragraph.AlignBottomMarginEdge}}
			}
			// Direct paragraph assembly isolates the retained owner strut from
			// subtree layout and fitting.
			result, err = x.paragraph.Layout(req)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.VList.Box().Extent != 15 || s.created != before {
			t.Fatal("owner strut/reuse", result.VList.Box(), s.created)
		}
		if err := session.Close(); err != nil {
			t.Fatal(err)
		}
		if result.Frozen.SourceLen() != uint64(len(input.Text)) || result.VList.Box().Extent != 15 {
			t.Fatal("release invalidated frozen output")
		}
	}
}

func TestBidiMappingAndPreflight(t *testing.T) {
	b, s := testBackend(t)
	p, session, err := content.Prepare(load(t, `<body style="hyphens:none">a<span dir="rtl">אב<span dir="ltr" style="display:inline-block">inside</span>ג</span>z`), b)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	input, _ := p.Input(p.Groups()[0].Group)
	m, err := b.mapInput(input)
	if err != nil {
		t.Fatal(err)
	}
	for pos, want := range map[uint64]int{1: bidi.MarkupRLI, 5: bidi.MarkupLRI, 8: bidi.MarkupPDI, 10: bidi.MarkupPDI} {
		if m.markup(pos) != want {
			t.Fatal("bidi coordinate", pos, m.markup(pos))
		}
	}
	input.Isolates = nil
	if m.markup(5) != bidi.MarkupLRI {
		t.Fatal("markup aliases source slices")
	}
	inner, _ := p.Input(p.Groups()[1].Group)
	if len(inner.Isolates) != 0 {
		t.Fatal("outer scope entered atomic interior")
	}
	baseCreated, baseCoverage := s.created, s.coverage
	bad := load(t, `<body style="hyphens:none"><p>valid first</p><p><span dir="rtl"><span dir="ltr">nested same bounds</span></span></p>`)
	failed, failedSession, err := content.Prepare(bad, b)
	if failed != nil || failedSession != nil || !errors.Is(err, khipu.ErrUnsupportedPolicy) || s.created != baseCreated || s.coverage != baseCoverage {
		t.Fatal("coincident boundaries reached services", err)
	}
	// Adjacent closing/opening at one position is representable.
	g := content.GroupInput{Text: "abcd", Isolates: []content.IsolateSpan{{Start: 0, End: 2, Direction: textstyle.RightToLeft}, {Start: 2, End: 4, Direction: textstyle.LeftToRight}}}
	markup, err := mapIsolates(g)
	if err != nil || markup(2) != bidi.MarkupPDI|bidi.MarkupLRI {
		t.Fatal("adjacent isolate events", err)
	}
}

func TestPreparedMetadataDetaches(t *testing.T) {
	b, s := testBackend(t)
	p, session, err := content.Prepare(fixture(t, "text-only.html"), b)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	g := p.Groups()[0]
	input, _ := p.Input(g.Group)
	if !slices.Contains(s.covered, '\uFFFC') {
		t.Fatal("authored marker bypassed text coverage")
	}
	r, err := b.PrepareGroup(input)
	if err != nil {
		t.Fatal(err)
	}
	h := r.Handle.(*preparedGroup)
	defer h.Release()
	before := copyInput(h.input)
	input.Text = "changed"
	input.Styles[0].Style.Family = "changed"
	input.Paint[0].Color.A = 0
	input.TextSources[0].Range.End = 999
	input.Hyphens[0].Policy = "auto"
	if !reflect.DeepEqual(h.input, before) {
		t.Fatal("handle aliases caller input")
	}
	// A known style-span error retains decoded provenance rather than a made-up
	// source identity for the anonymous paragraph group.
	input, _ = p.Input(g.Group)
	input.Styles[1].Style.FontSize = domdimen.DU(dimen.Fil)
	baseCreated, baseCoverage := s.created, s.coverage
	err = b.Preflight([]content.GroupInput{input})
	var located *content.PreparationError
	if !errors.As(err, &located) || located.Group != g.Group || located.Stage != content.StagePreflight || !located.HasRange || s.created != baseCreated || s.coverage != baseCoverage {
		t.Fatal("source-aware pure preflight", err)
	}
	var absent *markerPolicy
	b.config.Hyphenator = absent
	input, _ = p.Input(g.Group)
	for i := range input.Hyphens {
		input.Hyphens[i].Policy = "manual"
	}
	if err := b.Preflight([]content.GroupInput{input}); !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal("typed-nil hyphenator", err)
	}
}

func TestCopyInputSourceEvents(t *testing.T) {
	original := content.GroupInput{Events: []content.GroupEvent{{Start: 1, End: 3, Affinity: content.AffinityVisible}}}
	copied := copyInput(original)
	copied.Events[0].Start = 999
	if original.Events[0].Start != 1 {
		t.Fatal("bridge retained mutable source-event slice")
	}
}
