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

func ExampleFit() {
	ctx := context.Background()
	services := glyphot.Services{}
	face, err := services.Load(ctx, "../../../opentype/testdata/fonts/GentiumPlus-R.ttf", nil)
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
	doc, err := content.Load(strings.NewReader(`<body style="font-size:20px;hyphens:none">A paragraph continues through frames with different widths.</body>`), content.LoadOptions{})
	if err != nil {
		panic(err)
	}
	prepared, session, err := content.Prepare(doc, backend)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	root, _ := doc.Bank().First("main")
	fitter, err := content.NewFitter(prepared, session, root)
	if err != nil {
		panic(err)
	}
	choices, err := Fit(ctx, prepared, session, fitter, FitRequest{FitRequest: content.FitRequest{Width: 100, Height: 32}, Breaker: paragraph.FirstFit})
	if err != nil {
		panic(err)
	}
	if len(choices) == 0 {
		panic("no fitting material")
	}
	selected := choices[0]
	fmt.Println("first fragment:", selected.Progress(), selected.Complete())
	suffix, err := Fit(ctx, prepared, session, fitter, FitRequest{FitRequest: content.FitRequest{Width: 180, Height: 300, Continuation: selected.Continuation()}, Breaker: paragraph.FirstFit})
	if err != nil {
		panic(err)
	}
	fmt.Println("completed at new width:", suffix[0].Complete())
	first := selected.Fragment()
	a, _ := first.Group(first.Groups()[0])
	last := suffix[0].Fragment()
	z, _ := last.Group(last.Groups()[0])
	fmt.Println("same source, distinct occurrences:", a.Input.Group == z.Input.Group, a.ID != z.ID)
	if err := session.Close(); err != nil {
		panic(err)
	}
	fmt.Println("retained lines:", len(a.Paragraph.Lines()) > 0)
	unchanged, _ := doc.Bank().First("main")
	fmt.Println("bank unchanged:", unchanged == root)
	// Output:
	// first fragment: true false
	// completed at new width: true
	// same source, distinct occurrences: true true
	// retained lines: true
	// bank unchanged: true
}
