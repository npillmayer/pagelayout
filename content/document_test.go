package content

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/css-box-layout/registers"
	boxtext "github.com/npillmayer/css-box-layout/text"
	"github.com/npillmayer/w3c-dom/dimen"
	"golang.org/x/net/html"
)

func loaded(t *testing.T, input string, opts LoadOptions) *Document {
	t.Helper()
	d, err := Load(strings.NewReader(input), opts)
	if err != nil || d == nil {
		t.Fatalf("Load = %v, %v", d, err)
	}
	return d
}

func materialSource(t *testing.T, d *Document, name string) SourceInfo {
	t.Helper()
	for _, s := range d.material.sources[1:] {
		for _, a := range s.Attributes {
			if a.Key == "id" && a.Val == name {
				v, _ := d.NodeSource(s.Source.NodeID)
				return v
			}
		}
	}
	t.Fatalf("source #%s not found", name)
	return SourceInfo{}
}

func checkGroups(t *testing.T, d *Document, tree TreeID, owners ...string) []GroupInfo {
	t.Helper()
	groups := d.Groups(tree)
	if len(groups) != len(owners) {
		t.Fatalf("tree %d: %d groups, want %v", tree, len(groups), owners)
	}
	for i, g := range groups {
		gi, gok := g.Group.Info()
		oi, ook := g.Owner.Info()
		if !gok || !ook || gi.Tree != tree || oi.Tree != tree || gi.Kind != layout.BoxAnonymousInline || gi.Source.NodeID != 0 || !layout.IsBlockContainer(oi.Kind) {
			t.Fatalf("invalid group/owner: %+v %+v", gi, oi)
		}
		want := materialSource(t, d, owners[i])
		if g.Source != want.Source || g.TextStyle != want.TextStyle || oi.Context != want.Source {
			t.Fatalf("lost owner context %s: %+v", owners[i], g)
		}
	}
	return groups
}

func TestLoadMaterialAcceptance(t *testing.T) {
	for _, override := range []bool{false, true} {
		opts := LoadOptions{SourceName: "routing.html"}
		channel := "article"
		font := dimen.DU(20 * dimen.PX)
		if override {
			opts.Stylesheets = []Stylesheet{{SourceName: "routing-override.css", CSS: fixture(t, "routing-override.css")}}
			channel, font = "features", 24*dimen.PX
		}
		d := loaded(t, fixture(t, "routing.html"), opts)
		if !reflect.DeepEqual(d.Channels(), []string{"main", channel, "notes"}) || d.Bank().Len() != 3 {
			t.Fatal("channel inventory")
		}
		branches := d.Branches(channel)
		notes := d.Branches("notes")
		if len(branches) != 2 || len(notes) != 1 || branches[0].Order != 2 || notes[0].Order != 3 || branches[1].Order != 4 {
			t.Fatal("global branch order")
		}
		r := d.Residual()
		all := []TreeInfo{r, {branches[0].Tree, branches[0].Root}, {notes[0].Tree, notes[0].Root}, {branches[1].Tree, branches[1].Root}}
		seen := make(map[TreeID]bool)
		for _, tree := range all {
			if seen[tree.Tree] || !tree.Root.Valid() {
				t.Fatal("invalid tree identities")
			}
			seen[tree.Tree] = true
			for _, n := range d.material.trees[tree.Tree].nodes {
				if n.NodeID != 0 {
					source, ok := d.NodeSource(n.NodeID)
					if !ok || source.Tree != tree.Tree || !source.HasTextStyle {
						t.Fatal("invalid source correspondence")
					}
				}
				if n.Box == layout.BoxText {
					text, ok := d.TextSource(n.Text.Source)
					if !ok || n.Text.Range.Start != 0 || n.Text.Range.End != uint64(len(text.Text)) {
						t.Fatal("invalid decoded text range")
					}
				}
			}
		}
		checkGroups(t, d, r.Tree, "intro", "ending")
		a := checkGroups(t, d, branches[0].Tree, "paragraph-a", "tag")
		checkGroups(t, d, notes[0].Tree, "note")
		checkGroups(t, d, branches[1].Tree, "paragraph-b")
		if len(a[0].Atomics) != 1 || a[0].Atomics[0] != a[1].Owner || a[1].ParentAtomic != a[1].Owner || a[0].ParentAtomic.Valid() {
			t.Fatal("atomic interior association")
		}
		atomic, _ := a[0].Atomics[0].Info()
		if atomic.Kind != layout.BoxInlineBlock || atomic.Constraints.Width.Kind != layout.LenAuto || !atomic.EstablishesBFC {
			t.Fatal("atomic was sized or lost flow-root status")
		}
		for _, name := range []string{"source", "intro", "ending"} {
			s := materialSource(t, d, name)
			if s.Classification != SourceResidual || s.Tree != r.Tree || s.Channel != "main" {
				t.Fatal("residual ownership", name)
			}
		}
		for i, name := range []string{"story-a", "story-b"} {
			s := materialSource(t, d, name)
			b, _ := branches[i].Root.Info()
			if b.Source != s.Source || s.Tree != branches[i].Tree || s.Channel != channel || s.BranchRoot != s.Source.NodeID || s.Classification != SourceNamed {
				t.Fatal("branch ownership", name)
			}
			if b.TextStyle.FontSize != font || b.TextStyle.Language != "en" || b.TextStyle.Hyphens != "manual" {
				t.Fatal("branch source typography", name, b)
			}
		}
		b, _ := branches[1].Root.Info()
		if b.Constraints.Width != (layout.Length{Kind: layout.LenPercent, Value: .75}) || b.Constraints.Margin.Left.Value != float32(font/dimen.PX) || b.Constraints.Padding.Left.Value != 8 || b.Paint.Borders[3].Style != "solid" || b.Paint.Borders[3].Color != b.TextStyle.Foreground {
			t.Fatal("lost unsized constraints/paint", b)
		}
		// Raw decoded text survives normalization, including NBSP/soft hyphen.
		for _, tc := range []struct {
			parent, text string
			size         uint64
		}{
			{"intro", "Residual café & introduction.", 30},
			{"tag", "atomic interior", 15},
			{"paragraph-b", "Second\u00a0article with a soft\u00adhyphen.", 36},
		} {
			s := materialSource(t, d, tc.parent)
			txt, ok := d.TextSource(boxtext.TextSourceID(s.Children[0]))
			if !ok || txt.Text != tc.text || uint64(len(txt.Text)) != tc.size {
				t.Fatalf("raw text: %+v", txt)
			}
		}
	}
}

func TestResidualGroupsAndSuppression(t *testing.T) {
	d := loaded(t, fixture(t, "residual-inline.html"), LoadOptions{})
	g := checkGroups(t, d, d.Residual().Tree, "split", "emptied")
	n := g[0].Group.node()
	if len(n.Children) != 2 {
		t.Fatal("residual inline runs were not joined")
	}
	for i, want := range []string{"Before ", " after."} {
		txt, _ := d.TextSource(n.Children[i].Text.Source)
		if txt.Text != want || n.Children[i].Text.Range != (boxtext.TextRange{End: 7}) {
			t.Fatal("changed residual text")
		}
	}
	empty, _ := g[1].Owner.Info()
	if len(g[1].Group.node().Children) != 0 || empty.Constraints.Padding.Left.Value != 16 || empty.Constraints.Border.Left.Value != 1 || empty.Paint.Borders[0].Style != "solid" {
		t.Fatal("lost empty decorated ancestor")
	}
	for i, name := range []string{"aside", "moved"} {
		b := d.Branches("notes")[i]
		checkGroups(t, d, b.Tree, name)
	}
	d = loaded(t, fixture(t, "suppression.html"), LoadOptions{})
	if d.Bank().Len() != 1 || !reflect.DeepEqual(d.Channels(), []string{"main"}) {
		t.Fatal("excluded material entered bank")
	}
	checkGroups(t, d, d.Residual().Tree, "visible")
	for name, want := range map[string]SourceClass{"suppressed": SourceSuppressed, "suppressed-child": SourceSuppressed, "inert": SourceInert, "stored": SourceInert, "metadata": SourceMetadata} {
		s := materialSource(t, d, name)
		if s.Classification != want || s.Tree != 0 || s.Channel != "" {
			t.Fatal("excluded source", name, s)
		}
	}
	for _, s := range d.material.sources[1:] {
		if s.Kind == html.TextNode {
			if _, ok := d.TextSource(boxtext.TextSourceID(s.Source.NodeID)); !ok {
				t.Fatal("excluded raw text lost")
			}
		}
	}
	for _, src := range []string{fixture(t, "empty.html"), `<body lang="en"></body>`} {
		d = loaded(t, src, LoadOptions{})
		g := d.Groups(d.Residual().Tree)
		if len(g) != 1 || g[0].Owner != d.Residual().Root || g[0].TextStyle.Language != "en" || g[0].TextStyle.FontSize != 16*dimen.PX || len(g[0].Atomics) != 0 {
			t.Fatal("empty-body owner context", g)
		}
	}
	d = loaded(t, `<body style="display:none"><p>hidden</p>`, LoadOptions{})
	if d.Residual().Tree == 0 || d.Residual().Root.Valid() || len(d.Groups(d.Residual().Tree)) != 0 || d.Bank().Len() != 0 {
		t.Fatal("fully suppressed body")
	}
}

func TestNestedAtomicsAndAnonymousOwners(t *testing.T) {
	d := loaded(t, `<body style="font-size:20px"><div id="owner">before<span id="split">a<div id="block">block</div>b</span>after<span id="outer" style="display:inline-block;font-size:18px">outer<span id="inner" style="display:inline-block;font-size:12px">inner</span></span></div>`, LoadOptions{})
	groups := d.Groups(d.Residual().Tree)
	if len(groups) != 5 {
		t.Fatalf("groups: %d", len(groups))
	}
	owner := materialSource(t, d, "owner")
	for _, i := range []int{0, 2} {
		info, _ := groups[i].Owner.Info()
		if info.Kind != layout.BoxAnonymousBlock || info.Source.NodeID != 0 || groups[i].Source != owner.Source || groups[i].TextStyle.FontSize != 20*dimen.PX {
			t.Fatal("anonymous owner inherited from wrong source")
		}
	}
	if len(groups[2].Atomics) != 1 || groups[3].ParentAtomic != groups[2].Atomics[0] || len(groups[3].Atomics) != 1 || groups[4].ParentAtomic != groups[3].Atomics[0] || len(groups[4].Atomics) != 0 {
		t.Fatal("nested atomic boundaries flattened")
	}
	if groups[3].TextStyle.FontSize != 18*dimen.PX || groups[4].TextStyle.FontSize != 12*dimen.PX {
		t.Fatal("atomic owner styles")
	}
	// Split inline boxes share the source NodeID, never the qualified Box handle.
	split := materialSource(t, d, "split").Source.NodeID
	count := 0
	for _, n := range d.material.trees[d.Residual().Tree].nodes {
		if n.NodeID == split {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("split source count = %d", count)
	}
}

func TestMaterialBankIdentityCopiesAndLifetime(t *testing.T) {
	input := fixture(t, "routing.html")
	d := loaded(t, input, LoadOptions{})
	initial := d.Bank()
	a, next, ok := initial.Take("article")
	if !ok || !a.Valid() {
		t.Fatal("missing article")
	}
	b, drained, ok := next.Take("article")
	if !ok || a == b {
		t.Fatal("branch identity")
	}
	if zero, _, ok := drained.Take("article"); ok || zero.Valid() || drained.Len() != 2 {
		t.Fatal("drained channel")
	}
	if first, _ := initial.First("article"); first != a {
		t.Fatal("mutated initial bank")
	}
	if first, _ := next.First("article"); first != b {
		t.Fatal("mutated derived bank")
	}
	restored := next.Prepend("article", a)
	if first, _ := restored.First("article"); first != a {
		t.Fatal("restoring whole item")
	}
	_, alternate, _ := initial.Take("notes")
	if alternate.Len() != 2 || initial.Len() != 3 {
		t.Fatal("checkpoint mutation")
	}
	channels := d.Channels()
	channels[0] = "changed"
	branches := d.Branches("article")
	branches[0].Root = Box{}
	groups := d.Groups(a.tree)
	groups[0].Atomics[0] = Box{}
	groups[0].TextStyle.Family = "changed"
	src := materialSource(t, d, "story-a")
	src.Attributes[0].Val = "changed"
	src.Children[0] = 0
	info, _ := a.Info()
	info.Constraints.Width.Value = 999
	info.Paint.Background.R = 255
	if d.Channels()[1] != "article" || d.Branches("article")[0].Root != a || !d.Groups(a.tree)[0].Atomics[0].Valid() || materialSource(t, d, "story-a").Children[0] == 0 {
		t.Fatal("inspection mutated snapshot")
	}
	if again, _ := a.Info(); again.Constraints.Width.Value == 999 || again.Paint.Background == info.Paint.Background {
		t.Fatal("box info aliases snapshot")
	}
	other := loaded(t, input, LoadOptions{})
	x, _ := other.Bank().First("article")
	if x == a || x.id != a.id || x.tree != a.tree {
		t.Fatal("snapshot qualification")
	}
	override := loaded(t, input, LoadOptions{Stylesheets: []Stylesheet{{CSS: fixture(t, "routing-override.css")}}})
	if override.Channels()[1] != "features" || d.Channels()[1] != "article" {
		t.Fatal("reload changed original")
	}
	if failed, err := Load(strings.NewReader(fixture(t, "nested-flow_err.html")), LoadOptions{}); err == nil || failed != nil {
		t.Fatal("failed reload published material")
	}
	// Keep only a derived bank/handle; the snapshot and its text remain alive.
	d, other, override = nil, nil, nil
	runtime.GC()
	if first, _ := restored.First("article"); first != a {
		t.Fatal("bank lost lifetime")
	}
	if info, ok := a.Info(); !ok || info.TextStyle.FontSize != 20*dimen.PX || len(a.owner.texts) == 0 {
		t.Fatal("handle lost snapshot")
	}
}

func TestInvalidMaterialHandles(t *testing.T) {
	var d *Document
	if d.Bank().Len() != 0 || d.Channels() != nil || d.Branches("x") != nil || d.Residual().Root.Valid() || d.Groups(1) != nil {
		t.Fatal("nil Document")
	}
	if _, ok := d.NodeSource(1); ok {
		t.Fatal("nil source")
	}
	if _, ok := d.TextSource(1); ok {
		t.Fatal("nil text")
	}
	d = loaded(t, "", LoadOptions{})
	for _, b := range []Box{{}, {owner: d.material, tree: 999, id: 1}, {owner: d.material, tree: 1, id: 0}} {
		if b.Valid() {
			t.Fatal("invalid handle accepted")
		}
		if _, ok := b.Info(); ok {
			t.Fatal("invalid info")
		}
	}
	if _, ok := d.NodeSource(0); ok {
		t.Fatal("zero node")
	}
	if _, ok := d.NodeSource(999); ok {
		t.Fatal("unknown node")
	}
	if _, ok := d.TextSource(boxtext.TextSourceID(d.Residual().Root.node().NodeID)); ok {
		t.Fatal("element as text")
	}
	if d.Groups(0) != nil || d.Groups(999) != nil || d.Branches("missing") != nil {
		t.Fatal("unknown inventory")
	}
}

func TestMaterialConcurrentReadsAndLoads(t *testing.T) {
	input := fixture(t, "routing.html")
	d := loaded(t, input, LoadOptions{})
	a, _ := d.Bank().First("article")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				item, rest, ok := d.Bank().Take(registers.RegisterID(d.Channels()[1]))
				if !ok || item != a || rest.Len() != 3 {
					t.Error("concurrent bank")
				}
				info, _ := item.Info()
				d.NodeSource(info.Source.NodeID)
				d.Groups(info.Tree)[0].Atomics[0] = Box{}
			}
			fresh, err := Load(strings.NewReader(input), LoadOptions{})
			if err != nil {
				t.Error(err)
				return
			}
			b, _ := fresh.Bank().First("article")
			if b == a {
				t.Error("shared snapshot")
			}
		}()
	}
	wg.Wait()
}

func TestPublicLoadDiagnostics(t *testing.T) {
	for _, fixtureName := range []string{"float_err.html", "nested-flow_err.html"} {
		d, err := Load(strings.NewReader(fixture(t, fixtureName)), LoadOptions{SourceName: fixtureName})
		var e *LoadError
		if d != nil || !errors.As(err, &e) {
			t.Fatal("partial document or unstructured failure", err)
		}
		ds := e.Diagnostics()
		original := e.Error()
		if ds[0].Source.Name != fixtureName || ds[0].Error() == "" {
			t.Fatal("lost diagnostic source")
		}
		if fixtureName == "nested-flow_err.html" && (ds[0].Related.NodeID == 0 || ds[0].Source.NodeID == 0) {
			t.Fatal("lost conflicting root")
		}
		ds[0].Message = "changed"
		children := e.Unwrap()
		children[0] = nil
		if e.Error() != original {
			t.Fatal("diagnostics alias error")
		}
	}
}
