package template

import (
	"image/color"
	"testing"

	"github.com/npillmayer/w3c-dom/style/css"
	"golang.org/x/net/html"
)

func TestSketchCollectionShorthands(t *testing.T) {
	// This fixture is the collection example from doc/SKETCH.md, including its
	// original shorthand declarations. It must pass strict CSS inspection.
	s := styleCollectionOK(t, readCollectionFixture(t, "sketch.html"))
	if len(s.templates) != 2 {
		t.Fatal("lost sketch definitions")
	}
	for i, definition := range s.templates {
		if len(definition.active.frames) != []int{3, 2}[i] {
			t.Fatal("lost sketch frames")
		}
		page := definition.active.page
		wantProperty(t, definition, page, "width", "210mm")
		wantProperty(t, definition, page, "height", "297mm")
		boxes := []*html.Node{page}
		for j, frame := range definition.active.frames {
			boxes = append(boxes, frame.element)
			wantProperty(t, definition, frame.element, "position", "fixed")
			flow := "article-flow"
			if j == 0 {
				flow = "heading-flow"
			}
			wantProperty(t, definition, frame.element, "flow-from", flow)
		}
		for _, box := range boxes {
			n := definition.nodes[box]
			wantColor := color.NRGBA{}
			if box == page {
				wantColor = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
			}
			if got, err := css.GetColor(n, "background-color"); err != nil || got != wantColor {
				t.Errorf("background = %v %v, want %v", got, err, wantColor)
			}
			wantProperty(t, definition, box, "background-image", "none")
			for _, side := range []string{"top", "right", "bottom", "left"} {
				if b, err := css.GetBorder(n, side); err != nil || b.Width != 0 || b.Style != "none" {
					t.Errorf("%s border = %+v %v", side, b, err)
				}
				wantProperty(t, definition, box, "border-"+side+"-width", "0")
			}
		}
	}
}
