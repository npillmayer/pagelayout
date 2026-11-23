package content

import (
	"errors"
	"fmt"

	"github.com/npillmayer/css-box-layout/layout"
)

// InlineMeasureRequest qualifies a core measurement request without exposing
// mutable tree nodes. Edges and Atomics return value copies keyed by BoxInfo.ID
// in this group's tree. Border widths already include padding/borders, exclude
// margins and carry no final heights. Both views are borrowed only for this call.
type InlineMeasureRequest struct {
	Group, Owner Box
	Edges        layout.ResolvedEdgesView
	Atomics      layout.IntrinsicAtomicsView
}

// GroupMeasurer executes one already prepared group. It must consume the request
// synchronously without retaining its views or handle, and return content widths
// in CSS pixels. It must not release the handle, reenter the session, perform
// final layout or silently prepare missing material. No default is supplied.
type GroupMeasurer interface {
	MeasureGroup(GroupHandle, InlineMeasureRequest) (layout.IntrinsicWidths, error)
}

// MeasureIntrinsic measures a covered block container's border-box bounds,
// excluding its own margins, using the exact preparation/session pair. ctx is
// the root's containing-width/font context, not a target paragraph width.
// CSS core owns recursive aggregation; the backend measures only requested
// groups from existing handles. No tree pointers escape, no bank is consumed,
// and no geometry or final heights are published. Each invocation has fresh
// scratch; errors return zero widths and leave prior outputs/session reusable.
func MeasureIntrinsic(p *PreparedSnapshot, s *PreparationSession, root Box, ctx layout.ResolveContext, backend GroupMeasurer) (layout.IntrinsicWidths, error) {
	var result layout.IntrinsicWidths
	err := s.withAccess(p, func(st *preparationState) error {
		if !p.Covers(root) || !layout.IsBlockContainer(root.node().Box) || nilInterface(backend) {
			return &PreparationError{Stage: StageAccess, Err: ErrInvalidPreparation}
		}
		m := intrinsicAdapter{state: st, root: root, backend: backend}
		var err error
		result, err = layout.MeasureIntrinsic(root.node(), ctx, m)
		if err != nil {
			var detail *PreparationError
			if errors.As(err, &detail) {
				return err // Keep the failing group's source/stage and core context.
			}
			info, _ := root.Info()
			return &PreparationError{Stage: StageMeasurement, Owner: root, Source: info.Context, Err: err}
		}
		return nil
	})
	if err != nil {
		return layout.IntrinsicWidths{}, err
	}
	return result, nil
}

type intrinsicAdapter struct {
	state   *preparationState
	root    Box
	backend GroupMeasurer
}

func (m intrinsicAdapter) MeasureInline(r layout.InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	if r.Group == nil {
		return layout.IntrinsicWidths{}, ErrInvalidPreparation
	}
	g := Box{m.root.owner, m.root.tree, r.Group.BoxID}
	owner := Box{m.root.owner, m.root.tree, r.OwnerID}
	if g.node() != r.Group {
		return layout.IntrinsicWidths{}, ErrInvalidPreparation
	}
	var result layout.IntrinsicWidths
	err := m.state.useGroup(g, owner, func(h GroupHandle) error {
		var err error
		result, err = m.backend.MeasureGroup(h, InlineMeasureRequest{g, owner, r.Edges, r.Atomics})
		if err != nil {
			return preparationFailure(StageMeasurement, m.state.owned[m.state.index[g]].info, err)
		}
		return nil
	})
	if err != nil {
		return layout.IntrinsicWidths{}, fmt.Errorf("prepared measurement: %w", err)
	}
	return result, nil
}
