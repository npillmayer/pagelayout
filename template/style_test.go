package template

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/w3c-dom/style/css"
	"github.com/npillmayer/w3c-dom/style/cssom"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TestActiveTemplateOwnershipAndSources(t *testing.T) {
	c := readCollectionFixture(t, "two-pages.html")
	before := renderHTML(t, c.document)
	a := activateTemplate(c, c.templates[0])
	b := activateTemplate(c, c.templates[0])
	if a.html.Parent != a.document || a.body.Parent != a.html || a.page.Parent != a.body || a.body.FirstChild != a.body.LastChild || a.html.FirstChild != a.html.LastChild {
		t.Fatal("wrong active ancestry")
	}
	if len(a.frames) != 3 {
		t.Fatal("lost frames")
	}
	for i, f := range a.frames {
		if f.id != c.templates[0].frames[i].id || a.sources[f.element] != c.sources[c.templates[0].frames[i].element] {
			t.Fatal("lost frame/source identity")
		}
	}
	walkHTML(a.document, func(n *html.Node) {
		if c.sources[n].nodeID != 0 || b.sources[n].nodeID != 0 {
			t.Fatal("shared node storage")
		}
		if n.DataAtom == atom.Template || n.DataAtom == atom.Head || n.DataAtom == atom.Style {
			t.Fatal("storage context leaked into active document")
		}
		ref, exists := a.sources[n]
		if !exists || ref.nodeID == 0 {
			t.Fatal("missing clone source")
		}
		found := false
		for original, originalRef := range c.sources {
			if originalRef == ref {
				found = true
				if original.Type != n.Type || original.DataAtom != n.DataAtom || original.Data != n.Data || original.Namespace != n.Namespace || !reflect.DeepEqual(original.Attr, n.Attr) {
					t.Fatal("clone differs from source")
				}
			}
		}
		if !found {
			t.Fatal("invented source identity")
		}
	})
	second := renderHTML(t, b.document)
	a.html.Attr[0].Val = "changed"
	a.body.Attr[0].Val = "changed"
	a.page.Attr[0].Val = "changed"
	a.frames[0].element.Attr[0].Val = "changed"
	a.page.RemoveChild(a.frames[0].element)
	if renderHTML(t, c.document) != before || renderHTML(t, b.document) != second {
		t.Fatal("mutating a clone changed source or another active document")
	}
}

func TestIsolatedStylingContextAndStructure(t *testing.T) {
	input := `<html lang="en" class="collection" style="color:maroon"><head>
<style>
html[lang="en"].collection > body.print { font-family:serif }
.frame { width:7px; position:fixed }
body > .page:only-child > .frame:first-child { width:10px }
body > .page:only-child > .frame:nth-child(2) { width:20px }
template .frame, .page:nth-child(2) .frame { width:999px }
.first .frame { height:31px }
.second .frame { height:42px }
</style><style>.frame { flow-from:main-flow }</style></head>
<body class="print" style="font-size:14px">
<template><style>.frame[ { broken }</style></template>
<template id="first" data-type="page" style="this is not active CSS"><div class="page first">
<div class="frame" id="same" style="color:green"></div><div class="frame"></div></div></template>
<template id="second" data-type="page"><div class="page second"><div class="frame" id="same"></div></div></template>
</body></html>`
	c := parseCollectionOK(t, input)
	before := renderHTML(t, c.document)
	styled := styleCollectionOK(t, c)
	if len(styled.templates) != 2 || styled.templates[0] != styled.byID["first"] || styled.templates[1] != styled.byID["second"] || styled.byID["First"] != nil || len(styled.styles.sheets) != 2 {
		t.Fatal("definition/sheet order or lookup changed")
	}
	for i, definition := range styled.templates {
		for j, f := range definition.active.frames {
			width := []string{"10px", "20px"}[j]
			wantProperty(t, definition, f.element, "width", width)
			wantProperty(t, definition, f.element, "height", []string{"31px", "42px"}[i])
			wantProperty(t, definition, f.element, "font-family", "serif")
			wantProperty(t, definition, f.element, "font-size", "14px")
			wantProperty(t, definition, f.element, "flow-from", "main-flow")
			color := "maroon"
			if i == 0 && j == 0 {
				color = "green"
			}
			wantProperty(t, definition, f.element, "color", color)
			if ref := definition.active.sources[definition.nodes[f.element].HTMLNode()]; ref != c.sources[c.templates[i].frames[j].element] {
				t.Fatal("styled node lost original provenance")
			}
		}
	}
	if renderHTML(t, c.document) != before {
		t.Fatal("styling mutated collection")
	}
	// Both source occurrences of id=same are isolated; inline state cannot leak.
	first, second := styled.templates[0], styled.templates[1]
	if first.root.Parent().Payload.Styles() == second.root.Parent().Payload.Styles() {
		t.Fatal("CSSOM defaults are shared between definitions")
	}
	first.nodes[first.active.frames[0].element].SetStyles(nil)
	wantProperty(t, second, second.active.frames[0].element, "color", "maroon")
	again := styleCollectionOK(t, c)
	wantProperty(t, again.templates[0], again.templates[0].active.frames[0].element, "color", "green")
	if again.templates[0].root == first.root || again.templates[0].active.page == first.active.page {
		t.Fatal("styled session reused mutable storage")
	}
}

func TestTemplateCascade(t *testing.T) {
	for _, tc := range []struct {
		name         string
		sheets       []string
		inline, want string
	}{
		{"specificity", []string{`#frame {color:red} .frame {color:blue}`}, "", "red"},
		{"same rule duplicates", []string{`.frame {color:red;color:blue}`}, "", "blue"},
		{"rule order", []string{`.frame {color:red}.frame {color:blue}`}, "", "blue"},
		{"sheet order", []string{`.frame {color:red}`, `.frame {color:blue}`}, "", "blue"},
		{"sheet order keeps specificity", []string{`#frame {color:red}`, `.frame {color:blue}`}, "", "red"},
		{"nonmatching branch", []string{`#other,div {color:red} .frame {color:blue}`}, "", "blue"},
		{"normal inline", []string{`#frame {color:red}`}, "color:green", "green"},
		{"important sheet", []string{`.frame {color:red!important}`}, "color:green", "red"},
		{"important inline", []string{`#frame {color:red!important}`}, "color:green!important", "green"},
		{"duplicate inline importance", nil, "color:green!important;color:blue", "green"},
		{"equal inline importance", nil, "color:green!important;color:blue!important", "blue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var head strings.Builder
			for _, sheet := range tc.sheets {
				fmt.Fprintf(&head, "<style>%s</style>", sheet)
			}
			input := fmt.Sprintf(`<head>%s</head><body><template id="a" data-type="page"><div><div id="frame" class="frame" style="%s"></div></div></template>`, head.String(), tc.inline)
			styled := styleCollectionOK(t, parseCollectionOK(t, input)).templates[0]
			wantProperty(t, styled, styled.active.frames[0].element, "color", tc.want)
		})
	}
}

func TestStylingRetainsPreCascadeDeclarations(t *testing.T) {
	c := parseCollectionOK(t, `<head><style>.unused {width:1px;width:2px} .frame {width:3px!important;width:4px}</style></head><body><template id="a" data-type="page"><div><div class="frame" style="color:red;color:blue"></div></div></template>`)
	s := styleCollectionOK(t, c)
	if len(s.styles.sheets[0].inspection.Rules) != 2 || len(s.styles.sheets[0].inspection.Rules[0].Declarations) != 2 {
		t.Fatal("lost nonmatching or losing declarations")
	}
	decls := s.styles.sheets[0].inspection.Rules[1].Declarations
	if decls[0].Value != "3px" || !decls[0].Important || decls[1].Value != "4px" {
		t.Fatal("source declarations reduced too early")
	}
	frame := s.templates[0].active.frames[0].element
	ref := s.templates[0].active.sources[frame]
	if ds := s.styles.inline[ref.nodeID]; len(ds) != 2 || ds[0].Value != "red" || ds[1].Value != "blue" {
		t.Fatal("inline provenance missing")
	}
	wantProperty(t, s.templates[0], frame, "width", "3px")
}

func TestStylingDoesNotCompileGeometry(t *testing.T) {
	for _, input := range []string{
		`<body><template id="blank" data-type="page"><div></div></template>`,
		`<body><template id="a" data-type="page"><div style="width:50%"><div style="position:relative;width:10em"></div></div></template>`,
	} {
		s := styleCollectionOK(t, parseCollectionOK(t, input))
		page := s.templates[0]
		wantProperty(t, page, page.active.page, "display", "block")
		if len(page.active.frames) > 0 {
			wantProperty(t, page, page.active.frames[0].element, "width", "10em")
		}
	}
}

func TestStylingRejectsCSSFailures(t *testing.T) {
	for _, tc := range []struct{ name, head, inline, code string }{
		{"invalid unmatched selector", `<style>.unmatched[ {width:10px}</style>`, "", "css.selector-syntax"},
		{"missing rule block", `<style>.frame;</style>`, "", "css.rule-syntax"},
		{"malformed declaration", `<style>.frame { width }</style>`, "", "css.declaration-syntax"},
		{"unclosed block", `<style>.frame { width:10px</style>`, "", "css.rule-syntax"},
		{"conditional rule", `<style>@media print {.frame {width:10px}}</style>`, "", "css.unsupported-rule"},
		{"import rule", `<style>@import "never-fetch.css";</style>`, "", "css.unsupported-rule"},
		{"conditional sheet", `<style media="print">.frame {width:10px}</style>`, "", "css.style-attribute"},
		{"non-CSS sheet", `<style type="text/less"></style>`, "", "css.style-attribute"},
		{"scoped sheet", `<style scoped></style>`, "", "css.style-attribute"},
		{"inline syntax", "", "color red", "css.declaration-syntax"},
		{"inline display failure", "", "display:bogus", "css.invalid-value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := parseCollectionOK(t, fmt.Sprintf(`<head>%s</head><body><template id="a" data-type="page"><div><div class="frame" style="%s"></div></div></template>`, tc.head, tc.inline))
			s, err := styleTemplateCollection(c)
			if s != nil || err == nil {
				t.Fatalf("published partial collection: %v, %v", s, err)
			}
			ds := errorDiagnostics(t, err)
			if !slices.ContainsFunc(ds, func(d diagnostic) bool { return d.code == diagnosticCode(tc.code) }) {
				t.Fatalf("missing %s: %v", tc.code, err)
			}
			for _, d := range ds {
				if d.stage != stageCSS || d.source.path == "" || d.source.nodeID == 0 {
					t.Fatal("missing CSS source context")
				}
				if tc.inline != "" && (!d.css.inline || d.source.templateID != "a") {
					t.Fatal("missing inline template context")
				}
			}
		})
	}
}

func TestCSSDiagnosticOrderAndCause(t *testing.T) {
	c := parseCollectionOK(t, `<html style="bad"><head><style>.unused[ {width:10px}</style></head><body><template id="a" data-type="page"><div><div style="bad; also-bad"></div></div></template><template id="b" data-type="page"><div style="color:"></div></template>`)
	var first []diagnostic
	for run := 0; run < 2; run++ {
		s, err := styleTemplateCollection(c)
		if s != nil || err == nil {
			t.Fatal("expected atomic failure")
		}
		ds := errorDiagnostics(t, err)
		if len(ds) != 5 || ds[0].source.path != "/html[1]" || ds[1].css.rule != 1 || ds[2].source.templateID != "a" || ds[3].css.declaration != 2 || ds[4].source.templateID != "b" {
			t.Fatalf("wrong order/association: %+v", ds)
		}
		var cause cssom.Diagnostic
		if !errors.As(ds[1], &cause) || cause.Code != "css.selector-syntax" || errors.Unwrap(cause) == nil {
			t.Fatal("CSS parser cause lost")
		}
		if !strings.Contains(err.Error(), "CSS 1:") || !strings.Contains(err.Error(), "inline style") || !strings.Contains(err.Error(), `template "b"`) {
			t.Fatal("incomplete human diagnostic")
		}
		if run == 0 {
			first = ds
		} else if !reflect.DeepEqual(first, ds) {
			t.Fatal("unstable diagnostics")
		}
	}
}

func styleCollectionOK(t *testing.T, c *templateCollection) *styledCollection {
	t.Helper()
	s, err := styleTemplateCollection(c)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func wantProperty(t *testing.T, s *styledTemplate, node *html.Node, key, want string) {
	t.Helper()
	got, err := css.GetProperty(s.nodes[node], key)
	if err != nil || string(got) != want {
		t.Fatalf("%s: %s = %q, %v; want %q", s.active.sources[node].path, key, got, err, want)
	}
}

func walkHTML(n *html.Node, f func(*html.Node)) {
	f(n)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, f)
	}
}

func renderHTML(t *testing.T, n *html.Node) string {
	t.Helper()
	var b bytes.Buffer
	if err := html.Render(&b, n); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
