package content

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
)

func partition(t *testing.T, src string, opts LoadOptions) (*sourceProjection, *partitionProjection) {
	t.Helper()
	p := projection(t, src, opts)
	r, err := partitionSource(p)
	if err != nil {
		t.Fatal(err)
	}
	assertCoverage(t, p, r)
	return p, r
}

func assertCoverage(t *testing.T, p *sourceProjection, r *partitionProjection) {
	t.Helper()
	seen := map[sourceID]bool{}
	var walk func(*layout.RenderNode, sourceID)
	walk = func(n *layout.RenderNode, owner sourceID) {
		if n == nil {
			return
		}
		id := sourceID(n.ID)
		if seen[id] {
			t.Fatalf("duplicate %d", id)
		}
		seen[id] = true
		actual, ok := r.owners[id]
		if !ok || actual != owner {
			t.Fatalf("wrong owner %d: %d", id, actual)
		}
		original := p.nodes[id]
		if original == n || !reflect.DeepEqual(n.Style, original.Style) || !reflect.DeepEqual(n.Text, original.Text) || !reflect.DeepEqual(n.HTML, original.HTML) {
			t.Fatalf("changed source inputs or aliased node %d", id)
		}
		for _, c := range n.ChildrenNodes {
			walk(c, owner)
		}
	}
	walk(r.residual, 0)
	for _, b := range r.branches {
		if b.root == nil || sourceID(b.root.ID) != b.source {
			t.Fatal("missing branch root")
		}
		walk(b.root, b.source)
	}
	for _, s := range p.sources[1:] {
		_, owned := r.owners[s.ref.nodeID]
		if seen[s.ref.nodeID] != (s.participation == materialNode) || owned != seen[s.ref.nodeID] {
			t.Fatalf("coverage failure: %+v", s)
		}
	}
	if !reflect.DeepEqual(p.sources, r.sources) {
		t.Fatal("source ledger changed")
	}
}

func TestRouteAcceptanceAndOverrides(t *testing.T) {
	for _, override := range []bool{false, true} {
		opts := LoadOptions{}
		channel := "article"
		if override {
			channel = "features"
			opts.Stylesheets = []Stylesheet{{SourceName: "routing-override.css", CSS: fixture(t, "routing-override.css")}}
		}
		p, r := partition(t, fixture(t, "routing.html"), opts)
		if len(r.channels) != 2 || r.channels[0].name != channel || r.channels[1].name != "notes" || !reflect.DeepEqual(r.channels[0].branches, []int{0, 2}) || !reflect.DeepEqual(r.channels[1].branches, []int{1}) {
			t.Fatalf("channels: %+v", r.channels)
		}
		for i, name := range []string{"story-a", "note", "story-b"} {
			id := projectedID(t, p, name)
			if r.branches[i].source != id {
				t.Fatal("branch order")
			}
		}
		for _, name := range []string{"source", "intro", "ending"} {
			if r.owners[projectedID(t, p, name)] != 0 {
				t.Fatal("lost residual", name)
			}
		}
		for _, pair := range [][2]string{{"paragraph-a", "story-a"}, {"tag", "story-a"}, {"paragraph-b", "story-b"}} {
			if r.owners[projectedID(t, p, pair[0])] != projectedID(t, p, pair[1]) {
				t.Fatal("lost descendant ownership")
			}
		}
		// Partition twice without changing the original, and mutate one private
		// result to prove render/style/text/ledger storage is independent.
		again, err := partitionSource(p)
		if err != nil {
			t.Fatal(err)
		}
		r.branches[0].root.Style.FontSizePx = 999
		r.branches[0].root.HTML.Data = "changed"
		r.sources[projectedID(t, p, "story-a")].attributes[0].Val = "changed"
		r.residual.ChildrenNodes = nil
		assertCoverage(t, p, again)
	}
}

func TestRouteResidualAndExcludedSources(t *testing.T) {
	p, r := partition(t, fixture(t, "residual-inline.html"), LoadOptions{})
	if len(r.channels) != 1 || r.channels[0].name != "notes" || len(r.branches) != 2 {
		t.Fatal("notes order")
	}
	for i, name := range []string{"aside", "moved"} {
		if r.branches[i].source != projectedID(t, p, name) {
			t.Fatal("branch order")
		}
	}
	var find func(*layout.RenderNode, sourceID) *layout.RenderNode
	find = func(n *layout.RenderNode, id sourceID) *layout.RenderNode {
		if n == nil {
			return nil
		}
		if sourceID(n.ID) == id {
			return n
		}
		for _, c := range n.ChildrenNodes {
			if v := find(c, id); v != nil {
				return v
			}
		}
		return nil
	}
	split := find(r.residual, projectedID(t, p, "split"))
	if len(split.ChildrenNodes) != 2 || split.ChildrenNodes[0].HTML.Data != "Before " || split.ChildrenNodes[1].HTML.Data != " after." {
		t.Fatal("residual text order/bytes")
	}
	empty := find(r.residual, projectedID(t, p, "emptied"))
	if empty == nil || len(empty.ChildrenNodes) != 0 || empty.Style.Padding.Left.Value != 16 || empty.Style.Border.Left.Value != 1 {
		t.Fatal("emptied container lost")
	}
	for _, file := range []string{"suppression.html", "empty.html"} {
		_, r := partition(t, fixture(t, file), LoadOptions{})
		if len(r.channels) != 0 || len(r.branches) != 0 || r.residual == nil {
			t.Fatal("unexpected routes", file)
		}
	}
	for _, src := range []string{`<body style="display:none;flow-into:hidden"><div style="flow-into:nested">x</div>`, `<html style="display:none;flow-into:hidden">x`} {
		_, r := partition(t, src, LoadOptions{})
		if r.residual != nil || len(r.owners) != 0 || len(r.channels) != 0 {
			t.Fatal("suppressed document material")
		}
	}
	p, r = partition(t, `<section id="root" style="flow-into:Orphan;display:flow-root"><p id="child" style="flow-into:none">x</p><span style="display:inline-block">atomic</span></section><section style="flow-into:orphan">y</section>`, LoadOptions{})
	if r.owners[projectedID(t, p, "child")] != projectedID(t, p, "root") || len(r.channels) != 2 || r.channels[0].name != "Orphan" || r.channels[1].name != "orphan" {
		t.Fatal("none escaped ownership or names folded")
	}
}

func TestRouteFailuresAreAtomicAndLocated(t *testing.T) {
	for _, tc := range []struct{ src, code string }{
		{fixture(t, "nested-flow_err.html"), "route.nested"},
		{`<div style="flow-into:a"><div style="flow-into:a">x</div></div>`, "route.nested"},
		{`<div style="flow-into:a"><div style="flow-into:inherit">x</div></div>`, "route.nested"},
		{`<html style="flow-into:a">x`, "route.root"},
		{`<body style="flow-into:a">x`, "route.root"},
		{`<span style="flow-into:a">x</span>`, "route.display"},
		{`<span style="display:inline-block;flow-into:a">x</span>`, "route.display"},
		{`<span><section style="flow-into:a">x</section></span>`, "route.ancestry"},
		{`<div style="display:inline-block"><section style="flow-into:a">x</section></div>`, "route.ancestry"},
	} {
		r, err := loadPartition(strings.NewReader(tc.src), LoadOptions{SourceName: "routes.html"})
		var e *loadError
		if r != nil || !errors.As(err, &e) || len(e.items) == 0 {
			t.Fatalf("partial or missing failure: %v", err)
		}
		d := e.items[0]
		if d.stage != "routing" || d.code != tc.code || d.source.name != "routes.html" || d.source.path == "" || d.css.property != "flow-into" || d.css.value == "" {
			t.Fatalf("diagnostic: %+v", d)
		}
		if (tc.code == "route.nested" || tc.code == "route.ancestry") && (d.related.nodeID == 0 || d.related.path == "" || d.related.name != "routes.html") {
			t.Fatal("missing conflicting source", d)
		}
		if tc.src == fixture(t, "nested-flow_err.html") {
			for _, word := range []string{"#inner", "#outer", "notes", "article"} {
				if !strings.Contains(err.Error(), word) {
					t.Fatal("missing conflict", err)
				}
			}
		}
	}
	// Earlier phases remain authoritative, and no partial partition escapes.
	r, err := loadPartition(strings.NewReader(fixture(t, "float_err.html")), LoadOptions{})
	var e *loadError
	if r != nil || !errors.As(err, &e) || e.items[0].stage != "profile" {
		t.Fatal(err)
	}
}

func TestRouteCoverageGuardAndConcurrentPartitions(t *testing.T) {
	p := projection(t, `<div style="flow-into:one">a</div><div style="flow-into:two">b</div>`, LoadOptions{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			r, err := partitionSource(p)
			if err != nil || len(r.branches) != 2 {
				t.Errorf("concurrent partition: %v", err)
			}
		})
	}
	wg.Wait()
	// Corrupt only a private test projection: omission must not publish material.
	p.root.ChildrenNodes[0].ChildrenNodes = nil
	r, err := partitionSource(p)
	if r != nil || err == nil {
		t.Fatal("lost source accepted")
	}
}
