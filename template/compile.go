package template

import (
	"errors"
	"fmt"
	"math"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/w3c-dom/dimen"
)

// compileTemplates accepts only validated, detached TPL-03 inputs. The catalog
// retains no constraints, DOM, CSSOM or intermediate input storage.
func compileTemplates(input *validatedCollection) (*Catalog, error) {
	out := &Catalog{byID: make(map[string]*compiledTemplate)}
	var ds []diagnostic
	for _, definition := range input.templates {
		page := definition.page
		viewport := Rect{W: page.width, H: page.height}
		if !positiveRect(viewport) {
			ds = append(ds, geometryDiagnostic(page.source, codeGeometryInvalid, "page dimensions must be finite and positive", nil))
			continue
		}
		tolerance := geometryTolerance(viewport)
		t := &compiledTemplate{id: definition.id,
			page:  PageInfo{Width: page.width, Height: page.height, Paint: publicPaint(page.paint), Source: publicSource(page.source)},
			flows: make(map[string][]int)}
		for _, frame := range definition.frames {
			g, err := layout.ResolveFixedGeometry(frame.constraints, viewport, layout.FixedLayoutOptions{})
			if err != nil {
				code := codeGeometryResolve
				if errors.Is(err, layout.ErrFixedContentDependency) {
					code = codeGeometryContent
				}
				if errors.Is(err, layout.ErrUnsupportedPosition) {
					code = codeGeometryPosition
				}
				ds = append(ds, geometryDiagnostic(frame.source, code, "cannot resolve fixed frame", err))
				continue
			}
			if !positiveRect(g.Frame) || !positiveRect(g.Content) {
				ds = append(ds, geometryDiagnostic(frame.source, codeGeometryReceiving, "frame border and receiving content rectangles must be finite and positive", nil))
				continue
			}
			if !containsRect(viewport, g.Frame, tolerance) {
				ds = append(ds, geometryDiagnostic(frame.source, codeGeometryBounds, "frame border box extends outside the page", nil))
			}
			for _, earlier := range t.frames {
				if overlaps(earlier.BorderBox, g.Frame, tolerance) {
					ds = append(ds, geometryDiagnostic(frame.source, codeGeometryOverlap,
						fmt.Sprintf("frame %d overlaps frame %d at %s", frame.id, earlier.ID, earlier.Source.Path), nil))
				}
			}
			position := len(t.frames)
			t.frames = append(t.frames, FrameInfo{
				ID: FrameID(frame.id), Flow: frame.flow, BorderBox: g.Frame, ContentBox: g.Content,
				Margin: absoluteEdges(frame.constraints.Margin), Padding: absoluteEdges(frame.constraints.Padding), Border: absoluteEdges(frame.constraints.Border),
				Paint: publicPaint(frame.paint), Source: publicSource(frame.source)})
			t.flows[frame.flow] = append(t.flows[frame.flow], position)
		}
		out.ids = append(out.ids, t.id)
		out.byID[t.id] = t
	}
	if len(ds) > 0 {
		return nil, orderedStyleError(ds)
	}
	return out, nil
}

func geometryDiagnostic(source sourceRef, code diagnosticCode, message string, cause error) diagnostic {
	return diagnostic{stage: stageGeometry, code: code, source: source, message: message, cause: cause}
}

// TPL-03 restricts effective edges to absolute CSS pixels (margins are zero).
func absoluteEdges(e layout.EdgeLengths) Edges {
	return Edges{Top: e.Top.Value, Right: e.Right.Value, Bottom: e.Bottom.Value, Left: e.Left.Value}
}

func positiveRect(r Rect) bool {
	for _, v := range []float32{r.X, r.Y, r.W, r.H} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return r.W > 0 && r.H > 0
}

// Comparisons allow one source DU or eight float32 spacings below the larger
// page dimension, whichever is greater. Sum endpoints in float64 to avoid adding
// another rounding step. Tolerance never changes published geometry or relaxes
// strictly positive sizes. Cap it at half the smaller span for each axis so an
// entire tiny receiver cannot be treated as a rounding-sized overlap/overflow.
func geometryTolerance(page Rect) float64 {
	scale := max(page.W, page.H)
	spacing := float64(scale) - float64(math.Nextafter32(scale, 0))
	return max(1/float64(dimen.PX), 8*spacing)
}

func axisTolerance(tolerance float64, a, b float32) float64 {
	return min(tolerance, float64(min(a, b))/2)
}

func containsRect(outer, inner Rect, tolerance float64) bool {
	x, y := axisTolerance(tolerance, outer.W, inner.W), axisTolerance(tolerance, outer.H, inner.H)
	return float64(inner.X) >= float64(outer.X)-x && float64(inner.Y) >= float64(outer.Y)-y &&
		float64(inner.X)+float64(inner.W) <= float64(outer.X)+float64(outer.W)+x &&
		float64(inner.Y)+float64(inner.H) <= float64(outer.Y)+float64(outer.H)+y
}

func overlaps(a, b Rect, tolerance float64) bool {
	w := min(float64(a.X)+float64(a.W), float64(b.X)+float64(b.W)) - max(float64(a.X), float64(b.X))
	h := min(float64(a.Y)+float64(a.H), float64(b.Y)+float64(b.H)) - max(float64(a.Y), float64(b.Y))
	return w > axisTolerance(tolerance, a.W, b.W) && h > axisTolerance(tolerance, a.H, b.H)
}
