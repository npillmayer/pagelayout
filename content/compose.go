package content

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
)

// FrameComposition places independent branch fragments in source order inside
// one receiver. Fragments keep source identity and frozen paragraph payloads.
// Fits certifies physical bounds; finite intermediate arrangements can be too
// tall until a following branch collapses a trailing margin.
type FrameComposition struct {
	Fragments []*FitFragment
	Extent    layout.InlineExtent
	Fits      bool
}

type compositionReplay map[layout.BoxID]layout.InlineExtent

func (p compositionReplay) LayoutInline(r layout.InlineRequest) (layout.InlineResult, error) {
	v, ok := p[r.Group.BoxID]
	if !ok {
		return layout.InlineResult{}, ErrPreparationProtocol
	}
	return layout.InlineResult{Extent: v}, nil
}

// ComposeFragments repositions already fitted, independently rooted material.
// Each root remains a containment boundary for its frozen interior; its outside
// margins adjoin sibling roots through the core's normal FlowLayout rules.
// No paragraph service runs. Inputs, continuations and earlier output are unchanged.
func ComposeFragments(width, height float32, parts []*FitFragment) (FrameComposition, error) {
	fail := func() (FrameComposition, error) {
		return FrameComposition{}, fmt.Errorf("%w: invalid frame composition", ErrPreparationProtocol)
	}
	finite := func(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }
	if !finite(width) || !finite(height) || width < 0 || height < 0 {
		return fail()
	}
	style := layout.DefaultComputedStyle()
	root := &layout.LayoutNode{BoxID: 1, Box: layout.BoxBlock, FC: layout.FCBlock, Style: &style}
	used := layout.UsedValuesTable{1: {ContentWidth: width}}
	replay := compositionReplay{}
	seen := map[Box]bool{}
	for i, p := range parts {
		if p == nil || !p.root.Valid() || seen[p.root] {
			return fail()
		}
		seen[p.root] = true
		if i > 0 && parts[0].root.owner != p.root.owner {
			return fail()
		}
		g, ok := p.geometry[p.root]
		if !ok {
			return fail()
		}
		u, ok := p.used[p.root]
		if !ok {
			return fail()
		}
		id := layout.BoxID(2 + i*2)
		group := &layout.LayoutNode{BoxID: id + 1, Box: layout.BoxAnonymousInline, FC: layout.FCInline, Style: &style}
		block := &layout.LayoutNode{BoxID: id, Box: layout.BoxBlock, FC: layout.FCBlock, EstablishesBFC: true, Style: &style, Children: []*layout.LayoutNode{group}}
		root.Children = append(root.Children, block)
		used[id] = layout.UsedValues{Margin: u.Margin, ContentWidth: g.Frame.W}
		used[id+1] = layout.UsedValues{ContentWidth: g.Frame.W}
		replay[id+1] = layout.InlineExtent{W: g.Frame.W, H: g.Frame.H}
	}
	laid, err := layout.FlowLayout(root, used, replay, nil, layout.LayoutContext{}, layout.LayoutOptions{})
	if err != nil {
		return FrameComposition{}, err
	}
	out := FrameComposition{Fits: true, Extent: layout.InlineExtent{H: laid.Geometry[1].Frame.H}}
	check := func(r layout.Rect) bool {
		if !finite(r.X) || !finite(r.Y) || !finite(r.W) || !finite(r.H) || !finite(r.X+r.W) || !finite(r.Y+r.H) || r.W < 0 || r.H < 0 {
			return false
		}
		out.Extent.W = max(out.Extent.W, r.X+r.W)
		out.Extent.H = max(out.Extent.H, r.Y+r.H)
		if r.X < 0 || r.Y < 0 || r.X+r.W > width || r.Y+r.H > height {
			out.Fits = false
		}
		return true
	}
	for i, p := range parts {
		old := p.geometry[p.root]
		now := laid.Geometry[layout.BoxID(2+i*2)]
		advance := now.Frame.X + now.Frame.W + p.used[p.root].Margin.Right
		if !finite(advance) {
			return fail()
		}
		out.Extent.W = max(out.Extent.W, advance)
		dx, dy := now.Frame.X-old.Frame.X, now.Frame.Y-old.Frame.Y
		q := *p
		q.geometry = maps.Clone(p.geometry)
		q.containers = slices.Clone(p.containers)
		q.paragraphs = slices.Clone(p.paragraphs)
		old.Frame.X += dx
		old.Frame.Y += dy
		old.Content.X += dx
		old.Content.Y += dy
		q.geometry[p.root] = old
		for _, g := range q.geometry {
			r := g.Frame
			for parent := g.RelativeTo; parent != 0; {
				pg := q.geometry[Box{p.root.owner, p.root.tree, parent}]
				r.X += pg.Content.X
				r.Y += pg.Content.Y
				parent = pg.RelativeTo
			}
			if !check(r) {
				return fail()
			}
		}
		for j := range q.containers {
			q.containers[j].Geometry = q.geometry[q.containers[j].Box]
		}
		for j := range q.paragraphs {
			g := &q.paragraphs[j]
			g.X += dx
			g.Y += dy
			if !check(layout.Rect{X: g.X, Y: g.Y, W: g.Extent.W, H: g.Extent.H}) {
				return fail()
			}
		}
		out.Fragments = append(out.Fragments, &q)
	}
	if !finite(out.Extent.H) || !finite(out.Extent.W) || out.Extent.H < 0 {
		return fail()
	}
	if out.Extent.H > height || out.Extent.W > width {
		out.Fits = false
	}
	return out, nil
}

// ContainsBoundary reports whether actual selected material carries this
// structural edge. Merely opening a suspended container is not placement.
func (f *FitFragment) ContainsBoundary(p LogicalPosition) bool {
	if f == nil || !p.Valid() || p.IsText() {
		return false
	}
	for _, c := range f.containers {
		if c.Box == p.box {
			return p.edge == layout.BoundaryBefore && c.First || p.edge == layout.BoundaryAfter && c.Last
		}
	}
	// Geometry not in the crossing-container list is an indivisible, completely
	// consumed atomic interior. Both of its structural edges are present.
	_, ok := f.geometry[p.box]
	return ok
}
