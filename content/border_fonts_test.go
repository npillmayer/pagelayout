package content

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/npillmayer/w3c-dom/dimen"
	"github.com/npillmayer/w3c-dom/style/css"
	"github.com/npillmayer/w3c-dom/style/textstyle"
	"github.com/npillmayer/w3c-dom/styledtree"
)

// DOM-CONT-02's integration check uses the accepted collection of content
// fixtures, without implementing CONT-02's material/profile projection yet.
func TestRoutingFixtureSourceFontBorder(t *testing.T) {
	for _, tc := range []struct {
		name        string
		stylesheets []Stylesheet
		fontSize    dimen.DU
		width       dimen.DU
		color       color.NRGBA
	}{
		{"base", nil, 20 * dimen.PX, 98304, color.NRGBA{R: 0x20, G: 0x30, B: 0x40, A: 255}},
		{"override", []Stylesheet{{SourceName: "routing-override.css", CSS: fixture(t, "routing-override.css")}}, 24 * dimen.PX, 117965, color.NRGBA{G: 0x88, B: 0x55, A: 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := loadSource(strings.NewReader(fixture(t, "routing.html")), LoadOptions{Stylesheets: tc.stylesheets})
			if err != nil {
				t.Fatal(err)
			}
			before := renderSource(t, s.source.document)
			storySource := sourceByID(t, s.source, "story-b")
			story := s.nodes[storySource.ref.nodeID]
			root := s.nodes[s.source.byNode[s.source.html].ref.nodeID]
			styles, err := textstyle.Resolve(root, story)
			if err != nil {
				t.Fatal(err)
			}
			if styles[0].FontSize != 16*dimen.PX || styles[1].FontSize != tc.fontSize {
				t.Fatalf("wrong source font context: %+v", styles)
			}
			fonts := css.FontContext{
				RootFontSize: styles[0].FontSize,
				FontSize: func(n *styledtree.StyNode) (dimen.DU, error) {
					if n != story {
						return 0, fmt.Errorf("unexpected source font lookup: %s", n)
					}
					return styles[1].FontSize, nil
				},
			}
			border, err := css.GetBorderWithFontContext(story, "left", fonts)
			want := css.Border{Width: tc.width, Style: "solid", Color: tc.color}
			if err != nil || border != want {
				t.Fatalf("source border=%+v, %v; want %+v", border, err, want)
			}
			// No containing width, frame font or local CSS parser was needed.
			// Conversion to CSS pixels belongs to this adapter boundary.
			pixels := float64(border.Width) / float64(dimen.PX)
			if pixels < 2 || pixels > 2.40001 {
				t.Fatalf("wrong DU-to-pixel conversion: %g", pixels)
			}
			if renderSource(t, s.source.document) != before || story.HTMLNode() != storySource.node {
				t.Fatal("border resolution changed source identity or ancestry")
			}
		})
	}
}
