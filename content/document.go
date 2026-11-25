package content

import (
	"image/color"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/css-box-layout/registers"
	boxtext "github.com/npillmayer/css-box-layout/text"
	"github.com/npillmayer/w3c-dom/style/textstyle"
	"golang.org/x/net/html"
)

// TreeID identifies an independent normalized tree within one Document. Zero
// is invalid; numbering is not a persistent identity across loads.
type TreeID uint64

// SourceRef identifies an original node in the repaired HTML document. NodeID
// is local to its snapshot; zero means no node (for example, additional CSS).
// Path is a repaired-DOM path, not an HTML byte offset.
type SourceRef struct {
	Name   string
	NodeID layout.NodeID
	Path   string
}

func publicSource(s sourceRef) SourceRef {
	return SourceRef{Name: s.name, NodeID: layout.NodeID(s.nodeID), Path: s.path}
}

// SourceClass explains a source node's participation, including nodes omitted
// from material. Whitespace omitted by normalization retains its material class.
type SourceClass string

const (
	SourceContext    SourceClass = "context"
	SourceNamed      SourceClass = "named"
	SourceResidual   SourceClass = "residual"
	SourceSuppressed SourceClass = "suppressed"
	SourceInert      SourceClass = "inert"
	SourceMetadata   SourceClass = "metadata"
)

// SourceInfo is copied provenance and source typography, never layout ancestry.
// Tree/Channel/BranchRoot describe ownership before normalization. Excluded
// sources have no Tree. HasTextStyle distinguishes resolved context from sources
// excluded from styling. Children and Attributes are independent copies.
type SourceInfo struct {
	Source          SourceRef
	Parent          layout.NodeID
	Children        []layout.NodeID
	Kind            html.NodeType
	Name, Namespace string
	Attributes      []html.Attribute
	TextStyle       textstyle.Style
	HasTextStyle    bool
	Classification  SourceClass
	Tree            TreeID
	Channel         string
	BranchRoot      layout.NodeID
	// BreakBefore is auto or normalized page for styled elements; empty for excluded sources.
	BreakBefore string
}

// TextInfo retains raw decoded UTF-8, including excluded/omitted source text.
// TextRef ranges use byte offsets into Text, not offsets into HTML markup.
type TextInfo struct {
	ID     boxtext.TextSourceID
	Source SourceRef
	Text   string
}

// BorderPaint describes one computed border. Width is in BoxInfo.Constraints.
type BorderPaint struct {
	Style string
	Color color.NRGBA
}

// Paint contains source box paint. Borders are top, right, bottom, left.
// Anonymous wrappers have no authored paint; text paint is in TextStyle.
type Paint struct {
	Background color.NRGBA
	Borders    [4]BorderPaint
}

// Box is a comparable read-only handle qualified by snapshot, tree and BoxID.
// It retains its complete source snapshot independently of Document lifetime.
// The zero value is invalid. It contains no used geometry or continuation state.
type Box struct {
	owner *snapshot
	tree  TreeID
	id    layout.BoxID
}

// BoxInfo describes an unsized box. Source is absent for anonymous boxes;
// Context identifies the source supplying their paragraph context. Constraints
// are computed values, with percentages and auto sizes still unresolved.
// No tree node or writable style from the snapshot is exposed.
type BoxInfo struct {
	Tree              TreeID
	ID                layout.BoxID
	Kind              layout.BoxKind
	FormattingContext layout.FormattingContextKind
	EstablishesBFC    bool
	Source, Context   SourceRef
	Constraints       layout.ComputedStyle
	Paint             Paint
	TextStyle         textstyle.Style
	Text              boxtext.TextRef // Nonzero only for text boxes.
}

// Valid reports whether b refers to a box in its retained snapshot.
func (b Box) Valid() bool { return b.node() != nil }

func (b Box) node() *layout.LayoutNode {
	if b.owner == nil || b.tree == 0 || uint64(b.tree) >= uint64(len(b.owner.trees)) {
		return nil
	}
	return b.owner.trees[b.tree].nodes[b.id]
}

// Info returns an independent value description, or false for an invalid Box.
func (b Box) Info() (BoxInfo, bool) {
	n := b.node()
	if n == nil {
		return BoxInfo{}, false
	}
	t := b.owner.trees[b.tree]
	c := b.owner.sources[t.contexts[b.id]]
	v := BoxInfo{Tree: b.tree, ID: b.id, Kind: n.Box, FormattingContext: n.FC,
		EstablishesBFC: n.EstablishesBFC, Context: c.Source, TextStyle: c.TextStyle, Text: n.Text}
	if n.NodeID != 0 {
		v.Source = b.owner.sources[n.NodeID].Source
		v.Paint = b.owner.paint[n.NodeID]
	}
	v.Constraints = *n.Style
	if n.Style.Fixed != nil {
		fixed := *n.Style.Fixed
		v.Constraints.Fixed = &fixed
	}
	return v, true
}

// TreeInfo identifies a tree and its root. A fully suppressed body has a
// residual TreeID but an invalid Root and no groups.
type TreeInfo struct {
	Tree TreeID
	Root Box
}

// BranchInfo identifies a channel root. Main has Order 1; secondary roots follow
// in global source order. Order is not the position within a register.
type BranchInfo struct {
	Tree   TreeID
	Root   Box
	Source SourceRef
	Order  int
}

// GroupInfo identifies an actual anonymous inline group and its nearest block
// owner. Source/TextStyle come from the owner's source context, even for empty
// groups and anonymous owners, never from the first text run. Atomics lists the
// group's inline-block boundaries in order, excluding their interiors.
// ParentAtomic links an interior group to its nearest enclosing inline-block;
// it is invalid for groups outside atomics. Groups are not prepared paragraphs.
type GroupInfo struct {
	Group, Owner Box
	Source       SourceRef
	TextStyle    textstyle.Style
	Atomics      []Box
	ParentAtomic Box
}

type materialTree struct {
	root     *layout.LayoutNode
	nodes    map[layout.BoxID]*layout.LayoutNode
	contexts map[layout.BoxID]layout.NodeID
	groups   []GroupInfo
}

type snapshot struct {
	sources  []SourceInfo // source ID zero is reserved
	texts    map[boxtext.TextSourceID]TextInfo
	paint    []Paint
	trees    []materialTree // tree ID zero is reserved; residual is first
	channels []string
	branches map[string][]BranchInfo
	bank     registers.RegisterBank[Box]
	events   []SourceEvent
}

// Document owns immutable normalized content and its initial register bank.
// Inspection and independent bank derivation are safe concurrently. No fonts,
// frame constraints, used dimensions or preparation sessions are retained.
// A zero Document behaves as empty; use Load for a valid material snapshot.
type Document struct{ material *snapshot }

// Bank returns the initial persistent bank. Items are whole branch-root Boxes;
// main contains the existing residual root when present. Derivation does not update Document.
func (d *Document) Bank() registers.RegisterBank[Box] {
	if d == nil || d.material == nil {
		return registers.RegisterBank[Box]{}
	}
	return d.material.bank
}

// Channels returns main first, then secondary channels in first-occurrence source order.
func (d *Document) Channels() []string {
	if d == nil || d.material == nil {
		return nil
	}
	return slices.Clone(d.material.channels)
}

// Branches returns a channel's branch descriptions in source order.
func (d *Document) Branches(channel string) []BranchInfo {
	if d == nil || d.material == nil {
		return nil
	}
	return slices.Clone(d.material.branches[channel])
}

// Residual inspects the same tree stored under main, never a second material queue.
func (d *Document) Residual() TreeInfo {
	if d == nil || d.material == nil {
		return TreeInfo{}
	}
	s := d.material
	result := TreeInfo{Tree: 1}
	if root := s.trees[1].root; root != nil {
		result.Root = Box{s, 1, root.BoxID}
	}
	return result
}

// Groups returns copied groups in normalized preorder, including atomic
// interiors. Unknown tree identities return nil. IDs are local to Document.
func (d *Document) Groups(tree TreeID) []GroupInfo {
	if d == nil || d.material == nil || tree == 0 || uint64(tree) >= uint64(len(d.material.trees)) {
		return nil
	}
	groups := slices.Clone(d.material.trees[tree].groups)
	for i := range groups {
		groups[i].Atomics = slices.Clone(groups[i].Atomics)
	}
	return groups
}

// NodeSource returns copied provenance for a snapshot-local source node.
func (d *Document) NodeSource(id layout.NodeID) (SourceInfo, bool) {
	if d == nil || d.material == nil || id == 0 || uint64(id) >= uint64(len(d.material.sources)) {
		return SourceInfo{}, false
	}
	v := d.material.sources[id]
	v.Children = slices.Clone(v.Children)
	v.Attributes = slices.Clone(v.Attributes)
	return v, true
}

// TextSource returns decoded text for a snapshot-local ID, including text
// excluded from material. Unknown IDs and non-text node IDs return false.
func (d *Document) TextSource(id boxtext.TextSourceID) (TextInfo, bool) {
	if d == nil || d.material == nil {
		return TextInfo{}, false
	}
	v, ok := d.material.texts[id]
	return v, ok
}
