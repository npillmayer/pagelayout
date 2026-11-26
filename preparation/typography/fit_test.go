package typography

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

func fitRequest(width, height float32, continuation content.FitContinuation) FitRequest {
	return FitRequest{FitRequest: content.FitRequest{Width: width, Height: height, Continuation: continuation}, Breaker: paragraph.FirstFit}
}
func fitted(t *testing.T, p *content.PreparedSnapshot, s *content.PreparationSession, f *content.Fitter, r FitRequest) []FitAlternative {
	t.Helper()
	out, err := Fit(t.Context(), p, s, f, r)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFitRepeatedWidthsReusePreparationAndOccurrences(t *testing.T) {
	b, svc := testBackend(t)
	d := load(t, `<body style="hyphens:none;font-size:20px;color:#345678">alpha beta gamma delta epsilon zeta eta theta</body>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, err := content.NewFitter(p, s, d.Residual().Root)
	if err != nil {
		t.Fatal(err)
	}
	base, coverage := svc.created, svc.coverage
	var cursor content.FitContinuation
	var retained []*Fragment
	var end layout.ParagraphPosition
	seenIDs := map[content.FragmentID]bool{}
	for n := 0; n < 20; n++ {
		width := float32(80)
		if n%2 == 1 {
			width = 120
		}
		all := fitted(t, p, s, f, fitRequest(width, 20, cursor))
		if len(all) == 0 {
			t.Fatal("no fitting prefix", n)
		}
		a := all[0]
		fragment := a.Fragment()
		ids := fragment.Groups()
		if len(ids) != 1 || seenIDs[ids[0]] || !a.Progress() {
			t.Fatal("occurrence identity/progress")
		}
		seenIDs[ids[0]] = true
		g, _ := fragment.Group(ids[0])
		if paragraphPoint(g.Info.From) != end || len(g.Coverage) == 0 || g.Input.Paint[0].Color.R != 0x34 {
			t.Fatal("source/paint/continuation", g)
		}
		end = paragraphPoint(g.Info.To)
		retained = append(retained, fragment)
		cursor = a.Continuation()
		if a.Complete() {
			break
		}
	}
	input, _ := p.Input(p.Groups()[0].Group)
	if len(retained) < 2 || end.Offset != uint64(len(input.Text)) {
		t.Fatal("lost suffix", end, input.Text)
	}

	if svc.created != base || svc.coverage != coverage {
		t.Fatal("base shaping repeated")
	}
	first := retained[0]
	id := first.Groups()[0]
	old, _ := first.Group(id)
	copy, _ := first.Group(id)
	copy.Input.Paint[0].Color.R = 0
	copy.Coverage[0].Range.End = 999
	if again, _ := first.Group(id); !reflect.DeepEqual(again, old) {
		t.Fatal("mutable source inspection")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if len(old.Paragraph.Lines()) != 1 || first.Geometry()[first.Root()].Frame.W != 80 {
		t.Fatal("close changed fragment")
	}
	if got, err := Fit(t.Context(), p, s, f, fitRequest(100, 100, cursor)); got != nil || !errors.Is(err, content.ErrClosedSession) {
		t.Fatal(got, err)
	}
}

func TestFitManualContinuationAndEventMapping(t *testing.T) {
	b, svc := testBackend(t)
	d := load(t, `<body style="hyphens:manual;font-size:20px"><a href="#note" data-flow-anchor>co&shy;operate</a><aside id="note" style="flow-into:notes">Note.</aside></body>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, err := content.NewFitter(p, s, d.Residual().Root)
	if err != nil {
		t.Fatal(err)
	}
	first := fitted(t, p, s, f, fitRequest(40, 20, content.FitContinuation{}))
	if len(first) == 0 || first[0].Complete() {
		t.Fatal("manual did not split")
	}
	prefix := first[0].Fragment()
	g, _ := prefix.Group(prefix.Groups()[0])
	if len(g.Events) != 1 || !g.Events[0].HasCoverage || len(g.Coverage) == 0 {
		t.Fatal("anchor coverage", g.Events, g.Coverage)
	}
	replacements := 0
	for _, c := range g.Coverage {
		if c.Coverage.Kind == paragraph.ReplacementCoverage {
			replacements++
		}
	}
	if replacements == 0 {
		t.Fatal("manual replacement has no original-source coverage")
	}
	base := svc.created
	last := fitted(t, p, s, f, fitRequest(120, 100, first[0].Continuation()))[0]
	if !last.Complete() {
		t.Fatal("manual suffix incomplete")
	}
	suffix, _ := last.Fragment().Group(last.Fragment().Groups()[0])
	if len(suffix.Events) != 0 || paragraphPoint(suffix.Info.From) != paragraphPoint(g.Info.To) {
		t.Fatal("anchor replay or wrong carry")
	}
	if svc.created != base {
		t.Fatal("accepted manual discovery repeated")
	}
	if g.Info.To.Offset == suffix.Info.To.Offset || strings.Contains(g.Input.Text, "<") {
		t.Fatal("source coordinates")
	}
}

func TestFitAtomicInteriorsStayPairedAcrossWidths(t *testing.T) {
	b, svc := testBackend(t)
	d := load(t, `<body style="hyphens:none;font-size:20px"><span style="display:inline-block;width:50%;padding:1px">One</span> <span style="display:inline-block;width:50%;padding:1px">Two</span></body>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, err := content.NewFitter(p, s, d.Residual().Root)
	if err != nil {
		t.Fatal(err)
	}
	base := svc.created
	first := fitted(t, p, s, f, fitRequest(100, 30, content.FitContinuation{}))
	if len(first) == 0 || first[0].Complete() {
		t.Fatal("atomics did not split")
	}
	before := first[0].Fragment().Geometry()
	outer, _ := p.Input(p.Groups()[0].Group)
	firstBox, secondBox := outer.Atomics[0], outer.Atomics[1]
	if before[firstBox].Content.W != 50 {
		t.Fatal("first interior", before)
	}
	if _, ok := before[secondBox]; ok {
		t.Fatal("unconsumed atomic published")
	}
	last := fitted(t, p, s, f, fitRequest(80, 100, first[0].Continuation()))[0]
	if !last.Complete() || last.Fragment().Geometry()[secondBox].Content.W != 40 || len(last.Fragment().Groups()) != 2 {
		t.Fatal("suffix interior association", last.Fragment().Geometry(), last.Fragment().Groups())
	}
	if _, ok := last.Fragment().Geometry()[firstBox]; ok {
		t.Fatal("consumed interior replay")
	}
	if !reflect.DeepEqual(before, first[0].Fragment().Geometry()) || svc.created != base {
		t.Fatal("prefix changed/reprepared")
	}
}

func TestFitFractionalWidthAndEmptyGroup(t *testing.T) {
	for _, source := range []string{`<body style="hyphens:none;font-size:20px">a b c</body>`, `<body style="hyphens:none"><p><a data-flow-anchor href="#n"></a></p><aside id="n" style="flow-into:notes">Note</aside></body>`} {
		b, _ := testBackend(t)
		d := load(t, source)
		p, s, err := content.Prepare(d, b)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		f, err := content.NewFitter(p, s, d.Residual().Root)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range []float32{41.1, 73.3, 90.7} {
			all := fitted(t, p, s, f, fitRequest(w, 100, content.FitContinuation{}))
			if len(all) == 0 || !all[0].Complete() {
				t.Fatal("fractional width did not fit", w)
			}
		}
	}
}

func TestFitFailuresRetryAndForeignSessions(t *testing.T) {
	b, svc := testBackend(t)
	d := load(t, `<body style="hyphens:none;font-size:20px">alpha beta gamma delta</body>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, err := content.NewFitter(p, s, d.Residual().Root)
	if err != nil {
		t.Fatal(err)
	}
	old := fitted(t, p, s, f, fitRequest(80, 20, content.FitContinuation{}))[0]
	before := old.Fragment().Geometry()
	svc.face.metricError = errors.New("late metric failure")
	svc.face.failMetricAt = svc.face.metrics.Load() + 1
	if got, err := Fit(t.Context(), p, s, f, fitRequest(100, 100, old.Continuation())); got != nil || !errors.Is(err, svc.face.metricError) {
		t.Fatal("partial result", got, err)
	}
	svc.face.failMetricAt = 0
	if all := fitted(t, p, s, f, fitRequest(100, 100, old.Continuation())); len(all) == 0 || !all[0].Complete() {
		t.Fatal("retry failed")
	}
	if !reflect.DeepEqual(before, old.Fragment().Geometry()) {
		t.Fatal("retry changed accepted output")
	}
	other, session, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got, err := Fit(t.Context(), other, session, f, fitRequest(80, 100, old.Continuation())); got != nil || !errors.Is(err, content.ErrInvalidPreparation) {
		t.Fatal(got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := Fit(ctx, p, s, f, fitRequest(80, 100, old.Continuation())); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
}

func TestFitNestedAutoAtomicsAndCoreFailure(t *testing.T) {
	b, svc := testBackend(t)
	d := fixture(t, "nested.html")
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, err := content.NewFitter(p, s, d.Residual().Root)
	if err != nil {
		t.Fatal(err)
	}
	base := svc.created
	for _, width := range []float32{120, 240} {
		all := fitted(t, p, s, f, fitRequest(width, 500, content.FitContinuation{}))
		if len(all) == 0 || !all[0].Complete() || len(all[0].Fragment().Groups()) != 3 {
			t.Fatal("nested atomics", all)
		}
		for _, id := range all[0].Fragment().Groups() {
			g, _ := all[0].Fragment().Group(id)
			if g.Input.ParentAtomic.Valid() && g.Info.Status != paragraph.FitComplete {
				t.Fatal("atomic interior fragmented")
			}
		}
	}
	if svc.created != base {
		t.Fatal("auto sizing repeated preparation")
	}
	r := fitRequest(120, 500, content.FitContinuation{})
	r.MaxSteps = 2
	if all, err := Fit(t.Context(), p, s, f, r); all != nil || !errors.Is(err, layout.ErrFitLimit) {
		t.Fatal("partial limit bundle", all, err)
	}
	if all := fitted(t, p, s, f, fitRequest(120, 500, content.FitContinuation{})); !all[0].Complete() {
		t.Fatal("limit poisoned retry")
	}
}

func TestFitManualFailureRetry(t *testing.T) {
	b, svc := testBackend(t)
	d := load(t, `<body style="hyphens:manual;font-size:20px">co&shy;operate</body>`)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, err := content.NewFitter(p, s, d.Residual().Root)
	if err != nil {
		t.Fatal(err)
	}
	first := fitted(t, p, s, f, fitRequest(40, 20, content.FitContinuation{}))[0]
	svc.face.metricError = svc.cause
	svc.face.failMetricAt = svc.face.metrics.Load() + 1
	if all, err := Fit(t.Context(), p, s, f, fitRequest(120, 100, first.Continuation())); all != nil || !errors.Is(err, svc.cause) {
		t.Fatal("manual failure publication", all, err)
	}
	svc.face.failMetricAt = 0
	last := fitted(t, p, s, f, fitRequest(120, 100, first.Continuation()))[0]
	if !last.Complete() {
		t.Fatal("manual retry did not resume")
	}

}

func TestFitEmptyAnchorAffinities(t *testing.T) {
	for _, tc := range []struct {
		source   string
		affinity content.EventAffinity
		line     int
	}{
		{`alpha <a data-flow-anchor href="#n"></a> beta`, content.AffinityFollowing, 1},
		{`alpha beta<a data-flow-anchor href="#n"></a>`, content.AffinityPreceding, 1},
		{`<a data-flow-anchor href="#n"></a>`, content.AffinityStructural, 0},
	} {
		b, _ := testBackend(t)
		d := load(t, `<body style="font-size:20px;hyphens:none">`+tc.source+`<aside id="n" style="flow-into:notes">Note</aside></body>`)
		p, s, err := content.Prepare(d, b)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		f, err := content.NewFitter(p, s, d.Residual().Root)
		if err != nil {
			t.Fatal(err)
		}
		var cursor content.FitContinuation
		eventCount := 0
		for step := 0; step < 3; step++ {
			all := fitted(t, p, s, f, fitRequest(50, 20, cursor))
			if len(all) == 0 {
				t.Fatal("no fit", tc.source)
			}
			chosen := all[0]
			g, _ := chosen.Fragment().Group(chosen.Fragment().Groups()[0])
			for _, e := range g.Events {
				if step != tc.line || e.Event.Affinity != tc.affinity || e.HasCoverage == (tc.affinity == content.AffinityStructural) {
					t.Fatal("wrong affinity", step, e)
				}
				eventCount++
			}
			cursor = chosen.Continuation()
			if chosen.Complete() {
				break
			}
		}
		if eventCount != 1 {
			t.Fatal("empty anchor lost/replayed", tc, eventCount)
		}
	}
}
