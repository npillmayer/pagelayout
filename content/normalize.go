package content

import (
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/css-box-layout/registers"
	boxtext "github.com/npillmayer/css-box-layout/text"
	"golang.org/x/net/html"
)

// normalizePartition is the atomic publication boundary. All construction and
// indexing is invocation-local; successful earlier trees never escape a later
// failure. The builder owns normalization, including omission of collapsible
// runs. Source coverage is checked on the partition, not by counting boxes.
func normalizePartition(p *partitionProjection) (*Document, error) {
	if err := validatePartition(p); err != nil {
		return nil, err
	}
	s := &snapshot{sources: make([]SourceInfo, len(p.sources)), paint: make([]Paint, len(p.sources)),
		texts: make(map[boxtext.TextSourceID]TextInfo), trees: make([]materialTree, len(p.branches)+2),
		branches: make(map[string][]BranchInfo)}
	branchTrees := make(map[sourceID]TreeID)
	branchChannels := make(map[sourceID]string)
	for i, b := range p.branches {
		branchTrees[b.source], branchChannels[b.source] = TreeID(i+2), b.channel
	}
	for _, source := range p.sources[1:] {
		id := source.ref.nodeID
		v := SourceInfo{Source: publicSource(source.ref), Parent: layout.NodeID(source.parent),
			Kind: source.nodeType, Name: source.name, Namespace: source.namespace,
			Attributes: slices.Clone(source.attributes), TextStyle: source.typography,
			HasTextStyle: source.typography.FontSize > 0, BreakBefore: source.breakBefore}
		for _, child := range source.children {
			v.Children = append(v.Children, layout.NodeID(child))
		}
		switch source.participation {
		case materialNode:
			v.Classification, v.Tree, v.Channel = SourceResidual, 1, "main"
			if p.residual != nil {
				v.BranchRoot = p.residual.ID
			}
			if root := p.owners[id]; root != 0 {
				v.Classification, v.Tree = SourceNamed, branchTrees[root]
				v.Channel, v.BranchRoot = branchChannels[root], layout.NodeID(root)
			}
		case suppressedNode:
			v.Classification = SourceSuppressed
		case inertNode:
			v.Classification = SourceInert
		case metadataNode:
			v.Classification = SourceMetadata
		default:
			v.Classification = SourceContext
		}
		s.sources[id] = v
		s.paint[id].Background = source.paint.background
		for side, border := range source.paint.borders {
			s.paint[id].Borders[side] = BorderPaint{Style: border.style, Color: border.color}
		}
		if source.nodeType == html.TextNode {
			tid := boxtext.TextSourceID(id)
			s.texts[tid] = TextInfo{ID: tid, Source: v.Source, Text: source.text}
		}
	}
	build := func(id TreeID, input *layout.RenderNode) error {
		root, err := layout.BuildLayoutTree(input, layout.BuildOptions{ReadText: s.readText})
		if err != nil {
			var located *layout.BuildError
			var source sourceID
			if input != nil {
				source = sourceID(input.ID)
			}
			if errors.As(err, &located) {
				source = sourceID(located.RootID)
				if located.HasNode {
					source = sourceID(located.NodeID)
				}
			}
			return materialError(p, source, "box.build", "cannot normalize content branch", err)
		}
		if input != nil && root == nil {
			return materialError(p, sourceID(input.ID), "material.references", "active root produced no box", nil)
		}
		// Check the full projection, including text the builder omitted. Bounds
		// alone would permit a valid but truncated range to lose source bytes.
		if err := validateTextCoverage(p, input); err != nil {
			return err
		}
		s.trees[id].root = root
		if err := s.indexTree(id); err != nil {
			var source sourceID
			if input != nil {
				source = sourceID(input.ID)
			}
			return materialError(p, source, "material.references", "invalid normalized material", err)
		}
		return nil
	}
	if err := build(1, p.residual); err != nil {
		return nil, err
	}
	s.channels = []string{"main"}
	if root := s.trees[1].root; root != nil {
		box := Box{s, 1, root.BoxID}
		s.branches["main"] = []BranchInfo{{Tree: 1, Root: box, Source: publicSource(p.sources[p.residual.ID].ref), Order: 1}}
		s.bank = s.bank.Append("main", box)
	}
	for i, branch := range p.branches {
		tree := TreeID(i + 2)
		if err := build(tree, branch.root); err != nil {
			return nil, err
		}
		root := Box{s, tree, s.trees[tree].root.BoxID}
		s.branches[branch.channel] = append(s.branches[branch.channel], BranchInfo{
			Tree: tree, Root: root, Source: publicSource(p.sources[branch.source].ref), Order: i + 2})
		s.bank = s.bank.Append(registers.RegisterID(branch.channel), root)
	}
	for _, channel := range p.channels {
		s.channels = append(s.channels, channel.name)
	}
	s.captureEvents()
	return &Document{material: s}, nil
}

func validateTextCoverage(p *partitionProjection, n *layout.RenderNode) error {
	if n == nil {
		return nil
	}
	s := p.sources[n.ID]
	if s.nodeType == html.TextNode && (n.Text == nil || *n.Text != s.textRef) {
		return materialError(p, sourceID(n.ID), "material.references", "projection does not retain the complete source text range", nil)
	}
	for _, child := range n.ChildrenNodes {
		if err := validateTextCoverage(p, child); err != nil {
			return err
		}
	}
	return nil
}

func materialError(p *partitionProjection, id sourceID, code, message string, cause error) error {
	var ref sourceRef
	if id > 0 && uint64(id) < uint64(len(p.sources)) {
		ref = p.sources[id].ref
	}
	return newLoadError([]diagnostic{{stage: "normalization", code: code, source: ref, message: message, cause: cause}})
}

// Validate ownership before normalization, which may split an inline source or
// omit an entirely collapsible run. Neither is duplicate/missing source material.
func validatePartition(p *partitionProjection) error {
	fail := func(id sourceID, message string) error {
		return materialError(p, id, "material.ownership", message, nil)
	}
	seen := make(map[sourceID]bool)
	var visit func(*layout.RenderNode, sourceID) error
	visit = func(n *layout.RenderNode, owner sourceID) error {
		if n == nil {
			return fail(owner, "missing material node")
		}
		id := sourceID(n.ID)
		if id == 0 || uint64(id) >= uint64(len(p.sources)) {
			return fail(owner, "unknown material source")
		}
		actual, ok := p.owners[id]
		if seen[id] || !ok || actual != owner || p.sources[id].participation != materialNode {
			return fail(id, "duplicate or conflicting material ownership")
		}
		seen[id] = true
		for _, child := range n.ChildrenNodes {
			if err := visit(child, owner); err != nil {
				return err
			}
		}
		return nil
	}
	if p.residual != nil {
		if err := visit(p.residual, 0); err != nil {
			return err
		}
	}
	channels := make(map[string][]int)
	var order []string
	var previous sourceID
	for i, b := range p.branches {
		if b.channel == "" || b.root == nil || b.source != sourceID(b.root.ID) || b.source <= previous {
			return fail(b.source, "invalid branch root or source order")
		}
		previous = b.source
		if err := visit(b.root, b.source); err != nil {
			return err
		}
		if _, ok := channels[b.channel]; !ok {
			order = append(order, b.channel)
		}
		channels[b.channel] = append(channels[b.channel], i)
	}
	for _, source := range p.sources[1:] {
		id := source.ref.nodeID
		_, owned := p.owners[id]
		material := source.participation == materialNode
		if material != seen[id] || material != owned {
			return fail(id, "missing material or ownership for excluded source")
		}
	}
	if len(seen) != len(p.owners) || len(order) != len(p.channels) {
		return fail(0, "invalid ownership or channel inventory")
	}
	for i, c := range p.channels {
		if c.name != order[i] || !slices.Equal(c.branches, channels[c.name]) {
			return fail(0, "invalid channel branch order")
		}
	}
	return nil
}

// readText validates half-open decoded-byte ranges, including rune boundaries.
// It is deliberately separate from paragraph whitespace transformation.
func (s *snapshot) readText(ref boxtext.TextRef) (string, error) {
	t, ok := s.texts[ref.Source]
	if !ok || ref.Range.Start > ref.Range.End || ref.Range.End > uint64(len(t.Text)) {
		return "", fmt.Errorf("invalid text source/range: %+v", ref)
	}
	boundary := func(i uint64) bool { return i == uint64(len(t.Text)) || utf8.RuneStart(t.Text[i]) }
	if !boundary(ref.Range.Start) || !boundary(ref.Range.End) {
		return "", fmt.Errorf("text range splits UTF-8: %+v", ref)
	}
	return t.Text[ref.Range.Start:ref.Range.End], nil
}

// indexTree validates source/box correspondence, then consumes the core's shared
// group enumeration. Only content owns source context and snapshot qualification;
// group ownership and atomic relationships come from layout.InlineGroups.
func (s *snapshot) indexTree(id TreeID) error {
	t := &s.trees[id]
	t.nodes = make(map[layout.BoxID]*layout.LayoutNode)
	t.contexts = make(map[layout.BoxID]layout.NodeID)
	var walk func(*layout.LayoutNode, layout.NodeID) error
	walk = func(n *layout.LayoutNode, context layout.NodeID) error {
		if n == nil || n.BoxID == 0 || t.nodes[n.BoxID] != nil || n.Style == nil {
			return fmt.Errorf("missing or duplicate box identity/style")
		}
		t.nodes[n.BoxID] = n
		if n.NodeID != 0 {
			if uint64(n.NodeID) >= uint64(len(s.sources)) || s.sources[n.NodeID].Tree != id {
				return fmt.Errorf("box %d references source outside its tree", n.BoxID)
			}
			t.contexts[n.BoxID] = n.NodeID
		} else {
			if n.Box != layout.BoxAnonymousBlock && n.Box != layout.BoxAnonymousInline {
				return fmt.Errorf("non-anonymous box %d has no source", n.BoxID)
			}
			t.contexts[n.BoxID] = context
		}
		if n.Box == layout.BoxText {
			if layout.NodeID(n.Text.Source) != n.NodeID {
				return fmt.Errorf("text box %d references a different source", n.BoxID)
			}
			if _, err := s.readText(n.Text); err != nil {
				return err
			}
		}
		if layout.IsBlockContainer(n.Box) {
			if n.NodeID != 0 {
				context = n.NodeID
			}
		}
		if context == 0 || uint64(context) >= uint64(len(s.sources)) || !s.sources[context].HasTextStyle {
			return fmt.Errorf("box %d has no source paragraph context", n.BoxID)
		}
		for _, child := range n.Children {
			if err := walk(child, context); err != nil {
				return err
			}
		}
		return nil
	}
	if t.root == nil {
		return nil
	}
	if err := walk(t.root, 0); err != nil {
		return err
	}
	for g := range layout.InlineGroups(t.root) {
		if g.Owner == nil {
			return fmt.Errorf("group %d has no block owner", g.Group.BoxID)
		}
		c := s.sources[t.contexts[g.Owner.BoxID]]
		info := GroupInfo{Group: Box{s, id, g.Group.BoxID}, Owner: Box{s, id, g.Owner.BoxID},
			Source: c.Source, TextStyle: c.TextStyle}
		if g.ParentAtomic != nil {
			info.ParentAtomic = Box{s, id, g.ParentAtomic.BoxID}
		}
		for _, atomic := range g.Atomics {
			info.Atomics = append(info.Atomics, Box{s, id, atomic.BoxID})
		}
		t.groups = append(t.groups, info)
	}
	return nil
}
