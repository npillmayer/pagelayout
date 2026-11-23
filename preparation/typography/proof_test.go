package typography

import (
	"fmt"
	"html"
	"os"
	"slices"
	"strings"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/render"
)

// Test-only proof writer: render retained placements, never query layout or
// shaping. This intentionally does not introduce a public document renderer.
func writeRealProof(path string, results []*Result, proofs [][]*render.ParagraphProof) error {
	var width, height float64
	for _, r := range results {
		g := r.Geometry()[r.Root()]
		width += float64(g.Frame.W) + 40
		height = max(height, float64(g.Frame.H))
	}
	width += 20
	height += 115
	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%g" height="%g" viewBox="0 0 %g %g"><title>PREP-05: one preparation, two real-font layouts</title><rect width="100%%" height="100%%" fill="#fff"/>`, width, height, width, height)
	x := 20.0
	for n, r := range results {
		geometry := r.Geometry()
		ids := make(map[layout.BoxID]content.Box, len(geometry))
		for b := range geometry {
			info, _ := b.Info()
			ids[info.ID] = b
		}
		var origin func(content.Box) (float64, float64)
		origin = func(b content.Box) (float64, float64) {
			g := geometry[b]
			var px, py float64
			if g.RelativeTo != 0 {
				px, py = origin(ids[g.RelativeTo])
			}
			return px + float64(g.Content.X), py + float64(g.Content.Y)
		}
		fmt.Fprintf(&out, `<text x="%g" y="25" font-family="sans-serif" font-size="13" fill="#334155">%g CSS px</text><text x="%g" y="43" font-family="sans-serif" font-size="11" fill="#64748b">Gentium Plus</text><g transform="translate(%g 65)">`, x, geometry[r.Root()].Content.W, x, x)
		keys := make([]layout.BoxID, 0, len(ids))
		for id := range ids {
			keys = append(keys, id)
		}
		slices.Sort(keys)
		for _, id := range keys {
			g := geometry[ids[id]]
			var px, py float64
			if g.RelativeTo != 0 {
				px, py = origin(ids[g.RelativeTo])
			}
			fmt.Fprintf(&out, `<rect x="%g" y="%g" width="%g" height="%g" fill="none" stroke="#93c5fd" stroke-width="0.5"/>`, px+float64(g.Frame.X), py+float64(g.Frame.Y), g.Frame.W, g.Frame.H)
		}
		for j, g := range r.Groups() {
			ox, oy := origin(g.Owner)
			for _, glyph := range proofs[n][j].Glyphs {
				fmt.Fprintf(&out, `<path d="%s" fill="%s" transform="translate(%g %g) scale(%g)"/>`, html.EscapeString(glyph.Path), html.EscapeString(glyph.Color), ox+glyph.X*4/3, oy+glyph.Y*4/3, glyph.Scale*4/3)
			}
		}
		out.WriteString(`</g>`)
		x += float64(geometry[r.Root()].Frame.W) + 40
	}
	out.WriteString(`</svg>`)
	return os.WriteFile(path, []byte(out.String()), 0644)
}
