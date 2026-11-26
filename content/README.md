# content

`content` loads styled HTML into immutable, unsized box material. Use
`flow-into` to route block branches into named channels, inspect their source
styles and groups, and derive register banks without changing the document.
The package also provides preparation, measurement and layout operations for
caller-supplied backends.

```go
import "github.com/npillmayer/pagelayout/content"
```

## Load and route HTML

This complete program loads a default `main` tree and takes it from a derived
bank:

```go
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/npillmayer/pagelayout/content"
)

func main() {
	const source = `<body style="font-size:16px;hyphens:none">
      <p>Introductory content.</p>
      <section style="flow-into:main"><p>First article.</p></section>
      <aside style="flow-into:notes">A note.</aside>
      <section style="flow-into:main"><p>Second article.</p></section>
    </body>`
	doc, err := content.Load(strings.NewReader(source), content.LoadOptions{
		SourceName: "article.html",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("channels:", doc.Channels())
	fmt.Println("main branches:", len(doc.Branches("main")))

	initial := doc.Bank()
	first, remaining, ok := initial.Take("main")
	if !ok {
		log.Fatal("main is empty")
	}
	originalFirst, _ := initial.First("main")
	_, hasMore := remaining.First("main")
	fmt.Println("original bank unchanged:", originalFirst == first)
	fmt.Println("derived bank advanced:", !hasMore)
	fmt.Println("residual available:", doc.Residual().Root.Valid())
}
```

Output:

```text
channels: [main notes]
main branches: 1
original bank unchanged: true
derived bank advanced: true
residual available: true
```

`Load` reads a complete UTF-8 HTML document. The caller owns and closes the
reader. To load a file, open it with `os.Open`, defer its `Close`, and pass the
file to `Load`; the package does not search for files or fetch resources.
An error returns a nil `Document`, even if earlier branches were valid.

Content is styled in its original HTML context. Selectors and inheritance use
that document, including ancestors outside a routed branch. Receiving frames
and template styles have no effect on these source styles.

## Work with channels and residual content

`Channels()` returns `main` first, then secondary names in first-occurrence order. `Branches(name)` returns
that channel's branches in source order. Names are case-sensitive; branches
with the same secondary name remain separate items. Main has `BranchInfo.Order` 1;
secondary branches follow in global source order.

`Bank()` returns a persistent `RegisterBank[content.Box]` containing whole
branch roots. `First` inspects an item; `Take` returns the item, a derived bank,
and a success flag. `Prepend` can restore an unchanged whole item:

```go
restored := remaining.Prepend("main", first)
```

These operations do not change the original bank or `Document.Channels()`.
Taking a branch does not lay it out, prove that it fits or represent a partial
remainder. A `Box` is a comparable handle: compare complete handles, not bare
numeric IDs. It retains its source material independently of the public
`Document` handle.

`Residual()` aliases the single default `main` tree. Valid `flow-into:main`
directives are redundant: their content stays in source order in that tree.
Routing validation still rejects nested directives, even when both name `main`. An empty body is valid and has a residual root and an
empty group. A completely `display:none` body has a residual tree identity but
no root or groups; check `Root.Valid()` before using it. Its channel inventory
still contains `main`, but its bank and main branch list are empty.

Named flow roots must be non-nested block branches under block ancestry.
Routing html/body, inline elements, inline-blocks or descendants inside an
inline-block fails. `flow-into:none` leaves a descendant with its enclosing
branch; it does not extract another branch.

## Inspect boxes, groups and source text

| Accessor | Returned information |
| --- | --- |
| `box.Info()` | Copied constraints, paint, source context and text reference; false for an invalid handle. Widths can still be auto or percentages. |
| `doc.Groups(tree)` | Groups in normalized preorder, including empty groups and inline-block interiors. |
| `GroupInfo.Owner` / `TextStyle` | The paragraph owner's box and source typography, also for empty groups. |
| `GroupInfo.Atomics` / `ParentAtomic` | Immediate inline-block boundaries and the nearest containing inline-block. Interior text belongs to its own group. |
| `doc.NodeSource(id)` | Original source ancestry, attributes, classification and resolved source typography. |
| `doc.TextSource(id)` | Raw decoded UTF-8 text, including excluded or omitted source text. |

For example, after loading `doc`:

```go
for _, branch := range doc.Branches("main") {
	fmt.Printf("branch %d: %s\n", branch.Order, branch.Source.Path)
	for _, group := range doc.Groups(branch.Tree) {
		fmt.Printf("  owner=%s atomics=%d\n", group.Source.Path, len(group.Atomics))
	}
}
```

Inspection returns copies and is safe concurrently. `SourceInfo.Children`
describes original DOM ancestry, including routed and excluded descendants;
it is not a paragraph's text sequence. No mutable box tree is exposed.

Source text ranges use byte offsets in decoded UTF-8 text, not HTML offsets or
rune indices. Source font sizes use DOM `dimen.DU`; box lengths use CSS pixels
or typed auto/percentage constraints. Do not treat the numeric font size as a
pixel value. Source-font `em`/`rem` lengths are resolved during loading; receiving
widths and line breaks are supplied separately during layout.

## Supply additional CSS

Read any extra stylesheet yourself and pass its text in `LoadOptions`:

```go
doc, err := content.Load(reader, content.LoadOptions{
	SourceName: "article.html",
	Stylesheets: []content.Stylesheet{
		{SourceName: "reader.css", CSS: cssText},
	},
})
if err != nil {
	return err
}
```

Sheets follow head CSS in the supplied order, with normal specificity, inline
and `!important` precedence. Extra CSS can change source styles and routing.
Loading again gives a separate document; retained boxes and banks keep their
original styles. Content CSS is never applied to a template collection implicitly.

The accepted profile includes static block/inline flow, inline-block interiors,
content-box sizing, positive font sizes, normal whitespace and normal
line-height. Hyphenation metadata is preserved as authored. Floats, image/table
content, constrained heights and unsupported resource requirements produce errors.
Scripts do not execute and no resources are fetched.

Losing and nonmatching active CSS declarations are checked too. `display:none`
removes material but does not exempt its declarations from validation. Inert
`template` contents and metadata retain source records without becoming active
material. See the [content examples](testdata/README.md) for routing, suppression,
source coordinates, stylesheet overrides and expected errors.

## Report errors

Use `errors.As` with `*content.LoadError` to inspect individual diagnostics
(import `errors` and `fmt`):

```go
var loadErr *content.LoadError
if errors.As(err, &loadErr) {
	for _, diagnostic := range loadErr.Diagnostics() {
		fmt.Println(diagnostic.Error())
	}
}
```

Each diagnostic includes `Stage`, `Code`, `Message`, `Source` and `CSS`.
`Related` identifies a conflicting route root when applicable. Source paths
refer to the parsed HTML tree; CSS locations refer to stylesheet text.
Additional CSS uses its own source label. `errors.Is` and `errors.As` preserve
access to original reader, parser and other underlying causes.

## Handoff to preparation

Pass the loaded document and an explicit `Preparer` to `Prepare`:

```go
prepared, session, err := content.Prepare(doc, backend)
if err != nil {
	return err
}
defer session.Close()
```

The [typography package guide](../preparation/typography/README.md) supplies a
complete example of font selection, backend setup, measurement and layout.
Preparation covers all groups, residual first and then branches in global source
order, including empty groups and inline-block interiors. It takes no receiving
width and consumes no register items. Keep using the same source `Box` handles.

`prepared.Groups()` lists covered groups; `prepared.Input(group)` returns copied
processed text, source maps, style and paint spans, policies and atomic occurrences.
Ranges in processed text are UTF-8 byte ranges. Whitespace can collapse or be
omitted, while source maps retain its provenance. A structural atomic marker
has an occurrence record; an authored U+FFFC remains text. Atomic interiors
are separate groups.

Keep each `PreparedSnapshot` with its matching `PreparationSession`.
`content.MeasureIntrinsic(prepared, session, root, context, measurer)` returns
border-box width bounds in CSS pixels, excluding the root's own margins.
`content.Layout(prepared, session, root, width, layouter)` returns copied geometry
for a complete subtree. Pass a fresh per-invocation callback object to each
operation. The typography backend provides convenience functions that also
return frozen paragraphs. Neither operation advances banks or fits a subtree
to a finite frame height.

Serialize session operations and any use of shared font/shaping services,
including `Close`. Closing is idempotent and releases owned execution handles;
it does not close borrowed services. Prepared metadata remains inspectable
and frozen results remain usable after closure, provided their font resources
remain valid. Execution failures leave a successful preparation usable for
another request. Preparation failure returns neither snapshot nor session.

### Fit a finite receiver and resume

Create one `content.NewFitter(prepared, session, root)` for a covered normal-flow
block. Reuse it with `content.Fit(ctx, prepared, session, fitter, request, backend)`.
`FitRequest` supplies CSS-pixel `Width`/`Height`, an optional exact `Continuation`,
optional `Stop`, `IncludeEmpty`, and a local `MaxSteps` cap (zero means no cap).
The backend implements `GroupFitter`, including intrinsic measurement from
retained handles. For the typography backend, use
[`preparation/typography.Fit`](../preparation/typography/README.md#fit-frames-and-resume).

```go
fitter, err := content.NewFitter(prepared, session, root)
if err != nil {
    return err
}
choices, err := content.Fit(ctx, prepared, session, fitter,
    content.FitRequest{Width: 240, Height: 320}, groupFitter)
if err != nil {
    return err
}
if len(choices) == 0 {
    return nil // no progressing material fits this receiver
}
selected := choices[0] // apply the application's selection policy
frozen := selected.Fragment()
next := content.FitRequest{
    Width: 180, Height: 320, Continuation: selected.Continuation(),
}
_, _ = frozen, next
```

Alternatives preserve core enumeration order and pair `Progress`, `Complete`,
`StopReached`, a comparable logical `End`, and the exact remainder. Use
`fitter.Boundary(box, edge)` or `fitter.BoundaryAt(structuralPosition)` for stops.
A text position cannot manufacture a structural cut or continuation. Distinct
fitters have distinct cursors, even for the same source and preparation.
`End().Logical()` maps the fitted endpoint to a source-qualified position. Its
inspection includes the backend's within-offset step for discretionary progress;
use `Compare`, not raw offsets, to order it. It cannot replace a continuation.

`FitFragment` contains copied `Geometry`, `UsedValues`, container paint/edge
facts, and cloned paragraph payloads. Each paragraph has a distinct `FragmentID`
and qualified group/owner identity, including consumed atomic interiors. Paragraph
origins are absolute within the receiver; box geometry uses `RelativeTo` content
origins. First/last fragment edges come from the used values, not source widths.
Accepted atomic interiors stay fixed while suffix atomics may change width.

The complete operation reserves the session. Invalid identity, cancellation,
work limits, backend errors and voluntary interruption return nil, never a partial
candidate set. `ErrFitInterrupted` distinguishes voluntary interruption from
exhaustive non-fit. Failed attempts leave old cursors, banks and frozen fragments
usable. Extract a `FitFragment` to retain render data without a continuation;
resumption requires the matching live session. No fit selects pages or consumes
a register bank.

`prepared.EmptyRange(root, from, to)` conservatively certifies that an interval
within a covered block root contains no text, atomics, paint, spacing or nonzero
definite width. Obtain source-qualified endpoints from `TreeBounds` or positioned
events. False includes invalid, foreign and unproved ranges. Inspection works
after session close and executes no backend. True is not a continuation: callers
must still traverse the structural fitter and process owning annotations before
advancing the material. The root paginator uses this to avoid empty-span pages.

For use with the page constructor, frozen paragraph payloads also implement
`ParagraphOutput.LineCount() uint64`. Report lines in that occurrence, including
zero for empty material. The page constructor combines this fact with exact
from/to and completion facts to assess split penalties without a font dependency.

For groups carrying source events, also implement `ParagraphEvents.PlacedEvents()`.
Return copied original `GroupEvent` values actually reached by the rendered
prefix, respecting cluster/replacement coverage and empty-anchor affinity.
Consumed bytes alone are insufficient for silent discretionary output.
`LogicalPosition.IsText()` distinguishes these associations from structural
positions; `fragment.ContainsBoundary(position)` checks a retained structural
edge without treating a suspended container as placed material.

### Compose branches in one receiver

Use `ComposeFragments(width, height, fragments)` to place independently rooted
fragments in register order. Supply fragments fitted at that receiver's width.
Their frozen interiors and identities remain unchanged; their outer margins
collapse through the ordinary core flow rules. The result gives copied fragments
with translated geometry and paragraph origins, total `Extent`, and `Fits`.
No session or paragraph backend is needed. `FitFragment.Extent()` retains its
standalone allocation; use the composition's extent for the complete receiver.

When a later branch's negative leading margin can collapse an earlier trailing
margin, fit intermediate roots with `FitRequest.DeferVerticalBounds: true`.
Such candidates have not passed final vertical bounds checks. Compose the
selected roots and require `Fits` before accepting the arrangement. The flag
preserves finite geometry, horizontal checks and the paragraph height ceiling.
The default request checks complete bounds directly.

### Supply a custom backend

A `Preparer` has two methods:

| Method | Caller contract |
| --- | --- |
| `Preflight([]GroupInput) error` | Validate all policies and required services before performing font, shaping or layout work. |
| `PrepareGroup(GroupInput) (GroupPreparation, error)` | Return matching `Group`/`Owner` handles and an exclusively owned `GroupHandle` for each input. |

Each call receives independent input slices. `GroupHandle.Release()` releases
owned execution resources. On a preparation failure, acquired handles are
released in reverse order. `*content.PreparationError` carries stage, qualified
group/owner, source and range when known; wrapped causes include cleanup errors.

Use `session.WithGroup(prepared, group, owner, callback)` to borrow a backend
handle for one callback. Do not retain or release that handle or reenter the
session from the callback. Immutable output may be retained. See the
[recording-backend example](prepare_example_test.go) for a complete custom
`Preparer` and its lifecycle.

## Inspect pagination source facts

`doc.SourceEvents()` returns copied records for authored IDs, `data-flow-anchor`
presence with raw `href`, and effective forced `break-before` values. `page`,
`left`, `right`, `recto` and `verso` all produce `SourcePageBreak` with value
`page`; `auto` produces no break event. Additional CSS follows normal cascade
rules. Avoidance, column/region breaks and legacy `page-break-before` are rejected.

Use each event’s `Source` and `Parent` with `NodeSource` to inspect its original
position. Excluded ID/anchor annotations have `Active == false`; ordinary links
do not create anchor events. These records do not resolve targets, validate an
ownership graph, or identify prepared-text offsets. No boxes or text are inserted.

## Inspect logical event positions

`doc.EventPositions()` maps active annotations to original source blocks,
normalized positions and processed group coverage without fonts or layout.
Each `PositionedEvent` preserves its original event, affinity and copied coverage.
`UnsupportedBreak` flags a location for the root planner to reject; ordinary
content loading remains independent of ownership rules. Use `pagelayout.Plan`
when you need ownership validation and span membership.

`LogicalPosition.Compare` accepts positions only within the same source tree.
`Info()` exposes copied structural/processed coordinates and qualified box/group
handles. A comparison is not a permission to exchange layout continuations.
`TreeBounds` provides the endpoints of a retained tree; suppressed roots have
invalid endpoints. `SourceID()` is a comparable, opaque snapshot identity.

`content.Prepare` carries anchor coverage in `GroupInput.Events`. Each record
keeps source ranges separate from processed UTF-8 ranges; several records can
belong to one anchor, which must activate only once. Empty events have zero-width
ranges and add no text or boxes. PreparedSnapshot.Input and backend inputs own
independent event slices, retained after session closure. Structural events
outside groups remain in the source plan. Glyph/cluster coverage is supplied by
the paragraph backend when fitting actual fragments, not by this source map.

`session.Check(prepared)` checks preparation identity and whether the session is
open and idle, without invoking a backend. Serialize it with execution and Close.
Errors preserve `ErrInvalidPreparation`, `ErrClosedSession` or `ErrSessionBusy`.

Empty-anchor affinity considers retained blocks even when they contain no text.
When affinity lands on a block boundary, the event can have no paragraph coverage;
use its structural position in the plan. An anchor outside an inline atomic
attaches to the atomic occurrence, never an interior paragraph. Likewise, visible
coverage outside an atomic counts the occurrence once rather than also counting
its interior text.

Use `LogicalPosition.Compare`, not `PositionInfo.Order` or a lone group offset,
for progress ordering. Comparison canonicalizes processed byte boundaries onto
normalized leaves, placing an atomic's end after its interior and equating shared
processed boundaries across adjacent source ranges. Inspection still retains the
original source box and group-byte coordinate separately.
