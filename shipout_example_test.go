package pagelayout_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout"
	"github.com/npillmayer/pagelayout/content"
	prep "github.com/npillmayer/pagelayout/preparation/typography"
	"github.com/npillmayer/pagelayout/template"
	"github.com/npillmayer/typography/glyphing"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
)

func ExamplePagination_Paginate() {
	ctx := context.Background()
	// Select an explicit local font. The application owns its rendering resources.
	services := glyphot.Services{}
	face, err := services.Load(ctx, "../opentype/testdata/fonts/GentiumPlus-R.ttf", nil)
	if err != nil {
		panic(err)
	}
	mapper, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{
		Default: face.Resource.ID(), Size: 15 * dimen.BP,
		Faces: []glyphing.FaceEntry{face},
	}, services)
	if err != nil {
		panic(err)
	}
	backend, err := prep.New(prep.Config{Resolver: mapper, NewShaper: func() (contracts.StreamingShaper, error) { return glyphot.NewStream(32) }})
	if err != nil {
		panic(err)
	}
	doc, err := content.Load(strings.NewReader(`<body style="font-size:20px;hyphens:none"><p>First.</p><p style="break-before:page">Second.</p></body>`), content.LoadOptions{})
	if err != nil {
		panic(err)
	}
	prepared, session, err := content.Prepare(doc, backend)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	plan, err := pagelayout.Plan(doc)
	if err != nil {
		panic(err)
	}
	catalog, err := template.Load(strings.NewReader(`<style>.page{width:220px;height:160px}.frame{position:fixed;left:10px;top:10px;width:200px;height:120px;flow-from:main}</style><body><template id="body" data-type="page"><div class="page"><div class="frame"></div></div></template></body>`), template.LoadOptions{})
	if err != nil {
		panic(err)
	}
	run, err := pagelayout.NewPagination(pagelayout.PaginationConfig{Plan: plan, Catalog: catalog, Prepared: prepared, Session: session, Fitter: prep.FittingAdapter()})
	if err != nil {
		panic(err)
	}
	document, err := run.Paginate(ctx, run.InitialState())
	if err != nil {
		panic(err)
	}
	fmt.Println("complete:", document.Complete(), "spans:", len(document.Spans()))
	if !document.Complete() {
		return
	}
	if err := session.Close(); err != nil {
		panic(err)
	}
	for _, span := range document.Spans() {
		_, err := span.Selected().Shipout(ctx, func(p *pagelayout.Page) error {
			fmt.Println("deliver:", p.Number(), p.Template())
			// Inspect frozen paragraphs for your renderer, without reopening execution.
			for _, frame := range p.Frames() {
				for _, part := range frame.Fragments() {
					if _, err := prep.InspectFragment(part); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
	}
	// Output:
	// complete: true spans: 2
	// deliver: 1 body
	// deliver: 2 body
}
