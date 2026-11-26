# pagelayout

Load HTML/CSS page templates and content, inspect their flow channels, and
prepare content, compile ownership/span plans and construct pages across channels.

A **page template** describes a page and its receiving **frames**. A frame's
`flow-from` names the channel it receives. In a separate content document,
`flow-into` assigns a block branch to that channel. Content keeps the styles
from its own document; a template's styles do not restyle the content.

## Choose a package

| Package | Use it to |
| --- | --- |
| `pagelayout` | Validate owning links and page spans; construct pages with exact channel remainders and inspectable loss. |
| [`template`](template/README.md) | Load and validate collections; select templates and inspect page dimensions, frame rectangles, paint and flow chains. |
| [`content`](content/README.md) | Load styled HTML, inspect source material and take whole branches from persistent channel registers. Prepare material and fit fragments with an explicit backend. |
| [`preparation/typography`](preparation/typography/README.md) | Supply fonts and shaping services, prepare paragraphs once, measure intrinsic widths, lay out complete subtrees or fit resumable fragments. |
| [`pgtmpl`](pgtmpl/README.md) | Check, locate and preview template collections from the command line. |

Import `github.com/npillmayer/pagelayout` for source planning and page construction,
and the subpackages for loading, preparation and layout.

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
4. Measure or lay out a complete branch, or create a `content.NewFitter` and use
   `preparation/typography.Fit` with a frame's content width and height. Resume a
   selected fragment in another frame with its exact continuation. Reuse the
   preparation and close the session when execution is finished.

Layout produces a complete subtree at a width. Fit returns finite-height
alternatives with frozen output and exact continuations. Neither selects pages
or advances channel registers automatically. Taking an item from a register consumes a whole branch
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

## Initialize pagination state

Bind a plan, compiled catalog and the matching completed preparation/session:

```go
run, err := pagelayout.NewPagination(pagelayout.PaginationConfig{
    Plan: plan,
    Catalog: catalog,
    Prepared: prepared,
    Session: session,
    Fitter: prep.FittingAdapter(),
})
if err != nil {
    return err
}
state := run.InitialState()
fmt.Println(state.Info().Span, state.Info().NextPage) // 1 1
if err := run.Validate(state); err != nil {
    return err
}
```

Here `prep` is `github.com/npillmayer/pagelayout/preparation/typography`.
Omit `Fitter` only when using initialization and state inspection without execution.
Keep the borrowed preparation session open and serialize execution access.
Initialization and validation invoke no font or layout backend. A state from
another run is invalid even when its source and options match. Closing the
session prevents execution validation; copied source/state inspection remains
available. State inspection returns independent heads, activated-target and
pending-target slices. Deriving a bank with `state.Bank().Take(...)` does not
change the state.

A missing head continuation means that head's beginning. Completed branches
leave their registers together with their cursors; partial branches retain their
original handles. State advancement is private: source positions alone cannot
supply a resumable fitter checkpoint.

## Construct a page

Call `PageAttempt` with an explicit state and a template ID from the bound catalog:

```go
attempt, err := run.PageAttempt(ctx, state, "two-column")
if err != nil {
    return err
}
switch attempt.Outcome() {
case pagelayout.Constructed:
    candidate := attempt.Candidate()
    page := candidate.Page()
    fmt.Println(page.Number(), candidate.Score().Total.Info())
    state = candidate.Successor()
    for _, frame := range page.Frames() {
        for _, fragment := range frame.Fragments() {
            shaped, err := prep.InspectFragment(fragment)
            if err != nil {
                return err
            }
            // Draw shaped at frame.Frame.ContentBox.X/Y in page-local CSS pixels.
            _ = shaped
        }
    }
case pagelayout.NoFit:
    // Try another template from the same unchanged state.
case pagelayout.SpanComplete:
    // This span was already complete; no page was created.
}
```

The attempt compares whole-page arrangements, including shorter prefixes and
unused frames. It returns one candidate per template. Try several templates from
the same state to compare their pages; rejecting a candidate changes no bank or
previous output. `Origin()` identifies that original state. Successive attempts
use `Successor()` with its exact checkpoint, even when frame widths change.

`Page.Info()` and each frame retain the catalog's geometry and paint. Content
keeps its own source styles. Frozen fragments use frame-content-relative CSS
coordinates; typography paragraph coordinates are local big points. Inspect
`Containers()` for fragment-specific edges and paint. Keep a `Page` handle for
rendering after closing the session, while retaining its borrowed font resources.
It holds no successor or execution services.

`frame.Fragments()` returns branch fragments in register order. Several complete
branches and a final partial branch can share one receiver. Their outer margins
collapse within that frame; `frame.Extent()` reports its total occupied extent.
Fragments preserve source identities and frozen interiors.

Owning anchors activate targets only when their associated material is placed.
A target can appear above its owner on the same page, including transitive
ownership across channels. Registers retain authoring order: an inactive or
future-span head blocks later items. Ordinary links do not activate anything.
`NoFit` may carry copied `Obstructions()` describing entry-state queue heads and
pending items behind them; these facts do not prove that every template fails.

Forced main breaks stop the current span inside its retained head. Related
targets must drain before the span is complete; progressing drain pages may omit
a main receiver. Future-span branches remain untouched. Span transitions and
template-sequence search are unavailable through the public execution API.
A page that finishes its span is still `Constructed`; `SpanComplete` describes
an already-complete input.

Work limits and cancellation return errors without a candidate or successor.
The local budget counts arrangement visits, frame calls, measurements, paragraph
calls and yielded choices; core traversal is also bounded. Backend failures
preserve their original causes and identify the template/frame where known.
Custom fitting backends implement `content.GroupFitter`; their frozen paragraph
outputs must implement `content.ParagraphOutput` to supply factual line counts.
For groups with source events, outputs also implement `content.ParagraphEvents`:
report only events reached by the rendered prefix, using the original group
event values. Logical byte consumption alone does not activate an owner.

## Evaluate page loss and compare choices

`AssessPage(policy, facts)` evaluates supplied frame occupancy, pending targets
and paragraph-fragment facts. It does not establish that material fits.
`DefaultLossPolicy()` provides adjustable weights in integer cost units;
one point is `UnitsPerPoint` (1,000,000 units). Page cost must be positive.
For custom initialization, set `PaginationConfig.Policy` and/or `Limits`;
nil selects defaults. `BeamWidth: 0` selects 32.

`Loss.Info()` exposes exact units and a separate blocked tag. `Compare` prefers
smaller finite losses over blocked; `Add` detects overflow. `QuantizeLoss`
rounds a complete floating weighted component in units, with ties to even.
`AssessPage` preserves integer coefficients exactly and rounds each component
once. Errors return no partial score.

Underfill uses participating frame areas at entry. Newly activated incomplete
targets incur carry immediately, but their channels enter underfill on the next
attempt. The finishing and later main frames are exempt on a span-ending page,
with their areas retained in the denominator. Use normalized group handles for
`ParagraphFragment.Paragraph`; consecutive pieces in the same frame coalesce
before counting one-line split penalties.

`CompareLocal` ranks arrangements within one template by loss, further main
progress, secondary progress in source-channel order, then enumeration order.
`CompareSequences` ranks by accumulated loss then lexicographic template
ordinals, including sequences of different lengths. Neither comparison merges
states or certifies interchangeable continuations.

Outcome contracts distinguish `Constructed`, `NoFit` and `SpanComplete`.
Search reports distinguish completion, proven infeasibility, heuristic exhaustion
and interruption. An interrupted report's `Incumbent()` is explicitly unselected.
`PaginationError` retains a category, diagnostic location and original cause for
`errors.Is`/`errors.As`. Candidate/sequence inspection returns copies; public
constructors for speculative pages or forged successors are not provided.
