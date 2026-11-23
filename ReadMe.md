# pagelayout

Load HTML/CSS page templates and content, inspect their flow channels, and
prepare content for layout at different widths.

A **page template** describes a page and its receiving **frames**. A frame's
`flow-from` names the channel it receives. In a separate content document,
`flow-into` assigns a block branch to that channel. Content keeps the styles
from its own document; a template's styles do not restyle the content.

## Choose a package

| Package | Use it to |
| --- | --- |
| [`template`](template/README.md) | Load and validate collections; select templates and inspect page dimensions, frame rectangles, paint and flow chains. |
| [`content`](content/README.md) | Load styled HTML, inspect source material and take whole branches from persistent channel registers. Prepare material with an explicit backend. |
| [`preparation/typography`](preparation/typography/README.md) | Supply fonts and shaping services, prepare paragraphs once, measure intrinsic widths and obtain complete subtree layouts at chosen widths. |
| [`pgtmpl`](pgtmpl/README.md) | Check, locate and preview template collections from the command line. |

Use these subpackage imports in Go applications. The root `pagelayout` package
has no callable API.

## Preview a collection

From the module directory:

```sh
go run ./pgtmpl load examples/03-magazine.html
go run ./pgtmpl serve examples/03-magazine.html
```

Open the printed URL. Select a template and use **Show frame borders** to
display its flow channels. The preview shows frame geometry and paint, without
flowing content. Stop the server with Ctrl+C.

To apply your own stylesheet without editing the collection:

```sh
go run ./pgtmpl serve examples/03-magazine.html --css examples/magazine-override.css
```

See the [example collections](examples/README.md) for minimal, letterhead and
magazine inputs, CSS overrides and intentionally invalid collections.

## Use templates and content together

1. Load a collection with `template.Load`, `LoadFS` or `LoadFile`. Select a
   template by ID and use `FramesForFlow` to get each channel's frames in order.
   Frame content rectangles are in page-local CSS pixels.
2. Load the separate content document with `content.Load`. `Document.Bank()`
   holds whole branch roots by channel; `Document.Residual()` identifies content
   outside named channels.
3. Supply a preparation backend to `content.Prepare`. Keep the returned snapshot
   and session together. The [typography guide](preparation/typography/README.md)
   includes a complete example with an explicitly selected font.
4. Measure or lay out a branch at a frame's content width. Reuse the preparation
   for other widths, and close the session when those operations are finished.

Layout produces a complete subtree at a width. It does not limit the subtree to
a frame's height, select pages, split branches across frames or advance channel
registers automatically. Taking an item from a register consumes a whole branch
in a derived bank; the original bank remains usable.
