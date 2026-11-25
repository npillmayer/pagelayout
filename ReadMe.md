# pagelayout

Load HTML/CSS page templates and content, inspect their flow channels, and
prepare content for layout at different widths, and compile ownership/span plans.

A **page template** describes a page and its receiving **frames**. A frame's
`flow-from` names the channel it receives. In a separate content document,
`flow-into` assigns a block branch to that channel. Content keeps the styles
from its own document; a template's styles do not restyle the content.

## Choose a package

| Package | Use it to |
| --- | --- |
| `pagelayout` | Validate owning links, inspect page spans and source positions with `Plan`. |
| [`template`](template/README.md) | Load and validate collections; select templates and inspect page dimensions, frame rectangles, paint and flow chains. |
| [`content`](content/README.md) | Load styled HTML, inspect source material and take whole branches from persistent channel registers. Prepare material with an explicit backend. |
| [`preparation/typography`](preparation/typography/README.md) | Supply fonts and shaping services, prepare paragraphs once, measure intrinsic widths and obtain complete subtree layouts at chosen widths. |
| [`pgtmpl`](pgtmpl/README.md) | Check, locate and preview template collections from the command line. |

Import `github.com/npillmayer/pagelayout` for source planning and the subpackages
for loading, preparation and layout.

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
   holds whole branch roots by channel; `Document.Residual()` aliases the default
   `main` tree. Valid explicit `flow-into:main` declarations stay in source order.
3. Supply a preparation backend to `content.Prepare`. Keep the returned snapshot
   and session together. The [typography guide](preparation/typography/README.md)
   includes a complete example with an explicitly selected font.
4. Measure or lay out a branch at a frame's content width. Reuse the preparation
   for other widths, and close the session when those operations are finished.

Layout produces a complete subtree at a width. It does not limit the subtree to
a frame's height, select pages, split branches across frames or advance channel
registers automatically. Taking an item from a register consumes a whole branch
in a derived bank; the original bank remains usable.

## Compile ownership and page spans

Mark one owning link per secondary branch with `data-flow-anchor`. Its fragment
href must identify that branch's unique root ID. Ordinary links are references
only. Use `break-before:page` on main block content to close a span:

```html
<p>Chapter one <a href="#note" data-flow-anchor>1</a>.</p>
<section style="break-before:page"><p>Chapter two.</p></section>
<aside id="note" style="flow-into:notes">Note text.</aside>
```

After loading this content:

```go
plan, err := pagelayout.Plan(document)
if err != nil {
    return err
}
for _, span := range plan.Spans() {
    fmt.Println(span.ID, len(span.Targets))
}
// 1 1
// 2 0
```

`Branches`, `Anchors`, `Boundaries` and `Spans` return independent copies.
`Matches(document)` checks the original source snapshot; an identical reload is
a different source. The plan retains its source independently of your document
handle. A `PlanError` exposes diagnostic codes, source locations and related
owners/targets through `Diagnostics()` and matches `ErrInvalidPlan`.

Visible anchors carry logical source coverage for activation on their first
placed fragment. Empty anchors attach to following material in their containing
block, otherwise preceding material, otherwise the block's structural position.
Whitespace-only anchors use the same rule. Span targets include transitive
ownership, while branch queues keep authoring order. A future-span register head
can therefore obstruct an earlier span; planning does not reorder it.

Forced breaks on ordinary inline content, secondary branches or inside inline
atomics are rejected by `Plan`. Initial breaks produce no leading span, coincident
boundaries coalesce, and a terminal boundary closes the last span. Planning
activates no targets, consumes no material and produces no physical pages.
