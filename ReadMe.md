# pagelayout

Use `pagelayout` to fill page-template frames with styled HTML content, select
page sequences, and deliver the completed pages to your renderer or exporter.
Templates and content are separate HTML/CSS documents. Content keeps its source
styles; template frames supply receiving geometry and their own paint.

```go
import "github.com/npillmayer/pagelayout"
```

## Choose a package

| Package | Use it to |
| --- | --- |
| `pagelayout` | Validate owning links and page spans, paginate prepared content, and deliver selected pages. |
| [`template`](template/README.md) | Load collections and inspect compiled page sizes, frames, paint and flow chains. |
| [`content`](content/README.md) | Load styled HTML, inspect boxes and channels, and prepare or fit material with a supplied backend. |
| [`preparation/typography`](preparation/typography/README.md) | Supply fonts and shaping services; measure, lay out and fit prepared paragraphs. |
| [`pgtmpl`](pgtmpl/README.md) | Check, find and preview template collections from the command line. |

## Prepare the inputs

A template frame receives a channel through `flow-from`. Use `flow-from: main`
for ordinary body content. In the content document, `flow-into` routes non-nested
block branches to secondary channels. Each secondary branch needs one owning
link marked with `data-flow-anchor`:

```html
<body style="hyphens:none">
  <p>Chapter one <a href="#note" data-flow-anchor>1</a>.</p>
  <section style="break-before:page"><p>Chapter two.</p></section>
  <aside id="note" style="flow-into:notes">Note text.</aside>
</body>
```

The note becomes eligible when its owning link is placed. Ordinary links do not
activate material. A note can be deferred to another page, but it must finish
before the next chapter's span begins. Queues retain source order, so an earlier
unconsumed item can block a later target.

`break-before: page`, `left`, `right`, `recto` and `verso` all force a page boundary
without parity rules or blank-page insertion. Use them on main-flow blocks,
outside inline-blocks. Initial and coincident breaks do not create empty pages.

Load templates with `template.Load`, `LoadFS` or `LoadFile`, and content with
`content.Load`. Construct a typography backend using the
[font setup example](preparation/typography/README.md#prepare-once-and-lay-out-at-two-widths).
The backend requires explicitly selected fonts and fresh shaping streams. The
source above uses `hyphens:none`; the default `manual` policy also requires a
manual-marker hyphenator.

To inspect a collection before pagination, run from the module directory:

```sh
go run ./pgtmpl serve examples/03-magazine.html
```

The browser preview shows the templates' empty frames. See the
[collection examples](examples/README.md) for more inputs and stylesheet overrides.

## Paginate and deliver a document

This helper accepts loaded inputs and a backend created with `prep.New`. It
requires complete pagination before delivering any page. Your consumer renders
or exports each `Page`.

```go
package example

import (
    "context"
    "fmt"

    "github.com/npillmayer/pagelayout"
    "github.com/npillmayer/pagelayout/content"
    prep "github.com/npillmayer/pagelayout/preparation/typography"
    "github.com/npillmayer/pagelayout/template"
)

func Paginate(ctx context.Context, catalog *template.Catalog, source *content.Document,
    backend *prep.Backend, consume pagelayout.PageConsumer) error {
    plan, err := pagelayout.Plan(source)
    if err != nil {
        return err
    }
    prepared, session, err := content.Prepare(source, backend)
    if err != nil {
        return err
    }
    defer session.Close()

    run, err := pagelayout.NewPagination(pagelayout.PaginationConfig{
        Plan: plan, Catalog: catalog,
        Prepared: prepared, Session: session,
        Fitter: prep.FittingAdapter(),
    })
    if err != nil {
        return err
    }
    result, err := run.Paginate(ctx, run.InitialState())
    if err != nil {
        return err
    }
    if !result.Complete() {
        return fmt.Errorf("pagination incomplete at span %d: outcome %v",
            result.State().Info().Span, result.LastSearch().Outcome())
    }
    if err := session.Close(); err != nil {
        return err
    }
    for _, span := range result.Spans() {
        if _, err := span.Selected().Shipout(ctx, consume); err != nil {
            return err
        }
    }
    return nil
}
```

Keep borrowed font resources valid through rendering, even after closing the
preparation session. The [complete font-to-delivery example](shipout_example_test.go)
loads both documents and a local font. Run it from the module directory:

```sh
go test . -run '^ExamplePagination_Paginate$' -v
```

See the [multi-page sample](testdata/pagination/shipout-proof.svg) and
[sample guide](testdata/pagination/README.md#real-font-document-proofs) for changing
frame widths, inline-blocks, manual hyphenation and deferred notes/figures.

## Render selected pages

`Page.Number()` is one-based. `Template()` identifies the chosen catalog entry;
`Info()` contains its page dimensions, paint and source. `Frames()` includes
unused receivers as well as populated ones.

For each frame, draw its compiled paint and inspect the placed content:

```go
for _, frame := range page.Frames() {
    receiver := frame.Frame.ContentBox
    for _, part := range frame.Fragments() {
        shaped, err := prep.InspectFragment(part)
        if err != nil {
            return err
        }
        // Draw shaped with receiver.X/Y as its page-local origin.
        _, _ = receiver, shaped // supply both to your renderer
    }
}
```

Frame rectangles are page-local CSS pixels. Fragment paragraph origins are
relative to the frame's content origin, also in CSS pixels. Frozen typography
paragraph coordinates use local big points: 1 CSS pixel is 0.75 big points.
Apply each origin once; do not restack positioned lines or inline-block interiors.
The [typography guide](preparation/typography/README.md) describes paragraph,
source-coverage and paint accessors.

`frame.Extent()` reports occupied allocation after placing its branch fragments.
`Containers()` supplies fragment-specific edges and paint. Retained pages and
copied inspection data remain usable after execution closes. Rendering and
export are supplied by your application.

## Configure selection and work limits

Set these fields on `PaginationConfig` before creating a run:

| Option | Usage |
| --- | --- |
| `BeamWidth` | Maximum unfinished sequences retained per search round. Zero selects 32. A positive larger width can explore more alternatives at greater cost. |
| `Policy` | Pointer to adjusted `DefaultLossPolicy()` weights. Nil selects defaults. Keep page cost positive. |
| `Limits` | Pointer to positive `WorkLimits`. Nil selects `DefaultWorkLimits()`. Use context cancellation for deadlines. |
| `Eligible` | Deterministic template filter. Nil allows every template. An error aborts search. |

For example, reserve an opening template for the first page:

```go
config.Eligible = func(state pagelayout.State, t *template.Template) (bool, error) {
    if state.Info().NextPage == 1 {
        return t.ID() == "opening", nil
    }
    return t.ID() != "opening", nil
}
```

The filter must not reenter pagination or use its preparation session. Template
order breaks equal-loss ties; put preferred templates earlier in the collection.
Search chooses one locally best page per template and optimizes one span at a
time. It does not reconsider selected spans to improve subsequent ones.

`Score()` exposes page-loss components; `Loss.Info()` provides integer units and
a separate blocked flag. One point is `UnitsPerPoint` (1,000,000 units). Use
`Compare` and checked `Add` when comparing or combining losses. To assess your own
occupancy facts, use `AssessPage`; a score alone does not establish that content
fits. See the [loss reference](doc/SHIPOUT.md#loss-algebra) for component meanings.

## Handle incomplete pagination and errors

Always check both the error and `DocumentResult.Complete()`. A search can finish
normally without finding a complete sequence:

| Search outcome | Client action |
| --- | --- |
| `SearchComplete` | Use `Selected()` and inspect `HeuristicTruncated()` for whether alternatives were discarded. |
| `SearchInfeasible` | Revise templates, content or eligibility; no completion exists under the configured local page and eligibility policies. |
| `SearchHeuristicExhausted` | Feasibility is undetermined after beam cuts. Consider a larger beam or revised inputs. |
| Cancellation, limit or execution error | Inspect the error and exact retry state. Any `Incumbent()` is unselected and cannot be shipped. |

`result.Spans()` retains selections completed before a later failure. Deliver
those explicitly if your application permits partial documents. To retry within
the same live run, pass `result.State()` to `Paginate`; it resumes without
searching earlier spans again. A retry uses the run's existing configuration.
There is no automatic retry or beam widening. `LastSearch()` and `Stats()` provide
the outcome and work counts for diagnosis.

Use `errors.As` with `*PlanError` for source ownership/break diagnostics or
`*PaginationError` for category, location and cause. `errors.Is` reaches category
sentinels and original backend/cancellation causes. Invalid loads and preparations
publish no partial inputs; their subpackage guides describe those errors.

Serialize pagination, preparation-session use and shared font/shaping services.
States belong to one run; an identical reload or new run has different identity.
Use returned successors, never reconstructed source positions, to resume.

## Drive individual pages or spans

Use `PageAttempt(ctx, state, templateID)` to compare templates from the same
unchanged state. `Constructed` returns a candidate with `Page()`, `Score()` and
exact `Successor()`. `NoFit` can report queue `Obstructions()`; try another
template. `SpanComplete` means the input was already drained and produces no page.
An explicit attempt does not apply the `Eligible` filter or select a sequence
for `Shipout`.

Use `PaginationSearch(ctx, state)` to select one complete span. Retain its
`Selected()` sequence, then call `AdvanceSpan(sequence.FinalState())` unless
`DocumentComplete()` is true. Related secondary material must be fully drained
before this transition. Page numbering and exact continuations are preserved.

## Handle delivery failures

`Sequence.Shipout` calls your consumer synchronously in page order and accepts
only selected sequences. On failure, inspect its `DeliveryReport`:

- `CompletedPages()` lists successful consumer calls.
- `FailedPage()` identifies a consumer error, or is zero.
- `NextPage()` is the first page not successfully delivered, or zero.

A failed consumer may already have written output. There is no rollback or
hidden delivery cursor: calling `Shipout` again starts that sequence again.
`DeliveryError` preserves the cause and matches `ErrDelivery`. Cancellation is
checked around calls; cancellation during the final successful call can return
an error together with `Complete() == true`. Empty selections make no calls.
