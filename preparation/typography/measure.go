package typography

import (
	"fmt"
	"math"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
)

// MeasureIntrinsic returns a covered block container's border-box bounds in CSS
// pixels, excluding its own margins. It borrows the exact preparation/session
// pair created with this bridge. ctx supplies a definite root percentage basis
// and fallback font size; it does not constrain paragraph line widths.
//
// CSS core aggregates nested bounds; only retained paragraphs are measured.
// Calls are serialized with session/shared-service use. No request, geometry or
// width-dependent answer is retained. Base shaping is reused; manual alternatives
// may be discovered lazily. Failure returns zero widths without poisoning the
// session or prior outputs. Missing/mismatched handles never trigger preparation.
func MeasureIntrinsic(p *content.PreparedSnapshot, s *content.PreparationSession, root content.Box, ctx layout.ResolveContext) (layout.IntrinsicWidths, error) {
	return content.MeasureIntrinsic(p, s, root, ctx, groupMeasurer{})
}

type groupMeasurer struct{}

func (groupMeasurer) MeasureGroup(handle content.GroupHandle, r content.InlineMeasureRequest) (layout.IntrinsicWidths, error) {
	h, ok := handle.(*preparedGroup)
	if !ok || h == nil || h.paragraph == nil || h.input.Group != r.Group || h.input.Owner != r.Owner {
		return layout.IntrinsicWidths{}, content.ErrInvalidPreparation
	}
	fail := func(err error) (layout.IntrinsicWidths, error) {
		return layout.IntrinsicWidths{}, groupError(h.input, content.StageMeasurement, err)
	}
	if nilService(r.Edges) || nilService(r.Atomics) {
		return fail(invalidMeasurement("missing measurement views"))
	}
	atoms, err := atomicIntrinsics(h.input, r)
	if err != nil {
		return fail(err)
	}
	w, err := h.paragraph.MeasureIntrinsic(khipu.MeasureRequest{Atomics: atoms})
	if err != nil {
		return fail(err)
	}
	result, err := intrinsicSummary(w)
	if err != nil {
		return fail(err)
	}
	return result, nil
}

func atomicIntrinsics(g content.GroupInput, r content.InlineMeasureRequest) ([]khipu.AtomicIntrinsic, error) {
	atoms := make([]khipu.AtomicIntrinsic, 0, len(g.Occurrences))
	for _, occurrence := range g.Occurrences {
		info, _ := occurrence.Box.Info()
		atom, ok := r.Atomics.Lookup(info.ID)
		if !ok || atom.BoxID != info.ID {
			return nil, invalidMeasurement("missing or mismatched atomic bounds")
		}
		edges, ok := r.Edges.Lookup(info.ID)
		if !ok {
			return nil, invalidMeasurement("missing atomic margins")
		}
		// Check ordering before quantization: reversed endpoints can round equal.
		if atom.BorderWidths.MinContent > atom.BorderWidths.MaxContent {
			return nil, invalidMeasurement("reversed atomic border bounds")
		}
		var values [4]dimen.DU
		for i, px := range []float32{atom.BorderWidths.MinContent, atom.BorderWidths.MaxContent, edges.Margin.Left, edges.Margin.Right} {
			v, err := cssToDU(px, i >= 2)
			if err != nil {
				return nil, fmt.Errorf("atomic box %d: %w", info.ID, err)
			}
			values[i] = v
		}
		// Convert each primitive once; add in int64 and check the complete signed
		// advances. A wide intermediate prefix may be canceled by a margin.
		lo := int64(values[0]) + int64(values[2]) + int64(values[3])
		hi := int64(values[1]) + int64(values[2]) + int64(values[3])
		if lo <= -int64(dimen.Fil) || hi >= int64(dimen.Fil) || lo > hi {
			return nil, invalidMeasurement("atomic advance outside finite DU domain")
		}
		// Border widths already include padding/borders. Owner edges, vertical
		// margins and final heights are not part of the paragraph contribution.
		atoms = append(atoms, khipu.AtomicIntrinsic{ID: contracts.AtomicID(occurrence.ID), MinAdvance: dimen.DU(lo), MaxAdvance: dimen.DU(hi)})
	}
	return atoms, nil
}

const duPerCSSPixel = float64(dimen.BP) * 3 / 4 // 96 CSS px = 72 big points.

func invalidMeasurement(message string) error {
	return fmt.Errorf("%w: %s", contracts.ErrIllegalArguments, message)
}

func cssToDU(px float32, signed bool) (dimen.DU, error) {
	v := float64(px)
	if math.IsNaN(v) || math.IsInf(v, 0) || !signed && v < 0 {
		return 0, invalidMeasurement("non-finite or negative CSS dimension")
	}
	v = math.Round(v * duPerCSSPixel)
	if math.Abs(v) >= float64(dimen.Fil) {
		return 0, invalidMeasurement("CSS dimension outside finite DU domain")
	}
	return dimen.DU(v), nil
}

func intrinsicSummary(w khipu.IntrinsicWidths) (layout.IntrinsicWidths, error) {
	if w.MinContent < 0 || w.MaxContent < w.MinContent || w.MaxContent >= dimen.Fil {
		return layout.IntrinsicWidths{}, invalidMeasurement("invalid paragraph intrinsic bounds")
	}
	lo, hi := float32(float64(w.MinContent)/duPerCSSPixel), float32(float64(w.MaxContent)/duPerCSSPixel)
	if float64(hi)*duPerCSSPixel >= float64(dimen.Fil) {
		return layout.IntrinsicWidths{}, invalidMeasurement("CSS summary rounds outside finite DU domain")
	}
	return layout.IntrinsicWidths{MinContent: lo, MaxContent: hi}, nil
}
