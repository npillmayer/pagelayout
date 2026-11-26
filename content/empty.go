package content

import "github.com/npillmayer/css-box-layout/layout"

// EmptyRange certifies a source range with no text, atomics, paint, spacing or
// nonzero definite width. It is conservative: unproved cases return false.
// Endpoints must lie within the supplied block root. This only inspects prepared
// source values; callers still need a real structural continuation to advance.
// Empty owning annotations remain in the source and must be processed before
// a span can complete.
func (p *PreparedSnapshot) EmptyRange(root Box, from, to LogicalPosition) bool {
	if !p.Covers(root) || !layout.IsBlockLevel(root.node().Box) || !from.Valid() || !to.Valid() || from.box.owner != root.owner || to.box.owner != root.owner || from.box.tree != root.tree || to.box.tree != root.tree {
		return false
	}
	if order, ok := from.Compare(to); !ok || order >= 0 {
		return false
	}
	before, after := map[layout.BoxID]LogicalPosition{}, map[layout.BoxID]LogicalPosition{}
	for _, b := range root.owner.trees[root.tree].provenance.Boundaries() {
		pos := root.owner.logical(root.tree, b, SourceRef{})
		if b.Edge == layout.BoundaryBefore {
			before[b.Box] = pos
		} else {
			after[b.Box] = pos
		}
	}
	lo, loOK := from.Compare(before[root.id])
	hi, hiOK := to.Compare(after[root.id])
	if !loOK || !hiOK || lo < 0 || hi > 0 {
		return false
	}
	intersects := func(id layout.BoxID) bool {
		lo, ok1 := before[id].Compare(to)
		hi, ok2 := after[id].Compare(from)
		return !ok1 || !ok2 || lo < 0 && hi > 0
	}
	zeroEdges := func(e layout.EdgeLengths) bool { return e == (layout.EdgeLengths{}) }
	var empty func(*layout.LayoutNode) bool
	empty = func(n *layout.LayoutNode) bool {
		if !intersects(n.BoxID) {
			return true
		}
		if n.Box == layout.BoxInlineBlock || n.Style == nil {
			return false
		}
		style := n.Style
		if style.Position != layout.PositionStatic || style.Fixed != nil || style.Width.Kind != layout.LenAuto && style.Width != (layout.Length{}) || !zeroEdges(style.Margin) || !zeroEdges(style.Padding) || !zeroEdges(style.Border) {
			return false
		}
		if n.NodeID != 0 && root.owner.paint[n.NodeID].Background.A != 0 {
			return false
		}
		if n.Box == layout.BoxAnonymousInline {
			input, ok := p.Input(Box{root.owner, root.tree, n.BoxID})
			if !ok || input.Text != "" || len(input.Atomics) > 0 || len(input.Occurrences) > 0 {
				return false
			}
		}
		for _, child := range n.Children {
			if !empty(child) {
				return false
			}
		}
		return true
	}
	return empty(root.node())
}
