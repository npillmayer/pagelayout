package content

import (
	"errors"
	"maps"
	"math"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
)

// InlineLayoutRequest borrows invocation-local values, never mutable tree nodes.
// Width and core geometry are CSS pixels. Atomics are border dimensions realized
// for this layout invocation, separate from retained paragraph preparation.
type InlineLayoutRequest struct {
	Group, Owner Box
	Width        float32
	Used         layout.UsedValuesView
	Atomics      layout.RealizedAtomicsView
}

// GroupLayouter executes retained preparation. It must not retain request views,
// release handles or reenter the session. Backend output stays invocation-local
// until Layout succeeds. Core receives only geometry, never paragraph payloads.
// Callers provide a fresh collector/backend for each invocation.
type GroupLayouter interface {
	GroupMeasurer
	LayoutGroup(GroupHandle, InlineLayoutRequest) (layout.InlineResult, error)
}

// LayoutResult owns completed CSS geometry and source identities, not a session.
// Accessors return copies; Box identities remain qualified by their source tree.
type LayoutResult struct {
	root     Box
	geometry map[Box]layout.LayoutGeometry
	groups   []GroupInfo
}

// Root returns the source root whose complete geometry this result describes.
func (r *LayoutResult) Root() Box {
	if r == nil {
		return Box{}
	}
	return r.root
}

// Geometry returns copied CSS-pixel geometry. RelativeTo identifies a content
// coordinate parent by BoxID within Root's tree; zero denotes the root origin.
func (r *LayoutResult) Geometry() map[Box]layout.LayoutGeometry {
	if r == nil {
		return nil
	}
	return maps.Clone(r.geometry)
}

// Groups returns covered source group descriptions in structural preorder.
func (r *LayoutResult) Groups() []GroupInfo {
	if r == nil {
		return nil
	}
	out := slices.Clone(r.groups)
	for i := range out {
		out[i].Atomics = slices.Clone(out[i].Atomics)
	}
	return out
}

// Layout runs an independent whole-subtree invocation at a containing width in
// CSS pixels. It reserves the exact preparation/session pair and publishes only
// after all callbacks and core geometry succeed. Each group is laid out once per
// invocation; later invocations reuse preparation with fresh tracking/tables.
// Errors publish nil. Source material and registers are never modified.
func Layout(p *PreparedSnapshot, s *PreparationSession, root Box, width float32, backend GroupLayouter) (*LayoutResult, error) {
	var out *LayoutResult
	err := s.withAccess(p, func(st *preparationState) error {
		if !p.Covers(root) || !layout.IsBlockContainer(root.node().Box) || nilInterface(backend) {
			return &PreparationError{Stage: StageAccess, Err: ErrInvalidPreparation}
		}
		fail := func(err error) error {
			var e *PreparationError
			if errors.As(err, &e) {
				return err
			}
			info, _ := root.Info()
			return &PreparationError{Stage: StageLayout, Owner: root, Source: info.Context, Err: err}
		}
		if width < 0 || math.IsNaN(float64(width)) || math.IsInf(float64(width), 0) {
			return fail(ErrInvalidPreparation)
		}
		groups := slices.Collect(layout.InlineGroups(root.node()))
		i := &layoutAdapter{intrinsicAdapter: intrinsicAdapter{state: st, root: root, backend: backend}, backend: backend, expected: make(map[Box]Box), started: make(map[Box]bool)}
		for _, g := range groups {
			group := Box{root.owner, root.tree, g.Group.BoxID}
			owner := Box{root.owner, root.tree, g.Owner.BoxID}
			index, ok := st.index[group]
			if !ok || st.owned[index].info.Owner != owner {
				return fail(ErrInvalidPreparation)
			}
			i.expected[group] = owner
		}
		ctx := layout.ResolveContext{ContainingBlock: layout.Rect{W: width}}
		used, err := layout.ResolveUsedValues(root.node(), ctx)
		if err != nil {
			return fail(err)
		}
		flow, err := layout.FlowLayout(root.node(), used, i, i, layout.LayoutContext{ContainingBlock: ctx.ContainingBlock}, layout.LayoutOptions{})
		if err != nil {
			return fail(err)
		}
		if len(i.started) != len(i.expected) {
			return fail(ErrPreparationProtocol)
		}
		out = &LayoutResult{root: root, geometry: make(map[Box]layout.LayoutGeometry, len(flow.Geometry))}
		for id, g := range flow.Geometry {
			out.geometry[Box{root.owner, root.tree, id}] = g
		}
		for _, g := range groups {
			info := st.owned[st.index[Box{root.owner, root.tree, g.Group.BoxID}]].info
			info.Atomics = slices.Clone(info.Atomics)
			out.groups = append(out.groups, info)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

type layoutAdapter struct {
	intrinsicAdapter
	backend  GroupLayouter
	expected map[Box]Box
	started  map[Box]bool
}

var _ layout.InlineLayouter = (*layoutAdapter)(nil)
var _ layout.IntrinsicMeasurer = (*layoutAdapter)(nil)

func (i *layoutAdapter) LayoutInline(r layout.InlineRequest) (layout.InlineResult, error) {
	if r.Group == nil {
		return layout.InlineResult{}, ErrInvalidPreparation
	}
	g := Box{i.root.owner, i.root.tree, r.Group.BoxID}
	owner := Box{i.root.owner, i.root.tree, r.OwnerID}
	if g.node() != r.Group || i.expected[g] != owner || i.started[g] {
		return layout.InlineResult{}, ErrPreparationProtocol
	}
	i.started[g] = true
	var out layout.InlineResult
	err := i.state.useGroup(g, owner, func(h GroupHandle) error {
		var err error
		out, err = i.backend.LayoutGroup(h, InlineLayoutRequest{g, owner, r.ContentWidth, r.Used, r.Atomics})
		if err != nil {
			return preparationFailure(StageLayout, i.state.owned[i.state.index[g]].info, err)
		}
		return nil
	})
	if err != nil {
		return layout.InlineResult{}, err
	}
	return out, nil
}
