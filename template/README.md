# template

`template` loads HTML/CSS page-template collections into an immutable `Catalog`.
Use the catalog to select a template and inspect its resolved page size, frame
rectangles, paint and named flow chains.

```go
import "github.com/npillmayer/pagelayout/template"
```

## Load and inspect a collection

This complete program loads a page with one receiving frame:

```go
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/npillmayer/pagelayout/template"
)

func main() {
	const source = `<body>
      <template id="note" data-type="page">
        <div style="width:300px;height:200px">
          <div style="position:fixed;left:20px;top:10px;
                      width:260px;height:180px;flow-from:main"></div>
        </div>
      </template>
    </body>`
	catalog, err := template.Load(strings.NewReader(source), template.LoadOptions{
		SourceName: "note.html",
	})
	if err != nil {
		log.Fatal(err)
	}
	page, ok := catalog.Lookup("note")
	if !ok {
		log.Fatal("template note not found")
	}
	size := page.Page()
	fmt.Printf("%s: %g x %g CSS px\n", page.ID(), size.Width, size.Height)
	for _, frame := range page.FramesForFlow("main") {
		r := frame.ContentBox
		fmt.Printf("main: x=%g y=%g width=%g height=%g\n", r.X, r.Y, r.W, r.H)
	}
}
```

Output:

```text
note: 300 x 200 CSS px
main: x=20 y=10 width=260 height=180
```

`Catalog.IDs()` lists template IDs in document order. `Lookup` and
`FramesForFlow` use exact, case-sensitive names. `Frames()` returns all frames;
`FramesForFlow(name)` returns that channel's frames in document order. A
`FrameInfo.ID` is a one-based number within the template, not an HTML element ID.

Page dimensions and frame rectangles use CSS pixels. `BorderBox` includes
padding and borders; `ContentBox` is the receiving area inside them. Both
rectangles are already page-local: do not add the border-box origin to the
content-box origin. `Paint` describes solid backgrounds and borders, while
`Source` identifies the corresponding HTML node.

Accessors return independent descriptions and slices. You can retain a catalog
after closing its input and inspect it concurrently. A failed load returns no
partial catalog, even when some definitions are valid.

## Choose an input source

| Function | Input and file ownership |
| --- | --- |
| `Load(reader, options)` | Reads a caller-owned `io.Reader`. The caller closes it if needed. |
| `LoadFS(fsys, name)` | Loads an exact slash-separated path in an `fs.FS`, such as `embed.FS` or `os.DirFS`. Opens and closes the file. |
| `LoadFile(name, hostIO)` | Finds a collection on the host filesystem. Pass `nil` for normal OS access. Opens and closes the file. |

For an explicit local path, import `os` and use:

```go
catalog, err := template.LoadFS(os.DirFS("."), "examples/03-magazine.html")
if err != nil {
	return err
}
```

`LoadFile("pages.html", nil)` tries the user configuration directory's
`templates` subdirectory, then the working directory. Absolute paths are used
directly. Relative paths that escape through `..` use only the working
directory. Only a missing file permits fallback; an unreadable or invalid
collection stops the search. Include the extension: names are file paths, not
template IDs, and lookup is not recursive.

To supply your own filesystem access, pass an `IO` value with methods
`UserConfigDir() (string, error)` and `DirFS(string) fs.FS`.

## Add a stylesheet

Pass CSS text in `LoadOptions.Stylesheets`, or use `LoadFSWithOptions` and
`LoadFileWithOptions` for file input:

```go
catalog, err := template.LoadFSWithOptions(os.DirFS("."), "examples/03-magazine.html",
	template.LoadOptions{
		Stylesheets: []template.Stylesheet{
			{SourceName: "reader.css", CSS: `.page { background: #ffffff; }`},
		},
	})
if err != nil {
	return err
}
```

Additional sheets follow the collection's head stylesheets in the supplied
order. Normal specificity, inline-style and `!important` precedence still apply.
The loader checks losing and nonmatching declarations too. CSS must satisfy
the same profile as the collection; an invalid override rejects the load.
Geometry reflects the combined cascade, including changes to padding or borders.

`SourceName` labels reader input or supplied CSS in diagnostics. File loaders
use the selected file path for HTML diagnostics. CSS text is supplied by the
caller; the loader does not fetch stylesheet URLs or `@import` resources.

## Author a collection

Each definition is a direct body child:

```html
<template id="unique-name" data-type="page">
  <div class="page">
    <div class="frame"></div>
  </div>
</template>
```

Supply geometry through head styles or inline styles. Select the page and
frames directly, for example `.page > .frame`; the storage `<template>` wrapper
is not part of their selector ancestry. HTML/body attributes remain available
to selectors. Keep flowing content in a separate document.

| Element | Requirements |
| --- | --- |
| Storage template | Unique, nonempty, whitespace-free ID; exactly one page DIV. A collection needs at least one definition. |
| Page DIV | Positive absolute width and height; static block layout; zero margins, padding and effective borders. A page may have no frames. |
| Frame DIV | Direct, empty child of the page; `position:fixed`, block layout and content-box sizing; a `flow-from` name; zero margins. |
| Frame geometry | Enough absolute dimensions/insets to resolve both axes. Auto size requires opposing insets. Content width and height must be positive. |
| Edges and paint | Nonnegative padding and border widths; solid/none/hidden borders and solid background colors. |

Lengths accept `px`, `pt`, `in`, `cm`, `mm` and unitless zero. Percentages and
font-relative lengths are not supported for template geometry. Frame border
boxes must stay within the page and must not overlap; touching edges are allowed.
Nested frames, floats, transforms and external image resources are unsupported.

The [example collections](../examples/README.md) show complete inputs, including
auto-sized frames, multiple flow channels and stylesheet overrides. Use
[`pgtmpl serve`](../pgtmpl/README.md) to preview their receiving areas.

## Report errors

Use `errors.As` to inspect a `*template.LoadError`:

```go
var loadErr *template.LoadError
if errors.As(err, &loadErr) {
	for _, diagnostic := range loadErr.Diagnostics() {
		fmt.Println(diagnostic.Error())
	}
}
```

Import `errors` and `fmt` for this error-handling snippet. Each diagnostic
provides `Stage`, `Code`, `Message`, `Source` and `CSS`. `Source.Path` is a path
in the parsed HTML tree; CSS line/column values refer to CSS text, not HTML file
lines. Additional stylesheet diagnostics use their sheet label without an HTML
node identity. `errors.Is` and `errors.As` also reach the original I/O or parsing
cause through the load error.
