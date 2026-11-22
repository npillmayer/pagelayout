package content

import (
	"errors"
	"strings"
	"testing"

	"github.com/npillmayer/w3c-dom/style"
)

func TestContentAuthoredProfileChecksEveryDeclaration(t *testing.T) {
	for _, decl := range []string{
		"float:right", "position:fixed", "visibility:hidden", "box-sizing:border-box", "writing-mode:vertical-rl",
		"display:contents", "display:grid", "display:table", "display:list-item", "flow-from:article",
		"height:10px", "min-width:10px", "max-width:100%", "top:1px", "width:1vw", "padding:1ch",
		"border-left:1em dashed red", "white-space:pre", "line-height:1.2", "unicode-bidi:bidi-override",
		"word-spacing:1px", "letter-spacing:1px", "word-break:break-all", "overflow-wrap:anywhere",
		"font-size:0", "font-size:2vw", "font-family:serif,", "font-weight:450.5", "font-style:oblique 10deg", "hyphens:unknown",
		"font-size:revert", "width:calc(10px + 1em)",
		"transform:none", "opacity:1", "font-variant:normal", "content:'generated'", "background-image:none",
	} {
		t.Run(decl, func(t *testing.T) {
			// No matching element, and a later declaration could win. The source
			// audit must still report the authored unsupported intent.
			src := `<head><style>#absent {` + decl + `;color:blue}</style></head><body><p>x</p></body>`
			p, err := loadProjection(strings.NewReader(src), LoadOptions{SourceName: "authored.html"})
			ds := projectionFailure(t, p, err)
			if ds[0].stage != "profile" || ds[0].css.property != strings.SplitN(decl, ":", 2)[0] || ds[0].css.value == "" || ds[0].css.line == 0 || ds[0].css.declaration != 1 || ds[0].source.name != "authored.html" {
				t.Fatalf("lost authored declaration: %+v", ds)
			}
		})
	}
	for _, src := range []string{
		`<p style="float:right;float:none">x</p>`,
		`<div style="display:none"><p style="white-space:pre">x</p></div>`,
		`<style>#hidden {display:none} #hidden {float:left}</style><div id="hidden"></div>`,
	} {
		p, err := loadProjection(strings.NewReader(src), LoadOptions{})
		if ds := projectionFailure(t, p, err); ds[0].stage != "profile" {
			t.Fatalf("losing/hidden declaration escaped audit: %+v", ds)
		}
	}
}

func TestProfileAdditionalSourcesAndStableOrder(t *testing.T) {
	src := `<head><style>#unmatched {float:left}</style></head><body><p style="float:right">x</p></body>`
	opts := LoadOptions{SourceName: "input.html", Stylesheets: []Stylesheet{
		{SourceName: "one.css", CSS: "p {white-space:pre}"},
		{SourceName: "two.css", CSS: "#missing {position:absolute; width:1vw}"},
	}}
	for range 3 {
		p, err := loadProjection(strings.NewReader(src), opts)
		ds := projectionFailure(t, p, err)
		if len(ds) != 5 {
			t.Fatalf("missing diagnostics: %+v", ds)
		}
		for i, name := range []string{"input.html", "input.html", "one.css", "two.css", "two.css"} {
			if ds[i].source.name != name || ds[i].css.line == 0 {
				t.Fatalf("unstable diagnostic order: %+v", ds)
			}
			if i >= 2 && (ds[i].source.nodeID != 0 || ds[i].source.path != "") {
				t.Fatal("additional sheet acquired fictional HTML coordinates")
			}
		}
		if !ds[1].css.inline || ds[0].css.rule != 1 || ds[1].css.rule != 0 {
			t.Fatal("CSS coordinate systems merged")
		}
	}
}

func TestProfileAllowsSupportedDefaultsAndCSSKeywords(t *testing.T) {
	p := projection(t, `<html lang="de"><head><style>
article { display:block flow-root; font-family:'A Font', serif; font-weight:bolder; font-style:italic;
 position:static; float:none; visibility:visible; writing-mode:horizontal-tb; box-sizing:content-box;
 height:auto; min-width:auto; min-height:auto; max-width:none; max-height:none;
 top:auto; right:auto; bottom:auto; left:auto; line-height:n\6f rmal; white-space:NORMAL;
 word-spacing:normal; letter-spacing:normal; word-break:normal; overflow-wrap:normal;
 background:#102030; border:0; padding:0%; margin:auto; hyphens:auto; direction:rtl;
 flow-into:Article; flow-from:none;
}
</style></head><body><article id="a"><span id="s" style="font-size:inherit;font-weight:initial;font-style:unset;color:currentcolor;unicode-bidi:isolate;hyphens:none">a</span></article></body></html>`, LoadOptions{})
	a := p.sources[projectedID(t, p, "a")]
	s := p.sources[projectedID(t, p, "s")]
	if a.flow != "Article" || a.typography.Weight != 700 || a.typography.Family != "'A Font', serif" || s.typography.Weight != 400 || s.typography.Slant != "italic" || s.typography.Hyphens != "none" {
		t.Fatalf("supported cascade: %+v %+v", a.typography, s.typography)
	}
}

func TestProfileRetainsUpstreamCauseAndInertBoundary(t *testing.T) {
	p, err := loadProjection(strings.NewReader(`<p style="font-size:bogus">x</p>`), LoadOptions{})
	ds := projectionFailure(t, p, err)
	if ds[0].stage != "profile" || !errors.Is(err, style.ErrInvalidValue) {
		t.Fatalf("lost checked value cause: %+v %v", ds, err)
	}
	projection(t, `<template style="float:right"><p style="font-size:0;display:grid">inert</p></template><script style="font-size:0">ignored</script><p>visible</p>`, LoadOptions{})
	// Inspection of malformed declarations still takes precedence over profiles.
	p, err = loadProjection(strings.NewReader(`<p style="width:-1px;float:right">x</p>`), LoadOptions{})
	if ds := projectionFailure(t, p, err); ds[0].stage != "css" {
		t.Fatal("invalid syntax/value reached projection")
	}
}
