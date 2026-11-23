package typography

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

type intrinsicView map[layout.BoxID]layout.IntrinsicAtomic

func (v intrinsicView) Lookup(id layout.BoxID) (layout.IntrinsicAtomic, bool) {
	a, ok := v[id]
	return a, ok
}

type edgesView map[layout.BoxID]layout.BoxEdges

func (v edgesView) Lookup(id layout.BoxID) (layout.BoxEdges, bool) { a, ok := v[id]; return a, ok }

func measureGroup(p *content.PreparedSnapshot, s *content.PreparationSession, r content.InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	var w layout.IntrinsicWidths
	err := s.WithGroup(p, r.Group, r.Owner, func(h content.GroupHandle) error {
		var err error
		w, err = (groupMeasurer{}).MeasureGroup(h, r)
		return err
	})
	return w, err
}

func TestMeasurementReusesPreparationAndConsumesRequests(t *testing.T) {
	b, services := testBackend(t)
	d := load(t, `<body style="font-size:20px;hyphens:none">éa <span style="display:inline-block;width:20px">x</span> z`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g := p.Groups()[0]
	a, _ := g.Atomics[0].Info()
	atoms := intrinsicView{a.ID: {BoxID: a.ID, BorderWidths: layout.IntrinsicWidths{MinContent: 10, MaxContent: 20}}}
	edges := edgesView{a.ID: {Margin: layout.Edges{Left: -2, Right: 5}, Padding: layout.Edges{Left: 100}, Border: layout.Edges{Right: 100}}}
	r := content.InlineMeasureRequest{Group: g.Group, Owner: g.Owner, Atomics: atoms, Edges: edges}
	baseCreated, baseCoverage, baseScales := services.created, services.coverage, services.face.scales.Load()
	input, _ := p.Input(g.Group)
	first, err := measureGroup(p, s, r)
	if err != nil || first != (layout.IntrinsicWidths{MinContent: 20, MaxContent: 73}) {
		t.Fatal(first, err)
	}
	for range 2 {
		if got, err := measureGroup(p, s, r); err != nil || got != first {
			t.Fatal(got, err)
		}
	}
	atoms[a.ID] = layout.IntrinsicAtomic{BoxID: a.ID, BorderWidths: layout.IntrinsicWidths{MinContent: 30, MaxContent: 40}}
	if got, err := measureGroup(p, s, r); err != nil || got != (layout.IntrinsicWidths{MinContent: 33, MaxContent: 93}) {
		t.Fatal("cached answer", got, err)
	}
	if services.created != baseCreated || services.coverage != baseCoverage || services.face.scales.Load() != baseScales || services.face.metrics.Load() != 0 || services.hyphens != 0 {
		t.Fatal("measurement repeated base preparation or used vertical services")
	}
	// Mutable request storage is never part of the retained source or old answer.
	delete(atoms, a.ID)
	delete(edges, a.ID)
	after, _ := p.Input(g.Group)
	if !reflect.DeepEqual(input, after) || first != (layout.IntrinsicWidths{MinContent: 20, MaxContent: 73}) {
		t.Fatal("retained request storage")
	}
	_ = s.Close()
	if first.MaxContent != 73 {
		t.Fatal("closed session invalidated widths")
	}
}

func TestMeasureIntrinsicNestedAndRootConstraints(t *testing.T) {
	for _, tc := range []struct {
		html string
		want layout.IntrinsicWidths
	}{
		{`<body style="font-size:20px;hyphens:none">one two`, layout.IntrinsicWidths{MinContent: 30, MaxContent: 70}},
		{`<body style="hyphens:none"><p></p>`, layout.IntrinsicWidths{}},
		{`<body style="hyphens:none"><span style="display:inline-block;width:0"></span>`, layout.IntrinsicWidths{}},
		{`<body style="font-size:20px;hyphens:none"><span style="display:inline-block;padding:2px;border:1px solid;margin-left:-2px;margin-right:5px">aa <span style="display:inline-block;padding:1px;border:2px solid;margin-left:2px;margin-right:2px">b b</span></span>`, layout.IntrinsicWidths{MinContent: 29, MaxContent: 79}},
	} {
		b, services := testBackend(t)
		d := load(t, tc.html)
		p, s, err := content.Prepare(d, b)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		manifest := p.Groups()
		created, coverage := services.created, services.coverage
		for _, width := range []float32{200, 40, 0} {
			got, err := MeasureIntrinsic(p, s, d.Residual().Root, layout.ResolveContext{ContainingBlock: layout.Rect{W: width}})
			if err != nil || got != tc.want {
				t.Fatalf("%s: got %v, %v; want %v", tc.html, got, err, tc.want)
			}
		}
		if services.created != created || services.coverage != coverage || services.face.metrics.Load() != 0 || !reflect.DeepEqual(p.Groups(), manifest) {
			t.Fatal("source/preparation changed")
		}
	}
	// One covered box can be measured against multiple external percentage bases.
	b, services := testBackend(t)
	d := load(t, `<body style="hyphens:none;width:50%;padding-left:10%;margin-left:20px">text`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := services.created
	for _, width := range []float32{100, 200} {
		got, err := MeasureIntrinsic(p, s, d.Residual().Root, layout.ResolveContext{ContainingBlock: layout.Rect{W: width}})
		if err != nil || got != (layout.IntrinsicWidths{MinContent: width/2 + width/10, MaxContent: width/2 + width/10}) || services.created != base {
			t.Fatal(got, err)
		}
	}
}

func TestMeasurementPolicyFailureAndRetry(t *testing.T) {
	b, services := testBackend(t)
	d := load(t, `<body style="hyphens:none;font-size:20px"><p>valid</p><p style="hyphens:manual">co&shy;operate</p><p style="hyphens:manual">co&shy;oper&shy;ate</p></body>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	groups := p.Groups()
	first, err := MeasureIntrinsic(p, s, groups[0].Owner, layout.ResolveContext{})
	if err != nil {
		t.Fatal(err)
	}
	baseShapes := append([]shapeCall(nil), services.shapes...)
	services.failFactory = services.created + 1
	got, err := MeasureIntrinsic(p, s, groups[1].Owner, layout.ResolveContext{})
	var detail *content.PreparationError
	var upstream *paragraph.Error
	if got != (layout.IntrinsicWidths{}) || !errors.Is(err, services.cause) || !errors.As(err, &detail) || detail.Stage != content.StageMeasurement || detail.Group != groups[1].Group || !errors.As(err, &upstream) || upstream.Stage != paragraph.StageMeasure {
		t.Fatal("lazy failure lost source/cause/stage", got, err)
	}
	services.failFactory = 0
	manual, err := MeasureIntrinsic(p, s, groups[1].Owner, layout.ResolveContext{})
	if err != nil || manual.MinContent <= 0 || manual.MaxContent < manual.MinContent {
		t.Fatal("retry", manual, err)
	}
	created, hyphens := services.created, services.hyphens
	if got, err := MeasureIntrinsic(p, s, groups[1].Owner, layout.ResolveContext{}); err != nil || got != manual || services.created != created || services.hyphens != hyphens {
		t.Fatal("complete discoveries not reused", got, err)
	}
	for range 2 {
		// Earlier successful groups must not yield a partial whole-tree answer.
		got, err := MeasureIntrinsic(p, s, d.Residual().Root, layout.ResolveContext{})
		if got != (layout.IntrinsicWidths{}) || !errors.Is(err, khipu.ErrUnsupportedPolicy) || !errors.As(err, &detail) || detail.Group != groups[2].Group {
			t.Fatal(got, err)
		}
	}
	if got, err := MeasureIntrinsic(p, s, groups[0].Owner, layout.ResolveContext{}); err != nil || got != first {
		t.Fatal("failure poisoned session", got, err)
	}
	if !reflect.DeepEqual(services.shapes[:len(baseShapes)], baseShapes) || services.face.metrics.Load() != 0 {
		t.Fatal("base material/metrics changed")
	}
	for _, shape := range services.shapes[len(baseShapes):] {
		if shape.text == "valid" || shape.text == "co\u00adoperate" || shape.text == "co\u00adoper\u00adate" {
			t.Fatal("base text reshaped", shape)
		}
	}
	checkFinished(t, services)
}

func TestMeasurementPercentageFailureKeepsOtherTrees(t *testing.T) {
	b, services := testBackend(t)
	d := load(t, `<body style="hyphens:none;font-size:20px"><p style="flow-into:main">good</p><p style="flow-into:main"><span style="display:inline-block"><span style="display:block;width:50%">cycle</span></span></p></body>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	branches := d.Branches("main")
	oldBank := d.Bank()
	base := services.created
	first, err := MeasureIntrinsic(p, s, branches[0].Root, layout.ResolveContext{})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if got, err := MeasureIntrinsic(p, s, branches[1].Root, layout.ResolveContext{ContainingBlock: layout.Rect{W: 200}}); err == nil || got != (layout.IntrinsicWidths{}) {
			t.Fatal("percentage cycle accepted", got, err)
		}
	}
	got, err := MeasureIntrinsic(p, s, branches[0].Root, layout.ResolveContext{})
	if err != nil || got != first || services.created != base || !reflect.DeepEqual(oldBank, d.Bank()) {
		t.Fatal("failure changed other tree or bank", got, err)
	}
}

func TestAtomicMeasurementConversionAndValidation(t *testing.T) {
	b, _ := testBackend(t)
	p, s, err := content.Prepare(load(t, `<body style="hyphens:none"><span style="display:inline-block"></span><span style="display:inline-block"></span>`), b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g, _ := p.Input(p.Groups()[0].Group)
	a, _ := g.Atomics[0].Info()
	z, _ := g.Atomics[1].Info()
	atoms := intrinsicView{a.ID: {BoxID: a.ID, BorderWidths: layout.IntrinsicWidths{MinContent: 1.0 / 32768, MaxContent: 1.0 / 32768}}, z.ID: {BoxID: z.ID, BorderWidths: layout.IntrinsicWidths{MaxContent: 2}}}
	edges := edgesView{a.ID: {Margin: layout.Edges{Left: 1.0 / 32768, Right: 1.0 / 32768}, Padding: layout.Edges{Left: 100}}, z.ID: {Margin: layout.Edges{Left: -5, Right: 1}}}
	r := content.InlineMeasureRequest{Group: g.Group, Owner: g.Owner, Atomics: atoms, Edges: edges}
	got, err := atomicIntrinsics(g, r)
	want := []khipu.AtomicIntrinsic{{ID: 1, MinAdvance: 6, MaxAdvance: 6}, {ID: 2, MinAdvance: -4 * 49152, MaxAdvance: -2 * 49152}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("ordered IDs/rounding/signed advances", got, err)
	}
	for _, tc := range []struct {
		border, left, right float32
		valid               bool
	}{
		{30000, 30000, -30000, true}, {30000, 30000, 0, false}, {0, -30000, -30000, false},
	} {
		atoms[a.ID] = layout.IntrinsicAtomic{BoxID: a.ID, BorderWidths: layout.IntrinsicWidths{MinContent: tc.border, MaxContent: tc.border}}
		edges[a.ID] = layout.BoxEdges{Margin: layout.Edges{Left: tc.left, Right: tc.right}}
		got, err := atomicIntrinsics(g, r)
		if tc.valid {
			if err != nil || got[0].MinAdvance != 30000*49152 {
				t.Fatal(got, err)
			}
		} else if got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("invalid advance", got, err)
		}
	}
	for _, bounds := range []layout.IntrinsicWidths{{MinContent: -1e-7}, {MinContent: 1e-7}, {MinContent: float32(math.NaN())}, {MaxContent: float32(math.Inf(1))}, {MaxContent: float32(float64(dimen.Fil) / duPerCSSPixel)}} {
		atoms[a.ID] = layout.IntrinsicAtomic{BoxID: a.ID, BorderWidths: bounds}
		if got, err := measureGroup(p, s, r); got != (layout.IntrinsicWidths{}) || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("invalid before quantization", got, err)
		}
	}
	atoms[a.ID] = layout.IntrinsicAtomic{BoxID: a.ID, BorderWidths: layout.IntrinsicWidths{MinContent: 10, MaxContent: 20}}
	edges[a.ID] = layout.BoxEdges{}
	for _, mutate := range []func(*content.InlineMeasureRequest){
		func(r *content.InlineMeasureRequest) { r.Edges = nil }, func(r *content.InlineMeasureRequest) { r.Atomics = nil },
		func(r *content.InlineMeasureRequest) { r.Edges = edgesView{} }, func(r *content.InlineMeasureRequest) { r.Atomics = intrinsicView{} },
		func(r *content.InlineMeasureRequest) { r.Atomics = intrinsicView{a.ID: {BoxID: a.ID + 100}} },
		func(r *content.InlineMeasureRequest) {
			r.Edges = edgesView{a.ID: {Margin: layout.Edges{Left: float32(math.NaN())}}}
		},
	} {
		bad := r
		mutate(&bad)
		if got, err := measureGroup(p, s, bad); got != (layout.IntrinsicWidths{}) || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("invalid request", got, err)
		}
	}
	if _, err := measureGroup(p, s, r); err != nil {
		t.Fatal("failed request poisoned preparation", err)
	}
}

func TestIntrinsicSummaryRange(t *testing.T) {
	for _, w := range []khipu.IntrinsicWidths{{}, {MinContent: 1, MaxContent: 72 * dimen.BP}} {
		got, err := intrinsicSummary(w)
		if err != nil || got.MinContent != float32(float64(w.MinContent)/duPerCSSPixel) || got.MaxContent != float32(float64(w.MaxContent)/duPerCSSPixel) {
			t.Fatal(got, err)
		}
	}
	for _, w := range []khipu.IntrinsicWidths{{MinContent: -1}, {MinContent: 2, MaxContent: 1}, {MaxContent: dimen.Fil}, {MaxContent: dimen.Fil - 1}} {
		if got, err := intrinsicSummary(w); got != (layout.IntrinsicWidths{}) || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("nonfinite summary", got, err)
		}
	}
}

func TestMeasurementAcceptanceFixtures(t *testing.T) {
	for _, name := range []string{"text-only.html", "empty-atomic.html", "nested.html", "branches.html", "manual.html", "manual-many.html"} {
		t.Run(name, func(t *testing.T) {
			b, services := testBackend(t)
			d := fixture(t, name)
			p, s, err := content.Prepare(d, b)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			base := services.created
			for _, g := range p.Groups() {
				for range 2 {
					w, err := MeasureIntrinsic(p, s, g.Owner, layout.ResolveContext{})
					if name == "manual-many.html" {
						if w != (layout.IntrinsicWidths{}) || !errors.Is(err, khipu.ErrUnsupportedPolicy) {
							t.Fatal(w, err)
						}
					} else if err != nil || w.MinContent < 0 || w.MaxContent < w.MinContent {
						t.Fatal(w, err)
					}
				}
			}
			if name != "manual.html" && name != "manual-many.html" && services.created != base {
				t.Fatal("repeated base preparation")
			}
			if services.face.metrics.Load() != 0 {
				t.Fatal("measurement queried vertical metrics")
			}
		})
	}
}
