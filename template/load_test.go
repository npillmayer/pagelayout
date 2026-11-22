package template_test

import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/npillmayer/pagelayout/template"
)

func collection(pageCSS string, frames ...string) string {
	var body strings.Builder
	for _, css := range frames {
		fmt.Fprintf(&body, `<div style="position:fixed;left:0;top:0;width:10px;height:10px;flow-from:Main;%s"></div>`, css)
	}
	return fmt.Sprintf(`<body><template id="a" data-type="page"><div style="width:300px;height:200px;%s">%s</div></template></body>`, pageCSS, body.String())
}

func load(t *testing.T, input string) *template.Catalog {
	t.Helper()
	c, err := template.Load(strings.NewReader(input), template.LoadOptions{SourceName: "pages.html"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func lookup(t *testing.T, c *template.Catalog, id string) *template.Template {
	t.Helper()
	v, ok := c.Lookup(id)
	if !ok || v == nil {
		t.Fatalf("missing template %q", id)
	}
	return v
}

func TestLoadGeometryAndPaint(t *testing.T) {
	c := load(t, collection("background:#fff", `left:auto;right:20px;top:10px;width:80px;height:30px;padding:3px;border:2px solid red;background:rgba(0,0,255,.5)`))
	a := lookup(t, c, "a")
	p := a.Page()
	if a.ID() != "a" || p.Width != 300 || p.Height != 200 || p.Paint.Background != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("page: %+v", p)
	}
	f := a.Frames()[0]
	if f.BorderBox != (template.Rect{X: 190, Y: 10, W: 90, H: 40}) || f.ContentBox != (template.Rect{X: 195, Y: 15, W: 80, H: 30}) {
		t.Fatalf("wrong coordinates: %+v", f)
	}
	if f.ID != 1 || f.Flow != "Main" || f.Margin != (template.Edges{}) || f.Padding != (template.Edges{Top: 3, Right: 3, Bottom: 3, Left: 3}) || f.Border != (template.Edges{Top: 2, Right: 2, Bottom: 2, Left: 2}) {
		t.Fatalf("wrong edges/identity: %+v", f)
	}
	if f.Paint.Borders.Left.Style != template.BorderSolid || f.Paint.Borders.Left.Color != (color.NRGBA{R: 255, A: 255}) || f.Paint.Background != (color.NRGBA{B: 255, A: 128}) {
		t.Fatalf("paint: %+v", f.Paint)
	}
	if p.Source.Name != "pages.html" || p.Source.TemplateID != "a" || f.Source.Path != "/html[1]/body[1]/template[1]/div[1]/div[1]" || f.Source.NodeID <= p.Source.NodeID {
		t.Fatalf("source: %+v %+v", p.Source, f.Source)
	}
	stretch := load(t, collection("", `left:190px;right:20px;top:10px;bottom:150px;width:auto;height:auto;padding:3px;border:2px solid red`))
	sf := lookup(t, stretch, "a").Frames()[0]
	if sf.BorderBox != f.BorderBox || sf.ContentBox != f.ContentBox {
		t.Fatal("equivalent stretch changed geometry")
	}
}

func TestCatalogFrameAndFlowOrder(t *testing.T) {
	input := collection("", `left:200px;flow-from:Main`, `left:0;flow-from:main`, `left:100px;flow-from:\4d ain`)
	input += `<template id="Blank" data-type="page"><div style="width:20px;height:30px"></div></template>`
	c := load(t, input)
	if !reflect.DeepEqual(c.IDs(), []string{"a", "Blank"}) {
		t.Fatal(c.IDs())
	}
	if v, ok := c.Lookup("blank"); ok || v != nil {
		t.Fatal("lookup ignored case")
	}
	a := lookup(t, c, "a")
	frames := a.Frames()
	for i, x := range []float32{200, 0, 100} {
		if frames[i].ID != template.FrameID(i+1) || frames[i].BorderBox.X != x {
			t.Fatal("visual order replaced source order")
		}
	}
	main := a.FramesForFlow("Main")
	if len(main) != 2 || main[0].ID != 1 || main[1].ID != 3 || len(a.FramesForFlow("main")) != 1 || len(a.FramesForFlow("MAIN")) != 0 || len(a.FramesForFlow("")) != 0 {
		t.Fatal("flow chains lost source order or name case")
	}
	blank := lookup(t, c, "Blank")
	if len(blank.Frames()) != 0 || len(blank.FramesForFlow("Main")) != 0 || blank.Page().Width != 20 {
		t.Fatal("blank page acquired receivers")
	}
}

func TestCatalogCopiesAndLifetime(t *testing.T) {
	c := load(t, collection("background:red", `border:1px solid blue`))
	a := lookup(t, c, "a")
	page, frames := a.Page(), a.Frames()
	ids := c.IDs()
	ids[0] = "changed"
	changed := a.Frames()
	changed[0].ID = 99
	changed[0].Flow = "changed"
	changed[0].Source.Path = "changed"
	changed[0].BorderBox.X = 999
	changed[0].Border.Left = 999
	changed[0].Paint.Borders.Left.Color = color.NRGBA{}
	chain := a.FramesForFlow("Main")
	chain[0] = template.FrameInfo{}
	p := a.Page()
	p.Width = 999
	p.Paint.Background = color.NRGBA{}
	p.Source.Name = "changed"
	if !reflect.DeepEqual(a.Frames(), frames) || a.Page() != page || !reflect.DeepEqual(a.FramesForFlow("Main"), frames) || c.IDs()[0] != "a" {
		t.Fatal("accessor returned mutable catalog storage")
	}
	// Private fields alone would not protect a pointer stored directly in a map.
	*a = template.Template{}
	a = lookup(t, c, "a")
	if a.Page() != page || !reflect.DeepEqual(a.Frames(), frames) {
		t.Fatal("overwriting a lookup handle changed the catalog")
	}
	load(t, collection("width:400px", `width:30px`))
	if bad, err := template.Load(strings.NewReader(collection("", `left:999px`)), template.LoadOptions{}); bad != nil || err == nil {
		t.Fatal("expected failed replacement")
	}
	if a.Page() != page || !reflect.DeepEqual(a.Frames(), frames) {
		t.Fatal("later loads changed earlier results")
	}
	copy := *c
	*c = template.Catalog{}
	if a.Page() != page || lookup(t, &copy, "a").Page() != page || len(c.IDs()) != 0 {
		t.Fatal("catalog handles do not own independent handles to immutable data")
	}
}

func TestCatalogConcurrentInspection(t *testing.T) {
	c := load(t, collection("", `left:30px`, `left:0`))
	before := lookup(t, c, "a").Frames()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				a, ok := c.Lookup("a")
				if !ok {
					t.Error("lost lookup")
					return
				}
				if !reflect.DeepEqual(a.Frames(), before) {
					t.Error("concurrent read changed frames")
				}
				a.FramesForFlow("Main")[0] = template.FrameInfo{}
				c.IDs()[0] = "changed"
				*a = template.Template{}
			}
		}()
	}
	wg.Wait()
}

func TestPublicLoadFailures(t *testing.T) {
	for _, tc := range []struct{ name, input, stage, code string }{
		{"empty", "", "discovery", "collection.empty"},
		{"nested", `<body><template id="a" data-type="page"><div><div><div></div></div></div></template></body>`, "structure", "frame.content"},
		{"CSS syntax", collection("", `color:red;broken`), "css", "css.declaration-syntax"},
		{"CSS profile", collection("", `transform:none`), "profile", "profile.property"},
		{"static receiver", collection("", `position:static`), "profile", "profile.box"},
		{"zero stretch", collection("", `left:300px;right:0;width:auto`), "geometry", "geometry.receiving-size"},
		{"negative stretch", collection("", `top:150px;bottom:100px;height:auto`), "geometry", "geometry.receiving-size"},
		{"bounds", collection("", `left:295px`), "geometry", "geometry.bounds"},
		{"border outside", collection("", `left:-1px;padding:3px;border:2px solid`), "geometry", "geometry.bounds"},
		{"overlap", collection("", `left:10px`, `left:15px;flow-from:Other`), "geometry", "geometry.overlap"},
		{"edges overlap", collection("", `left:10px;padding:10px`, `left:35px;padding:10px`), "geometry", "geometry.overlap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := template.Load(strings.NewReader(tc.input), template.LoadOptions{SourceName: "bad.html"})
			var failure *template.LoadError
			if c != nil || !errors.As(err, &failure) {
				t.Fatalf("partial result or unstructured error: %v", err)
			}
			found := false
			for _, d := range failure.Diagnostics() {
				found = found || d.Stage == tc.stage && d.Code == tc.code
				if d.Source.Name != "bad.html" {
					t.Fatal("source label lost")
				}
			}
			if !found {
				t.Fatalf("missing %s/%s: %v", tc.stage, tc.code, err)
			}
		})
	}
}

func TestGeometryTouchingAndTolerance(t *testing.T) {
	for _, frames := range [][]string{
		{`left:0`, `left:10px`}, {`top:0`, `top:10px`}, {`left:0;top:0`, `left:10px;top:10px`},
		{`left:0;width:50px`, `left:49.99996px;width:50px`},
		{`left:-.00004px`}, {`left:90.00004px`},
	} {
		load(t, collection("width:100px;height:100px", frames...))
	}
	for _, frames := range [][]string{
		{`left:0;width:50px`, `left:49.999px;width:50px`},
		{`left:-.001px`}, {`left:90.001px`},
		{`width:.00002px;height:.00002px`, `width:.00002px;height:.00002px`},
		{`left:-.00002px;width:.00002px;height:.00002px`},
	} {
		if c, err := template.Load(strings.NewReader(collection("width:100px;height:100px", frames...)), template.LoadOptions{}); c != nil || err == nil {
			t.Fatal("meaningful overlap/overflow was accepted")
		}
	}
	// Source-unit rounding of decimal lengths is tolerated without snapping.
	load(t, collection("width:.3px;height:.3px", `left:.1px;top:0;width:.2px;height:.2px`))
	f := lookup(t, load(t, collection("width:100px;height:100px", `left:-.00004px`)), "a").Frames()[0]
	if f.BorderBox.X >= 0 {
		t.Fatal("tolerance changed published solver geometry")
	}
}

func TestLoadAtomicFailureAndDiagnosticCopies(t *testing.T) {
	first := collection("", `left:999px`, `left:998px`)
	second := strings.ReplaceAll(collection("", `left:0;right:0;top:200px;bottom:0;height:auto`), `id="a"`, `id="b"`)
	input := first + second
	var previous []template.Diagnostic
	for i := 0; i < 3; i++ {
		c, err := template.Load(strings.NewReader(input), template.LoadOptions{SourceName: "two.html"})
		var failure *template.LoadError
		if c != nil || !errors.As(err, &failure) {
			t.Fatal("invalid definition published")
		}
		ds := failure.Diagnostics()
		if len(ds) != 4 || ds[0].Code != "geometry.bounds" || ds[1].Code != "geometry.bounds" || ds[2].Code != "geometry.overlap" || ds[3].Code != "geometry.receiving-size" || ds[3].Source.TemplateID != "b" {
			t.Fatalf("wrong aggregation: %v", err)
		}
		if i > 0 && !reflect.DeepEqual(previous, ds) {
			t.Fatal("diagnostic order changed")
		}
		previous = ds
		for j := 1; j < len(ds); j++ {
			if ds[j].Source.NodeID < ds[j-1].Source.NodeID {
				t.Fatal("not in source order")
			}
		}
		text := err.Error()
		copy := failure.Diagnostics()
		copy[0].Message = "changed"
		copy[0].Source.Name = "changed"
		children := failure.Unwrap()
		children[0] = nil
		if err.Error() != text || !reflect.DeepEqual(ds, failure.Diagnostics()) {
			t.Fatal("diagnostics share mutable storage")
		}
		var d template.Diagnostic
		if !errors.As(err, &d) || d.Code != "geometry.bounds" {
			t.Fatal("diagnostic is not an error child")
		}
	}
}

type trackedReader struct {
	io.Reader
	reads  int
	closed bool
}

func (r *trackedReader) Read(p []byte) (int, error) { r.reads++; return r.Reader.Read(p) }
func (r *trackedReader) Close() error               { r.closed = true; return nil }

type failedReader struct{ err error }

func (r failedReader) Read([]byte) (int, error) { return 0, r.err }

func TestLoadReaderOwnershipAndErrors(t *testing.T) {
	r := &trackedReader{Reader: strings.NewReader(collection("", `left:10px`))}
	c, err := template.Load(r, template.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	reads := r.reads
	for i := 0; i < 10; i++ {
		a := lookup(t, c, "a")
		a.Page()
		a.Frames()
		a.FramesForFlow("Main")
	}
	if reads == 0 || r.reads != reads || r.closed {
		t.Fatal("selection reread input or Load closed it")
	}
	problem := errors.New("reader failed")
	bad, err := template.Load(failedReader{problem}, template.LoadOptions{SourceName: "reader"})
	if bad != nil || !errors.Is(err, problem) {
		t.Fatalf("original reader error lost: %v", err)
	}
	var d template.Diagnostic
	if !errors.As(err, &d) || d.Stage != "parse" || d.Source.Name != "reader" {
		t.Fatal("parse context lost")
	}
	if bad, err := template.Load(nil, template.LoadOptions{}); bad != nil || err == nil {
		t.Fatal("nil reader accepted")
	}
}

func TestLoadSketchAndZeroHandles(t *testing.T) {
	r, err := os.Open("testdata/templates/sketch.html")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	c, err := template.Load(r, template.LoadOptions{SourceName: "sketch.html"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.IDs()) != 2 || len(lookup(t, c, "two-column-article").FramesForFlow("article-flow")) != 2 || len(lookup(t, c, "chapter-opening").Frames()) != 2 {
		t.Fatal("sketch did not compile")
	}
	for _, c := range []*template.Catalog{nil, {}} {
		if len(c.IDs()) != 0 {
			t.Fatal("nonempty zero catalog")
		}
		if p, ok := c.Lookup("x"); ok || p != nil {
			t.Fatal("zero lookup succeeded")
		}
	}
	for _, p := range []*template.Template{nil, {}} {
		if p.ID() != "" || p.Page() != (template.PageInfo{}) || len(p.Frames()) != 0 || len(p.FramesForFlow("x")) != 0 {
			t.Fatal("nonempty zero template")
		}
	}
}
