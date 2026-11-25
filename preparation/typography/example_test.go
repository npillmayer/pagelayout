package typography

import (
	"context"
	"fmt"
	"strings"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

func ExampleLayout() {
	// The application explicitly selects a real font and backend. No discovery,
	// host font installation or default backend is involved.
	services := glyphot.Services{}
	face, err := services.Load(context.Background(), "../../../opentype/testdata/fonts/GentiumPlus-R.ttf", nil)
	if err != nil {
		panic(err)
	}
	mapper, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{Default: face.Resource.ID(), Size: 15 * dimen.BP, Faces: []glyphing.FaceEntry{face}}, services)
	if err != nil {
		panic(err)
	}
	backend, err := New(Config{Resolver: mapper, NewShaper: func() (contracts.StreamingShaper, error) { return glyphot.NewStream(32) }})
	if err != nil {
		panic(err)
	}
	doc, err := content.Load(strings.NewReader(`<body style="font-size:20px;hyphens:none"><section style="flow-into:main">Before <span style="display:inline-block;font-size:12px">Wide <span style="display:inline-block">Inner.</span></span> after.</section></body>`), content.LoadOptions{})
	if err != nil {
		panic(err)
	}
	prepared, session, err := content.Prepare(doc, backend)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	bank := doc.Bank()
	root, ok := bank.First("main")
	if !ok {
		panic("missing main branch")
	}
	first, err := Layout(prepared, session, root, LayoutRequest{Width: 120, Breaker: paragraph.FirstFit})
	if err != nil {
		panic(err)
	}
	second, err := Layout(prepared, session, root, LayoutRequest{Width: 240, Breaker: paragraph.FirstFit})
	if err != nil {
		panic(err)
	}
	fmt.Println("prepared groups:", len(prepared.Groups()))
	fmt.Println("candidate widths:", first.Geometry()[root].Content.W, second.Geometry()[root].Content.W)
	unchanged, _ := bank.First("main")
	fmt.Println("bank unchanged:", unchanged == root)
	if err := session.Close(); err != nil {
		panic(err)
	}
	g, ok := first.Group(first.Groups()[0].Group)
	fmt.Println("retained after Close:", ok && len(g.Paragraph.VList.Lines()) > 0)
	// Output:
	// prepared groups: 3
	// candidate widths: 120 240
	// bank unchanged: true
	// retained after Close: true
}
