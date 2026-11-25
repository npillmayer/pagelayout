package content

import (
	"errors"
	"strings"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	boxtext "github.com/npillmayer/css-box-layout/text"
)

func materialPartition(t *testing.T) *partitionProjection {
	t.Helper()
	p, err := loadPartition(strings.NewReader(fixture(t, "routing.html")), LoadOptions{SourceName: "routing.html"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNormalizationFailureAtomicity(t *testing.T) {
	retained := loaded(t, fixture(t, "routing.html"), LoadOptions{})
	before := retained.Branches("article")[1].Root
	beforeInfo, _ := before.Info()
	p := materialPartition(t)
	// The residual and two earlier branch builds succeed before this source
	// fails in the third tree. Use an actual sibling error, not a string match.
	failing := p.branches[2].root
	failing.Style.Position = layout.PositionKind(255)
	d, err := normalizePartition(p)
	var located *layout.BuildError
	var load *LoadError
	if d != nil || !errors.As(err, &load) || !errors.As(err, &located) || !errors.Is(err, layout.ErrUnsupportedPosition) {
		t.Fatal("lost atomic failure/original cause", d, err)
	}
	if located.RootID != failing.ID || !located.HasNode || located.NodeID != failing.ID {
		t.Fatal("builder source identity", located)
	}
	ds := load.Diagnostics()
	if len(ds) != 1 || ds[0].Stage != "normalization" || ds[0].Code != "box.build" || ds[0].Source.NodeID != failing.ID || ds[0].Source.Name != "routing.html" || ds[0].Source.Path == "" {
		t.Fatal("diagnostic mapping", ds)
	}
	var public Diagnostic
	if !errors.As(err, &public) || !errors.Is(public, layout.ErrUnsupportedPosition) {
		t.Fatal("public diagnostic unwrap")
	}
	if after, _ := before.Info(); after != beforeInfo {
		t.Fatal("failure changed retained snapshot")
	}
}

func TestNormalizationTextValidation(t *testing.T) {
	for _, mutation := range []string{"missing", "reversed", "past-end", "split-rune", "wrong-node", "truncated", "empty"} {
		t.Run(mutation, func(t *testing.T) {
			p := materialPartition(t)
			var text *layout.RenderNode
			var find func(*layout.RenderNode)
			find = func(n *layout.RenderNode) {
				if n.Text != nil && strings.Contains(n.HTML.Data, "Second") {
					text = n
				}
				for _, c := range n.ChildrenNodes {
					find(c)
				}
			}
			find(p.branches[2].root)
			if text == nil {
				t.Fatal("test text missing")
			}
			switch mutation {
			case "missing":
				text.Text.Source = 99999
			case "reversed":
				text.Text.Range.Start = text.Text.Range.End + 1
			case "past-end":
				text.Text.Range.End++
			case "split-rune":
				text.Text.Range.Start = 7 // Inside U+00A0 after "Second".
			case "wrong-node":
				for _, s := range p.sources {
					if s.text == "atomic interior" {
						text.Text.Source = boxtext.TextSourceID(s.ref.nodeID)
						text.Text.Range = boxtext.TextRange{End: uint64(len(s.text))}
					}
				}
			case "truncated":
				text.Text.Range.End = 6
			case "empty":
				text.Text.Range.End = 0
			}
			d, err := normalizePartition(p)
			var e *LoadError
			if d != nil || !errors.As(err, &e) {
				t.Fatal("invalid text published", err)
			}
			if mutation != "wrong-node" && mutation != "truncated" && mutation != "empty" {
				var be *layout.BuildError
				if !errors.As(err, &be) || be.RootID != p.branches[2].root.ID || be.NodeID != text.ID || e.Diagnostics()[0].Source.NodeID != text.ID {
					t.Fatal("text error did not retain node and branch", err)
				}
			}
		})
	}
	d := loaded(t, `<body><p>café</p>`, LoadOptions{})
	for id, text := range d.material.texts {
		if text.Text != "café" {
			continue
		}
		if part, err := d.material.readText(boxtext.TextRef{Source: id, Range: boxtext.TextRange{Start: 3, End: 5}}); err != nil || part != "é" {
			t.Fatal("valid decoded range", part, err)
		}
		if _, err := d.material.readText(boxtext.TextRef{Source: id, Range: boxtext.TextRange{Start: 4, End: 4}}); err == nil {
			t.Fatal("empty range at invalid UTF-8 boundary")
		}
	}
}

func TestPublicationValidatesPartition(t *testing.T) {
	for _, mutation := range []string{"duplicate", "missing", "wrong-owner", "foreign-owner", "channel", "order", "branch", "excluded-owner"} {
		t.Run(mutation, func(t *testing.T) {
			p := materialPartition(t)
			root := p.branches[0].root
			switch mutation {
			case "duplicate":
				root.ChildrenNodes = append(root.ChildrenNodes, root.ChildrenNodes[0])
			case "missing":
				root.ChildrenNodes = root.ChildrenNodes[1:]
			case "wrong-owner":
				p.owners[sourceID(root.ID)] = 0
			case "foreign-owner":
				p.owners[99999] = 0
			case "channel":
				p.channels[0].name = "changed"
			case "order":
				p.channels[0].branches[0], p.channels[0].branches[1] = p.channels[0].branches[1], p.channels[0].branches[0]
			case "branch":
				p.branches[0].channel = ""
			case "excluded-owner":
				p.owners[1] = 0
			}
			d, err := normalizePartition(p)
			var e *LoadError
			if d != nil || !errors.As(err, &e) || e.Diagnostics()[0].Code != "material.ownership" {
				t.Fatal("invalid ownership published", err)
			}
		})
	}
}

func TestNormalizedSnapshotDetachesWorkingInput(t *testing.T) {
	p := materialPartition(t)
	d, err := normalizePartition(p)
	if err != nil {
		t.Fatal(err)
	}
	a := d.Branches("article")[0].Root
	old, _ := a.Info()
	root := p.branches[0].root
	root.Style.FontSizePx = 999
	root.ChildrenNodes = nil
	src := &p.sources[root.ID]
	src.typography.Family = "changed"
	src.children[0] = 0
	src.attributes[0].Val = "changed"
	src.paint.background.R++
	p.channels[0].name = "changed"
	if fresh, _ := a.Info(); fresh != old {
		t.Fatal("input mutates published box")
	}
	s, _ := d.NodeSource(root.ID)
	if s.Children[0] == 0 || s.Attributes[0].Val == "changed" || d.Channels()[1] != "article" || len(d.Groups(a.tree)) != 2 {
		t.Fatal("input mutates published source/index")
	}
}
