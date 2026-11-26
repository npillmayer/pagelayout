package content

import (
	"cmp"
	"slices"
	"strings"

	"github.com/npillmayer/css-box-layout/layout"
	boxtext "github.com/npillmayer/css-box-layout/text"
)

// SourceID qualifies one immutable loaded document. Zero is invalid.
type SourceID struct{ source *snapshot }

func (d *Document) SourceID() SourceID {
	if d == nil {
		return SourceID{}
	}
	return SourceID{d.material}
}
func (id SourceID) Valid() bool { return id.source != nil }

// LogicalPosition is an opaque source/build-qualified structural or text
// position. It is independent of width and line breaking, not a continuation.
type LogicalPosition struct {
	box                 Box
	group               Box
	order               uint64
	offset              uint64
	edge                layout.BoundaryEdge
	source              SourceRef
	keyOrder, keyOffset uint64
	textPosition        bool
	step                uint64 // fitted discretionary progress at one processed offset
}
type PositionInfo struct {
	Box, Group    Box
	Source        SourceRef
	Order, Offset uint64 // source-box traversal order and processed group byte offset; use Compare for ordering
	Edge          layout.BoundaryEdge
	Step          uint64 // backend logical step, not a line count or checkpoint
}

func (p LogicalPosition) Info() PositionInfo {
	return PositionInfo{p.box, p.group, p.source, p.order, p.offset, p.edge, p.step}
}
func (p LogicalPosition) Valid() bool { return p.box.Valid() && p.order != 0 }

// IsText distinguishes processed paragraph coordinates from structural edges.
func (p LogicalPosition) IsText() bool { return p.Valid() && p.textPosition }

// Compare only compares positions within the same source tree. Equal positions
// do not imply interchangeable fitter continuations.
func (p LogicalPosition) Compare(q LogicalPosition) (int, bool) {
	if !p.Valid() || !q.Valid() || p.box.owner != q.box.owner || p.box.tree != q.box.tree {
		return 0, false
	}
	if c := cmp.Compare(p.keyOrder, q.keyOrder); c != 0 {
		return c, true
	}
	if c := cmp.Compare(p.keyOffset, q.keyOffset); c != 0 {
		return c, true
	}
	return cmp.Compare(p.step, q.step), true
}

type EventAffinity string

const (
	AffinityVisible    EventAffinity = "first-rendered-content"
	AffinityFollowing  EventAffinity = "following"
	AffinityPreceding  EventAffinity = "preceding"
	AffinityStructural EventAffinity = "structural"
)

// EventCoverage retains processed bytes, original decoded bytes and the
// qualified normalized box separately. Atomic coverage denotes an existing
// atomic occurrence, never a marker introduced for an event.
type EventCoverage struct {
	Position   LogicalPosition
	Start, End uint64
	Source     SourceRef
	Range      boxtext.TextRange
	Atomic     bool
}

// PositionedEvent is a source fact with logical coverage. UnsupportedBreak is
// policy information for Plan; generic content loading does not reject it.
type PositionedEvent struct {
	Event            SourceEvent
	Block            SourceRef
	Position         LogicalPosition
	Affinity         EventAffinity
	Coverage         []EventCoverage
	UnsupportedBreak bool
}

// GroupEvent is copied alongside preparation input. Multiple coverage records
// for one Source identify one activation, not repeated ownership events.
type GroupEvent struct {
	Source         SourceRef
	Kind           SourceEventKind
	Affinity       EventAffinity
	Start, End     uint64
	CoverageSource SourceRef
	Range          boxtext.TextRange
	Atomic         bool
}

// EventPositions extracts source coverage without fonts or layout. It does not
// validate ownership, mutate source material or publish a partial result.
func (d *Document) EventPositions() ([]PositionedEvent, error) {
	if d == nil || d.material == nil || !d.material.hasPositionEvents() {
		return nil, nil
	}
	inputs, err := d.material.positionInputs()
	if err != nil {
		return nil, err
	}
	return d.material.positionEvents(inputs), nil
}
func (s *snapshot) positionInputs() ([]GroupInput, error) {
	var inputs []GroupInput
	for tree := 1; tree < len(s.trees); tree++ {
		for i := range s.trees[tree].groups {
			g, err := s.extractGroup(TreeID(tree), i)
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, *g)
		}
	}
	return inputs, nil
}

// TreeBounds returns structural endpoints of a retained tree, including the
// terminal position of empty material. Suppressed roots have no positions.
func (d *Document) TreeBounds(tree TreeID) (LogicalPosition, LogicalPosition) {
	if d == nil || d.material == nil || tree == 0 || uint64(tree) >= uint64(len(d.material.trees)) {
		return LogicalPosition{}, LogicalPosition{}
	}
	s := d.material
	t := s.trees[tree]
	if t.root == nil {
		return LogicalPosition{}, LogicalPosition{}
	}
	bounds := t.provenance.Boundaries()
	return s.logical(tree, bounds[0], SourceRef{}), s.logical(tree, bounds[len(bounds)-1], SourceRef{})
}
func (s *snapshot) logical(tree TreeID, b layout.StructuralBoundary, src SourceRef) LogicalPosition {
	if b.Order == 0 {
		return LogicalPosition{}
	}
	p := LogicalPosition{box: Box{s, tree, b.Box}, order: b.Order, keyOrder: b.Order, edge: b.Edge, source: src}
	if b.Group != 0 {
		p.group = Box{s, tree, b.Group}
	}
	return p
}
func (s *snapshot) descendant(id, ancestor layout.NodeID) bool {
	for id != 0 {
		if id == ancestor {
			return true
		}
		id = s.sources[id].Parent
	}
	return false
}

func (s *snapshot) hasPositionEvents() bool {
	for _, e := range s.events {
		if e.Active && e.Kind != SourceTargetID {
			return true
		}
	}
	return false
}

func (s *snapshot) positionEvents(inputs []GroupInput) []PositionedEvent {
	if !s.hasPositionEvents() {
		return nil
	}

	before := map[Box]layout.StructuralBoundary{}
	after := map[Box]layout.StructuralBoundary{}
	sourceBoxes := map[layout.NodeID][]Box{}
	parents := map[Box]Box{}
	for ti := 1; ti < len(s.trees); ti++ {
		tree := TreeID(ti)
		var indexParents func(*layout.LayoutNode)
		indexParents = func(n *layout.LayoutNode) {
			if n == nil {
				return
			}
			for _, c := range n.Children {
				parents[Box{s, tree, c.BoxID}] = Box{s, tree, n.BoxID}
				indexParents(c)
			}
		}
		indexParents(s.trees[ti].root)
		for _, b := range s.trees[ti].provenance.Boundaries() {
			box := Box{s, tree, b.Box}
			if b.Edge == layout.BoundaryBefore {
				before[box] = b
			} else {
				after[box] = b
			}
		}
		for _, src := range s.trees[ti].provenance.Sources() {
			for _, id := range src.Occurrences {
				sourceBoxes[src.Source] = append(sourceBoxes[src.Source], Box{s, tree, id})
			}
		}
	}
	sourceBlock := func(id layout.NodeID) Box {
		for id != 0 {
			for _, b := range sourceBoxes[id] {
				if layout.IsBlockContainer(b.node().Box) {
					return b
				}
			}
			id = s.sources[id].Parent
		}
		return Box{}
	}
	var material []EventCoverage
	for _, g := range inputs {
		for _, span := range g.TextSources {
			if span.Start == span.End {
				continue
			}
			value := g.Text[span.Start:span.End]
			// CSS collapsible whitespace is not visible anchor content. NBSP remains.
			trimmed := strings.Trim(value, " \t\r\n\f")
			if trimmed == "" {
				continue
			}
			start := span.Start + uint64(len(value)-len(strings.TrimLeft(value, " \t\r\n\f")))
			end := span.End - uint64(len(value)-len(strings.TrimRight(value, " \t\r\n\f")))
			p := s.logical(span.Box.tree, before[span.Box], span.Source)
			p.group = g.Group
			p.offset = start
			p.keyOffset = start
			p.textPosition = true
			ref := span.Range
			if span.Kind == "" {
				ref.Start += start - span.Start
				ref.End -= span.End - end
			}
			material = append(material, EventCoverage{p, start, end, span.Source, ref, false})
		}
		for _, a := range g.Occurrences {
			p := s.logical(a.Box.tree, before[a.Box], a.Source)
			p.group = g.Group
			p.offset = a.Start
			p.keyOffset = a.Start
			p.textPosition = true
			material = append(material, EventCoverage{p, a.Start, a.End, a.Source, boxtext.TextRange{}, true})
		}
	}
	slices.SortStableFunc(material, func(a, b EventCoverage) int {
		if a.Position.box.tree != b.Position.box.tree {
			return cmp.Compare(a.Position.box.tree, b.Position.box.tree)
		}
		v, _ := a.Position.Compare(b.Position)
		return v
	})
	// Material in a different atomic interior is not independently placeable.
	accessible := func(anchor, source layout.NodeID) bool {
		for id := s.sources[source].Parent; id != 0; id = s.sources[id].Parent {
			for _, b := range sourceBoxes[id] {
				if b.node().Box == layout.BoxInlineBlock && !s.descendant(anchor, id) {
					return false
				}
			}
		}
		return true
	}
	// Retained blocks matter even when they have no rendered text. Keep these
	// structural affinity candidates separate from visible anchor coverage.
	affinityMaterial := slices.Clone(material)
	for id, boxes := range sourceBoxes {
		for _, b := range boxes {
			if b.node().Box == layout.BoxBlock {
				affinityMaterial = append(affinityMaterial, EventCoverage{Position: s.logical(b.tree, before[b], s.sources[id].Source), Source: s.sources[id].Source})
			}
		}
	}
	slices.SortStableFunc(affinityMaterial, func(a, b EventCoverage) int {
		if a.Position.box.tree != b.Position.box.tree {
			return cmp.Compare(a.Position.box.tree, b.Position.box.tree)
		}
		v, _ := a.Position.Compare(b.Position)
		return v
	})
	var out []PositionedEvent
	for _, e := range s.events {
		if !e.Active || e.Kind == SourceTargetID {
			continue
		}
		r := PositionedEvent{Event: e, Affinity: AffinityStructural}
		block := sourceBlock(e.Source.NodeID)
		if block.Valid() {
			r.Block = s.sources[block.node().NodeID].Source
			r.Position = s.logical(block.tree, before[block], r.Block)
		}
		if e.Kind == SourcePageBreak {
			boxes := sourceBoxes[e.Source.NodeID]
			r.UnsupportedBreak = e.Channel != "main" || len(boxes) != 1 || boxes[0].node().Box != layout.BoxBlock
			for id := e.Source.NodeID; id != 0; id = s.sources[id].Parent {
				for _, b := range sourceBoxes[id] {
					if b.node().Box == layout.BoxInlineBlock {
						r.UnsupportedBreak = true
					}
				}
			}
			if !r.UnsupportedBreak {
				b := boxes[0]
				// Propagate only across actual first normalized children, skipping no
				// retained material. Anonymous wrappers do not introduce source ancestry.
				for parent := parents[b]; parent.Valid() && len(parent.node().Children) > 0 && parent.node().Children[0].BoxID == b.id && parent.node().Box == layout.BoxBlock; parent = parents[b] {
					b = parent
				}

				r.Position = s.logical(b.tree, before[b], s.sources[b.node().NodeID].Source)
			}
			out = append(out, r)
			continue
		}
		for _, m := range material {
			if m.Position.box.tree == e.Tree && s.descendant(m.Source.NodeID, e.Source.NodeID) && accessible(e.Source.NodeID, m.Source.NodeID) {
				r.Coverage = append(r.Coverage, m)
			}
		}
		if len(r.Coverage) > 0 {
			r.Affinity = AffinityVisible
			r.Position = r.Coverage[0].Position
		} else if block.Valid() {
			var following, preceding *EventCoverage
			for i := range affinityMaterial {
				m := &affinityMaterial[i]
				if m.Position.box.tree != e.Tree || !s.descendant(m.Source.NodeID, block.node().NodeID) || s.descendant(m.Source.NodeID, e.Source.NodeID) || s.descendant(e.Source.NodeID, m.Source.NodeID) || !accessible(e.Source.NodeID, m.Source.NodeID) {
					continue
				}
				if m.Source.NodeID > e.Source.NodeID {
					following = m
					break
				}
				preceding = m
			}
			chosen := following
			if chosen != nil {
				r.Affinity = AffinityFollowing
			} else {
				chosen = preceding
				if chosen != nil {
					r.Affinity = AffinityPreceding
				}
			}
			if chosen != nil {
				c := *chosen
				r.Position = c.Position
				if r.Affinity == AffinityPreceding {
					r.Position.offset = c.End
					c.Start = c.End
					c.Range.Start = c.Range.End
				} else {
					c.End = c.Start
					c.Range.End = c.Range.Start
				}
				// Crossing into a descendant block attaches at that block's opening
				// boundary, so a forced break there puts the event into the new span.
				targetBlock := sourceBlock(chosen.Source.NodeID)
				if targetBlock.Valid() && targetBlock != block && targetBlock.node().Box == layout.BoxBlock {
					edge := before[targetBlock]
					if r.Affinity == AffinityPreceding {
						edge = after[targetBlock]
					}
					r.Position = s.logical(targetBlock.tree, edge, s.sources[targetBlock.node().NodeID].Source)
				}
				c.Position.offset = c.Start
				if c.Position.textPosition {
					r.Coverage = []EventCoverage{c}
				} else {
					edge := before[c.Position.box]
					if r.Affinity == AffinityPreceding {
						edge = after[c.Position.box]
					}
					r.Position = s.logical(c.Position.box.tree, edge, c.Source)
				}
			}
		}
		if r.Affinity == AffinityStructural && block.Valid() {
			for _, g := range inputs {
				if g.Owner == block && g.Text == "" {
					pos := s.logical(block.tree, before[g.Group], r.Block)
					pos.group = g.Group
					pos.textPosition = true
					r.Coverage = []EventCoverage{{Position: pos, Source: e.Source}}
					break
				}
			}
		}
		out = append(out, r)
	}
	canonicalizeEventPositions(inputs, before, after, out)
	return out
}

func attachGroupEvents(inputs []GroupInput, events []PositionedEvent) {
	for i := range inputs {
		for _, e := range events {
			for _, c := range e.Coverage {
				if c.Position.group == inputs[i].Group {
					inputs[i].Events = append(inputs[i].Events, GroupEvent{e.Event.Source, e.Event.Kind, e.Affinity, c.Start, c.End, c.Source, c.Range, c.Atomic})
				}
			}
		}
	}
}

// Canonicalize group byte boundaries onto normalized leaf positions. In
// particular, the end of an atomic marker lies after its entire interior, and
// adjacent source ranges sharing a processed offset compare equally.
func canonicalizeEventPositions(inputs []GroupInput, before, after map[Box]layout.StructuralBoundary, events []PositionedEvent) {
	type unit struct {
		start, end uint64
		box        Box
	}
	type groupCoordinates struct {
		units []unit
		first map[Box]uint64
	}
	coordinates := map[Box]groupCoordinates{}
	for _, g := range inputs {
		c := groupCoordinates{first: map[Box]uint64{}}
		for _, t := range g.TextSources {
			if t.Start < t.End {
				c.units = append(c.units, unit{t.Start, t.End, t.Box})
			}
		}
		for _, a := range g.Occurrences {
			c.units = append(c.units, unit{a.Start, a.End, a.Box})
		}
		slices.SortStableFunc(c.units, func(a, b unit) int { return cmp.Compare(a.start, b.start) })
		for _, u := range c.units {
			if _, ok := c.first[u.box]; !ok {
				c.first[u.box] = u.start
			}
		}
		coordinates[g.Group] = c
	}
	canonical := func(p LogicalPosition) LogicalPosition {
		if !p.textPosition {
			return p
		}
		c := coordinates[p.group]
		for _, u := range c.units {
			if p.offset < u.end {
				p.keyOrder = before[u.box].Order
				p.keyOffset = p.offset - c.first[u.box]
				return p
			}
		}
		p.keyOrder = after[p.group].Order
		p.keyOffset = 0
		if len(c.units) == 0 {
			p.keyOrder = before[p.group].Order
		}
		return p
	}
	for i := range events {
		events[i].Position = canonical(events[i].Position)
		for j := range events[i].Coverage {
			events[i].Coverage[j].Position = canonical(events[i].Coverage[j].Position)
		}
	}
}
