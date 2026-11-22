package content

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/w3c-dom/style/css"
	"github.com/npillmayer/w3c-dom/style/cssom"
	"golang.org/x/net/html"
)

func expectFailure(t testing.TB, source *styledSource, err error) []diagnostic {
	t.Helper()
	var failure *loadError
	if source != nil || err == nil || !errors.As(err, &failure) || len(failure.items) == 0 {
		t.Fatalf("expected atomic structured error, got source=%v, err=%v", source, err)
	}
	return failure.diagnostics()
}

func sourceProperty(t testing.TB, s *styledSource, id, key string) string {
	t.Helper()
	n := sourceByID(t, s.source, id)
	styled := s.nodes[n.ref.nodeID]
	if styled == nil || styled.HTMLNode() != n.node {
		t.Fatalf("source/style identity lost: %s", id)
	}
	p, err := css.GetProperty(styled, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(p)
}

func renderSource(t testing.TB, doc *html.Node) string {
	t.Helper()
	var b strings.Builder
	if err := html.Render(&b, doc); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestSourceCascadeAndAdditionalSheets(t *testing.T) {
	source, err := parseSource(strings.NewReader(fixture(t, "routing.html")), "routing.html")
	if err != nil {
		t.Fatal(err)
	}
	before := renderSource(t, source.document)
	base, err := styleSource(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	cssText := fixture(t, "routing-override.css")
	override, err := styleSource(source, []Stylesheet{{SourceName: "routing-override.css", CSS: cssText}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ id, key, base, override string }{
		{"story-a", "flow-into", "article", "features"},
		{"story-b", "flow-into", "article", "features"},
		{"note", "flow-into", "notes", "notes"},
		{"note", "color", "#884400", "#006699"},
		{"source", "font-size", "20px", "24px"},
		{"note", "font-size", "20px", "24px"},
		{"story-a", "color", "#224466", "#224466"},
		{"story-b", "color", "#203040", "#008855"},
		{"paragraph-b", "color", "#203040", "#008855"},
		{"intro", "font-size", "16px", "16px"},
		{"story-b", "width", "75%", "75%"},
		{"tag", "width", "auto", "auto"},
	} {
		if got := sourceProperty(t, base, tc.id, tc.key); got != tc.base {
			t.Errorf("base %s.%s=%s, want %s", tc.id, tc.key, got, tc.base)
		}
		if got := sourceProperty(t, override, tc.id, tc.key); got != tc.override {
			t.Errorf("override %s.%s=%s, want %s", tc.id, tc.key, got, tc.override)
		}
	}
	if renderSource(t, source.document) != before {
		t.Fatal("styling changed the original DOM")
	}
	// Same source pointers, fresh style state, and no channel extraction yet.
	id := sourceByID(t, source, "note").ref.nodeID
	if base.nodes[id] == override.nodes[id] || sourceByID(t, source, "note").node.Parent != sourceByID(t, source, "source").node {
		t.Fatal("styling reused state or moved source branches")
	}
	if len(override.styles.sheets) != 2 || override.styles.sheets[1].source.name != "routing-override.css" || override.styles.sheets[1].source.nodeID != 0 {
		t.Fatal("additional CSS lost its independent source identity")
	}
}

func TestAdditionalSheetOrderImportanceAndInputLifetime(t *testing.T) {
	input := `<head><style>p {color:black}</style></head><body><p id="p" style="color:blue!important">text</p><p id="q">text</p></body>`
	opts := LoadOptions{Stylesheets: []Stylesheet{
		{SourceName: "first.css", CSS: `p {color:red!important} #q {width:10px}`},
		{SourceName: "second.css", CSS: `p {color:green!important} #q {width:20px}`},
	}}
	s, err := loadSource(strings.NewReader(input), opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Stylesheets[1] = Stylesheet{SourceName: "changed.css", CSS: `p {color:purple}`}
	for _, tc := range []struct{ id, key, want string }{{"p", "color", "blue"}, {"q", "color", "green"}, {"q", "width", "20px"}} {
		if got := sourceProperty(t, s, tc.id, tc.key); got != tc.want {
			t.Errorf("%s.%s=%s want %s", tc.id, tc.key, got, tc.want)
		}
	}
	if s.styles.sheets[2].source.name != "second.css" {
		t.Fatal("retained caller's mutable options slice")
	}
}

func TestOriginalSelectorAncestryAndInertBoundaries(t *testing.T) {
	input := `<html style="color:blue"><head>
<style>section > template + script + p:nth-child(3) {width:7px} bdi, small, dl, dt, dd, x-custom {color:red}</style>
<template><style>@import "https://invalid.example/ignored.css";</style><link rel="stylesheet" href="ignored"></template>
</head><body><section id="section" style="font-size:20px">
<template id="inert" style="width:bad"><p id="stored" style="width:bad"><img src="ignored"></p></template>
<script id="script" src="https://invalid.example/ignored.js" style="width:bad">never execute</script>
<p id="active"><bdi id="bdi">a</bdi><small id="small">b</small><x-custom id="custom">c</x-custom></p>
<dl id="dl"><dt id="dt">term</dt><dd id="dd">definition</dd></dl></section></body></html>`
	s, err := loadSource(strings.NewReader(input), LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ id, key, want string }{{"active", "width", "7px"}, {"active", "color", "blue"}, {"active", "font-size", "20px"}} {
		if got := sourceProperty(t, s, tc.id, tc.key); got != tc.want {
			t.Errorf("%s.%s=%s want %s", tc.id, tc.key, got, tc.want)
		}
	}
	for _, id := range []string{"bdi", "small", "custom", "dl", "dt", "dd"} {
		if got := sourceProperty(t, s, id, "color"); got != "red" {
			t.Errorf("included element %s lost its own declaration: %s", id, got)
		}
	}
	for _, id := range []string{"inert", "stored", "script"} {
		n := sourceByID(t, s.source, id)
		if s.nodes[n.ref.nodeID] != nil || s.styles.inline[n.ref.nodeID] != nil {
			t.Errorf("inspected/styled excluded source: %s", id)
		}
	}
	if len(s.styles.sheets) != 1 {
		t.Fatal("collected stored template stylesheet")
	}
}

func TestInspectionRetainsLosingAndNonmatchingDeclarations(t *testing.T) {
	input := `<head><style>#absent {color:purple} p {width:1px;width:2px} p {width:3px!important}</style></head>
<body><p id="p" style="color:red;color:blue">text</p></body>`
	s, err := loadSource(strings.NewReader(input), LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rules := s.styles.sheets[0].inspection.Rules
	if len(rules) != 3 || rules[0].Prelude != "#absent" || len(rules[1].Declarations) != 2 || string(rules[1].Declarations[0].Value) != "1px" || !rules[2].Declarations[0].Important {
		t.Fatalf("lost source declarations: %+v", rules)
	}
	inline := s.styles.inline[sourceByID(t, s.source, "p").ref.nodeID]
	if len(inline) != 2 || string(inline[0].Value) != "red" || sourceProperty(t, s, "p", "color") != "blue" || sourceProperty(t, s, "p", "width") != "3px" {
		t.Fatal("inline inspection or cascade incorrect")
	}
}

func TestCSSInspectionFailuresAreAtomic(t *testing.T) {
	for name, input := range map[string]string{
		"losing-invalid-value":    `<style>p {width:bad;width:10px}</style><p>text</p>`,
		"unmatched-invalid-value": `<style>#absent {width:bad}</style><p>text</p>`,
		"malformed-inline":        `<p style="color:">text</p>`,
		"suppressed-inline":       `<div style="display:none"><p style="width:bad">hidden</p></div>`,
		"malformed-selector":      `<style>p[ {color:red}</style>`,
		"malformed-sheet":         `<style>p {color:red</style>`,
		"import":                  `<style>@import "https://invalid.example/style.css";</style>`,
		"font-resource":           `<style>@font-face {font-family:foo;src:url(https://invalid.example/f.woff)}</style>`,
		"conditional":             `<style>@media print {p {color:red}}</style>`,
	} {
		t.Run(name, func(t *testing.T) {
			s, err := loadSource(strings.NewReader(input), LoadOptions{SourceName: "bad.html"})
			ds := expectFailure(t, s, err)
			if ds[0].stage != "css" || ds[0].source.name != "bad.html" || ds[0].source.nodeID == 0 || ds[0].css.line == 0 {
				t.Errorf("missing CSS provenance: %+v", ds)
			}
			var original cssom.Diagnostic
			if !errors.As(err, &original) {
				t.Fatal("upstream CSS diagnostic cause lost")
			}
		})
	}
}

func TestSourceRequirementsAndStylesheetLocations(t *testing.T) {
	for _, tc := range []struct{ name, input, code string }{
		{"link", `<link rel="alternate StyleSheet" href="https://invalid.example/a.css"><p>text</p>`, "input.resource"},
		{"image", `<p><img src="https://invalid.example/image.png"></p>`, "input.resource"},
		{"embed", `<iframe src="https://invalid.example/document"></iframe>`, "input.resource"},
		{"body-style", `<body><div><style>p {color:red}</style></div></body>`, "css.style-location"},
		{"media", `<style media="print">p {color:red}</style>`, "css.style-attribute"},
		{"type", `<style type="text/less">p {color:red}</style>`, "css.style-attribute"},
		{"scope", `<style scoped>p {color:red}</style>`, "css.style-attribute"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := loadSource(strings.NewReader(tc.input), LoadOptions{})
			ds := expectFailure(t, s, err)
			if len(ds) != 1 || ds[0].code != tc.code || ds[0].source.path == "" {
				t.Fatalf("unexpected requirement diagnostics: %+v", ds)
			}
		})
	}
	for _, input := range []string{
		`<head><style type=" Text/CSS " media=" ALL ">p {color:red}</style></head><p>text</p>`,
		`<head><link rel="canonical" href="https://invalid.example/doc"></head><body><a href="https://invalid.example/doc">link</a></body>`,
		`<template><img src="unused"><style media="print">broken{</style></template>`,
	} {
		if _, err := loadSource(strings.NewReader(input), LoadOptions{}); err != nil {
			t.Errorf("valid input boundary rejected: %v", err)
		}
	}
}

func TestDiagnosticOrderingCoordinatesAndCopies(t *testing.T) {
	input := `<html style="color:"><head><style>p[ {color:red}</style></head><body><p style="width:bad">text</p></body></html>`
	opts := LoadOptions{SourceName: "source.html", Stylesheets: []Stylesheet{
		{SourceName: "first.css", CSS: "#missing {\nwidth:bad;\n}"},
		{CSS: `@import "unused";`},
	}}
	var previous []diagnostic
	for run := 0; run < 2; run++ {
		s, err := loadSource(strings.NewReader(input), opts)
		ds := expectFailure(t, s, err)
		if len(ds) != 5 || ds[0].source.path != "/html[1]" || !ds[0].css.inline || ds[1].source.path != "/html[1]/head[1]/style[1]" || ds[2].source.path != "/html[1]/body[1]/p[1]" {
			t.Fatalf("wrong HTML diagnostic order: %+v", ds)
		}
		if ds[3].source.name != "first.css" || ds[3].source.nodeID != 0 || ds[3].source.path != "" || ds[3].css.line != 2 || ds[3].css.rule != 1 || ds[3].css.declaration != 1 || ds[3].css.property != "width" || ds[3].css.inline {
			t.Fatalf("wrong additional CSS coordinates: %+v", ds[3])
		}
		if ds[4].source.name != "<stylesheet 2>" || ds[4].source.stylesheet != 2 {
			t.Fatal("missing generated label")
		}
		if run != 0 && !reflect.DeepEqual(previous, ds) {
			t.Fatal("unstable diagnostic order")
		}
		previous = ds
		original := err.Error()
		var failure *loadError
		errors.As(err, &failure)
		copy := failure.diagnostics()
		copy[0].message = "mutated"
		children := failure.Unwrap()
		children[0] = nil
		if err.Error() != original || !strings.Contains(original, "CSS 2:") || !strings.Contains(original, "inline style") {
			t.Fatal("mutable diagnostics or missing readable locations")
		}
		var upstream cssom.Diagnostic
		if !errors.As(ds[1], &upstream) || upstream.Code != "css.selector-syntax" || errors.Unwrap(upstream) == nil {
			t.Fatal("selector parser cause lost")
		}
	}
}

func TestAdditionalStylesheetUTF8(t *testing.T) {
	s, err := loadSource(strings.NewReader(""), LoadOptions{Stylesheets: []Stylesheet{{SourceName: "encoding.css", CSS: "p {font-family:'\xff'}"}}})
	ds := expectFailure(t, s, err)
	if len(ds) != 1 || ds[0].code != "input.encoding" || ds[0].source.name != "encoding.css" || ds[0].source.nodeID != 0 || ds[0].css.line != 0 {
		t.Fatalf("incorrect encoding coordinates: %+v", ds)
	}
}

func TestLaterProfileAndRoutingFixturesRemainSourceOnly(t *testing.T) {
	// A private source result does not promise that later phases will accept it.
	for _, name := range []string{"empty.html", "residual-inline.html", "nested-flow_err.html", "float_err.html"} {
		if _, err := loadSource(strings.NewReader(fixture(t, name)), LoadOptions{SourceName: name}); err != nil {
			t.Errorf("%s should reach later validation: %v", name, err)
		}
	}
}

func TestIndependentConcurrentLoads(t *testing.T) {
	input := fixture(t, "routing.html")
	for i := 0; i < 8; i++ {
		t.Run(fmt.Sprintf("load-%d", i), func(t *testing.T) {
			t.Parallel()
			want := []string{"red", "blue"}[i%2]
			opts := LoadOptions{Stylesheets: []Stylesheet{{CSS: ".story + .note {color:" + want + "}"}}}
			var prior *styledSource
			for repeat := 0; repeat < 3; repeat++ {
				s, err := loadSource(strings.NewReader(input), opts)
				if err != nil {
					t.Fatal(err)
				}
				if sourceProperty(t, s, "note", "color") != want {
					t.Fatal("cascade leaked across independent loads")
				}
				if prior != nil && (prior.source.document == s.source.document || prior.root == s.root || sourceProperty(t, prior, "note", "color") != want) {
					t.Fatal("earlier snapshot changed or shared working state")
				}
				prior = s
			}
		})
	}
}
