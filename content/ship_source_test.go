package content

import (
	"reflect"
	"strings"
	"testing"
)

func TestDefaultMainAndRedundantRouting(t *testing.T) {
	const input = `<body style="hyphens:none"><p id="before">Before</p><section id="a" STYLE><p>A</p></section><aside style="flow-into:notes">Note</aside><section id="b" STYLE>B</section></body>`
	var texts []string
	for _, directive := range []string{"", `style="flow-into:main"`} {
		d := loaded(t, strings.ReplaceAll(input, "STYLE", directive), LoadOptions{})
		if !reflect.DeepEqual(d.Channels(), []string{"main", "notes"}) {
			t.Fatal(d.Channels())
		}
		branches := d.Branches("main")
		first, rest, ok := d.Bank().Take("main")
		if !ok || len(branches) != 1 || first != d.Residual().Root || branches[0].Root != first || rest.Len() != 1 || d.Bank().Len() != 2 {
			t.Fatal("main alias/bank")
		}
		if len(d.material.trees) != 3 {
			t.Fatal("duplicated trees")
		}
		var got []string
		for _, g := range d.Groups(branches[0].Tree) {
			got = append(got, extractKnownGroup(t, g.Group).Text)
		}
		if texts == nil {
			texts = got
		} else if !reflect.DeepEqual(texts, got) {
			t.Fatal(texts, got)
		}
		for _, id := range []string{"before", "a", "b"} {
			s := materialSource(t, d, id)
			if s.Channel != "main" || s.Tree != branches[0].Tree {
				t.Fatal(s)
			}
		}
		backend := &mockPreparer{}
		_, session, err := Prepare(d, backend)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		seen := map[Box]bool{}
		for _, b := range backend.calls {
			if seen[b] {
				t.Fatal("prepared twice")
			}
			seen[b] = true
		}
		if len(seen) != len(d.Groups(d.Residual().Tree))+len(d.Groups(d.Branches("notes")[0].Tree)) {
			t.Fatal("missing preparation")
		}
	}
	for _, source := range []string{`<body style="flow-into:main">x`, `<div style="flow-into:main"><div style="flow-into:main">x</div></div>`, `<div style="flow-into:notes"><div style="flow-into:main">x</div></div>`, `<div style="flow-into:main"><div style="flow-into:notes">x</div></div>`, `<span style="flow-into:main">x</span>`} {
		if d, err := Load(strings.NewReader(source), LoadOptions{}); d != nil || err == nil {
			t.Fatal("invalid route accepted", source)
		}
	}
	d := loaded(t, `<body style="display:none">hidden`, LoadOptions{})
	if !reflect.DeepEqual(d.Channels(), []string{"main"}) || len(d.Branches("main")) != 0 || d.Bank().Len() != 0 {
		t.Fatal("suppressed main")
	}
}

func TestSourceEventsAndBreakCascade(t *testing.T) {
	for _, value := range []string{"page", "left", "right", "recto", "verso"} {
		d := loaded(t, `<body><p id="p" style="break-before:`+value+`">A<a id="owner" href="#note" data-flow-anchor></a> B<a href="#note">ordinary</a></p><aside id="note" style="flow-into:notes">Note</aside><div style="display:none;break-before:page"><a id="hidden" href="#missing" data-flow-anchor></a></div><template><a id="inert" data-flow-anchor></a></template>`, LoadOptions{})
		counts := map[SourceEventKind]int{}
		for _, event := range d.SourceEvents() {
			counts[event.Kind]++
			s, ok := d.NodeSource(event.Source.NodeID)
			if !ok || event.Parent != s.Parent || event.Source != s.Source {
				t.Fatal("source identity")
			}
			if event.Kind == SourcePageBreak && (event.Value != "page" || !event.Active || event.Channel != "main") {
				t.Fatal(event)
			}
			if event.Kind == SourceFlowAnchor {
				if s.Source == materialSource(t, d, "owner").Source {
					if !event.Active || event.Value != "#note" || len(s.Children) != 0 {
						t.Fatal(event)
					}
				} else if event.Active {
					t.Fatal("excluded anchor active")
				}
			}
		}
		if counts[SourcePageBreak] != 1 || counts[SourceFlowAnchor] != 3 || counts[SourceTargetID] != 5 {
			t.Fatal(counts)
		}
		events := d.SourceEvents()
		events[0].Value = "mutated"
		if d.SourceEvents()[0].Value == "mutated" {
			t.Fatal("aliased event slice")
		}
		if got := extractKnownGroup(t, d.Groups(d.Residual().Tree)[0].Group).Text; got != "A Bordinary" {
			t.Fatal("marker changed text", got)
		}
	}
	d := loaded(t, `<style>p {break-before:page !important}</style><body><p id="a" style="break-before:auto">A</p><div style="break-before:recto"><p id="b" style="break-before:inherit">B</p><p id="c" style="break-before:unset">C</p></div>`, LoadOptions{Stylesheets: []Stylesheet{{SourceName: "override.css", CSS: `p {break-before:auto !important} #b {break-before:inherit !important}`}}})
	if materialSource(t, d, "a").BreakBefore != "auto" || materialSource(t, d, "b").BreakBefore != "page" || materialSource(t, d, "c").BreakBefore != "auto" {
		t.Fatal("cascade/inherit")
	}
	for _, css := range []string{`p {break-before:avoid;break-before:page}`, `.missing {break-before:column}`, `p {break-before:revert}`, `p {page-break-before:always}`} {
		doc, err := Load(strings.NewReader(`<body style="display:none"><p>hidden</p>`), LoadOptions{Stylesheets: []Stylesheet{{SourceName: "invalid.css", CSS: css}}})
		if doc != nil || err == nil || !strings.Contains(err.Error(), "invalid.css") {
			t.Fatal("unaudited declaration", css, err)
		}
	}
	// Ownership resolution is deliberately deferred; raw unresolved hrefs survive.
	d = loaded(t, `<p><a data-flow-anchor href="#missing"> </a></p>`, LoadOptions{})
	if e := d.SourceEvents(); len(e) != 1 || e[0].Value != "#missing" || !e[0].Active {
		t.Fatal(e)
	}
	var zero *Document
	if zero.SourceEvents() != nil || (&Document{}).SourceEvents() != nil {
		t.Fatal("zero document")
	}
}
