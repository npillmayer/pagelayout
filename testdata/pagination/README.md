# Pagination samples

Use these separate HTML template and content documents with the
[root pagination API](../../README.md). The sample output shows how main content,
notes and figures occupy frames over multiple pages. The SVG includes font
outlines and can be viewed without installing a font.

## Real-font document proofs

Open [shipout-proof.svg](shipout-proof.svg) in a browser or SVG viewer. It contains
two five-page documents, each split into two spans. Blue, orange and green outlines
identify main, notes and figures; dashed boxes identify paragraph occurrences.
Labels show source nodes and processed text ranges.

| Input | What to inspect |
| --- | --- |
| [proof.html](templates/proof.html) | Receiving templates for narrow/wide main content and secondary drain pages. |
| [proof-atomics.html](content/proof-atomics.html) | Inline-block interiors retain their accepted sizes while unconsumed material adapts to a new frame width. |
| [proof-manual.html](content/proof-manual.html) | An authored soft hyphen splits `co-` / `operate` across pages, retaining its source association. |

The sample uses explicit template eligibility to select these sequences:

| Content | Templates in span 1 | Templates in span 2 |
| --- | --- | --- |
| Atomics | narrow, wide, notes, figures | wide |
| Manual | manual, wide, notes, figures | wide |

Notes and figures finish before the second main span begins. The first inline-block
interior is 50 CSS pixels wide; an unconsumed interior becomes 80 pixels wide in
the wider receiver. The accepted first interior stays unchanged. The manual example
requires an explicit-marker hyphenator as well as font and shaping services.

The rendered sample uses Gentium Plus from
[GentiumPlus-R.ttf](../../../opentype/testdata/fonts/GentiumPlus-R.ttf), with SHA-256
`fa09788adcc651d77eba7d96f8cc3a600d6484c58e3b4ff79c2b2464dbfea6f8`.
See its [font license](../../../opentype/testdata/fonts/ShaperTestLatin-LICENSE.txt).
Choose your own font through the
[typography setup example](../../preparation/typography/README.md#prepare-once-and-lay-out-at-two-widths);
metrics and line breaks depend on that choice.

[proof-expected.json](proof-expected.json) accompanies the pictured output with
page/template sequences, component losses and source-qualified fragment geometry.
It describes this sample with its selected font and policy; other inputs and
selection options can produce different pages.

## Preview receiving frames

From the module directory:

```sh
go run ./pgtmpl load testdata/pagination/templates/proof.html
go run ./pgtmpl serve testdata/pagination/templates/proof.html
```

Open the printed URL and switch templates to compare receivers. This preview
shows the empty template geometry. Use the root package to paginate content and
your renderer to draw the returned pages.

The smaller [receivers.html](templates/receivers.html) collection is useful for
inspecting multiple widths, ordered frame chains and secondary-only receivers:

```sh
go run ./pgtmpl serve testdata/pagination/templates/receivers.html
```

## Load a sample in an application

Load the collection and content separately. This snippet assumes imports for
`os`, `template` and `content`, and a surrounding function returning an error:

```go
catalog, err := template.LoadFS(os.DirFS("testdata/pagination/templates"), "proof.html")
if err != nil {
    return err
}
f, err := os.Open("testdata/pagination/content/proof-atomics.html")
if err != nil {
    return err
}
defer f.Close()
source, err := content.Load(f, content.LoadOptions{SourceName: "proof-atomics.html"})
if err != nil {
    return err
}
_, _ = catalog, source // pass to the pagination workflow
```

Follow the [pagination example](../../README.md#paginate-and-deliver-a-document)
to prepare the content and deliver selected pages. Set `PaginationConfig.Eligible`
when you want to restrict template choice. Without a filter, all collection
definitions are eligible, so the pictured sequence is not prescribed.

## Explore source authoring

| Content | Demonstrates |
| --- | --- |
| [main-default.html](content/main-default.html), [main-explicit.html](content/main-explicit.html) | Default main content and redundant explicit `flow-into:main`. |
| [anchors.html](content/anchors.html), [boundary-anchors.html](content/boundary-anchors.html) | Visible, empty and trailing owning links, plus ordinary references. |
| [breaks.html](content/breaks.html) | Forced break variants and first-child boundaries. Apply [break-override.css](content/break-override.css) as additional author CSS to remove one boundary. |
| [transitive.html](content/transitive.html) | Main owns a note that owns a figure; related material drains before the next span. |
| [fifo.html](content/fifo.html) | A target waits behind earlier source material even when its owning link occurs first. |
| [cross-span-fifo.html](content/cross-span-fifo.html) | A valid ownership plan whose queue order prevents the current span from draining. |
| [fragments.html](content/fragments.html) | Asymmetric box edges and child paint across fragments. |
| [atomics-manual.html](content/atomics-manual.html) | Percentage-width inline-blocks and authored soft hyphens. |
| [suppression.html](content/suppression.html), [empty.html](content/empty.html) | Inactive annotations, inert content and an empty document. |

Load these with `content.Load` and inspect `pagelayout.Plan` to explore channels,
owners and span membership. They demonstrate source behavior, not a promise that
every source fits every receiver or that every backend accepts its text policy.
Files ending in `_err.html` deliberately contain invalid routing, ownership or
break placement. Use `LoadError` or `PlanError` diagnostics to inspect the failure.
