package template_test

import (
	"bytes"
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/npillmayer/pagelayout/template"
)

func TestIntegrationSketchLoaders(t *testing.T) {
	data, err := exampleCollections.ReadFile("testdata/templates/sketch.html")
	if err != nil {
		t.Fatal(err)
	}
	config := t.TempDir()
	repo := filepath.Join(config, "templates")
	host := &collectionIO{configDir: config, roots: map[string]fs.FS{repo: os.DirFS("testdata/templates")}}
	for _, tc := range []struct {
		name, source string
		load         func() (*template.Catalog, error)
	}{
		{"reader", "sketch.html", func() (*template.Catalog, error) {
			return template.Load(bytes.NewReader(data), template.LoadOptions{SourceName: "sketch.html"})
		}},
		{"filesystem", "sketch.html", func() (*template.Catalog, error) {
			return template.LoadFS(os.DirFS("testdata/templates"), "sketch.html")
		}},
		{"discovery", filepath.Join(repo, "sketch.html"), func() (*template.Catalog, error) {
			return template.LoadFile("sketch.html", host)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog, err := tc.load()
			if err != nil {
				t.Fatal(err)
			}
			checkSketch(t, catalog, tc.source)
		})
	}
}

func checkSketch(t *testing.T, catalog *template.Catalog, source string) {
	t.Helper()
	ids := []string{"two-column-article", "chapter-opening"}
	if !reflect.DeepEqual(catalog.IDs(), ids) {
		t.Fatalf("definition order: %v", catalog.IDs())
	}
	// Independent expectations from the authored millimetres, without using the
	// DOM conversion helpers or the geometry resolver under test.
	boxesMM := [][][4]float64{
		{{20, 20, 170, 18}, {20, 46, 80, 231}, {110, 46, 80, 231}},
		{{20, 20, 170, 38}, {20, 70, 170, 207}},
	}
	for i, id := range ids {
		page := lookup(t, catalog, id)
		info := page.Page()
		checkPixels(t, info.Width, 210*96/25.4)
		checkPixels(t, info.Height, 297*96/25.4)
		if info.Paint.Background != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
			t.Fatalf("page background shorthand: %+v", info.Paint)
		}
		checkNoBorderPaint(t, info.Paint)
		pagePath := fmt.Sprintf("/html[1]/body[1]/template[%d]/div[1]", i+1)
		if info.Source.Name != source || info.Source.TemplateID != id || info.Source.Path != pagePath || info.Source.NodeID == 0 {
			t.Fatalf("page provenance: %+v", info.Source)
		}
		frames := page.Frames()
		if len(frames) != len(boxesMM[i]) {
			t.Fatalf("frame count for %s: %d", id, len(frames))
		}
		previousNode := info.Source.NodeID
		for j, f := range frames {
			checkMMRect(t, f.ContentBox, boxesMM[i][j])
			if f.BorderBox != f.ContentBox || f.Margin != (template.Edges{}) || f.Padding != (template.Edges{}) || f.Border != (template.Edges{}) {
				t.Fatalf("zero-edge shorthand geometry: %+v", f)
			}
			flow := "article-flow"
			if j == 0 {
				flow = "heading-flow"
			}
			if f.ID != template.FrameID(j+1) || f.Flow != flow || f.Paint.Background != (color.NRGBA{}) {
				t.Fatalf("frame identity/flow/paint: %+v", f)
			}
			checkNoBorderPaint(t, f.Paint)
			if f.Source.Name != source || f.Source.TemplateID != id || f.Source.Path != fmt.Sprintf("%s/div[%d]", pagePath, j+1) || f.Source.NodeID <= previousNode {
				t.Fatalf("frame provenance: %+v", f.Source)
			}
			previousNode = f.Source.NodeID
		}
		if !reflect.DeepEqual(page.FramesForFlow("heading-flow"), frames[:1]) || !reflect.DeepEqual(page.FramesForFlow("article-flow"), frames[1:]) {
			t.Fatal("flow chains lost source order or frame data")
		}
	}
}

func checkPixels(t *testing.T, got float32, want float64) {
	t.Helper()
	// Allow source-DU and float32 rounding, well below one CSS pixel.
	if math.IsNaN(float64(got)) || math.Abs(float64(got)-want) > .001 {
		t.Fatalf("CSS pixels: got %.8f, want %.8f", got, want)
	}
}

func checkMMRect(t *testing.T, got template.Rect, wantMM [4]float64) {
	t.Helper()
	for i, v := range []float32{got.X, got.Y, got.W, got.H} {
		checkPixels(t, v, wantMM[i]*96/25.4)
	}
}

func checkNoBorderPaint(t *testing.T, paint template.Paint) {
	t.Helper()
	for _, b := range []template.BorderPaint{paint.Borders.Top, paint.Borders.Right, paint.Borders.Bottom, paint.Borders.Left} {
		if b.Style != template.BorderNone || b.Color != (color.NRGBA{A: 255}) {
			t.Fatalf("border shorthand/default color: %+v", b)
		}
	}
}

type templateView struct {
	ID               string
	Page             template.PageInfo
	Frames           []template.FrameInfo
	Heading, Article []template.FrameInfo
}

func catalogView(t *testing.T, catalog *template.Catalog) []templateView {
	t.Helper()
	var result []templateView
	for _, id := range catalog.IDs() {
		page := lookup(t, catalog, id)
		result = append(result, templateView{id, page.Page(), page.Frames(), page.FramesForFlow("heading-flow"), page.FramesForFlow("article-flow")})
	}
	return result
}

func TestIntegrationCatalogReloadLifetime(t *testing.T) {
	const name = "pages.html"
	original, err := exampleCollections.ReadFile("testdata/templates/sketch.html")
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := exampleCollections.ReadFile("testdata/templates/invalid-overlap.html")
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{name: &fstest.MapFile{Data: original}}
	observed := &collectionFS{FS: files}
	config := t.TempDir()
	repo := filepath.Join(config, "templates")
	host := &collectionIO{configDir: config, roots: map[string]fs.FS{repo: observed}}
	first, err := template.LoadFile(name, host)
	if err != nil {
		t.Fatal(err)
	}
	wantFirst := catalogView(t, first)

	// Reload the same name and template IDs with different CSS and source order.
	// The earlier catalog must retain its own paint, geometry and flow chains.
	changed := strings.ReplaceAll(string(original), "width: 210mm", "width: 220mm")
	changed = strings.ReplaceAll(changed, "background: #ffffff", "background: #fff8e1")
	changed = strings.Replace(changed,
		`<div class="frame body body-left"></div>`+"\n      "+`<div class="frame body body-right"></div>`,
		`<div class="frame body body-right"></div>`+"\n      "+`<div class="frame body body-left"></div>`, 1)
	changed = strings.Replace(changed, "</style>", `.page > .body { padding: 1px; border: 2px solid #123456; background: #ddeeff; }</style>`, 1)
	files[name].Data = []byte(changed)
	second, err := template.LoadFile(name, host)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range second.IDs() {
		page := lookup(t, second, id)
		checkPixels(t, page.Page().Width, 220*96/25.4)
		if page.Page().Paint.Background != (color.NRGBA{R: 255, G: 248, B: 225, A: 255}) {
			t.Fatal("reload reused the earlier page paint")
		}
		for _, frame := range page.FramesForFlow("article-flow") {
			if frame.Padding != (template.Edges{Top: 1, Right: 1, Bottom: 1, Left: 1}) || frame.Border != (template.Edges{Top: 2, Right: 2, Bottom: 2, Left: 2}) || frame.Paint.Background != (color.NRGBA{R: 221, G: 238, B: 255, A: 255}) {
				t.Fatalf("reload lost paint/edge shorthands: %+v", frame)
			}
			for _, b := range []template.BorderPaint{frame.Paint.Borders.Top, frame.Paint.Borders.Right, frame.Paint.Borders.Bottom, frame.Paint.Borders.Left} {
				if b.Style != template.BorderSolid || b.Color != (color.NRGBA{R: 18, G: 52, B: 86, A: 255}) {
					t.Fatalf("reload lost border paint: %+v", b)
				}
			}
			checkPixels(t, frame.ContentBox.X-frame.BorderBox.X, 3)
			checkPixels(t, frame.ContentBox.Y-frame.BorderBox.Y, 3)
			checkPixels(t, frame.BorderBox.W-frame.ContentBox.W, 6)
			checkPixels(t, frame.BorderBox.H-frame.ContentBox.H, 6)
		}
	}
	chain := lookup(t, second, "two-column-article").FramesForFlow("article-flow")
	if len(chain) != 2 || chain[0].ID != 2 || chain[1].ID != 3 || chain[0].ContentBox.X <= chain[1].ContentBox.X {
		t.Fatal("reload sorted flow frames visually instead of retaining source order")
	}
	wantSecond := catalogView(t, second)
	files[name].Data = invalid
	bad, err := template.LoadFile(name, host)
	ds := fileDiagnostics(t, bad, err)
	if len(ds) != 1 || ds[0].Code != "geometry.overlap" || ds[0].Source.Name != filepath.Join(repo, name) || ds[0].Source.TemplateID != "overlap" {
		t.Fatalf("failed reload lost source or published a partial catalog: %v", err)
	}
	if observed.closes != 3 || len(observed.opens) != 3 {
		t.Fatalf("load lifecycle: opens=%d closes=%d", len(observed.opens), observed.closes)
	}

	// Make source access fail after loading; selection can only inspect the
	// completed catalogs. The race run also checks their simultaneous use.
	observed.openErr = errors.New("repository no longer available")
	observed.FS = nil
	configCalls, dirCalls := host.configCalls, len(host.dirs)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				for _, tc := range []struct {
					catalog *template.Catalog
					want    []templateView
				}{{first, wantFirst}, {second, wantSecond}} {
					if got := catalogView(t, tc.catalog); !reflect.DeepEqual(got, tc.want) {
						t.Error("later loads or accessor mutations changed a completed catalog")
						return
					}
					page := lookup(t, tc.catalog, "two-column-article")
					page.Frames()[0] = template.FrameInfo{}
					page.FramesForFlow("article-flow")[0] = template.FrameInfo{}
					tc.catalog.IDs()[0] = "changed"
					*page = template.Template{}
				}
			}
		}()
	}
	wg.Wait()
	if len(observed.opens) != 3 || observed.closes != 3 || host.configCalls != configCalls || len(host.dirs) != dirCalls {
		t.Fatal("catalog inspection returned to host I/O")
	}
}

func TestIntegrationSketchDocumentation(t *testing.T) {
	doc, err := os.ReadFile("../doc/SKETCH.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(doc), "## Collection in one HTML file")
	if !ok {
		t.Fatal("missing collection sketch")
	}
	_, code, ok := strings.Cut(section, "```html\n")
	if !ok {
		t.Fatal("missing collection HTML")
	}
	code, _, ok = strings.Cut(code, "```")
	fixture, err := exampleCollections.ReadFile("testdata/templates/sketch.html")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || strings.TrimSpace(code) != strings.TrimSpace(string(fixture)) {
		t.Fatal("documented collection differs from the executable fixture")
	}
	if strings.Contains(code, "flow-into") {
		t.Fatal("template collection contains content routing rules")
	}
	// The content document is input for the content loader, not a
	// collection to pass to the template loader.
	catalog, err := template.LoadFS(os.DirFS("testdata/content"), "article.html")
	ds := fileDiagnostics(t, catalog, err)
	for _, d := range ds {
		if d.Code == "collection.body-content" && d.Source.Name == "article.html" {
			return
		}
	}
	t.Fatalf("content document did not report its collection boundary: %v", err)
}
