package pagelayout_test

import (
	"fmt"
	"html"
	"os"
	"strings"

	"github.com/npillmayer/css-box-layout/layout"
	prep "github.com/npillmayer/pagelayout/preparation/typography"
)

// Render only retained geometry and outlines. Labels are proof annotations;
// this is deliberately a test utility, not a public document renderer.
func writeShipProof(path string, documents []retainedShipProof) error {
	const column, row = 250, 220
	columns := 0
	for _, d := range documents {
		columns = max(columns, len(d.pages))
	}
	width, height := columns*column+40, len(documents)*row+70
	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d"><title>Pagination proof: changing widths, atomics, manual carry and transitive draining</title><rect width="100%%" height="100%%" fill="#edf2f7"/><g font-family="sans-serif"><text x="20" y="26" font-size="18" fill="#17324d">Pagination / Gentium Plus / frozen output after execution release</text>`, width, height, width, height)
	colors := map[string]string{"main": "#2563eb", "notes": "#c2410c", "figures": "#15803d"}
	for di, d := range documents {
		y := 50 + di*row
		fmt.Fprintf(&out, `<text x="20" y="%d" font-size="13" fill="#334155">%s — source intervals below each page; dashed boxes mark paragraph occurrences</text>`, y, html.EscapeString(d.record.Name))
		for pi, p := range d.pages {
			x := 20 + pi*column
			top := y + 15
			fmt.Fprintf(&out, `<g transform="translate(%d %d)"><rect width="220" height="130" fill="white" stroke="#94a3b8"/><text x="8" y="16" font-size="11" fill="#334155">%d / %s / span %d</text>`, x, top, p.Number(), html.EscapeString(p.Template()), d.record.Pages[pi].Span)
			proofIndex := 0
			for _, frame := range p.Frames() {
				box := frame.Frame.ContentBox
				col := colors[frame.Frame.Flow]
				fmt.Fprintf(&out, `<rect x="%g" y="%g" width="%g" height="%g" fill="none" stroke="%s"/><text x="%g" y="%g" font-size="9" fill="%s">%s · %g px</text>`, box.X, box.Y, box.W, box.H, col, box.X, box.Y+box.H+13, col, html.EscapeString(frame.Frame.Flow), box.W)
				for _, part := range frame.Fragments() {
					geometry := part.Geometry()
					byID := make(map[layout.BoxID]layout.LayoutGeometry, len(geometry))
					for box, g := range geometry {
						info, _ := box.Info()
						byID[info.ID] = g
					}
					var origin func(layout.BoxID) (float32, float32)
					origin = func(id layout.BoxID) (float32, float32) {
						if id == 0 {
							return 0, 0
						}
						g := byID[id]
						x, y := origin(g.RelativeTo)
						return x + g.Content.X, y + g.Content.Y
					}
					for _, c := range part.Containers() {
						color := c.Paint.Background
						if color.A == 0 {
							continue
						}
						x, y := origin(c.Geometry.RelativeTo)
						r := c.Geometry.Frame
						fmt.Fprintf(&out, `<rect x="%g" y="%g" width="%g" height="%g" fill="#%02x%02x%02x" fill-opacity="%g"/>`, box.X+x+r.X, box.Y+y+r.Y, r.W, r.H, color.R, color.G, color.B, float64(color.A)/255)
					}
					// Inline atomic interiors are indivisible and keep their full
					// source paint, independently of fragmented block containers.
					for boxHandle, g := range geometry {
						info, _ := boxHandle.Info()
						color := info.Paint.Background
						if info.Kind != layout.BoxInlineBlock || color.A == 0 {
							continue
						}
						x, y := origin(g.RelativeTo)
						r := g.Frame
						fmt.Fprintf(&out, `<rect x="%g" y="%g" width="%g" height="%g" fill="#%02x%02x%02x" fill-opacity="%g"/>`, box.X+x+r.X, box.Y+y+r.Y, r.W, r.H, color.R, color.G, color.B, float64(color.A)/255)
					}
					bundle, err := prep.InspectFragment(part)
					if err != nil {
						return err
					}
					for _, id := range bundle.Groups() {
						g, _ := bundle.Group(id)
						proof := d.proofs[pi][proofIndex]
						proofIndex++
						fmt.Fprintf(&out, `<g transform="translate(%g %g)"><rect width="%g" height="%g" fill="none" stroke="#94a3b8" stroke-width="0.4" stroke-dasharray="2 2"/>`, box.X+g.X, box.Y+g.Y, g.Info.Width*4/3, g.Info.Height*4/3)
						for _, glyph := range proof.Glyphs {
							fmt.Fprintf(&out, `<path d="%s" fill="%s" transform="translate(%g %g) scale(%g)"/>`, html.EscapeString(glyph.Path), html.EscapeString(glyph.Color), glyph.X*4/3, glyph.Y*4/3, glyph.Scale*4/3)
						}
						out.WriteString(`</g>`)
					}
				}
			}
			for gi, g := range d.record.Pages[pi].Groups {
				label := "paragraph"
				if g.Atomic {
					label = "atomic"
				}
				fmt.Fprintf(&out, `<text x="8" y="%d" font-size="9" fill="#475569">%s / tree %d / source %d : [%d,%d)</text>`, 88+gi*11, label, g.Tree, g.Source.NodeID, g.From, g.To)
			}
			fmt.Fprintf(&out, `<text x="8" y="148" font-size="10" fill="#334155">loss %d · carry %d</text><text x="8" y="163" font-size="10" fill="#64748b">pending %v</text></g>`, d.record.Pages[pi].Score.Total.Info().Units, d.record.Pages[pi].Score.Carry.Info().Units, d.record.Pages[pi].Pending)
		}
	}
	out.WriteString(`</g></svg>`)
	return os.WriteFile(path, []byte(out.String()), 0644)
}
