package template

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
)

func TestCompilePreservesGeometryCauses(t *testing.T) {
	for _, tc := range []struct {
		code  diagnosticCode
		cause error
		edit  func(*frameInput)
	}{
		{codeGeometryContent, layout.ErrFixedContentDependency, func(f *frameInput) { f.constraints.Width = layout.Length{Kind: layout.LenAuto} }},
		{codeGeometryPosition, layout.ErrUnsupportedPosition, func(f *frameInput) { f.constraints.Fixed.Insets.Left = layout.Length{Kind: layout.LenAuto} }},
		{codeGeometryResolve, layout.ErrInvalidFixedGeometry, func(f *frameInput) { f.constraints.Width.Value = float32(math.NaN()) }},
	} {
		// Exercise the compiler's upstream failure mapping without changing the
		// stricter template authoring profile or injecting a public geometry engine.
		v := validatedHTML(t, profileHTML("", "", ""))
		tc.edit(&v.templates[0].frames[0])
		c, err := compileTemplates(v)
		if c != nil || !errors.Is(err, tc.cause) {
			t.Fatalf("partial result/lost cause: %v", err)
		}
		ds := errorDiagnostics(t, err)
		if len(ds) != 1 || ds[0].stage != stageGeometry || ds[0].code != tc.code || ds[0].source != v.templates[0].frames[0].source {
			t.Fatalf("bad geometry diagnostic: %v", err)
		}
	}
}

func TestCompiledDataDetachedFromInputs(t *testing.T) {
	v := validatedHTML(t, profileHTML("", "", `padding:3px;border:2px solid`))
	c, err := compileTemplates(v)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := c.Lookup("a")
	before, p := a.Frames(), a.Page()
	v.templates[0].id = "changed"
	v.templates[0].page.width = 999
	v.templates[0].page.source.path = "changed"
	f := &v.templates[0].frames[0]
	f.flow = "changed"
	f.constraints.Fixed.Insets.Left.Value = 999
	f.paint.borders[0].style = "changed"
	f.source.path = "changed"
	v.templates = nil
	v.byID = nil
	if !reflect.DeepEqual(a.Frames(), before) || a.Page() != p || a.ID() != "a" || len(a.FramesForFlow("Main")) != 1 {
		t.Fatal("catalog retained mutable compilation input")
	}
}

func TestGeometryPrecisionPolicy(t *testing.T) {
	for _, size := range []float32{.3, 100, 40000} {
		page := Rect{W: size, H: size}
		tol := geometryTolerance(page)
		if !containsRect(page, Rect{X: float32(-tol / 2), W: size / 2, H: size / 2}, tol) || containsRect(page, Rect{X: float32(-2 * tol), W: size / 2, H: size / 2}, tol) {
			t.Fatalf("bounds tolerance at %g: %g", size, tol)
		}
		a := Rect{W: size / 2, H: size / 2}
		if overlaps(a, Rect{X: float32(float64(size/2) - tol/2), W: size / 2, H: size / 2}, tol) || !overlaps(a, Rect{X: float32(float64(size/2) - 2*tol), W: size / 2, H: size / 2}, tol) {
			t.Fatalf("overlap tolerance at %g: %g", size, tol)
		}
	}
	for _, size := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		v := validatedHTML(t, profileHTML("", "", ""))
		v.templates[0].page.width = size
		if c, err := compileTemplates(v); c != nil || err == nil || errorDiagnostics(t, err)[0].code != codeGeometryInvalid {
			t.Fatal("invalid viewport published")
		}
	}
}
