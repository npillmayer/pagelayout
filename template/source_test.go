package template

import (
	"errors"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TestCollectionDiscovery(t *testing.T) {
	c := readCollectionFixture(t, "two-pages.html")
	if got := templateIDs(c); !slices.Equal(got, []string{"two-column-article", "chapter-opening"}) {
		t.Fatalf("definition order = %v", got)
	}
	if len(c.byID) != 2 || c.byID["widget"] != nil || c.byID["Two-column-article"] != nil {
		t.Fatalf("unexpected template index: %v", c.byID)
	}
	for i, definition := range c.templates {
		if c.byID[definition.id] != definition {
			t.Fatal("ordered definitions and lookup must identify the same source")
		}
		if definition.element.Parent != c.body || definition.page.Parent != definition.element {
			t.Fatal("discovery reparented source nodes")
		}
		wantFrames := []int{3, 2}[i]
		if len(definition.frames) != wantFrames {
			t.Fatalf("%s: frames = %d, want %d", definition.id, len(definition.frames), wantFrames)
		}
		for j, frame := range definition.frames {
			if frame.id != frameID(j+1) || frame.element.Parent != definition.page {
				t.Fatalf("%s: incorrect frame identity or parent: %+v", definition.id, frame)
			}
			ref := c.sources[frame.element]
			if ref.templateID != definition.id || ref.name != "two-pages.html" || ref.nodeID == 0 {
				t.Fatalf("frame source = %+v", ref)
			}
		}
	}
	left, _ := attribute(c.templates[0].frames[1].element, "class")
	right, _ := attribute(c.templates[0].frames[2].element, "class")
	if left != "frame body body-left" || right != "frame body body-right" {
		t.Fatalf("frame source order changed: %q then %q", left, right)
	}
	if len(c.styles) != 2 || c.styles[0].Parent != c.head || c.styles[1].Parent != c.head {
		t.Fatal("head styles were not retained in their source context")
	}
	if !strings.Contains(c.styles[0].FirstChild.Data, "210mm") || !strings.Contains(c.styles[1].FirstChild.Data, "article-flow") {
		t.Fatal("stylesheet text or order changed")
	}
	if lang, _ := attribute(c.html, "lang"); lang != "en" {
		t.Fatalf("lost collection html attributes: %q", lang)
	}
	if class, _ := attribute(c.body, "class"); class != "print" {
		t.Fatalf("lost collection body attributes: %q", class)
	}
}

func TestCollectionSourcePaths(t *testing.T) {
	c := readCollectionFixture(t, "two-pages.html")
	checks := []struct {
		node *html.Node
		path string
	}{
		{c.document, "/"},
		{c.html, "/html[1]"},
		{c.head, "/html[1]/head[1]"},
		{c.body, "/html[1]/body[1]"},
		{c.styles[1], "/html[1]/head[1]/style[2]"},
		{c.templates[0].element, "/html[1]/body[1]/template[2]"},
		{c.templates[0].page, "/html[1]/body[1]/template[2]/div[1]"},
		{c.templates[0].frames[2].element, "/html[1]/body[1]/template[2]/div[1]/div[3]"},
		{c.templates[1].frames[0].element, "/html[1]/body[1]/template[3]/div[1]/div[1]"},
	}
	for _, check := range checks {
		if got := c.sources[check.node].path; got != check.path {
			t.Errorf("source path = %q, want %q", got, check.path)
		}
	}
	assertSourceIndex(t, c)
	// Parsing the same source twice preserves identities and paths without
	// sharing mutable HTML storage between the two collections.
	other := readCollectionFixture(t, "two-pages.html")
	byID := make(map[sourceNodeID]sourceRef)
	for _, ref := range other.sources {
		byID[ref.nodeID] = ref
	}
	for node, ref := range c.sources {
		if other.sources[node].nodeID != 0 || byID[ref.nodeID] != ref {
			t.Fatalf("source identity was unstable or storage shared: %+v", ref)
		}
	}
}

func TestElementPathsIgnoreFormatting(t *testing.T) {
	a := parseCollectionOK(t, `<body><template id="a" data-type="page"><div><div></div><div></div></div></template>`)
	b := parseCollectionOK(t, "<body>\n<!-- comment -->\n<template id=\"a\" data-type=\"page\">\n<div>\n<div></div><!-- gap --><div></div>\n</div></template>")
	for i, f := range a.templates[0].frames {
		if a.sources[f.element].path != b.sources[b.templates[0].frames[i].element].path {
			t.Fatal("indentation/comments changed an element's path")
		}
	}
}

func TestCollectionStructuralProfileAccepted(t *testing.T) {
	tests := []struct {
		name string
		html string
		ids  []string
	}{
		{"classless blank page", `<body><template data-type="page" id="blank"><div></div></template>`, []string{"blank"}},
		{"classless frames", `<body><template data-type="page" id="plain"><div><div></div><div></div></div></template>`, []string{"plain"}},
		{"case-sensitive ids", `<body><template data-type="page" id="A"><div></div></template><template data-type="page" id="a"><div></div></template>`, []string{"A", "a"}},
		{"unicode ids", `<body><template data-type="page" id="章&#160;一"><div></div></template>`, []string{"章\u00a0一"}},
		{"HTML name case", `<BODY><TEMPLATE DATA-TYPE="page" ID="A"><DIV><DIV></DIV></DIV></TEMPLATE>`, []string{"A"}},
		{"storage wrapper excluded from active ids", `<body><template data-type="page" id="a"><div id="a"><div></div></div></template>`, []string{"a"}},
		{"ignored inert storage", `<body><template id="widget"><style>invalid css</style><script>not executed</script><p>ignored</p></template><template data-type="page" id="a"><div></div></template>`, []string{"a"}},
		{"other template kind is case-sensitive", `<body><template data-type="PAGE"><p>ignored</p></template><template data-type="page" id="a"><div></div></template>`, []string{"a"}},
		{"formatting and empty comment", "<body>\n<template id=\"a\" data-type=\"page\">\t<!-- -->\n<div>\n<div>\t\r\n\f <!-- frame --> </div>\n</div> </template>", []string{"a"}},
		{"duplicate ids across definitions", `<body><template data-type="page" id="a"><div id="page"><div id="frame"></div></div></template><template data-type="page" id="b"><div id="page"><div id="frame"></div></div></template>`, []string{"a", "b"}},
		{"head ids are not active context", `<head><title id="page">Title</title></head><body><template data-type="page" id="a"><div id="page"></div></template>`, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := parseCollectionOK(t, tt.html)
			if !slices.Equal(templateIDs(c), tt.ids) {
				t.Fatalf("ids = %v, want %v", templateIDs(c), tt.ids)
			}
			assertSourceIndex(t, c)
		})
	}
}

func TestCollectionStructuralProfileRejected(t *testing.T) {
	valid := `<template data-type="page" id="good"><div></div></template>`
	tests := []struct {
		name string
		html string
		code diagnosticCode
	}{
		{"empty input", ``, codeEmptyCollection},
		{"only unmarked templates", `<body><template><div></div></template>`, codeEmptyCollection},
		{"missing template id", `<body><template data-type="page"><div></div></template>`, codeMissingTemplateID},
		{"empty template id", `<body><template data-type="page" id=""><div></div></template>`, codeMissingTemplateID},
		{"whitespace template id", `<body><template data-type="page" id="a&#9;b"><div></div></template>`, codeInvalidTemplateID},
		{"duplicate template id", `<body>` + valid + valid, codeDuplicateTemplateID},
		{"marked template in head", `<head><template data-type="page" id="a"><div></div></template></head><body>`, codeTemplateLocation},
		{"nested marked template", `<body><section><template data-type="page" id="a"><div></div></template></section>`, codeTemplateLocation},
		{"marked template inside ignored storage", `<body><template><template data-type="page" id="a"><div></div></template></template>`, codeTemplateLocation},
		{"marked template inside frame", `<body><template data-type="page" id="a"><div><div><template data-type="page" id="b"><div></div></template></div></div></template>`, codeTemplateLocation},
		{"missing page", `<body><template data-type="page" id="a"><!-- blank --></template>`, codeMissingPage},
		{"multiple pages", `<body><template data-type="page" id="a"><div></div><div></div></template>`, codeMultiplePages},
		{"page must be div", `<body><template data-type="page" id="a"><section></section></template>`, codePageElement},
		{"template text", `<body><template data-type="page" id="a">text<div></div></template>`, codeTemplateContent},
		{"page text", `<body><template data-type="page" id="a"><div>text</div></template>`, codePageContent},
		{"frame must be div", `<body><template data-type="page" id="a"><div><span></span></div></template>`, codeFrameElement},
		{"frame text", `<body><template data-type="page" id="a"><div><div>text</div></div></template>`, codeFrameContent},
		{"nonbreaking space is content", `<body><template data-type="page" id="a"><div><div>&nbsp;</div></div></template>`, codeFrameContent},
		{"nested frame", `<body><template data-type="page" id="a"><div><div><div></div></div></div></template>`, codeFrameContent},
		{"frame element content", `<body><template data-type="page" id="a"><div><div><img src="x"></div></div></template>`, codeFrameContent},
		{"foreign page root", `<body><template data-type="page" id="a"><svg></svg></template>`, codePageElement},
		{"foreign content repair creates two roots", `<body><template data-type="page" id="a"><svg><div/></svg></template>`, codeMultiplePages},
		{"foreign frame", `<body><template data-type="page" id="a"><div><svg></svg></div></template>`, codeFrameElement},
		{"body text", `<body>text` + valid, codeBodyContent},
		{"body element", `<body><div></div>` + valid, codeBodyContent},
		{"body script", `<body><script>throw 'never executed'</script>` + valid, codeBodyContent},
		{"head script", `<head><script>throw 'never executed'</script></head><body>` + valid, codeHeadContent},
		{"external sheet", `<head><link rel="stylesheet" href="missing.css"></head><body>` + valid, codeHeadContent},
		{"base URL", `<head><base href="https://example.invalid/"></head><body>` + valid, codeHeadContent},
		{"body stylesheet", `<body><style>.page{}</style>` + valid, codeBodyContent},
		{"template stylesheet", `<body><template data-type="page" id="a"><style>.page{}</style><div></div></template>`, codeMultiplePages},
		{"page stylesheet", `<body><template data-type="page" id="a"><div><style>.frame{}</style></div></template>`, codeFrameElement},
		{"empty element id", `<body><template data-type="page" id="a"><div id=""></div></template>`, codeInvalidElementID},
		{"invalid element id", `<body><template data-type="page" id="a"><div><div id="a b"></div></div></template>`, codeInvalidElementID},
		{"duplicate frame ids", `<body><template data-type="page" id="a"><div><div id="f"></div><div id="f"></div></div></template>`, codeDuplicateElementID},
		{"page/frame duplicate ids", `<body><template data-type="page" id="a"><div id="p"><div id="p"></div></div></template>`, codeDuplicateElementID},
		{"body/page duplicate ids", `<body id="p"><template data-type="page" id="a"><div id="p"></div></template>`, codeDuplicateElementID},
		{"html/body duplicate ids", `<html id="same"><body id="same">` + valid, codeDuplicateElementID},
		{"invalid context id", `<html id="a b"><body>` + valid, codeInvalidElementID},
		{"frameset document", `<html><head></head><frameset><frame src="x"></frameset></html>`, codeDocumentStructure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := parseTemplateCollection(strings.NewReader(tt.html), "invalid.html")
			if err == nil || c != nil {
				t.Fatalf("must reject without partial collection: collection=%v, error=%v", c, err)
			}
			diagnostics := errorDiagnostics(t, err)
			if !slices.ContainsFunc(diagnostics, func(d diagnostic) bool { return d.code == tt.code }) {
				t.Fatalf("missing diagnostic %s:\n%v", tt.code, err)
			}
			for _, d := range diagnostics {
				if d.source.name != "invalid.html" || d.source.path == "" || d.source.nodeID == 0 || d.stage == stageParse {
					t.Fatalf("incomplete structural diagnostic: %+v", d)
				}
			}
		})
	}
}

func TestCollectionHTMLRepair(t *testing.T) {
	c := readCollectionFixture(t, "repaired.html")
	if len(c.templates) != 1 || len(c.templates[0].frames) != 1 {
		t.Fatalf("unexpected repaired shape: %+v", c.templates)
	}
	template := c.templates[0]
	if !isElement(c.html, atom.Html) || !isElement(c.head, atom.Head) || !isElement(c.body, atom.Body) {
		t.Fatal("parser did not supply the omitted document structure")
	}
	// x/net/html stores these nodes as template children, not a browser-style
	// DocumentFragment. A closing template repairs the unclosed DIVs.
	if template.page.Parent != template.element || template.frames[0].element.Parent != template.page {
		t.Fatal("unexpected template-content representation or repair")
	}
	if got := c.sources[template.frames[0].element].path; got != "/html[1]/body[1]/template[1]/div[1]/div[1]" {
		t.Fatalf("repaired path = %q", got)
	}
	// A template before an explicit body is parsed into head. Discovery must
	// reject that location rather than treating the input as a body fragment.
	_, err := parseTemplateCollection(strings.NewReader(`<template id="a" data-type="page"><div></div></template>`), "implicit.html")
	if !slices.ContainsFunc(errorDiagnostics(t, err), func(d diagnostic) bool { return d.code == codeTemplateLocation }) {
		t.Fatalf("head template was not diagnosed: %v", err)
	}
}

func TestCollectionDoesNotInterpretCSS(t *testing.T) {
	input := `<html style="color: inherited-later"><head><style>@unsupported { ??? }</style></head><body style="width: undecided"><template id="a" data-type="page"><div style="width: bogus"><div style="position: relative; flow-from: none"></div></div></template></body></html>`
	c := parseCollectionOK(t, input)
	if c.styles[0].FirstChild.Data != "@unsupported { ??? }" {
		t.Fatal("source CSS was changed")
	}
	if style, _ := attribute(c.templates[0].frames[0].element, "style"); style != "position: relative; flow-from: none" {
		t.Fatalf("inline source CSS changed: %q", style)
	}
}

func TestCollectionErrorOrderAndAtomicPublication(t *testing.T) {
	input := `<head><script>not executed</script></head><body>
<template data-type="page" id="good"><div><div></div></div></template>
<template data-type="page" id="bad"><div><div>content</div></div></template>
unexpected body text
<template data-type="page" id="good"><div></div></template>
</body>`
	var first []diagnostic
	for i := 0; i < 3; i++ {
		c, err := parseTemplateCollection(strings.NewReader(input), "ordered.html")
		if c != nil || err == nil {
			t.Fatalf("invalid collection was published: %v, %v", c, err)
		}
		diagnostics := errorDiagnostics(t, err)
		var codes []diagnosticCode
		for j, d := range diagnostics {
			codes = append(codes, d.code)
			if j > 0 && diagnostics[j-1].source.nodeID > d.source.nodeID {
				t.Fatal("diagnostics are not in document order")
			}
		}
		want := []diagnosticCode{codeHeadContent, codeFrameContent, codeBodyContent, codeDuplicateTemplateID}
		if !slices.Equal(codes, want) {
			t.Fatalf("diagnostic order = %v, want %v", codes, want)
		}
		if diagnostics[1].source.templateID != "bad" || diagnostics[3].source.templateID != "good" {
			t.Fatal("diagnostics lost template identities")
		}
		if i == 0 {
			first = diagnostics
		} else if !reflect.DeepEqual(first, diagnostics) {
			t.Fatal("diagnostics changed across identical parses")
		}
	}
}

func TestCollectionLoadsAreIndependent(t *testing.T) {
	first := parseCollectionOK(t, `<body><template data-type="page" id="same"><div><div id="original"></div></div></template>`)
	second := parseCollectionOK(t, `<body><template data-type="page" id="same"><div><div id="new"></div><div></div></div></template>`)
	if _, err := parseTemplateCollection(strings.NewReader(`<body><template data-type="page" id="same"><div><div>bad</div></div></template>`), "later.html"); err == nil {
		t.Fatal("expected failing later load")
	}
	if first.byID["same"] == second.byID["same"] || len(first.byID["same"].frames) != 1 {
		t.Fatal("later parsing altered an earlier collection")
	}
	if id, _ := attribute(first.byID["same"].frames[0].element, "id"); id != "original" {
		t.Fatal("earlier source was overwritten")
	}
}

func TestCollectionInputErrors(t *testing.T) {
	c, err := parseTemplateCollection(nil, "nil.html")
	if c != nil || err == nil || errorDiagnostics(t, err)[0].code != codeNilInput {
		t.Fatalf("nil reader result = %v, %v", c, err)
	}
	broken := errors.New("reader failed")
	// Fail only after a complete-looking definition: no successful subset may
	// be published when reading the rest of the source fails.
	r := io.MultiReader(strings.NewReader(`<body><template data-type="page" id="good"><div></div></template>`), failingReader{broken})
	c, err = parseTemplateCollection(r, "broken.html")
	if c != nil || !errors.Is(err, broken) {
		t.Fatalf("reader failure or partial publication: %v, %v", c, err)
	}
	d := errorDiagnostics(t, err)[0]
	if d.code != codeParseFailure || d.stage != stageParse || d.source.name != "broken.html" || d.source.nodeID != 0 || d.source.path != "" {
		t.Fatalf("bad parse diagnostic: %+v", d)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func readCollectionFixture(t *testing.T, name string) *templateCollection {
	t.Helper()
	f, err := os.Open("testdata/templates/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := parseTemplateCollection(f, name)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func parseCollectionOK(t *testing.T, input string) *templateCollection {
	t.Helper()
	c, err := parseTemplateCollection(strings.NewReader(input), "test.html")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func errorDiagnostics(t *testing.T, err error) []diagnostic {
	t.Helper()
	var failure *loadError
	if !errors.As(err, &failure) {
		t.Fatalf("expected structured load error, got %T: %v", err, err)
	}
	return failure.diagnostics()
}

func templateIDs(c *templateCollection) []string {
	ids := make([]string, len(c.templates))
	for i, definition := range c.templates {
		ids[i] = definition.id
	}
	return ids
}

func assertSourceIndex(t *testing.T, c *templateCollection) {
	t.Helper()
	var previous sourceNodeID
	seenPaths := make(map[string]bool)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		ref, ok := c.sources[n]
		if !ok || ref.nodeID <= previous || seenPaths[ref.path] {
			t.Fatalf("missing, unordered or duplicate source identity: %+v", ref)
		}
		previous = ref.nodeID
		seenPaths[ref.path] = true
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(c.document)
	if len(seenPaths) != len(c.sources) {
		t.Fatal("source index contains nodes outside the source document")
	}
}
