package template

import (
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
)

// This is the TPL-03 / BOX-01 boundary proof. Compilation/publication tests
// separately cover receiving-size/bounds/overlap checks and flow chains.
func TestValidatedInputsResolveWithoutContent(t *testing.T) {
	for _, tc := range []struct {
		name, css      string
		frame, content layout.Rect
	}{
		{"explicit", `left:auto;right:20px;top:10px;padding:3px;border:2px solid`,
			layout.Rect{X: 190, Y: 10, W: 90, H: 40}, layout.Rect{X: 195, Y: 15, W: 80, H: 30}},
		{"stretch", `left:10px;right:20px;top:30px;bottom:40px;width:auto;height:auto;padding:3px;border:2px solid`,
			layout.Rect{X: 10, Y: 30, W: 270, H: 130}, layout.Rect{X: 15, Y: 35, W: 260, H: 120}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := validatedHTML(t, profileHTML("", "", tc.css))
			page := v.templates[0]
			g, err := layout.ResolveFixedGeometry(page.frames[0].constraints, layout.Rect{W: page.page.width, H: page.page.height}, layout.FixedLayoutOptions{})
			if err != nil || g != (layout.LayoutGeometry{Frame: tc.frame, Content: tc.content}) {
				t.Fatalf("geometry = %+v %v", g, err)
			}
		})
	}
}
