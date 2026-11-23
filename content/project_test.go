package content

import (
	"errors"
	"image/color"
	"strings"
	"sync"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/w3c-dom/dimen"
	"github.com/npillmayer/w3c-dom/style/textstyle"
)

func projection(t *testing.T, input string, opts LoadOptions) *sourceProjection {
	t.Helper()
	p, err := loadProjection(strings.NewReader(input), opts)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func projectedID(t testing.TB, p *sourceProjection, name string) sourceID {
	t.Helper()
	for _, s := range p.sources[1:] {
		for _, a := range s.attributes {
			if a.Key == "id" && a.Val == name {
				return s.ref.nodeID
			}
		}
	}
	t.Fatalf("missing projected source %s", name)
	return 0
}

func projectionFailure(t *testing.T, p *sourceProjection, err error) []diagnostic {
	t.Helper()
	var failure *LoadError
	if p != nil || !errors.As(err, &failure) || len(failure.items) == 0 {
		t.Fatalf("expected atomic projection failure: %v %v", p, err)
	}
	return failure.diagnostics()
}

func TestProjectionAcceptanceSourceStyles(t *testing.T) {
	for _, override := range []bool{false, true} {
		name := "base"
		opts := LoadOptions{SourceName: "routing.html"}
		font := float32(20)
		flow := "article"
		noteColor := color.NRGBA{R: 0x88, G: 0x44, A: 255}
		storyColor := color.NRGBA{R: 0x20, G: 0x30, B: 0x40, A: 255}
		border := float32(2)
		if override {
			name = "override"
			font = 24
			flow = "features"
			noteColor = color.NRGBA{G: 0x66, B: 0x99, A: 255}
			storyColor = color.NRGBA{G: 0x88, B: 0x55, A: 255}
			border = float32(117965.0 / 49152)
			opts.Stylesheets = []Stylesheet{{SourceName: "routing-override.css", CSS: fixture(t, "routing-override.css")}}
		}
		t.Run(name, func(t *testing.T) {
			p := projection(t, fixture(t, "routing.html"), opts)
			if p.root == nil || p.root.HTML.Data != "body" || p.rootFontSize != 16*dimen.PX {
				t.Fatal("wrong content/root font", p.root, p.rootFontSize)
			}
			for _, tc := range []struct {
				id         string
				font       float32
				foreground color.NRGBA
				flow       string
			}{
				{"source", font, color.NRGBA{R: 0x20, G: 0x30, B: 0x40, A: 255}, ""},
				{"story-a", font, color.NRGBA{R: 0x22, G: 0x44, B: 0x66, A: 255}, flow},
				{"tag", font, color.NRGBA{R: 0x22, G: 0x44, B: 0x66, A: 255}, ""},
				{"note", font, noteColor, "notes"}, {"story-b", font, storyColor, flow},
				{"paragraph-b", font, storyColor, ""}, {"intro", 16, color.NRGBA{R: 0x20, G: 0x30, B: 0x40, A: 255}, ""},
			} {
				id := projectedID(t, p, tc.id)
				s := p.sources[id]
				n := p.nodes[id]
				if s.participation != materialNode || s.constraints.FontSizePx != tc.font || s.typography.Foreground != tc.foreground || s.flow != tc.flow {
					t.Fatalf("%s: %+v", tc.id, s)
				}
				if s.typography.Language != "en" || s.typography.Hyphens != "manual" || s.typography.WhiteSpace != "normal" || s.typography.LineHeight.Kind != textstyle.NormalLineHeight || s.typography.Weight != 400 || s.typography.Family != "serif" {
					t.Fatalf("source defaults: %+v", s.typography)
				}
				if n == nil || n.Style == nil || *n.Style != s.constraints || n.Styles != nil || n.Style.Fixed != nil || n.Replaced != nil {
					t.Fatalf("untyped or sized projection: %+v", n)
				}
				if n.HTML.Parent != nil || n.HTML.FirstChild != nil || len(n.HTML.Attr) != 0 {
					t.Fatal("projection retained source DOM structure")
				}
			}
			b := p.sources[projectedID(t, p, "story-b")]
			if b.constraints.Width != (layout.Length{Kind: layout.LenPercent, Value: .75}) || b.constraints.Margin.Left.Value != font || b.constraints.Padding.Left.Value != 8 || b.constraints.Border.Left.Value != border || b.paint.borders[3] != (borderPaint{"solid", storyColor}) {
				t.Fatalf("story-b edges: %+v %+v", b.constraints, b.paint)
			}
			tag := p.nodes[projectedID(t, p, "tag")]
			if tag.Display.Outer != layout.OuterInline || tag.Display.Inner != layout.InnerFlowRoot || tag.Style.Width.Kind != layout.LenAuto || len(tag.ChildrenNodes) != 1 {
				t.Fatalf("lost unsized atomic interior: %+v", tag)
			}
			// Named roots remain at their original locations until CONT-03.
			source := p.nodes[projectedID(t, p, "source")]
			found := 0
			for _, n := range source.ChildrenNodes {
				if p.sources[n.ID].flow != "" {
					found++
				}
			}
			if found != 3 {
				t.Fatal("projection routed or lost a branch", found)
			}
			for _, tc := range []struct {
				id, text string
				size     uint64
			}{
				{"intro", "Residual café & introduction.", 30}, {"paragraph-b", "Second\u00a0article with a soft\u00adhyphen.", 36},
			} {
				n := p.nodes[projectedID(t, p, tc.id)].ChildrenNodes[0]
				if n.Text == nil || n.Text.Range.Start != 0 || n.Text.Range.End != tc.size || n.HTML.Data != tc.text || p.sources[n.ID].text != tc.text || uint64(n.Text.Source) != uint64(n.ID) || n.Display != (layout.Display{}) {
					t.Fatalf("decoded source ranges lost: %+v", n)
				}
			}
		})
	}
}

func TestProjectionSourceRelativeGeometryAndInheritance(t *testing.T) {
	p := projection(t, `<html style="font-size:24px"><body style="font-size:20px">
<div id="parent" style="width:10em;margin-left:-1em;padding-left:.5em;border-left:.25em solid currentcolor;color:blue">
<section id="child" style="font-size:2em;width:inherit;margin-left:inherit;padding-left:inherit;border-left:inherit"></section>
<div id="relative" style="width:2rem;margin:1% auto;padding:2%;border-top:.5rem solid;background:currentcolor"></div>
</div></body></html>`, LoadOptions{})
	c := p.sources[projectedID(t, p, "child")]
	if c.typography.FontSize != 40*dimen.PX || c.constraints.Width.Value != 200 || c.constraints.Margin.Left.Value != -20 || c.constraints.Padding.Left.Value != 10 || c.constraints.Border.Left.Value != 5 {
		t.Fatalf("relative inherit recomputed on child: %+v", c.constraints)
	}
	r := p.sources[projectedID(t, p, "relative")]
	if r.constraints.Width != (layout.Length{Kind: layout.LenPx, Value: 48}) || r.constraints.Margin.Left.Kind != layout.LenAuto || r.constraints.Margin.Top != (layout.Length{Kind: layout.LenPercent, Value: .01}) || r.constraints.Padding.Left != (layout.Length{Kind: layout.LenPercent, Value: .02}) || r.constraints.Border.Top.Value != 12 || r.paint.background != (color.NRGBA{B: 255, A: 255}) {
		t.Fatalf("source roots/percentages/paint: %+v", r)
	}
	for _, v := range []string{"96px", "72pt", "1in", "25.4mm", "2.54cm"} {
		q := projection(t, `<div id="size" style="width:`+v+`"></div>`, LoadOptions{})
		if got := q.sources[projectedID(t, q, "size")].constraints.Width; got != (layout.Length{Kind: layout.LenPx, Value: 96}) {
			t.Errorf("%s = %+v", v, got)
		}
	}
}

func TestProjectionSuppressionLedger(t *testing.T) {
	p := projection(t, fixture(t, "suppression.html"), LoadOptions{})
	for _, tc := range []struct {
		id   string
		kind participation
	}{{"visible", materialNode}, {"suppressed", suppressedNode}, {"suppressed-child", suppressedNode}, {"inert", inertNode}, {"stored", inertNode}, {"metadata", metadataNode}} {
		id := projectedID(t, p, tc.id)
		if p.sources[id].participation != tc.kind || (p.nodes[id] != nil) != (tc.kind == materialNode) {
			t.Errorf("%s: %+v", tc.id, p.sources[id])
		}
	}
	for _, src := range []string{
		`<div style="display:none"><table><tr><td lang="bad_!">hidden</td></tr></table><pre>unrendered</pre></div><p>visible</p>`,
		`<body style="display:none"><section>hidden</section></body>`,
		`<html style="display:none"><body><section>hidden</section></body></html>`,
	} {
		q := projection(t, src, LoadOptions{})
		if !strings.Contains(src, "visible") && q.root != nil {
			t.Fatal("suppressed body generated material")
		}
	}
	// Excluded source strings/ancestry are retained, without hidden material.
	hasText := false
	for _, s := range p.sources {
		if s.text == "Suppressed source text." {
			hasText = true
			if s.participation != suppressedNode || s.parent == 0 {
				t.Fatal("lost suppression provenance")
			}
		}
	}
	if !hasText {
		t.Fatal("suppressed text disappeared from ledger")
	}
}

func TestProjectionTypographyPoliciesAndEmptyOwner(t *testing.T) {
	p := projection(t, `<html lang="he" dir="rtl"><body><div id="owner" style="font-size:20px;hyphens:auto"><bdi id="isolated">abc</bdi><strong id="bold">b</strong><em id="em">i</em><span id="manual" style="hyphens:manual">soft&shy;hyphen</span><span id="none" style="hyphens:none">soft&shy;hyphen</span></div><p id="empty" style="font-size:24px"></p></body></html>`, LoadOptions{})
	for _, tc := range []struct{ id, hyphens string }{{"owner", "auto"}, {"manual", "manual"}, {"none", "none"}} {
		s := p.sources[projectedID(t, p, tc.id)]
		if s.typography.Hyphens != tc.hyphens || s.typography.Language != "he" {
			t.Fatal("lost policy", s.typography)
		}
	}
	if s := p.sources[projectedID(t, p, "isolated")].typography; s.Direction != textstyle.LeftToRight || s.UnicodeBidi != "isolate" {
		t.Fatalf("lost bidi: %+v", s)
	}
	if p.sources[projectedID(t, p, "bold")].typography.Weight != 700 || p.sources[projectedID(t, p, "em")].typography.Slant != "italic" {
		t.Fatal("HTML emphasis lost")
	}
	// A route-free builder contract check, not normalization in the loader.
	// Empty anonymous groups take the owner's explicit source font, not a first run.
	empty := p.nodes[projectedID(t, p, "empty")]
	box, err := layout.BuildLayoutTree(empty, layout.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(box.Children) != 1 || box.Children[0].Box != layout.BoxAnonymousInline || box.Style.FontSizePx != 24 || box.Children[0].Style.FontSizePx != 24 {
		t.Fatalf("empty owner context: %+v", box)
	}
	q := projection(t, fixture(t, "empty.html"), LoadOptions{})
	if q.root == nil || q.root.Style.FontSizePx != 16 || q.root.Style.Width.Kind != layout.LenAuto {
		t.Fatal("empty body lost defaults")
	}
	decorated := projection(t, fixture(t, "residual-inline.html"), LoadOptions{})
	s := decorated.sources[projectedID(t, decorated, "emptied")]
	if s.constraints.Padding.Left.Value != 16 || s.constraints.Border.Left.Value != 1 || len(decorated.nodes[s.ref.nodeID].ChildrenNodes) == 0 {
		t.Fatal("decorated ancestor or child lost before routing")
	}
}

func TestProjectionNestedAtomicsAndOwnerContext(t *testing.T) {
	p := projection(t, `<div id="owner" style="font-size:20px"><span style="font-size:30px">large</span><span id="outer" style="display:inline-block;width:50%;padding:2%;margin-left:1em"><span id="inner" style="display:inline-block;width:auto">inside</span></span></div>`, LoadOptions{})
	outer := p.nodes[projectedID(t, p, "outer")]
	inner := p.nodes[projectedID(t, p, "inner")]
	if outer.Style.Width != (layout.Length{Kind: layout.LenPercent, Value: .5}) || outer.Style.Padding.Left != (layout.Length{Kind: layout.LenPercent, Value: .02}) || outer.Style.Margin.Left.Value != 20 || inner.Style.Width.Kind != layout.LenAuto || len(outer.ChildrenNodes) != 1 || outer.ChildrenNodes[0] != inner {
		t.Fatal("nested atomic boundary or unsized constraints changed")
	}
	// Build only this route-free contract example, with no used-value resolution.
	root, err := layout.BuildLayoutTree(p.nodes[projectedID(t, p, "owner")], layout.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if root.Style.FontSizePx != 20 || root.Children[0].Box != layout.BoxAnonymousInline || root.Children[0].Style.FontSizePx != 20 {
		t.Fatal("paragraph owner adopted the first 30px text run's font")
	}
}

func TestProjectionEffectiveProfileFailures(t *testing.T) {
	for _, src := range []string{
		`<table style="display:block"><tr><td>x</td></tr></table>`, `<ul style="display:block"><li style="display:block">x</li></ul>`,
		`<form style="display:block">x</form>`, `<svg></svg>`, `<x-custom>x</x-custom>`, `<br>`, `<hr>`, `<pre>x</pre>`,
		`<div hidden>x</div>`, `<body style="display:inline">x</body>`,
		`<span style="padding:1em">x</span>`, `<span style="margin-left:auto">x</span>`, `<span style="border:1px solid">x</span>`,
	} {
		p, err := loadProjection(strings.NewReader(src), LoadOptions{SourceName: "profile.html"})
		ds := projectionFailure(t, p, err)
		if ds[0].stage != "profile" || ds[0].source.path == "" {
			t.Fatalf("unlocated effective failure: %+v", ds)
		}
	}

	p, err := loadProjection(strings.NewReader(`<pre>x</pre>`), LoadOptions{})
	if ds := projectionFailure(t, p, err); ds[0].css.property != "white-space" || ds[0].css.value != "pre" {
		t.Fatalf("HTML text hint missing from effective diagnostic: %+v", ds)
	}
	for _, src := range []string{`<div style="width:1e100%">x</div>`, `<div style="width:1e100em">x</div>`, `<div style="width:1e-100%">x</div>`} {
		p, err := loadProjection(strings.NewReader(src), LoadOptions{})
		ds := projectionFailure(t, p, err)
		if ds[0].stage != "projection" || ds[0].css.property != "width" {
			t.Fatalf("range failure: %+v", ds)
		}
	}
	p, err = loadProjection(strings.NewReader(`<p lang="not_a_lang_!">x</p>`), LoadOptions{})
	ds := projectionFailure(t, p, err)
	var upstream *textstyle.Error
	if ds[0].stage != "source-style" || ds[0].css.property != "lang" || !errors.As(err, &upstream) {
		t.Fatalf("source cause lost: %+v %v", ds, err)
	}
	// Finite but tiny computed sizes fail at their actual source context.
	p, err = loadProjection(strings.NewReader(`<p style="font-size:0.0000001em">x</p>`), LoadOptions{})
	if ds := projectionFailure(t, p, err); ds[0].stage != "source-style" {
		t.Fatalf("invented font context during authored audit: %+v", ds)
	}
}

func TestProjectionDetachedAndIndependent(t *testing.T) {
	s, err := loadSource(strings.NewReader(`<div id="parent" style="font-size:20px"><p id="p" style="width:50%;color:red">café</p></div>`), LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	before := renderSource(t, s.source.document)
	p, err := projectSource(s)
	if err != nil {
		t.Fatal(err)
	}
	if renderSource(t, s.source.document) != before {
		t.Fatal("projection modified original DOM")
	}
	id := projectedID(t, p, "p")
	n := sourceByID(t, s.source, "p")
	n.node.Attr[0].Val = "changed"
	n.node.FirstChild.Data = "changed"
	s.nodes[n.ref.nodeID].Styles().Add("color", "blue")
	if p.sources[id].attributes[0].Val != "p" || p.sources[id].typography.Color != "red" || p.nodes[id].ChildrenNodes[0].HTML.Data != "café" {
		t.Fatal("snapshot aliases source")
	}
	p.nodes[id].Style.Width.Value = .9
	if p.sources[id].constraints.Width.Value != .5 {
		t.Fatal("render inputs alias source-style record")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			q, e := loadProjection(strings.NewReader(`<div style="width:75%;font-size:20px">same</div>`), LoadOptions{})
			if e != nil || q.root == p.root || q.root.Style.Width.Kind != layout.LenAuto {
				t.Errorf("concurrent independent projection: %v", e)
			}
		})
	}
	wg.Wait()
}

func TestProjectionDoesNotValidateRoutesYet(t *testing.T) {
	for _, src := range []string{fixture(t, "nested-flow_err.html"), `<html style="flow-into:root"><body style="flow-into:body"><span style="flow-into:inline">x</span></body></html>`} {
		projection(t, src, LoadOptions{})
	}
	p, err := loadProjection(strings.NewReader(fixture(t, "float_err.html")), LoadOptions{SourceName: "float_err.html"})
	ds := projectionFailure(t, p, err)
	if len(ds) != 1 || ds[0].code != "profile.value" || ds[0].css.property != "float" || ds[0].css.value != "right" || ds[0].css.line == 0 || ds[0].source.name != "float_err.html" {
		t.Fatalf("float contract: %+v", ds)
	}
}
