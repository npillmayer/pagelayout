package template_test

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/npillmayer/pagelayout/template"
)

// The examples run without depending on the working directory or user files.
//
//go:embed testdata/templates/sketch.html testdata/templates/invalid-overlap.html
var exampleCollections embed.FS

func ExampleLoad() {
	const collection = `<body>
  <template id="note" data-type="page">
    <div style="width:300px;height:200px">
      <div style="position:fixed;left:20px;top:10px;width:260px;height:180px;flow-from:main"></div>
    </div>
  </template>
</body>`
	catalog, err := template.Load(strings.NewReader(collection), template.LoadOptions{SourceName: "note.html"})
	if err != nil {
		fmt.Println(err)
		return
	}
	page, ok := catalog.Lookup("note")
	if !ok {
		fmt.Println("missing note template")
		return
	}
	geometry := page.Page()
	frame := page.FramesForFlow("main")[0]
	fmt.Printf("%s: %.0f x %.0f CSS px\n", page.ID(), geometry.Width, geometry.Height)
	fmt.Printf("main: x=%.0f y=%.0f w=%.0f h=%.0f\n", frame.ContentBox.X, frame.ContentBox.Y, frame.ContentBox.W, frame.ContentBox.H)
	// Output:
	// note: 300 x 200 CSS px
	// main: x=20 y=10 w=260 h=180
}

func ExampleLoadFS() {
	catalog, err := template.LoadFS(exampleCollections, "testdata/templates/sketch.html")
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, id := range catalog.IDs() {
		page, _ := catalog.Lookup(id)
		geometry := page.Page()
		fmt.Printf("%s: %.2f x %.2f CSS px\n", id, geometry.Width, geometry.Height)
		// Filtering preserves the template's frame order, which defines the chain.
		for _, frame := range page.FramesForFlow("article-flow") {
			r := frame.ContentBox
			fmt.Printf("  article frame %d: x=%.2f y=%.2f w=%.2f h=%.2f\n", frame.ID, r.X, r.Y, r.W, r.H)
		}
	}
	// Output:
	// two-column-article: 793.70 x 1122.52 CSS px
	//   article frame 2: x=75.59 y=173.86 w=302.36 h=873.07
	//   article frame 3: x=415.75 y=173.86 w=302.36 h=873.07
	// chapter-opening: 793.70 x 1122.52 CSS px
	//   article frame 2: x=75.59 y=264.57 w=642.52 h=782.36
}

func ExampleLoadFSWithOptions() {
	opts := template.LoadOptions{Stylesheets: []template.Stylesheet{
		{SourceName: "custom.css", CSS: `.page {background:#eeeeee} .page > .heading {height:20mm}`},
	}}
	catalog, err := template.LoadFSWithOptions(exampleCollections, "testdata/templates/sketch.html", opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	page, _ := catalog.Lookup("two-column-article")
	fmt.Printf("background: %v\n", page.Page().Paint.Background)
	// The original .page.two-column > .heading has greater specificity.
	fmt.Printf("heading height: %.2f CSS px\n", page.FramesForFlow("heading-flow")[0].ContentBox.H)
	// Output:
	// background: {238 238 238 255}
	// heading height: 68.03 CSS px
}

// exampleIO substitutes an embedded repository for host access. Applications
// using the normal user-config/current-directory search can pass nil instead.
type exampleIO struct{ repository fs.FS }

func (exampleIO) UserConfigDir() (string, error) { return "config", nil }
func (h exampleIO) DirFS(dir string) fs.FS {
	if dir == filepath.Join("config", "templates") {
		return h.repository
	}
	return embed.FS{} // All other directories are empty in this example.
}

func ExampleLoadFile() {
	repository, err := fs.Sub(exampleCollections, "testdata/templates")
	if err != nil {
		fmt.Println(err)
		return
	}
	catalog, err := template.LoadFile("sketch.html", exampleIO{repository: repository})
	if err != nil {
		fmt.Println(err)
		return
	}
	page, _ := catalog.Lookup("chapter-opening")
	fmt.Printf("found: %s\n", page.ID())
	fmt.Printf("heading frames: %d\n", len(page.FramesForFlow("heading-flow")))
	fmt.Printf("article frames: %d\n", len(page.FramesForFlow("article-flow")))
	// Output:
	// found: chapter-opening
	// heading frames: 1
	// article frames: 1
}

func ExampleLoadError() {
	catalog, err := template.LoadFS(exampleCollections, "testdata/templates/invalid-overlap.html")
	var loadErr *template.LoadError
	if !errors.As(err, &loadErr) {
		fmt.Printf("expected a load error, got %v\n", err)
		return
	}
	fmt.Printf("catalog published: %t\n", catalog != nil)
	for _, d := range loadErr.Diagnostics() {
		fmt.Printf("%s: template=%s stage=%s code=%s\n", d.Source.Name, d.Source.TemplateID, d.Stage, d.Code)
	}
	// Output:
	// catalog published: false
	// testdata/templates/invalid-overlap.html: template=overlap stage=geometry code=geometry.overlap
}
