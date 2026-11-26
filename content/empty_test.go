package content

import (
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
)

func TestPreparedEmptyRange(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		empty      bool
	}{
		{"body", `<body></body>`, true},
		{"whitespace", "<body><p> \n </p><!-- ignored --></body>", true},
		{"annotation", `<body><p><a href="#n" data-flow-anchor></a></p><aside id="n" style="flow-into:notes">note</aside></body>`, true},
		{"text", `<body><p>X</p></body>`, false},
		{"nbsp", `<body><p>&nbsp;</p></body>`, false},
		{"atomic", `<body><span style="display:inline-block"></span></body>`, false},
		{"background", `<body><p style="background:red"></p></body>`, false},
		{"border", `<body style="border:1px solid black"></body>`, false},
		{"padding", `<body style="padding:1px"></body>`, false},
		{"margin", `<body><p style="margin-bottom:1px"></p></body>`, false},
		{"width", `<body style="width:10px"></body>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := loaded(t, tc.html, LoadOptions{})
			p, s := prepared(t, d, &mockPreparer{})
			root := d.Residual().Root
			info, _ := root.Info()
			from, to := d.TreeBounds(info.Tree)
			// Certification uses only immutable source/preparation, after close too.
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			if got := p.EmptyRange(root, from, to); got != tc.empty {
				t.Fatalf("empty = %t, want %t", got, tc.empty)
			}
			if p.EmptyRange(root, to, from) || p.EmptyRange(root, from, from) || p.EmptyRange(Box{}, from, to) || (*PreparedSnapshot)(nil).EmptyRange(root, from, to) {
				t.Fatal("accepted invalid interval")
			}
			foreign := loaded(t, tc.html, LoadOptions{})
			ff, ft := foreign.TreeBounds(info.Tree)
			if p.EmptyRange(root, ff, ft) {
				t.Fatal("accepted foreign positions")
			}
		})
	}
}

func TestEmptyRangeStopsAtSourceBoundary(t *testing.T) {
	d := loaded(t, `<body><section></section><p style="break-before:page">Future text</p></body>`, LoadOptions{})
	p, _ := prepared(t, d, &mockPreparer{})
	root := d.Residual().Root
	info, _ := root.Info()
	from, to := d.TreeBounds(info.Tree)
	events, err := d.EventPositions()
	if err != nil || len(events) != 1 {
		t.Fatal(events, err)
	}
	if !p.EmptyRange(root, from, events[0].Position) || p.EmptyRange(root, from, to) || p.EmptyRange(root, events[0].Position, to) {
		t.Fatal("future text changed empty-prefix certification")
	}
	// Bounds belonging to the same tree can still lie outside a supplied subtree.
	var section Box
	for _, n := range root.node().Children {
		if n.Box == layout.BoxBlock {
			section = Box{root.owner, root.tree, n.BoxID}
			break
		}
	}
	if !section.Valid() || p.EmptyRange(section, from, to) {
		t.Fatal("accepted interval outside subtree")
	}
}
