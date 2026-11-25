package typography

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

func TestLayoutTrialsReuseAndLifetime(t *testing.T) {
	b, services := testBackend(t)
	d := fixture(t, "nested.html")
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	manifest := p.Groups()
	created, coverage := services.created, services.coverage
	first, err := Layout(p, s, d.Residual().Root, LayoutRequest{Width: 120, Breaker: paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	saved := first.Geometry()
	outer := first.Groups()[0].Group
	old, _ := first.Group(outer)
	second, err := Layout(p, s, d.Residual().Root, LayoutRequest{Width: 240, Breaker: paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	newGroup, _ := second.Group(outer)
	secondGeometry := second.Geometry()
	if reflect.DeepEqual(saved, second.Geometry()) || len(old.Paragraph.VList.Lines()) == len(newGroup.Paragraph.VList.Lines()) {
		t.Fatal("width did not affect geometry/line count")
	}
	if services.created != created || services.coverage != coverage || !reflect.DeepEqual(manifest, p.Groups()) {
		t.Fatal("base preparation/source changed")
	}
	if _, err := Layout(p, s, d.Residual().Root, LayoutRequest{80, paragraph.FirstFit}); err != nil {
		t.Fatal(err)
	}
	secondAfter, _ := second.Group(outer)
	if !reflect.DeepEqual(secondGeometry, second.Geometry()) || !reflect.DeepEqual(secondAfter, newGroup) || services.created != created {
		t.Fatal("later trial changed second candidate")
	}
	// Mutating accessor envelopes must not change either retained result.
	geometry := first.Geometry()
	delete(geometry, first.Root())
	copyGroup, _ := first.Group(outer)
	copyGroup.Input.Text = "changed"
	copyGroup.Input.Paint[0].Color.A = 0
	copyGroup.Paragraph.BreakPositions[0] = 999
	groups := first.Groups()
	groups[0].Atomics[0] = content.Box{}
	got, _ := first.Group(outer)
	if got.Input.Text != old.Input.Text || !reflect.DeepEqual(got.Paragraph.BreakPositions, old.Paragraph.BreakPositions) || !reflect.DeepEqual(first.Geometry(), saved) {
		t.Fatal("result accessor aliases")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if got.Paragraph.Frozen.SourceLen() != uint64(len(got.Input.Text)) || !reflect.DeepEqual(first.Geometry(), saved) {
		t.Fatal("close invalidated output")
	}
	if result, err := Layout(p, s, d.Residual().Root, LayoutRequest{Width: 120, Breaker: paragraph.FirstFit}); result != nil || !errors.Is(err, content.ErrClosedSession) {
		t.Fatal(result, err)
	}
}

type failingLayout struct {
	*layoutInvocation
	group content.Box
	cause error
	calls []content.Box
}

func (f *failingLayout) LayoutGroup(h content.GroupHandle, r content.InlineLayoutRequest) (layout.InlineResult, error) {
	f.calls = append(f.calls, r.Group)
	if r.Group == f.group {
		return layout.InlineResult{}, f.cause
	}
	return f.layoutInvocation.LayoutGroup(h, r)
}

func TestLayoutLateFailureAndRetry(t *testing.T) {
	b, services := testBackend(t)
	d := fixture(t, "nested.html")
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	old, err := Layout(p, s, d.Residual().Root, LayoutRequest{120, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	saved := old.Geometry()
	base := services.created
	f := &failingLayout{layoutInvocation: &layoutInvocation{breaker: paragraph.FirstFit, groups: make(map[content.Box]GroupResult)}, group: p.Groups()[0].Group, cause: errors.New("late outer paragraph failure")}
	css, err := content.Layout(p, s, d.Residual().Root, 200, f)
	var detail *content.PreparationError
	if css != nil || !errors.Is(err, f.cause) || !errors.As(err, &detail) || detail.Group != f.group || detail.Stage != content.StageLayout || len(f.groups) != 2 {
		t.Fatal("late publication/source", css, err, len(f.groups))
	}
	retry, err := Layout(p, s, d.Residual().Root, LayoutRequest{200, paragraph.FirstFit})
	if err != nil || retry == nil || services.created != base || !reflect.DeepEqual(old.Geometry(), saved) {
		t.Fatal("retry changed prior output/preparation", err)
	}
}

func TestLayoutProfilesAndIdentity(t *testing.T) {
	for _, source := range []string{`<body style="hyphens:none"><p></p>`, `<body style="hyphens:none;font-size:20px">a<span style="display:inline-block;width:0"></span>b`, `<body style="hyphens:none;font-size:20px">a<span style="display:inline-block;width:20px;padding:2px;border:1px solid;margin-left:-2px;margin-right:3px">x</span>z`, `<body style="hyphens:none"><span dir="rtl">אבג</span> text`} {
		b, svc := testBackend(t)
		d := load(t, source)
		p, s, err := content.Prepare(d, b)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		base := svc.created
		for _, width := range []float32{0, 100, 200} {
			r, err := Layout(p, s, d.Residual().Root, LayoutRequest{width, paragraph.FirstFit})
			if err != nil {
				t.Fatal(source, width, err)
			}
			if len(r.Groups()) != len(p.Groups()) || svc.created != base {
				t.Fatal("coverage/reprepare")
			}
			for _, g := range r.Groups() {
				result, _ := r.Group(g.Group)
				if len(result.Paragraph.VList.AtomicPlacements()) != len(g.Atomics) {
					t.Fatal("atomic placements")
				}
			}
		}
	}
	b, svc := testBackend(t)
	d := fixture(t, "nested.html")
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	other, os, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Close()
	base := svc.created
	if r, err := Layout(other, s, d.Residual().Root, LayoutRequest{100, paragraph.FirstFit}); r != nil || !errors.Is(err, content.ErrInvalidPreparation) {
		t.Fatal(r, err)
	}
	for _, r := range []LayoutRequest{{-1, paragraph.FirstFit}, {float32(math.NaN()), paragraph.FirstFit}, {100, "unknown"}, {100, paragraph.KnuthPlass}} {
		if result, err := Layout(p, s, d.Residual().Root, r); err == nil || result != nil {
			t.Fatal(result, err)
		}
	}
	if svc.created != base {
		t.Fatal("invalid calls prepared")
	}
}

func TestLayoutMetricFailurePreservesPriorParagraphs(t *testing.T) {
	b, svc := testBackend(t)
	d := fixture(t, "nested.html")
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := Layout(p, s, d.Residual().Root, LayoutRequest{120, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	metrics := svc.face.metrics.Load()
	geometry := r.Geometry()
	groups := make([]GroupResult, 0)
	for _, g := range r.Groups() {
		v, _ := r.Group(g.Group)
		groups = append(groups, v)
	}
	base := svc.created
	svc.face.failMetricAt = 2 * metrics // Last metric query, in the outer paragraph.
	svc.face.metricError = errors.New("injected vertical metric failure")
	failed, err := Layout(p, s, d.Residual().Root, LayoutRequest{120, paragraph.FirstFit})
	var detail *content.PreparationError
	var upstream *paragraph.Error
	if failed != nil || !errors.Is(err, svc.face.metricError) || !errors.As(err, &detail) || detail.Group != p.Groups()[0].Group || !errors.As(err, &upstream) || upstream.Stage != paragraph.StageAssemble {
		t.Fatal("late assembly failure", failed, err)
	}
	svc.face.failMetricAt = 0
	retry, err := Layout(p, s, d.Residual().Root, LayoutRequest{240, paragraph.FirstFit})
	if err != nil || retry == nil || svc.created != base {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Geometry(), geometry) {
		t.Fatal("earlier geometry changed")
	}
	for i, g := range r.Groups() {
		got, _ := r.Group(g.Group)
		if !reflect.DeepEqual(got, groups[i]) {
			t.Fatal("earlier paragraph changed")
		}
	}
}

func TestLayoutAtomicGeometry(t *testing.T) {
	b, _ := testBackend(t)
	d := load(t, `<body style="font-size:20px;hyphens:none">a<span style="display:inline-block;width:20px;padding:2px;border:1px solid;margin-left:-2px;margin-right:3px;margin-top:2px;margin-bottom:3px">x</span>z`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := Layout(p, s, d.Residual().Root, LayoutRequest{100, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	g := r.Groups()[0]
	a := g.Atomics[0]
	geometry := r.Geometry()
	owner, _ := g.Owner.Info()
	if geometry[a].Frame != (layout.Rect{X: 8, Y: 2, W: 26, H: 26}) || geometry[a].Content != (layout.Rect{X: 11, Y: 5, W: 20, H: 20}) || geometry[a].RelativeTo != owner.ID || geometry[g.Owner].Content.H != 35 {
		t.Fatal("atomic padding/border/margin conversion", geometry[a], geometry[g.Owner])
	}
	group, _ := r.Group(g.Group)
	placed := group.Paragraph.VList.AtomicPlacements()
	if len(placed) != 1 || placed[0].X != 6 || placed[0].Y != 1.5 {
		t.Fatal("retained big-point geometry", placed)
	}
}

type usedView map[layout.BoxID]layout.UsedValues

func (v usedView) Lookup(id layout.BoxID) (layout.UsedValues, bool) { u, ok := v[id]; return u, ok }

type realizedView map[layout.BoxID]layout.RealizedAtomic

func (v realizedView) Lookup(id layout.BoxID) (layout.RealizedAtomic, bool) {
	a, ok := v[id]
	return a, ok
}

func TestLayoutAtomicRequestValidation(t *testing.T) {
	b, _ := testBackend(t)
	d := load(t, `<body style="hyphens:none"><span style="display:inline-block;width:1px">x</span>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g, _ := p.Input(p.Groups()[0].Group)
	info, _ := g.Atomics[0].Info()
	r := content.InlineLayoutRequest{Group: g.Group, Owner: g.Owner, Width: 100, Used: usedView{info.ID: {Margin: layout.Edges{Left: 1.0 / 32768, Top: -2}}}, Atomics: realizedView{info.ID: {BoxID: info.ID, BorderWidth: 1, BorderHeight: 2}}}
	a, err := atomicBoxes(g, r)
	if err != nil || a[0].BorderWidth != 49152 || a[0].BorderHeight != 98304 || a[0].Margin.Left != 2 || a[0].Margin.Top != -98304 {
		t.Fatal("primitive conversion", a, err)
	}
	for _, mutate := range []func(*content.InlineLayoutRequest){
		func(r *content.InlineLayoutRequest) { r.Used = nil }, func(r *content.InlineLayoutRequest) { r.Atomics = nil },
		func(r *content.InlineLayoutRequest) { r.Used = usedView{} }, func(r *content.InlineLayoutRequest) { r.Atomics = realizedView{} },
		func(r *content.InlineLayoutRequest) { r.Used = usedView{info.ID: {WidthPending: true}} },
		func(r *content.InlineLayoutRequest) { r.Used = usedView{info.ID: {EdgesPending: true}} },
		func(r *content.InlineLayoutRequest) { r.Atomics = realizedView{info.ID: {BoxID: info.ID + 1}} },
		func(r *content.InlineLayoutRequest) {
			r.Atomics = realizedView{info.ID: {BoxID: info.ID, BorderHeight: -.000001}}
		},
		func(r *content.InlineLayoutRequest) {
			r.Used = usedView{info.ID: {Margin: layout.Edges{Top: float32(math.Inf(1))}}}
		},
	} {
		bad := r
		mutate(&bad)
		i := &layoutInvocation{breaker: paragraph.FirstFit, groups: make(map[content.Box]GroupResult)}
		err := s.WithGroup(p, g.Group, g.Owner, func(h content.GroupHandle) error { _, err := i.LayoutGroup(h, bad); return err })
		if !errors.Is(err, contracts.ErrIllegalArguments) || len(i.groups) != 0 {
			t.Fatal("invalid geometry published", err)
		}
	}
	if _, err := Layout(p, s, d.Residual().Root, LayoutRequest{100, paragraph.FirstFit}); err != nil {
		t.Fatal("invalid request poisoned preparation", err)
	}
	for _, points := range []float64{math.NaN(), math.Inf(1), -1, float64(dimen.Fil) / float64(dimen.BP), float64(dimen.Fil-1) / float64(dimen.BP)} {
		if _, err := pointsToCSS(points, false); !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("invalid summary", points, err)
		}
	}
}

func TestLayoutMeasurementFailureAndBankIsolation(t *testing.T) {
	b, svc := testBackend(t)
	d := load(t, `<body style="hyphens:none;font-size:20px"><p style="flow-into:article">Good text.</p><p style="flow-into:article"><span style="display:inline-block"><span style="display:block;width:50%">cycle</span></span></p></body>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	branches := d.Branches("article")
	base := svc.created
	old, err := Layout(p, s, branches[0].Root, LayoutRequest{80, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	before := old.Geometry()
	if r, err := Layout(p, s, branches[1].Root, LayoutRequest{200, paragraph.FirstFit}); err == nil || r != nil {
		t.Fatal("percentage cycle accepted", r, err)
	}
	if _, err := Layout(p, s, branches[0].Root, LayoutRequest{200, paragraph.FirstFit}); err != nil {
		t.Fatal(err)
	}
	if svc.created != base || !reflect.DeepEqual(before, old.Geometry()) {
		t.Fatal("failure changed preparation/output")
	}
	bank := d.Bank()
	for _, branch := range branches {
		item, next, ok := bank.Take("article")
		if !ok || item != branch.Root {
			t.Fatal("trial changed bank")
		}
		bank = next
	}
	if len(old.Groups()) != len(d.Groups(branches[0].Tree)) {
		t.Fatal("foreign tree paragraphs")
	}
}
