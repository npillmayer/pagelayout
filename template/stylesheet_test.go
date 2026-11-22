package template_test

import (
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/npillmayer/pagelayout/template"
)

func overrideCollection(css, inline string) string {
	return fmt.Sprintf(`<head><style>
.page {width:300px;height:200px}
.frame {position:fixed;left:20px;top:20px;width:40px;height:60px;flow-from:main;background:#ff0000}
%s</style></head><body>
<template id="first" data-type="page"><div class="page"><div id="frame" class="frame" style="%s"></div></div></template>
<template id="second" data-type="page"><div class="page second"><div id="frame" class="frame" style="%s"></div></div></template>
</body>`, css, inline, inline)
}

func TestAdditionalStylesheetCascade(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	green := color.NRGBA{G: 255, A: 255}
	for _, tc := range []struct {
		name, original, inline string
		css                    []string
		want                   color.NRGBA
	}{
		{"later sheet", "", "", []string{`.frame {background:#0000ff}`}, blue},
		{"sheet order", "", "", []string{`.frame {background:#00ff00}`, `.frame {background:#0000ff}`}, blue},
		{"original specificity", `#frame {background:#ff0000}`, "", []string{`.frame {background:#0000ff}`}, red},
		{"normal inline", "", `background:#00ff00`, []string{`#frame {background:#0000ff}`}, green},
		{"important sheet beats normal inline", "", `background:#00ff00`, []string{`.frame {background:#0000ff!important}`}, blue},
		{"original important", `.frame {background:#ff0000!important}`, "", []string{`#frame {background:#0000ff}`}, red},
		{"important inline", "", `background:#00ff00!important`, []string{`#frame {background:#0000ff!important}`}, green},
		{"storage wrapper is absent", "", "", []string{`template .frame {background:#0000ff}`}, red},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := template.LoadOptions{SourceName: "pages.html"}
			for i, css := range tc.css {
				opts.Stylesheets = append(opts.Stylesheets, template.Stylesheet{SourceName: fmt.Sprintf("user%d.css", i), CSS: css})
			}
			catalog, err := template.Load(strings.NewReader(overrideCollection(tc.original, tc.inline)), opts)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range catalog.IDs() {
				page, _ := catalog.Lookup(id)
				if got := page.Frames()[0].Paint.Background; got != tc.want {
					t.Fatalf("%s: got %v, want %v", id, got, tc.want)
				}
			}
		})
	}
}

func TestAdditionalStylesheetGeometryAndLifetime(t *testing.T) {
	input := overrideCollection("", "")
	base, err := template.Load(strings.NewReader(input), template.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	opts := template.LoadOptions{SourceName: "pages.html", Stylesheets: []template.Stylesheet{{
		SourceName: "user.css", CSS: `.frame {left:30px;width:70px;padding:5px;border:2px solid #0000ff}
.second .frame {left:180px}`,
	}}}
	updated, err := template.Load(strings.NewReader(input), opts)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := updated.Lookup("first")
	frame := first.Frames()[0]
	if frame.BorderBox != (template.Rect{X: 30, Y: 20, W: 84, H: 74}) || frame.ContentBox != (template.Rect{X: 37, Y: 27, W: 70, H: 60}) {
		t.Fatalf("override geometry not recompiled: %+v", frame)
	}
	if frame.Source.Name != "pages.html" || frame.Source.TemplateID != "first" {
		t.Fatal("frame lost its original source identity")
	}
	second, _ := updated.Lookup("second")
	if second.Frames()[0].BorderBox.X != 180 {
		t.Fatal("definition-specific override did not apply")
	}
	opts.Stylesheets[0].CSS = `.frame {left:290px}`
	bad, err := template.Load(strings.NewReader(input), opts)
	if bad != nil || err == nil || !strings.Contains(err.Error(), "geometry.bounds") {
		t.Fatalf("invalid override published geometry: %v", err)
	}
	if first.Frames()[0] != frame {
		t.Fatal("failed reload or changed options mutated an existing catalog")
	}
	original, _ := base.Lookup("first")
	if original.Frames()[0].BorderBox.W != 40 {
		t.Fatal("override changed original catalog")
	}
}

func TestAdditionalStylesheetDiagnostics(t *testing.T) {
	for _, tc := range []struct{ css, stage string }{
		{`.frame[ {width:40px}`, "css"},
		{`@import "never-fetch.css";`, "css"},
		{`.unused {width:50%;width:40px}`, "profile"},
	} {
		opts := template.LoadOptions{Stylesheets: []template.Stylesheet{{SourceName: "overrides.css", CSS: tc.css}}}
		catalog, err := template.Load(strings.NewReader(overrideCollection("", "")), opts)
		var failure *template.LoadError
		if catalog != nil || !errors.As(err, &failure) {
			t.Fatalf("invalid additional CSS accepted: %s", tc.css)
		}
		for _, d := range failure.Diagnostics() {
			if d.Source.Name != "overrides.css" || d.Source.NodeID != 0 || d.Source.Path != "" || d.Stage != tc.stage || d.CSS.Line == 0 {
				t.Fatalf("incorrect stylesheet diagnostic: %+v", d)
			}
		}
	}
	// Source validation still rejects losing original declarations. Additional
	// diagnostics follow HTML diagnostics, then supplied stylesheet order.
	opts := template.LoadOptions{SourceName: "pages.html", Stylesheets: []template.Stylesheet{
		{SourceName: "first.css", CSS: `.frame {width:40px} .unused {height:20%}`},
		{CSS: `.unused {height:30%}`},
	}}
	_, err := template.Load(strings.NewReader(overrideCollection(`.frame {width:50%}`, "")), opts)
	var failure *template.LoadError
	if !errors.As(err, &failure) {
		t.Fatal("losing source errors were hidden")
	}
	ds := failure.Diagnostics()
	if len(ds) != 3 {
		t.Fatalf("expected three source diagnostics: %v", err)
	}
	for i, name := range []string{"pages.html", "first.css", "<stylesheet 2>"} {
		if ds[i].Source.Name != name {
			t.Fatalf("diagnostic order: %v", err)
		}
	}
}

func TestFileLoadersWithStylesheets(t *testing.T) {
	opts := template.LoadOptions{SourceName: "ignored.html", Stylesheets: []template.Stylesheet{{SourceName: "user.css", CSS: `.frame {width:75px}`}}}
	files := &collectionFS{FS: fstest.MapFS{"pages.html": &fstest.MapFile{Data: []byte(overrideCollection("", ""))}}}
	config := t.TempDir()
	repo := filepath.Join(config, "templates")
	host := &collectionIO{configDir: config, roots: map[string]fs.FS{repo: files}}
	for _, entry := range []struct {
		name string
		load func() (*template.Catalog, error)
	}{
		{"pages.html", func() (*template.Catalog, error) { return template.LoadFSWithOptions(files, "pages.html", opts) }},
		{filepath.Join(repo, "pages.html"), func() (*template.Catalog, error) { return template.LoadFileWithOptions("pages.html", host, opts) }},
	} {
		catalog, err := entry.load()
		if err != nil {
			t.Fatal(err)
		}
		page, _ := catalog.Lookup("first")
		if page.Frames()[0].ContentBox.W != 75 || page.Page().Source.Name != entry.name {
			t.Fatal("file loader lost options or selected path")
		}
	}
	if files.closes != 2 {
		t.Fatal("file loaders did not close input")
	}
	files.closeErr = errors.New("close failed")
	opts.Stylesheets[0].CSS = `.frame {width:50%}`
	host.dirs = nil
	catalog, err := template.LoadFileWithOptions("pages.html", host, opts)
	ds := fileDiagnostics(t, catalog, err)
	if len(host.dirs) != 1 || files.closes != 3 || len(ds) != 2 || ds[0].Source.Name != "user.css" || ds[1].Code != "input.close" || !errors.Is(err, files.closeErr) {
		t.Fatalf("additional stylesheet failure lost diagnostics, closure or stopped search: %v", err)
	}
}
