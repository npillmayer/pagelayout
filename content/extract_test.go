package content

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/npillmayer/css-box-layout/layout"
	boxtext "github.com/npillmayer/css-box-layout/text"
	"github.com/npillmayer/w3c-dom/dimen"
	"github.com/npillmayer/w3c-dom/style/textstyle"
)

func extractKnownGroup(t *testing.T, box Box) *GroupInput {
	t.Helper()
	index := slices.IndexFunc(box.owner.trees[box.tree].groups, func(g GroupInfo) bool { return g.Group == box })
	g, err := box.owner.extractGroup(box.tree, index)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func extractAllGroups(t *testing.T, d *Document) []*GroupInput {
	t.Helper()
	var result []*GroupInput
	for tree := 1; tree < len(d.material.trees); tree++ {
		for _, group := range d.Groups(TreeID(tree)) {
			g := extractKnownGroup(t, group.Group)
			checkExtractedRanges(t, g)
			result = append(result, g)
		}
	}
	return result
}

// Verify byte-coordinate contracts independently of the extraction algorithm.
func checkExtractedRanges(t *testing.T, g *GroupInput) {
	t.Helper()
	valid := func(text string, start, end uint64) {
		t.Helper()
		if start > end || end > uint64(len(text)) || !utf8.ValidString(text[:start]) || !utf8.ValidString(text[:end]) {
			t.Fatalf("invalid range [%d,%d) in %q", start, end, text)
		}
	}
	covered, atomic, policies := make([]bool, len(g.Text)), make([]bool, len(g.Text)), make([]int, len(g.Text))
	if len(g.Occurrences) != len(g.Atomics) {
		t.Fatal("incomplete atomic coverage")
	}
	for i, a := range g.Occurrences {
		valid(g.Text, a.Start, a.End)
		info, ok := a.Box.Info()
		if !ok || a.Box != g.Atomics[i] || info.Source != a.Source || a.ID != uint64(i+1) || g.Text[a.Start:a.End] != "\uFFFC" {
			t.Fatal("invalid atomic mapping")
		}
		for k := a.Start; k < a.End; k++ {
			if covered[k] {
				t.Fatal("overlapping atomic occurrences")
			}
			covered[k], atomic[k] = true, true
		}
	}
	for _, span := range g.TextSources {
		valid(g.Text, span.Start, span.End)
		info, ok := span.Box.Info()
		if !ok || info.Kind != layout.BoxText || info.Source != span.Source {
			t.Fatal("invalid source box")
		}
		raw := span.Box.owner.texts[info.Text.Source].Text
		valid(raw, span.Range.Start, span.Range.End)
		part, output := raw[span.Range.Start:span.Range.End], g.Text[span.Start:span.End]
		switch span.Kind {
		case "":
			if part != output {
				t.Fatal("identity mapping")
			}
		case "collapsed", "omitted":
			if strings.Trim(part, " \t\n\r\f") != "" || span.Kind == "collapsed" && output != " " || span.Kind == "omitted" && output != "" {
				t.Fatal("whitespace mapping")
			}
		default:
			t.Fatal("unknown source transform")
		}
		for k := span.Start; k < span.End; k++ {
			if atomic[k] {
				t.Fatal("atomic marker has fabricated text source")
			}
			covered[k] = true
		}
	}
	for _, policy := range g.Hyphens {
		valid(g.Text, policy.Start, policy.End)
		info, ok := policy.Box.Info()
		if !ok || info.Kind != layout.BoxText || policy.Source != info.Source || policy.Policy != info.TextStyle.Hyphens {
			t.Fatal("policy lost source context")
		}
		raw := policy.Box.owner.texts[info.Text.Source].Text
		valid(raw, policy.Range.Start, policy.Range.End)
		if policy.Start == policy.End || policy.Range.Start == policy.Range.End {
			t.Fatal("empty policy contribution")
		}
		for k := policy.Start; k < policy.End; k++ {
			policies[k]++
		}
	}
	for i := range covered {
		if !covered[i] || atomic[i] && policies[i] != 0 || !atomic[i] && policies[i] != 1 {
			t.Fatalf("incomplete source/policy coverage at byte %d", i)
		}
	}
	end := uint64(0)
	for _, span := range g.Styles {
		valid(g.Text, span.Start, span.End)
		if span.Start != end || span.Start == span.End {
			t.Fatal("style coverage")
		}
		end = span.End
	}
	if end != uint64(len(g.Text)) {
		t.Fatal("missing styles")
	}
	end = 0
	for _, span := range g.Paint {
		valid(g.Text, span.Start, span.End)
		if span.Start != end || span.Start == span.End {
			t.Fatal("paint coverage")
		}
		end = span.End
	}
	if end != uint64(len(g.Text)) {
		t.Fatal("missing paint")
	}
	for _, isolate := range g.Isolates {
		valid(g.Text, isolate.Start, isolate.End)
	}
}

func TestExtractWhitespaceStylePolicyAndScopes(t *testing.T) {
	input := "<body style='hyphens:none'><div id='p'> A <span id='first' dir='rtl' style='color:red;hyphens:manual'>  B \t</span><span dir='ltr' style='color:blue;hyphens:auto'> \nC</span> </div>"
	d := loaded(t, input, LoadOptions{})
	g := extractAllGroups(t, d)[0]
	if g.Text != "A B C" || len(g.Styles) != 3 || len(g.Paint) != 3 {
		t.Fatal(g.Text, g.Styles, g.Paint)
	}
	for i, want := range []struct {
		start, end uint64
		policy     string
	}{{0, 2, "none"}, {2, 4, "manual"}, {4, 5, "auto"}} {
		s := g.Styles[i]
		if s.Start != want.start || s.End != want.end || s.Style.Hyphens != want.policy {
			t.Fatal("first-unit style", g.Styles)
		}
		if g.Paint[i].Start != want.start || g.Paint[i].End != want.end || g.Paint[i].Color != s.Style.Foreground {
			t.Fatal("paint mapping")
		}
	}
	if len(g.Isolates) != 2 || g.Isolates[0].Start != 2 || g.Isolates[0].End != 4 || g.Isolates[1].Start != 4 || g.Isolates[1].End != 5 {
		t.Fatal("collapsed space belongs only to first contributing isolate", g.Isolates)
	}
	var policies []string
	for _, p := range g.Hyphens {
		policies = append(policies, p.Policy)
	}
	if !slices.Equal(policies, []string{"none", "none", "manual", "manual", "auto"}) {
		t.Fatal(policies)
	}
	// An omitted auto source cannot create a processed policy conflict, but its
	// original policy remains auditable in the source records.
	d = loaded(t, `<body style="hyphens:none"><p><span style="hyphens:auto"> </span>x<span style="hyphens:manual"> </span></p>`, LoadOptions{})
	g = extractAllGroups(t, d)[0]
	if len(g.Hyphens) != 1 || g.Hyphens[0].Policy != "none" || g.TextSources[0].Style.Hyphens != "auto" || g.TextSources[2].Style.Hyphens != "manual" {
		t.Fatal("omitted policy lost")
	}
}

func TestExtractAtomicScopesAndOwnerDefaults(t *testing.T) {
	d := loaded(t, `<body style="font-size:20px;hyphens:none"><div id="p">a<span dir="rtl">אב<span id="atom" dir="ltr" style="display:inline-block;width:50%;font-size:12px;hyphens:auto">inside</span>ג</span>z</div><p id="empty"></p><p id="only"><span style="display:inline-block;hyphens:auto"></span></p>`, LoadOptions{})
	groups := extractAllGroups(t, d)
	g := groups[0]
	if len(groups) != 5 || g.Text != "aאב\uFFFCגz" || len(g.Isolates) != 2 {
		t.Fatal(groups)
	}
	if g.Isolates[0].Start != 5 || g.Isolates[0].End != 8 || g.Isolates[0].Direction != textstyle.LeftToRight || g.Isolates[1].Start != 1 || g.Isolates[1].End != 10 || g.Isolates[1].Direction != textstyle.RightToLeft {
		t.Fatal(g.Isolates)
	}
	if g.Occurrences[0].Box != groups[1].Owner || groups[1].ParentAtomic != groups[1].Owner || groups[1].Text != "inside" || len(groups[1].Isolates) != 0 {
		t.Fatal("atomic interior leaked")
	}
	info, _ := g.Occurrences[0].Box.Info()
	if info.Constraints.Width != (layout.Length{Kind: layout.LenPercent, Value: .5}) {
		t.Fatal("atomic sized")
	}
	for _, p := range g.Hyphens {
		if p.Policy != "none" {
			t.Fatal("interior policy leaked")
		}
	}
	if len(groups[2].Styles) != 0 || len(groups[3].Hyphens) != 0 || groups[3].Text != "\uFFFC" || groups[3].TextStyle.FontSize != 20*dimen.PX || groups[3].TextStyle.Hyphens != "none" || groups[4].TextStyle.Hyphens != "auto" {
		t.Fatal("owner context lost")
	}
}

func TestExtractAfterRoutingAndSplit(t *testing.T) {
	d := loaded(t, `<body><div id="p">left <div style="flow-into:article">removed</div> right</div></body>`, LoadOptions{})
	groups := extractAllGroups(t, d)
	if len(groups) != 2 || groups[0].Text != "left right" || groups[1].Text != "removed" {
		t.Fatal("routing adjacency", groups)
	}
	d = loaded(t, `<body><div id="root">a<span dir="rtl">אב<section>ב</section>ג</span>z</div>`, LoadOptions{})
	groups = extractAllGroups(t, d)
	for i, want := range []string{"aאב", "ב", "גz"} {
		if groups[i].Text != want {
			t.Fatal("split text", i, groups[i].Text)
		}
	}
	if groups[0].Source != materialSource(t, d, "root").Source || groups[2].Source != groups[0].Source || groups[0].TextStyle.Direction != textstyle.LeftToRight || groups[1].TextStyle.Direction != textstyle.RightToLeft {
		t.Fatal("anonymous owner context")
	}
	if len(groups[0].Isolates) != 1 || groups[0].Isolates[0].Start != 1 || groups[0].Isolates[0].End != 5 || len(groups[1].Isolates) != 0 || len(groups[2].Isolates) != 1 || groups[2].Isolates[0].Start != 0 || groups[2].Isolates[0].End != 2 {
		t.Fatal("split scopes")
	}
}

func TestExtractEmptyAndSuppressed(t *testing.T) {
	for _, input := range []string{
		`<body style="font-size:23px;hyphens:auto"></body>`,
		"<body style='font-size:23px;hyphens:auto'> \n\t </body>",
		`<body style="font-size:23px;hyphens:auto"><span dir="rtl"></span></body>`,
	} {
		d := loaded(t, input, LoadOptions{})
		groups := extractAllGroups(t, d)
		if len(groups) != 1 || groups[0].Text != "" || len(groups[0].Isolates) != 0 || len(groups[0].Hyphens) != 0 || groups[0].TextStyle.Hyphens != "auto" || groups[0].TextStyle.FontSize != 23*dimen.PX {
			t.Fatal("empty group lost owner context", groups)
		}
		if strings.Contains(input, "\t") && (len(groups[0].TextSources) != 1 || groups[0].TextSources[0].Kind != "omitted") {
			t.Fatal("empty paragraph lost omitted whitespace")
		}
	}
	d := loaded(t, `<body style="display:none">hidden</body>`, LoadOptions{})
	if len(extractAllGroups(t, d)) != 0 {
		t.Fatal("suppressed tree produced extraction")
	}
}

func TestExtractDetachedAndConcurrent(t *testing.T) {
	d := loaded(t, `<body><div style="flow-into:article">a<span dir="rtl">ב<span style="display:inline-block">in</span>ג</span>z</div>`, LoadOptions{})
	group := d.Groups(d.Branches("article")[0].Tree)[0].Group
	want := extractKnownGroup(t, group)
	// JSON snapshots all exported structural/style/child/text data; retained
	// public descriptions also check source and bank lifetime.
	before, err := json.Marshal(d.material.trees[group.tree].root)
	if err != nil {
		t.Fatal(err)
	}
	branch := d.Branches("article")[0].Root
	branchInfo, _ := branch.Info()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 10 {
				g := extractKnownGroup(t, group)
				if !reflect.DeepEqual(g, want) {
					t.Error("concurrent extraction changed")
				}
				g.Atomics[0] = Box{}
				g.Styles[0].Style.Family = "changed"
				g.Paint[0].Color.A = 0
				g.Hyphens[0].Policy = "changed"
				g.Isolates[0].End = 99
				g.Occurrences[0].Box = Box{}
				g.TextSources[0].Range.End = 99
			}
		})
	}
	wg.Wait()
	after, err := json.Marshal(d.material.trees[group.tree].root)
	gotInfo, _ := branch.Info()
	if err != nil || string(before) != string(after) || gotInfo != branchInfo || !reflect.DeepEqual(want, extractKnownGroup(t, group)) {
		t.Fatal("extraction mutated retained material")
	}
	got, bank, ok := d.Bank().Take("article")
	if !ok || got != branch || bank.Len() != 1 || d.Bank().Len() != 2 {
		t.Fatal("extraction consumed bank")
	}
	other := loaded(t, `<body><div style="flow-into:article">a<span dir="rtl">ב<span style="display:inline-block">in</span>ג</span>z</div>`, LoadOptions{})
	foreign := other.Groups(other.Branches("article")[0].Tree)[0].Group
	if group.id != foreign.id || group == foreign || want.Occurrences[0].Box == extractKnownGroup(t, foreign).Occurrences[0].Box {
		t.Fatal("unqualified identity")
	}
}

func TestExtractCheckedReferences(t *testing.T) {
	for _, tc := range []struct {
		s     *snapshot
		tree  TreeID
		index int
	}{{nil, 1, 0}, {&snapshot{}, 1, 0}} {
		if g, err := tc.s.extractGroup(tc.tree, tc.index); g != nil || err == nil {
			t.Fatal("invalid group accepted")
		}
	}
	d := loaded(t, `<body><p>café</p>`, LoadOptions{})
	for _, index := range []int{-1, 1} {
		if g, err := d.material.extractGroup(1, index); g != nil || err == nil {
			t.Fatal("invalid index accepted")
		}
	}
	group := d.Groups(1)[0]
	n := group.Group.node().Children[0]
	// A valid subrange maps back to its nonzero original decoded-byte origin.
	n.Text.Range = boxtext.TextRange{Start: 3, End: 5}
	g := extractKnownGroup(t, group.Group)
	if g.Text != "é" || g.TextSources[0].Range != n.Text.Range {
		t.Fatal("rebased source range")
	}
	n.Text.Range.Start = 4
	g, err := d.material.extractGroup(1, 0)
	var failure *extractionError
	if g != nil || !errors.As(err, &failure) || failure.Group != group.Group || failure.Owner != group.Owner || failure.Source.NodeID != n.NodeID || !failure.HasRange || failure.Range != n.Text.Range || errors.Unwrap(err) == nil {
		t.Fatal("missing extraction failure location", err)
	}
}
