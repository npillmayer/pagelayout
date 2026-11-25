package content

import (
	"fmt"
	"io"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/w3c-dom/dimen"
	"golang.org/x/net/html"
)

type routedBranch struct {
	source  sourceID
	channel string
	root    *layout.RenderNode
}

type routedChannel struct {
	name     string
	branches []int // indices into branches, in original source order
}

// partitionProjection is private unsized material, before normalization. Branch
// order is global source order; channel order is first occurrence. owners has
// exactly one entry per material source: zero means residual, otherwise the
// owning branch's source ID. Excluded sources remain in sources, not owners.
type partitionProjection struct {
	sources      []projectedSource
	rootFontSize dimen.DU
	residual     *layout.RenderNode
	branches     []routedBranch
	channels     []routedChannel
	owners       map[sourceID]sourceID
}

func loadPartition(r io.Reader, opts LoadOptions) (*partitionProjection, error) {
	p, err := loadProjection(r, opts)
	if err != nil {
		return nil, err
	}
	return partitionSource(p)
}

// partitionSource never mutates its input or reruns the cascade. Validate all
// roots before constructing independent trees; no partial result escapes.
func partitionSource(p *sourceProjection) (*partitionProjection, error) {
	owners := make(map[sourceID]sourceID)
	named := make([]sourceID, len(p.sources))
	inline := make([]sourceID, len(p.sources))
	var roots []sourceID
	var ds []diagnostic
	fail := func(s projectedSource, code, message string, related sourceID) {
		d := diagnostic{stage: "routing", code: code, source: s.ref,
			css: cssLocation{property: "flow-into", value: s.flow}, message: message}
		if related != 0 {
			d.related = p.sources[related].ref
		}
		ds = append(ds, d)
	}
	for _, s := range p.sources[1:] {
		id := s.ref.nodeID
		if s.participation != materialNode && s.participation != contextNode {
			continue
		}
		named[id], inline[id] = named[s.parent], inline[s.parent]
		if s.nodeType == html.ElementNode && s.flow != "" {
			switch {
			case s.name == "html" || (p.root != nil && id == sourceID(p.root.ID)):
				fail(s, "route.root", "flow-into is unsupported on html/body", 0)
			case named[id] != 0:
				ancestor := p.sources[named[id]]
				fail(s, "route.nested", fmt.Sprintf("%s routes to %q inside %s routing to %q", routeLabel(s), s.flow, routeLabel(ancestor), ancestor.flow), named[id])
			case s.display.Kind != layout.DisplayBox || s.display.Outer != layout.OuterBlock ||
				(s.display.Inner != layout.InnerFlow && s.display.Inner != layout.InnerFlowRoot):
				fail(s, "route.display", "named roots must be block flow or block flow-root", 0)
			case inline[id] != 0:
				fail(s, "route.ancestry", "cannot extract a branch from inline or atomic ancestor "+routeLabel(p.sources[inline[id]]), inline[id])
			default:
				roots = append(roots, id)
			}
			named[id] = id
		}
		if s.participation == materialNode {
			owners[id] = named[id]
		}
		if s.nodeType == html.ElementNode && s.display.Outer == layout.OuterInline {
			inline[id] = id
		}
	}
	if len(ds) != 0 {
		return nil, newLoadError(ds)
	}
	// Validation above sees every original directive, including redundant main.
	// Only after it succeeds may main-owned material stay in the residual tree.
	roots = slices.DeleteFunc(roots, func(id sourceID) bool { return p.sources[id].flow == "main" })
	for id, owner := range owners {
		if owner != 0 && p.sources[owner].flow == "main" {
			owners[id] = 0
		}
	}
	result := &partitionProjection{sources: slices.Clone(p.sources), rootFontSize: p.rootFontSize, owners: owners}
	for i := range result.sources {
		result.sources[i].attributes = slices.Clone(p.sources[i].attributes)
		result.sources[i].children = slices.Clone(p.sources[i].children)
	}
	// Copy only edges within a destination. Explicit typed styles preserve source
	// context even when a branch root no longer has its original layout parent.
	seen := make(map[sourceID]bool)
	var copyTree func(*layout.RenderNode, sourceID) *layout.RenderNode
	copyTree = func(n *layout.RenderNode, owner sourceID) *layout.RenderNode {
		if n == nil {
			return nil
		}
		id := sourceID(n.ID)
		actual, ok := owners[id]
		if !ok || actual != owner {
			return nil
		}
		if seen[id] {
			ds = append(ds, diagnostic{stage: "routing", code: "route.ownership", source: p.sources[id].ref, message: "duplicate material source"})
			return nil
		}
		seen[id] = true
		c := *n
		c.ChildrenNodes = nil
		if n.HTML != nil {
			h := *n.HTML
			h.Parent = nil
			h.FirstChild = nil
			h.LastChild = nil
			h.PrevSibling = nil
			h.NextSibling = nil
			h.Attr = slices.Clone(n.HTML.Attr)
			c.HTML = &h
		}
		if n.Style != nil {
			v := *n.Style
			c.Style = &v
		}
		if n.Text != nil {
			v := *n.Text
			c.Text = &v
		}
		for _, child := range n.ChildrenNodes {
			if childCopy := copyTree(child, owner); childCopy != nil {
				c.ChildrenNodes = append(c.ChildrenNodes, childCopy)
			}
		}
		return &c
	}
	result.residual = copyTree(p.root, 0)
	channelIndex := make(map[string]int)
	for _, id := range roots {
		name := p.sources[id].flow
		ci, ok := channelIndex[name]
		if !ok {
			ci = len(result.channels)
			channelIndex[name] = ci
			result.channels = append(result.channels, routedChannel{name: name})
		}
		result.channels[ci].branches = append(result.channels[ci].branches, len(result.branches))
		result.branches = append(result.branches, routedBranch{source: id, channel: name, root: copyTree(p.nodes[id], id)})
	}
	for _, s := range p.sources[1:] {
		if s.participation == materialNode && !seen[s.ref.nodeID] {
			ds = append(ds, diagnostic{stage: "routing", code: "route.ownership", source: s.ref, message: "material source missing from partition"})
		}
	}
	if len(ds) > 0 {
		return nil, newLoadError(ds)
	}
	return result, nil
}

func routeLabel(s projectedSource) string {
	for _, a := range s.attributes {
		if a.Namespace == "" && a.Key == "id" {
			return "#" + a.Val + " (" + s.ref.path + ")"
		}
	}
	return s.ref.path
}
