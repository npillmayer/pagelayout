package content

import (
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
)

// SourceEventKind identifies detached source facts, not placed pagination events.
type SourceEventKind string

const (
	// SourceTargetID records an authored ID, including duplicate and excluded IDs.
	SourceTargetID SourceEventKind = "target-id"
	// SourceFlowAnchor records data-flow-anchor presence and the unmodified href.
	SourceFlowAnchor SourceEventKind = "flow-anchor"
	// SourcePageBreak records an effective forced break, normalized to page.
	SourcePageBreak SourceEventKind = "page-break"
)

// SourceEvent preserves an original element position via Source and Parent.
// NodeIDs are source-preorder identities local to this Document; NodeSource gives
// original ancestry and decoded descendant text through TextSource. These are
// not normalized-box offsets or prepared-text coordinates. Value holds the ID,
// raw href, or "page". Active means material participates after suppression;
// excluded target IDs/anchor annotations remain available for later diagnostics.
// Plan validation and target resolution belong to pagelayout. EventPositions
// supplies normalized boundary affinity and prepared group coverage. No ownership is inferred from ordinary links.
type SourceEvent struct {
	Kind       SourceEventKind
	Source     SourceRef
	Parent     layout.NodeID
	Value      string
	Active     bool
	Tree       TreeID
	Channel    string
	BranchRoot layout.NodeID
}

// SourceEvents returns independent source-order records. Within an element,
// target ID precedes owning-link annotation, which precedes its effective break.
// Empty and suppressed annotations are retained without generating material.
func (d *Document) SourceEvents() []SourceEvent {
	if d == nil || d.material == nil {
		return nil
	}
	return slices.Clone(d.material.events)
}

func (s *snapshot) captureEvents() {
	for _, source := range s.sources[1:] {
		active := source.Classification == SourceResidual || source.Classification == SourceNamed
		add := func(kind SourceEventKind, value string) {
			s.events = append(s.events, SourceEvent{Kind: kind, Source: source.Source, Parent: source.Parent, Value: value, Active: active, Tree: source.Tree, Channel: source.Channel, BranchRoot: source.BranchRoot})
		}
		var id, href string
		var hasID, anchor bool
		for _, a := range source.Attributes {
			if a.Namespace != "" {
				continue
			}
			switch a.Key {
			case "id":
				id, hasID = a.Val, true
			case "href":
				href = a.Val
			case "data-flow-anchor":
				anchor = true
			}
		}
		if hasID {
			add(SourceTargetID, id)
		}
		if anchor {
			add(SourceFlowAnchor, href)
		}
		if active && source.BreakBefore == "page" {
			add(SourcePageBreak, "page")
		}
	}
}
