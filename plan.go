package pagelayout

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/pagelayout/content"
)

// ErrInvalidPlan identifies invalid source ownership or page-break placement.
// Use errors.As with PlanError to inspect source diagnostics.
var ErrInvalidPlan = errors.New("invalid pagination plan")

// PlanDiagnostic identifies source-contract failures without parsing prose.
// Related retains conflicting owners, targets or ownership-chain locations.
type PlanDiagnostic struct {
	Code    string
	Source  content.SourceRef
	Related []content.SourceRef
	Message string
}

// PlanError reports source ownership/break validation failures. Diagnostics are
// ordered and copied on inspection; errors.Is matches ErrInvalidPlan and original causes.
type PlanError struct {
	diagnostics []PlanDiagnostic
	cause       error
}

// Error formats the ordered plan diagnostics and original cause.
func (e *PlanError) Error() string {
	var messages []string
	for _, d := range e.diagnostics {
		messages = append(messages, fmt.Sprintf("%s:%s: %s: %s", d.Source.Name, d.Source.Path, d.Code, d.Message))
	}
	if e.cause != nil {
		messages = append(messages, e.cause.Error())
	}
	return "pagination plan: " + strings.Join(messages, "; ")
}

// Unwrap exposes ErrInvalidPlan and the original cause, when present.
func (e *PlanError) Unwrap() []error {
	if e.cause != nil {
		return []error{ErrInvalidPlan, e.cause}
	}
	return []error{ErrInvalidPlan}
}

// Diagnostics returns independent source-aware failures and related-location slices.
func (e *PlanError) Diagnostics() []PlanDiagnostic {
	out := slices.Clone(e.diagnostics)
	for i := range out {
		out[i].Related = slices.Clone(out[i].Related)
	}
	return out
}

// BranchID is a stable ordinal within this plan, not a global source identity.
type BranchID uint64

// PlanBranch associates a source-order branch with its channel, owner and pagespan.
// Main spans the document and has Span zero; secondary spans are one-based.
type PlanBranch struct {
	ID       BranchID
	Channel  string
	Material content.BranchInfo
	Owner    content.SourceRef
	Span     int // one-based for secondary branches; main spans the whole document
}

// Anchor describes one placement-dependent activation. A visible anchor has
// all logical source coverage; the first placed covered fragment activates it
// once. An empty anchor uses its affinity position. Planning activates nothing.
type Anchor struct {
	Source              content.SourceRef
	OwnerBranch, Target BranchID
	Span                int
	Block               content.SourceRef
	Position            content.LogicalPosition
	Affinity            content.EventAffinity
	Coverage            []content.EventCoverage
}

// SpanBoundary records a coalesced main-flow stop and its contributing source nodes.
// Initial and Terminal mark document endpoints, which do not create blank pages.
type SpanBoundary struct {
	Position          content.LogicalPosition
	Sources           []content.SourceRef
	Initial, Terminal bool
}

// PageSpan is one main interval together with all transitively owned secondary targets.
// IDs are one-based; Targets preserve source branch order, not anchor order.
type PageSpan struct {
	ID         int
	Start, End content.LogicalPosition
	Targets    []BranchID // original branch order, never anchor order
}

// SpanPlan is immutable source planning only. It owns no fonts, layout services,
// preparation session, mutable activation state or register advances.
type SpanPlan struct {
	document   content.Document
	branches   []PlanBranch
	anchors    []Anchor
	boundaries []SpanBoundary
	spans      []PageSpan
}

// Document returns an independent handle to the plan's unchanged source snapshot.
func (p *SpanPlan) Document() *content.Document {
	if p == nil {
		return nil
	}
	d := p.document
	return &d
}

// Matches reports exact source identity, not structural equality with another load.
func (p *SpanPlan) Matches(d *content.Document) bool {
	return p != nil && d != nil && p.document.SourceID().Valid() && p.document.SourceID() == d.SourceID()
}

// Branches returns branch descriptions in source order.
func (p *SpanPlan) Branches() []PlanBranch {
	if p == nil {
		return nil
	}
	return slices.Clone(p.branches)
}

// Anchors returns copied owning-link descriptions and logical coverage slices.
func (p *SpanPlan) Anchors() []Anchor {
	if p == nil {
		return nil
	}
	out := slices.Clone(p.anchors)
	for i := range out {
		out[i].Coverage = slices.Clone(out[i].Coverage)
	}
	return out
}

// Boundaries returns copied coalesced stops, including document endpoints.
func (p *SpanPlan) Boundaries() []SpanBoundary {
	if p == nil {
		return nil
	}
	out := slices.Clone(p.boundaries)
	for i := range out {
		out[i].Sources = slices.Clone(out[i].Sources)
	}
	return out
}

// Spans returns copied optimization intervals and their transitive target lists.
func (p *SpanPlan) Spans() []PageSpan {
	if p == nil {
		return nil
	}
	out := slices.Clone(p.spans)
	for i := range out {
		out[i].Targets = slices.Clone(out[i].Targets)
	}
	return out
}

// Plan validates owning links and forced breaks, preserving authored queues.
// Source planning is independent of template collections and preparation. On
// any error it returns nil; an invalid graph is never partially published.
func Plan(d *content.Document) (*SpanPlan, error) {
	if d == nil || !d.SourceID().Valid() {
		return nil, &PlanError{cause: fmt.Errorf("a loaded document is required")}
	}
	p := &SpanPlan{document: *d}
	var diagnostics []PlanDiagnostic
	fail := func(code string, source content.SourceRef, message string, related ...content.SourceRef) {
		diagnostics = append(diagnostics, PlanDiagnostic{code, source, slices.Clone(related), message})
	}
	byTree := map[content.TreeID]BranchID{}
	byNode := map[layout.NodeID]BranchID{}
	for _, channel := range d.Channels() {
		for _, b := range d.Branches(channel) {
			p.branches = append(p.branches, PlanBranch{Channel: channel, Material: b})
		}
	}
	slices.SortFunc(p.branches, func(a, b PlanBranch) int { return a.Material.Order - b.Material.Order })
	for i := range p.branches {
		b := &p.branches[i]
		b.ID = BranchID(i + 1)
		byTree[b.Material.Tree] = b.ID
		byNode[b.Material.Source.NodeID] = b.ID
	}
	sourceEvents := d.SourceEvents()
	ids := map[string][]content.SourceEvent{}
	for _, e := range sourceEvents {
		if e.Kind == content.SourceTargetID {
			ids[e.Value] = append(ids[e.Value], e)
		}
	}
	events, err := d.EventPositions()
	if err != nil {
		return nil, &PlanError{cause: err}
	}
	start, end := d.TreeBounds(d.Residual().Tree)
	owner := map[BranchID]int{}
	for _, e := range events {
		if e.Event.Kind == content.SourcePageBreak {
			if e.UnsupportedBreak {
				fail("unsupported-break-location", e.Event.Source, "forced breaks require block-level main material outside atomics")
				continue
			}
			initial := false
			if c, ok := e.Position.Compare(start); ok && c == 0 {
				initial = true
			}
			p.boundaries = append(p.boundaries, SpanBoundary{Position: e.Position, Sources: []content.SourceRef{e.Event.Source}, Initial: initial})
			continue
		}
		src, _ := d.NodeSource(e.Event.Source.NodeID)
		if src.Name != "a" {
			fail("invalid-owner", e.Event.Source, "data-flow-anchor requires an a element")
			continue
		}
		u, parseErr := url.Parse(e.Event.Value)
		if parseErr != nil || !strings.HasPrefix(e.Event.Value, "#") || u == nil || u.Fragment == "" {
			fail("unresolved-target", e.Event.Source, "owner requires a nonempty same-document fragment href")
			continue
		}
		targets := ids[u.Fragment]
		if len(targets) == 0 {
			fail("unresolved-target", e.Event.Source, "no target for "+e.Event.Value)
			continue
		}
		if len(targets) > 1 {
			related := make([]content.SourceRef, len(targets))
			for i, t := range targets {
				related[i] = t.Source
			}
			fail("duplicate-target", e.Event.Source, "target ID is not unique", related...)
			continue
		}
		target := targets[0]
		if !target.Active {
			fail("inactive-target", e.Event.Source, "target does not participate", target.Source)
			continue
		}
		bid, ok := byNode[target.Source.NodeID]
		if !ok || target.Channel == "main" {
			related := []content.SourceRef{target.Source}
			if root, ok := d.NodeSource(target.BranchRoot); ok {
				related = append(related, root.Source)
			}
			fail("non-root-target", e.Event.Source, "owner must target an independently scheduled secondary branch root", related...)
			continue
		}
		if previous, exists := owner[bid]; exists {
			fail("duplicate-owner", e.Event.Source, "branch has more than one active owner", p.anchors[previous].Source, target.Source)
			continue
		}
		owner[bid] = len(p.anchors)
		p.anchors = append(p.anchors, Anchor{Source: e.Event.Source, OwnerBranch: byTree[e.Event.Tree], Target: bid, Block: e.Block, Position: e.Position, Affinity: e.Affinity, Coverage: slices.Clone(e.Coverage)})
	}
	for _, b := range p.branches {
		if b.Channel != "main" {
			if _, ok := owner[b.ID]; !ok {
				var related []content.SourceRef
				for _, event := range sourceEvents {
					if event.Kind != content.SourceFlowAnchor || event.Active {
						continue
					}
					u, err := url.Parse(event.Value)
					if err == nil && u != nil && strings.HasPrefix(event.Value, "#") {
						for _, target := range ids[u.Fragment] {
							if target.Source == b.Material.Source {
								related = append(related, event.Source)
							}
						}
					}
				}
				fail("missing-owner", b.Material.Source, "secondary branch requires one active owner", related...)
			}
		}
	}
	// Coalesce propagated forced boundaries and retain all original locations.
	sort.SliceStable(p.boundaries, func(i, j int) bool { c, _ := p.boundaries[i].Position.Compare(p.boundaries[j].Position); return c < 0 })
	var boundaries []SpanBoundary
	for _, b := range p.boundaries {
		if len(boundaries) > 0 {
			last := &boundaries[len(boundaries)-1]
			if c, ok := last.Position.Compare(b.Position); ok && c == 0 {
				last.Sources = append(last.Sources, b.Sources...)
				continue
			}
		}
		boundaries = append(boundaries, b)
	}
	p.boundaries = boundaries
	from := start
	for _, b := range p.boundaries {
		if b.Initial {
			continue
		}
		p.spans = append(p.spans, PageSpan{ID: len(p.spans) + 1, Start: from, End: b.Position})
		from = b.Position
	}
	p.spans = append(p.spans, PageSpan{ID: len(p.spans) + 1, Start: from, End: end})
	p.boundaries = append(p.boundaries, SpanBoundary{Position: end, Terminal: true})
	mainSpan := func(pos content.LogicalPosition) int {
		span := 1
		for _, b := range p.boundaries {
			if b.Initial || b.Terminal {
				continue
			}
			if c, ok := pos.Compare(b.Position); ok && c >= 0 {
				span++
			}
		}
		return span
	}
	state := map[BranchID]uint8{}
	var resolve func(BranchID) int
	resolve = func(id BranchID) int {
		if id == 0 {
			return 0
		}
		b := &p.branches[int(id)-1]
		if b.Channel == "main" {
			return 0
		}
		if state[id] == 2 {
			return b.Span
		}
		if state[id] == 1 {
			var related []content.SourceRef
			for _, candidate := range p.branches {
				if state[candidate.ID] == 1 {
					related = append(related, candidate.Material.Source)
					if i, ok := owner[candidate.ID]; ok {
						related = append(related, p.anchors[i].Source)
					}
				}
			}
			fail("ownership-cycle", b.Material.Source, "ownership chain does not reach main", related...)
			return 0
		}
		i, ok := owner[id]
		if !ok {
			return 0
		}
		state[id] = 1
		a := &p.anchors[i]
		if a.OwnerBranch == 0 {
			fail("unrooted-owner", a.Source, "owner has no active material branch")
		} else if p.branches[int(a.OwnerBranch)-1].Channel == "main" {
			b.Span = mainSpan(a.Position)
		} else {
			b.Span = resolve(a.OwnerBranch)
		}
		a.Span = b.Span
		b.Owner = a.Source
		state[id] = 2
		return b.Span
	}
	for _, b := range p.branches {
		if b.Channel != "main" {
			resolve(b.ID)
		}
	}
	if len(diagnostics) > 0 {
		sort.SliceStable(diagnostics, func(i, j int) bool { return diagnostics[i].Source.NodeID < diagnostics[j].Source.NodeID })
		return nil, &PlanError{diagnostics: diagnostics}
	}
	for _, b := range p.branches {
		if b.Span > 0 {
			p.spans[b.Span-1].Targets = append(p.spans[b.Span-1].Targets, b.ID)
		}
	}
	return p, nil
}
