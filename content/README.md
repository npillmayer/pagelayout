# content

`content` loads styled HTML into immutable, unsized box material. Use
`flow-into` to route block branches into named channels, inspect their source
styles and groups, and derive register banks without changing the document.
Use an explicit backend for preparation, intrinsic measurement, whole-tree
layout or finite-height fitting. For automatic page selection and delivery, use
the [root package](../README.md).

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
      <p>Introductory content <a href="#note" data-flow-anchor>1</a>.</p>
      <section style="flow-into:main"><p>First article.</p></section>
      <aside id="note" style="flow-into:notes">A note.</aside>
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

`Channels()` returns `main` first, then secondary names in first-occurrence order.
`Branches(name)` returns that channel's branches in source order. Names are case-sensitive; branches
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
Routing validation rejects nested directives even when both name `main`. An
empty body is valid and has a residual root and an empty group. A completely `display:none` body has a residual tree identity but
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

## Prepare material

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

## Fit a finite receiver and resume

Create one `NewFitter(prepared, session, root)` for a covered normal-flow block.
The fitting backend implements `GroupFitter`; for font-backed paragraphs, use
[`preparation/typography.Fit`](../preparation/typography/README.md#fit-frames-and-resume).
With a custom backend:

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
    return nil // no material fits; choose another receiver
}
selected := choices[0] // apply your selection policy
fragment := selected.Fragment()
if !selected.Complete() {
    choices, err = content.Fit(ctx, prepared, session, fitter, content.FitRequest{
        Width: 180, Height: 320, Continuation: selected.Continuation(),
    }, groupFitter)
    if err != nil {
        return err
    }
}
_ = fragment // retain or render the selected prefix independently of its remainder
```

Width and height are finite, nonnegative CSS pixels. `MaxSteps` sets a local work
cap; zero means no cap. `IncludeEmpty` also requests the unchanged-cursor choice.
Use `Progress`, `Complete` and `StopReached` to distinguish progress, exhaustion
and arrival at an optional `Stop`. Obtain a structural stop with
`fitter.Boundary(box, edge)` or `BoundaryAt(position)`.

Resume with the exact `Continuation`, the same fitter and its matching live
preparation/session. `End()` and `End().Logical()` are inspection coordinates,
not replacement continuations. Use `Compare` rather than sorting raw offsets.
A different fitter has different cursor identity even for the same source.

Failed or interrupted calls return nil, never a partial choice set. An empty
successful set means no fit. `ErrFitInterrupted` identifies voluntary interruption.
Old fragments and checkpoints remain usable after failure. Fitting neither
advances a bank nor selects pages.

`FitFragment` exposes copied `Geometry`, `UsedValues`, `Containers` and `Paragraphs`.
Each paragraph occurrence has its own FragmentID and source group/owner. Paragraph
origins use receiver-relative CSS pixels; box geometry uses the content origin
identified by `RelativeTo`. Use the retained container edges and paint when drawing
split boxes. Consumed inline-block interiors stay fixed across width changes.

## Compose branches in one receiver

Call `ComposeFragments(width, height, fragments)` with fragments fitted at the
receiver's width and ordered as they should appear. It returns positioned fragments,
combined `Extent` and `Fits`, without needing a session or paragraph backend.
Inputs remain unchanged. Use the composition's extent for occupied receiver space;
individual fragments retain their standalone allocations.

For intermediate branches whose trailing margins may collapse with a following
negative leading margin, fit with `DeferVerticalBounds: true`. Such choices have
not passed final vertical bounds checks: compose them and require `Fits` before
accepting the arrangement. Use the default bounds checks for standalone fragments.

## Inspect pagination source facts

Use `pagelayout.Plan(doc)` for validated ownership and span membership. For source
inspection, `SourceEvents()` reports authored IDs, owning-link annotations and
normalized forced breaks; `EventPositions()` adds qualified positions, affinity
and group coverage. Both return independent records without layout or font calls.
Ordinary links do not create owning events, and excluded annotations are inactive.

`break-before: page`, left, right, recto and verso normalize to page. Auto creates
no break event. Avoidance, column/region breaks and `page-break-before` are
unsupported. Plan validates whether an active break is at a supported location.

Source decoded-byte ranges, processed group-byte ranges and structural positions
are different coordinates. Use `LogicalPosition.Compare` only within one source
tree; `SourceID()` identifies the exact loaded snapshot. `TreeBounds` gives tree
endpoints; suppressed roots have invalid endpoints. Empty owning links can attach
to a block boundary without paragraph coverage. Source positions never authorize
resumption or establish that an anchor was placed.

`prepared.EmptyRange(root, from, to)` can certify a range without text, atomics,
paint, spacing or nonzero definite width. False also covers invalid or unproved
ranges. True is not a continuation: a custom coordinator must still advance via
the fitter and account for owning annotations. Use root pagination to handle
empty spans automatically.

## Supply a custom backend

Implement `Preparer` to provide reusable group execution handles:

| Method | Client contract |
| --- | --- |
| `Preflight([]GroupInput) error` | Validate all policies and required services without font, shaping or layout work. |
| `PrepareGroup(GroupInput) (GroupPreparation, error)` | Return the matching group/owner and an exclusively owned GroupHandle. |
| `GroupHandle.Release() error` | Release owned execution resources without closing borrowed services or invalidating frozen output. |

Calls receive independent input slices. Preparation failure releases acquired
handles in reverse order and returns neither snapshot nor session. The
[recording-backend example](prepare_example_test.go) shows a complete implementation.
For measurement, layout and fitting, implement `GroupMeasurer`, `GroupLayouter`
and `GroupFitter` as needed; see their godoc for callback inputs and outputs.

`session.WithGroup` borrows a handle for one callback. Do not retain or release
it or reenter the session. `session.Check(prepared)` checks identity and open/idle
state without calling the backend. Serialize these calls with execution and Close.

For use with root pagination, frozen paragraph payloads implement
`ParagraphOutput.LineCount()` and, for groups with annotations,
`ParagraphEvents.PlacedEvents()`. Return copied original events actually covered
by rendered material, including cluster/replacement and empty-anchor affinity.
Consumed byte offsets alone do not establish visible placement. The supplied
[typography adapter](../preparation/typography/README.md) handles these contracts.

Use `errors.As` with `*PreparationError` for stage, group/owner and source/range
details. `errors.Is` also reaches original causes and lifecycle sentinels such as
`ErrInvalidPreparation`, `ErrClosedSession` and `ErrSessionBusy`.
