# Preparation between CSS box construction and layout

Date: 2026-11-21. Status: design discussion, before implementation.

This note develops the preparation phase introduced in [DESIGN.md](DESIGN.md).
Preparing reusable content before page trials is established direction. The
interfaces, pass driver and ownership details below are **proposals**, not agreed
APIs or implemented capabilities. The sibling modules are under our control;
their current limitations identify integration work rather than permanent limits
on this design.

## Recommendation

Add a preparation callback alongside the existing typography callbacks, and
introduce an explicit pass **after box normalization and before used-value
resolution**. Its result should be reusable input to both intrinsic measurement
and layout.

The addition is more than another callback: it establishes a prepared-material
lifetime spanning many layout trials. The box core owns the traversal;
the typography adapter prepares each horizontal group; `pagelayout` decides
when to prepare its flow material and which page trials consume it.

This need not depart from CSS formatting semantics. CSS does not prescribe this
particular implementation architecture. CSS Text explicitly permits a different
processing order when the resulting layout is equivalent. Our distinctive choice
is to expose preparation as a first-class, reusable stage.
[CSS Text processing order](https://www.w3.org/TR/css-text-3/#order).

## 1. What the existing pipeline already provides

The current `css-box-layout` sequence is approximately:

```text
BuildLayoutTrees
    -> ResolveUsedValues
    -> FlowLayout
        -> intrinsic measurement where needed
        -> resolve remaining widths
        -> lay out atomic interiors
        -> delegate paragraph layout
        -> stack blocks
```

There are already two typography-facing callbacks:

| Existing callback | Inputs and responsibility |
| --- | --- |
| `IntrinsicMeasurer.MeasureInline` | Measure a group's intrinsic contributions using resolved measurement edges and atomic intrinsic bounds. No final paragraph geometry is published. |
| `InlineLayouter.LayoutInline` | Lay out a group using a definite content width, used values and completed atomic dimensions; return extent and atomic placements. |

The layout callback's inputs make it too late to serve as our preparation
boundary. See the [existing interfaces](../../css-box-layout/layout/interfaces.go)
and [layout orchestration](../../css-box-layout/layout/layout.go).

The `tytui` adapter already calls a private `prepareGroup` from both callbacks.
It extracts text, selects fonts, constructs bidi information and calls
`paragraph.Prepare`, caching the result for that layout invocation. The proposed
pass would promote this hidden, lazy operation into an explicit phase and extend
its lifetime beyond one invocation.
See [the preparation adapter](../../tytui/documentlayout/preparation.go),
[measurement](../../tytui/documentlayout/measure.go) and
[layout](../../tytui/documentlayout/layout.go).

## 2. Where preparation belongs

Preparation follows structural normalization. At that point we know which text
and inline boxes form each actual horizontal group. Anonymous wrappers, block
children inside inline elements and extracted flow branches have already been
accounted for.

Preparing individual DOM text nodes, `<span>` elements, or blindly preparing each
`<p>` would be too early: those units need not match the groups that box layout
eventually delegates. The preparation callback should receive normalized groups.

![Preparation after box normalization, with reusable material feeding repeated layout trials](diagrams/preparation-pipeline.svg)

[Graphviz DOT source](diagrams/preparation-pipeline.dot)

The diagram separates reusable loading results from each trial's private sizing
and placement work. Its trial stages show dependencies: laying out an atomic
interior itself invokes paragraph layout for the groups inside that atomic box.
Used-value resolution and intrinsic sizing are grouped together because they
interact recursively. The coordinator advances private cursors while traversing
frames and commits only a selected complete page. The fragment fitter remains
proposed.

Preparation also precedes intrinsic measurement: measurement should consume the
same shaped material that subsequent layout uses.

The remaining layout work need not become a rigid sequence of independent
passes. Used-value resolution and intrinsic sizing depend on one another. An
auto-width inline-block needs intrinsic bounds before its final width can be
chosen, and its descendants then need that chosen width. Preparation supplies
reusable material to this process without choosing its geometry.

## 3. What preparation establishes

| Concern | Preparation establishes | A trial determines |
| --- | --- | --- |
| Source text | Processed logical text and source mapping | Which source range fits |
| Typography | Font selection, shaping inputs and base shaped material | Line-dependent adjustments and selected alternatives |
| Bidi | Paragraph context and resolved logical information | Ordering within the resulting lines |
| Breaks | Opportunities and preparation policy | Selected line and fragment boundaries |
| Inline atoms | Stable occurrence identity and separately prepared interior | Width, height, margins and placement |
| Box structure | Normalized hierarchy and style constraints | Used dimensions and positions |
| Continuation | Material against which continuation is meaningful | Accepted prefix and remaining position |

Preparation can know glyph advances and authored absolute dimensions. “Unsized”
means that the material has not been laid out for a particular receiver.

Source-dependent font information must be settled at this boundary. A paragraph
must not acquire its font size from the frame's later resolution context. Both
typography and box-length resolution must use consistent source font inputs.
A percentage box width can remain unresolved until containing geometry is known.
This follows CSS's distinction between computed values and layout-dependent used
values. [CSS value processing](https://www.w3.org/TR/css-cascade-5/#computed).

## 4. A preparation callback returning a reusable handle

The proposed sibling callback should return a prepared handle, rather than
merely populate an adapter's hidden cache. An illustrative interface is:

```go
// PROPOSED interfaces, not existing css-box-layout APIs.
type InlinePrepareRequest struct {
    Group   *LayoutNode
    OwnerID BoxID
}

type InlinePreparer interface {
    PrepareInline(InlinePrepareRequest) (PreparedInline, error)
}

type PreparedInline interface {
    MeasureInline(InlineMeasureRequest) (IntrinsicWidths, error)
    LayoutInline(InlineRequest) (InlineTrialResult, error)
}
```

The preparation request contains no frame width, height budget, used-value table
or sized atomic objects. The adapter obtains source text, effective styles and
font services from its stable source context.

`PreparedInline` would be a small interface defined by `css-box-layout`. Its
concrete implementation in the typography adapter would wrap `paragraph.Prepared`
together with source mappings and atomic occurrence mappings. The box core
would not import `khipu` or inspect shaped glyphs.

The three operations have distinct purposes:

- `PrepareInline` establishes reusable horizontal material.
- `MeasureInline` obtains intrinsic contributions under supplied measurement
  conditions.
- `LayoutInline` produces one independent candidate using definite dimensions.

The prepared receiver makes the dependency explicit: measurement and layout
operate on the material returned by preparation. They cannot accidentally
prepare a different version of the paragraph.

`InlineTrialResult` is also a proposed type. It represents today's extent and
atomic placements plus a retained, immutable paragraph-result handle. Currently
the adapter stores paragraph output in invocation-local maps. Multiple trials
need independent results; overwriting one “current paragraph result” per group
would defeat that. The precise payload interface remains open.

When fragmentation is introduced, a `FitInline` operation can add height budgets
and continuations while using the same prepared handle. Whole-group layout and
partial fitting should remain distinct operations. The request/result types and
compatibility with existing callbacks need refinement before implementation.

## 5. The box-level preparation driver

Add an explicit driver in `css-box-layout`, conceptually:

```text
PrepareBoxes(roots, inlinePreparer) -> PreparedBoxes
```

`PreparedBoxes` need not be a second copy of the box tree. It can contain:

```text
immutable normalized roots
+ group identity -> prepared inline handle
```

Group identity must be scoped to the corresponding tree/preparation; a bare
`BoxID` is not globally unique across independently built trees.

This preserves the existing separation between structural nodes and layout
geometry. Each trial gets its own used-value tables, atomic realizations,
paragraph decisions and placements. Preparation adds reusable data alongside
the structure.

The driver belongs in `css-box-layout`, because that module understands which
nodes are horizontal groups and how nested formatting contexts are represented.
`pagelayout` invokes it for flow material. The typography adapter implements the
callback and retains the source correspondence it needs.

![Requests, returned handles and reuse between css-box-layout and typography during preparation and trials](diagrams/preparation-interactions.svg)

[Graphviz DOT source](diagrams/preparation-interactions.dot)

The left column is the box core; the right is the typography adapter and the
paragraph services it calls. Solid arrows show requests or orchestration,
dashed arrows return values, and dotted arrows reuse of the same preparation.
The preparation pass and handle interfaces are proposed; the named paragraph
operations already exist. The adapter preserves source/style context and handles
unit and identity conversion, keeping khipu types out of the box core.

The trial lane illustrates whole-group layout with independent retained results.
Partial paragraph fitting would use the separate proposed `FitInline` contract;
neither existing `LayoutInline` nor `Trial.Assemble` alone supplies that service.
The core repeats measurement and layout for nested atomic interiors as needed.

## 6. Nested inline-blocks demonstrate the boundary

Consider:

```html
<p>
  Before
  <span style="display: inline-block; width: auto">
    Some text inside the inline-block.
  </span>
  after.
</p>
```

There are two independent horizontal groups: the outer paragraph and the text
inside the inline-block.

During preparation:

1. The outer group becomes prepared text containing an unresolved atomic
   occurrence.
2. The inline-block's interior gets its own prepared paragraph.
3. The association between that atomic occurrence and its box subtree is retained.
4. Neither paragraph receives a target width, and the inline-block has no final
   height.

The outer paragraph's preparation does not need the inner paragraph's dimensions
or even its preparation result. It needs the atomic identity.

The driver can therefore walk top down, as envisaged in the author's remarks.
The outer callback stops at the atomic boundary when extracting its text; the
driver continues into the atomic subtree and prepares its separate groups.
The current [extraction code](../../tytui/documentlayout/extract.go) already
distinguishes outer atomic occurrences from their interior groups.

During a trial, the dependency order changes:

1. The box core obtains intrinsic bounds for the inline-block using its prepared
   interior.
2. It chooses the inline-block's width under the receiving constraints.
3. It lays out the prepared interior at that width, obtaining its height.
4. The outer paragraph receives the resulting atomic dimensions.
5. Its line breaker and vertical assembly place the object among surrounding text.

A narrower trial can change both the inline-block's interior line breaks and the
outer paragraph's line breaks while reusing both preparations.

This exposes a naming problem: the existing `prepareAtomics` performs actual
layout of atomic interiors and returns completed border dimensions. It is trial
work, not the preparation phase described here. Rename it to something such as
`layoutAtomicInteriors`, and similarly clarify `PreparedAtomic`, to avoid two
materially different meanings of “prepared.”
See [the current atomic operation](../../css-box-layout/layout/atomic.go).

## 7. Intrinsic measurement remains separate

A text-only paragraph might have reusable intrinsic bounds. A paragraph
containing objects needs the objects' intrinsic contributions, including
applicable horizontal edges. Those contributions are not automatically known
when the outer paragraph is prepared.

Preparation should therefore produce **material that can be measured**, without
requiring every paragraph to have final min/max-content widths already.

`khipu` already reflects this separation. `MeasureIntrinsic` operates on prepared
material and accepts atomic contribution ranges without binding final object
geometry or breaking lines. See its
[measurement and binding contracts](../../khipu/doc/PREPARED_PARAGRAPHS.md).

## 8. What must remain deferred

Ordinary inline-box edges can contribute to line width. Some contributions may
depend on containing geometry. Supporting them generally requires preserving
inline boundaries and binding resolved contributions later. Extend the prepared
representation where necessary rather than requiring used dimensions before
preparation. The current `tytui` profile rejects nonzero ordinary-inline edges;
that integration has not yet solved the general case.

Not every text operation can be completed independently of line selection.
Line-edge whitespace treatment, justification and some glyph decisions interact
with wrapping. Preparation must retain reusable base material and information
needed for deferred operations. CSS Text's processing model explicitly includes
these interactions. [CSS Text processing order](https://www.w3.org/TR/css-text-3/#order).

`khipu` provides a useful model: base preparation is reusable, while hyphenation
alternatives can be discovered and shaped lazily. The guarantee should be reuse
of stable preparation across trials, rather than an unconditional promise that
no shaping service can ever run again. See the
[paragraph preparation contract](../../typography/glyphing/paragraph/prepared.go).

## 9. Ownership and lifetime rules

Proposed rules for the new phase:

- A successful pass publishes a complete prepared set for the requested roots.
  Failure publishes no partially ready set. Internal completed caches need not
  be erased merely because a later operation fails.
- Prepared material contains no mutable current width, selected page or accepted
  register cursor.
- Every layout trial returns independent results. Rejecting it leaves preparation
  usable and does not consume any channel material.
- Continuations identify their preparation and accepted prefix. Width changes
  reuse preparation; changes to text, shaping styles, fonts or preparation policy
  require new preparation.
- Memoized discoveries may be shared, but concurrency is a separate contract.
  Current typography preparations require serialized use.
- Trial requests must not leave borrowed used-value or atomic tables inside the
  prepared handle.
- Source/style/font services remain stable for the lifetimes in which they are
  borrowed. Accepted output must retain its rendering dependencies without
  requiring the entire mutable trial state.

These rules make preparation an architectural boundary rather than a cache
optimization that callers must understand.

## 10. First contract experiment and remaining decisions

Prepare an outer paragraph and a nested auto-width inline-block, then lay them
out at two receiving widths. Both groups should be prepared before any
measurement or layout callback. Both candidates should coexist; rejecting either
should change nothing in the preparations. Base text should not be reshaped for
the second width, while explicitly deferred alternatives remain permitted.

This experiment would establish the phase and its lifetime before adding the
harder fragmentation contract. The direct geometry-only path for empty template
frames remains separate: it has no horizontal content to prepare.

Before implementation, refine:

- The prepared-handle interface and compatibility with the existing callbacks.
- The representation and ownership of retained paragraph trial results.
- Driver scope, tree/group identities and failure publication behavior.
- Deferred inline contributions and the supported initial text/box profile.
- The later `FitInline` contract for partial results and continuation.

These are design questions. This note does not implement APIs or authorize
changes to sibling repositories.

## Diagram sources

Editable Graphviz `.dot` files and their rendered `.svg` previews live in
`doc/diagrams/`. From the repository root, regenerate the previews with:

```sh
dot -Tsvg doc/diagrams/preparation-pipeline.dot -o doc/diagrams/preparation-pipeline.svg
dot -Tsvg doc/diagrams/preparation-interactions.dot -o doc/diagrams/preparation-interactions.svg
```

The diagrams illustrate the proposal, not implemented public APIs.
