# Template-based page layout: revised design discussion

Date: 2026-11-22. Status: design discussion; TPL-01/02 source loading and isolated styling implemented.

---
## REMARKS

We will change the fundamental decision on what's flow material is. According to your
proposal the flow material is essentially DOM fragments, which are then put into an
empty page (with frames) and then layed out. This is not what we are going to do.
Flow material will already be boxed, but their dimensions not yet frozen. Styling is
exclusively dictated by the location of the flow material's ancestor in the DOM. The
region/frame onyl influences layout, i.e., sizing.

W3C: The region influences layout without replacing the original styling ancestry.

### Reading in the Source and Building Trees

We find 2 documents:

  1. HTML/CSS for page templates: Templ-Doc
  2. HTML/CSS for the content: Cont-Doc

We will read in all page templates (Templ-Doc) at once. For each page template, we will perform
the boxing step, including sizing. The result is (conceptually) a sequence of render-trees,
each one identified by an ID.

The parser of Cont-Doc produces a box-tree. It then identfies all the branches which denote
flow material. This flow-material is put into a register each (per flow-"channel"). 
The flow material is not yet sized, but "prepared" (see below).
At the end of this phase, conceptually there are

  1. a box-tree of the main content
  2. several registers filled with flow material, which has been split from the content
  3. still a sequence of page render-trees

I am unsure about the main content. Possibly we have to have a "default flow channel" where
it flows into and each page template needs to provide a target for it. This is to be 
discussed.

### Preparation of Flow-Material

Flow-material will go through a non-standard phase, which we will call "preparation" for
lack of a better term right now. The process will traverse the box-tree top to bottom,
like for layout/sizing. Style properties will be in effect and known. As soon as the
traversal encounters horizontal material (e.g. a paragraph of text), the horizontal material
will be put to the shaper and something like a TeX `\hlist` will be produced. In our
world, this will be called a `khipu` (see sibling package `khipu`). A khipu is prepared
for line-breaking, and all the styling information affecting the sizing has been processed
(i.e., font selection, etc). However, atoms (inline-block) are not yet sized, and line-breaking
will only by possible later with the width information known. Applying the sizing
multiple times (for trials) will be much more efficient on prepared material than it
would be for a box-tree.

### Trial of a Template Page

A template page is tried. How and in which sequence templates are selected for trial is to
be discussed. Flow-material is read out from a register and attached as a branch of
the corresponding frame. Then the sizing algorithm will travel down from the attachment
point, providing width information for the prepared flow-material. For each top-level
element, the sizing will result in a height. This is repeated until the sum of the heights
of the tried out top-level boxes is equal or greater than the sum of heights for a region
(comprised of the heights of its frames). The rest of the flow-material stays in the
register. The typesetter tries to find suitable breakpoints and split points for each 
filled frame. Continuations are put into the following frame. This way, all frames of
a region get filled. Unused Flow-material will be prepended back in the register again.

(End of Remarks)

---

## Revised direction

The remarks above define the new process. This discussion replaces the previous
linear outline; it is still a design proposal, not implementation authorization.
The source remarks are preserved verbatim. Template examples are in
[SKETCH.md](SKETCH.md); background is in [TEMPLATES.md](TEMPLATES.md) and
[REGIONS.md](REGIONS.md).

**Design freedom across sibling modules:** `css-box-layout`, `w3c-dom`, `khipu`,
`typography` and the other sibling typesetting packages are under our control.
Their designs and APIs will be influenced and shaped by the needs of page layout.
None of their existing restrictions should prevent page-layout ideation. Design
the desired process first, then identify appropriate changes and responsibilities
across the modules. References below to current limitations describe integration
work to be addressed, not fixed boundaries of the intended design. Distinguish
those facts from deliberate page-layout policies and proposed implementation
stages; do not turn a temporary module limitation into a product requirement.
The [upstream simplification review](#opportunities-to-simplify-through-sibling-changes)
below identifies concrete consequences of this principle.

**The layout engine tries already-sized page templates against registers of
prepared, unsized box material.** A trial supplies containing geometry, resolves
used dimensions, breaks paragraphs, and chooses fragments. Choosing a page
commits its consumption of the registers. Rejecting a trial consumes nothing.

The earlier outline already kept source styling separate and proposed fixed
frame geometry independent of content. Those parts survive. It did not make
boxed content, preparation, registers, or competing page trials the organizing
concepts. Those are now central:

| Earlier proposal | Revised treatment |
| --- | --- |
| Style and compile a template when selected | Compile **all** templates before page trials; selection chooses a compiled page tree by ID. |
| Supply separate HTML content to a later flow stage | Style Cont-Doc, build source-associated boxes, extract flow branches, and prepare their horizontal material before trials. |
| A compiled page chiefly described rectangles and bindings | Retain a reusable page/frame tree with geometry, paint and bindings; rectangles are part of that result. |
| Flow assignment and continuation largely left open | Registers, trial snapshots, per-frame fitting and atomic page commit are explicit proposed contracts. |
| Empty-frame geometry resolver as the main integration concern | Keep it as one service used by the template compiler; it does not size or paginate content. |
| A general future content-flow call | Separate reusable preparation, trial-specific sizing/breaking, fragment selection, and accepted output. |

### Direction established by the remarks and earlier decisions

- Templ-Doc and Cont-Doc are separate HTML/CSS inputs. Templ-Doc contains
  `<template id="..." data-type="page">` definitions with an inner page DIV and
  direct fixed-position frame DIVs. All template trees are boxed and sized at
  loading time. Content box dimensions are not frozen at that point.
- Source DOM ancestry exclusively determines content styling. The frame changes
  layout constraints, not selector matching, inheritance, fonts, or paint styles.
  Attaching material during a trial is a **layout attachment**, not DOM reparenting.
- CSS `flow-into` assigns source branches to channels; `flow-from` binds template
  frames to channels. Frames for one channel are visited in template document
  order. Content does not duplicate the template's frame elements.
- Preparation traverses flow box material and turns horizontal groups into
  reusable khipus. It resolves source-dependent text preparation while deferring
  line breaking and the used geometry of inline atoms until constraints are known.
- Page trials size and fragment material into frames, retaining unplaced material
  and continuations for subsequent frames and pages.
- `w3c-dom`, `css-box-layout`, typography and `khipu` remain collaborators. We can
  propose changes to their APIs, but this document does not implement them.

**Terminology proposal:** use *frame* for one physical rectangle, *flow channel*
for a named content stream/register, and *region chain* for the ordered frames
receiving that channel on a page. The remarks use “region” for a collection of
frames, whereas earlier documents used it as a synonym for one frame. These
explicit terms avoid deciding a new public vocabulary prematurely.

## Proposed process and ownership

![Two input pipelines feeding page trials and accepted pages](diagrams/design-pipeline.svg)

[Graphviz DOT source](diagrams/design-pipeline.dot)

There are two loading pipelines followed by a repeated trial/commit cycle. The
pipelines can be developed independently; no parallel execution is implied.
The proposed conceptual objects are:

| Object | Contents and ownership |
| --- | --- |
| Template catalog | Ordered, ID-indexed immutable page/frame trees, resolved geometry, paint, and channel bindings. Shared by all trials. |
| Source content model | Original styling context, source identities and box structure. Retained wherever property lookup or borrowed text/font services require it. |
| Prepared box material | Unsized box structure, computed style inputs, prepared horizontal groups, and stable atomic occurrence IDs. Shared across trials; no accepted line breaks or page positions stored here. |
| Channel register | Ordered references to owned flow branches and a committed cursor. A cursor may include an unfinished block/paragraph continuation. |
| Page trial | A template instance, private cursor snapshots, temporary sizing/atomic results, candidate breaks and placements. It never mutates catalog geometry or shared box styles. |
| Accepted page | Detached placed fragments and their frame/page coordinates, paint and source references; paired with the committed register advances. |

“Immutable” here is a publication/ownership rule. Some upstream services retain
memoized discovery state or borrowed fonts. A discarded trial may warm such a
cache, but may not alter source meaning, accepted geometry, or register positions.
Serialize calls sharing a paragraph preparation until stronger upstream
concurrency contracts exist.

### 1. Compile the entire template collection

See [TEMPLATE-LOADING.md](TEMPLATE-LOADING.md) for the detailed package/API
proposal, loading stages, diagnostics, and PR plan across `pagelayout`,
`w3c-dom` and `css-box-layout`.

Template loading is encapsulated in `pagelayout/template`, including its source
model and the later styling, validation and compilation stages. The root package
will consume compiled catalogs for page composition; the template package does
not depend on the root package.

Parse Templ-Doc once, index unique nonempty template IDs, collect its stylesheets,
and compile every definition. Publish the catalog only when all definitions
pass the selected template profile. This replaces deferred compilation at page
selection. A source/style revision requires a new catalog; a trial never restyles
or resizes its template.

For each definition, clone the inner page/frame tree into a fresh styling
document, preserving source order and copied attributes. The proposed context is
`html > body > .page > .frame`, with the collection's `html`/`body` attributes.
The storage `<template>` is absent from that ancestry. Use correct HTML tag atoms
when constructing nodes programmatically. This happens once per definition,
not once per trial or output page.

The following is a route through the existing `w3c-dom` CSSOM API. A reusable
stylesheet program and isolated styling sessions, proposed below, could replace
the repeated setup without changing each template's styling context:

1. Extract accepted collection sheets in order with
   [`ExtractStyleElementsWithError`](../../w3c-dom/style/cssom/douceuradapter/douceuradapter.go).
   Proposed first policy: direct head styles and inline declarations, with external
   sheets, conditional rules and template-local sheets deferred. Scan accepted
   style elements explicitly; recursive whole-document extraction crosses stored
   template boundaries. Extraction failure rejects the catalog.
2. Create a fresh `cssom.NewCSSOM(nil)` per definition. Register each sheet with
   `AddStylesForScope(nil, sheet, cssom.Author)` and call `Style(activeDocument)`.
   Inline declarations are collected by the engine. Nil scope also styles the
   `html`/`body` context needed for inheritance. Do not register sheets twice.
3. Read inherited/defaulted properties through `css.GetProperty` and normalized
   display through `css.GetDisplay`. Keep the styled context alive until all
   required properties have been adapted. Local `Styles()` maps are not complete
   computed-style snapshots. `dom.NodeFromTreeNode` supplies the DOM wrapper if
   needed; raw HTML identities link styled nodes back to the cloned definitions.
4. Validate structure and CSS, construct the page/frame box description, resolve
   fixed geometry, then publish the compiled template tree.

The published result should preserve the meaningful box hierarchy even if the
geometry service needs only typed constraints. Conceptually it is the remarks'
page render-tree; it need not be exactly a `css-box-layout.LayoutNode` tree.
Keep page paint, frame border/content rectangles, channel names, source order and
identities alongside that structure. Normalized empty inline groups from a
fallback implementation are not part of its public meaning.

The preferred proposed `ResolveFixedGeometry` operation still fits this phase.
It sizes each fixed frame; it does **not** substitute for constructing the
template's box description. The existing `BuildLayoutTrees` / `FixedLayout`
route remains a fallback. Their contracts and the proposed CSS profile are
summarized below.

### 2. Style, box and route the content document

Parse and style **all of Cont-Doc in its original context**, then construct
source-associated box material with computed style inputs but no final widths,
heights, line breaks or page positions. “Parsing produces a box tree” describes
this pipeline, not a capability of the HTML parser alone. `w3c-dom` supplies the
styled tree; a content adapter and `css-box-layout` supply boxing.

Template CSS is not applied to Cont-Doc. Any shared stylesheet must explicitly
be an input to each document. In the existing collection sketch, `.story` and
`.page-heading` source-flow rules should therefore accompany Cont-Doc; frame
geometry and `flow-from` rules belong to Templ-Doc. Neither trial attachment nor
the output tree triggers another CSS cascade.

Preserve two different relationships: **source ancestry for style** and
**layout containment for used-value resolution**. For example, a paragraph keeps
its Cont-Doc font and line-height while its percentage width is resolved in the
receiving layout context. Retain source context or detached effective style
inputs; do not inherit missing styles from a frame after routing. Unknown or
width-dependent typography needs an explicit preparation policy, not silent
restyling on a later trial.

Proposed extraction algorithm:

1. Identify the source box roots whose effective `flow-into` names a channel.
   Append their branches to that register in source document order. A branch has
   one flow owner; it must not also produce material in its former content parent.
2. Keep the residual main-content box tree with those branches excluded from its
   layout projection. Preserve ancestry and source IDs for style/diagnostics;
   no mutation of the source DOM is needed. Retain meaningful container boxes
   and their edges rather than flattening all descendants into paragraphs.
3. Normalize box structure for each extracted branch and the residual tree,
   then prepare their horizontal groups. Routing must precede normalization that
   merges source siblings into anonymous groups or loses extraction boundaries.

This refines what “box-tree, then split branches” means at the API boundary:
create a source-associated box projection first, route its branches, then perform
final flow normalization. Today's `RenderNode` projection and builder offer a
starting point, but there is no existing named-flow extraction API. Prefer a
source-aware boxing hook if needed over trying to recover ownership from anonymous
boxes after the fact. Source identity must survive any normalization or rebuild.

**Initial-profile proposal:** extract non-nested block branches only. Reject
nested `flow-into` roots, inline source extraction, and unsupported out-of-flow
content with source diagnostics. These are tractability limits, not a claim
about the Regions model. In particular, no text or branch may disappear merely
because it has no selected receiver yet.

### 3. Decide how residual main content participates

**Recommendation, still open:** route the residual main-content tree to a reserved
default channel, instead of maintaining a second page-layout algorithm beside
the named registers. At the end of loading, the main tree still exists, but its
layout ownership belongs to that channel. Named-flow branches have already been
excluded, so they are not duplicated there.

The reserved channel's public name or configuration mechanism is undecided; do
not invent a CSS keyword yet. A template opts into receiving it through an
explicit binding. For ordinary body pages, require a receiver whenever default
material remains. A cover or a page serving only another channel could be an
explicit policy exception. Thus every template need not always have a default
frame, but the scheduler must not endlessly choose pages incapable of advancing
pending content.

An alternative first mode is to require all content to be explicitly named and
reject nonempty residual material. Silently dropping residual content is not an
option. Keep this choice configurable or unsettled until a concrete document
example establishes the desired authoring convention.

An empty register leaves its frames empty; a nonempty register with no receiver
on one candidate remains unchanged. Before pagination, diagnose channels that
have no receiver anywhere in the permitted catalog. Later selection must also
detect starvation or lack of progress among otherwise receivable channels.

### 4. Prepare boxes without fixing their dimensions

See [PREPARATION.md](PREPARATION.md) for the detailed proposal: where preparation
sits between CSS box-layout phases, a sibling typography callback returning
reusable material, nested atomic preparation, and ownership across trials.

Traverse each flow branch top down, carrying its already-established source
styles and formatting context. At each horizontal group, normalize/project text
as required, perform font selection/itemization, preserve source and bidi mapping,
then prepare reusable paragraph material. Preparation is an explicit loading
phase, not a side effect of the first frame's final-layout callback. In the
initial proposal all register material is prepared before trials; lazy loading
would be a later resource-policy change.

The prepared box tree retains its block and inline-box structure. A paragraph
node references its prepared khipu/paragraph handle plus line-height and paint
inputs. An inline-block contributes a stable atomic occurrence anchor to the
parent paragraph and retains its own prepared subtree. It is **not** sized during
parent preparation. Nested text can be prepared recursively without resolving
the atomic box's eventual width or height.

Unsized does not mean that all dimensions are unknown: an authored absolute
width and shaped glyph advances can already be known. It means used box geometry,
line selection and placement have not been fixed for a receiving frame.

Distinguish three kinds of result:

| Reusable preparation | Per-trial work | Accepted output |
| --- | --- | --- |
| Source styles, selected fonts, base shaping, bidi context, khipu material, atomic IDs, box constraints | Used dimensions, atomic advances and interiors, line-break decisions, vertical assembly, candidate splits | Placed frozen fragments, geometry and paint; live continuation only for unfinished material |

Fonts and shaping-affecting style must be stable across trials. Geometry-dependent
typographic choices cannot be hidden inside this cache; initially reject them
or require resolution against the source context before preparation. Width-dependent
box lengths remain unresolved and are expected. Hyphenation alternatives may be
discovered and shaped lazily by existing services, so “prepare once” means reuse
base material, not a promise of zero subsequent shaping calls for every policy.

### 5. Try a page from register snapshots

The page scheduler chooses candidate template IDs. **Initial policy proposal:**
the caller supplies a finite ordered candidate list for the current document
position; try those from the same committed register state. Start with first
feasible selection and explicit required-channel/progress rules. Later scoring
may consider unused space, breaks and template preference. No scoring formula,
global optimum or facing-page sequence is agreed yet.

For a candidate, instantiate a lightweight page result from the compiled tree.
Open a trial snapshot covering all channel cursors. Conceptually attach branches
read from each register under the corresponding frame. Implement that attachment
as a trial-owned reference or wrapper carrying the containing rectangle; do not
reparent shared boxes, modify catalog nodes, or rerun CSS.

For each channel's region chain, visit frames in source order:

1. Supply the current frame's content width and remaining height. Resume the
   register's leading unfinished branch, or start its next branch. Resolve box
   used values downward from this layout attachment; retain internal parent-child
   containing relationships and source style inputs.
2. When a horizontal group is reached, size its atomic children under the actual
   containing constraints, recursively laying out their prepared interiors.
   Bind their horizontal advances to the prepared paragraph and supply their
   heights, margins and alignment to vertical assembly. Keep each atomic interior
   paired with the trial that produced its metrics.
3. Break the prepared paragraph at the available inline width, assemble enough
   vertical geometry to evaluate fitting, and expose legal line boundaries.
   Whole blocks provide block-boundary candidates; splittable containers need
   continuation state for their children and edges.
4. Accept a fitting prefix provisionally in the trial. A paragraph may split at
   an allowed line boundary; an indivisible box moves intact to a later frame.
   Continue with the next block if space remains, or carry the unfinished branch
   into the next frame. Changed width means a new suffix layout, not clipping
   lines previously broken at another width.
5. At the end of the chain, retain the exact unplaced tail followed by untouched
   register entries. Record proposed cursor advances and diagnostics; do not yet
   change the committed registers.

The sizing walk supplies width downward and obtains height upward, as in the
remarks. However, **the sum of frame heights is only a look-ahead heuristic**.
Different widths produce different line counts; even equal-width frames have
individual fit limits, unbreakable items, margins and legal breakpoints. Two
100-unit-high frames cannot contain a single indivisible 150-unit-high box just
because their total height is 200. Evaluate actual frames to certify a fit.

Similarly, summing independent block heights is insufficient when margin collapse,
container padding/borders or break rules apply. A fragment fitter must account
for these effects at each boundary. Propose initially using simple block stacks
with zero inter-block margins/edges, ordinary paragraphs and indivisible inline
atoms. General nested-block splitting, decoration at fragment boundaries,
widows/orphans and keep-with-next remain explicit extensions, not silently ignored
CSS. Existing ordinary box layout still provides sizing; it is not itself a
general fragmenting block-layout service.

Use bounded look-ahead and retain candidate break records if a later break choice
can move provisional material back. Stopping on the first legal greedy fit is
a workable first strategy. Choosing a breakpoint must preserve the exact source
suffix; rebreaking later must not lose bidi context, duplicate lines, or alter
already committed prefixes.

A branch continuation should identify its source tree and open-container path,
the next child or paragraph position, accepted line checkpoint, and any pending
boundary state. Keeping only a height or a DOM offset cannot resume a partially
filled container. Reuse prepared text across widths, but reuse measured heights
or atomic interiors only when their constraints and style/font inputs match.

### 6. Commit one trial, or consume nothing

![Trial-local register cursors and frame continuation](diagrams/design-flows.svg)

[Graphviz DOT source](diagrams/design-flows.dot)

**Proposal:** registers behave transactionally. A trial reads through a private
cursor; its result includes candidate placements and a replacement cursor for
every channel it would advance. The scheduler commits one complete page and all
of those cursor updates together. A rejected candidate or any sizing/assembly
failure leaves every committed register unchanged. Do not consume a heading in
one register merely because a body-flow trial later failed.

This implements the remarks' “prepend unused material back” without destructively
popping and reconstructing shared queues. Logically, if a trial reads `[A, B, C]`
and places all of A and a prefix of B, committing leaves
`[continuation-of-B, C, ...]`; rejecting leaves `[A, B, C, ...]`. Read-ahead after
B is never accidentally consumed. A continuation references prepared source
material and accepted decisions, not merely the height or a frozen diagnostic
remainder.

Competing page trials start from the same committed checkpoints. Within a trial,
advancing to another frame may create new checkpoints, but those become durable
only on page commit. Previously committed lines and consumed atomic geometry
remain locked. Unsized suffix atoms can receive new geometry for a new width;
changes must not retroactively alter consumed atoms or their interiors.

Return explicit outcomes: usable candidate (possibly with remaining material),
no progress, unsupported input, or operational error. If no permitted candidate
can place a pending indivisible item, report its identity and required/available
geometry rather than generating empty pages indefinitely. Allow deliberate blank
pages only through a bounded scheduler rule. Completion means all participating
registers are exhausted, not just the default channel.

Retain preparations, discovery state and checkpoints while their material may
still be retried or continued. Release completed/abandoned working material once
no live candidate needs it. Accepted pages retain frozen fragments and stable
rendering dependencies, not the entire mutable trial state. A repeated heading
requires an explicit copy/repetition policy; a consumable heading channel alone
does not repeat it.

## Existing services and proposed integration work

These observations come from the current sibling sources; they are not proof
that the full new pipeline is already implemented or that all checkouts build
together without integration work. The limitations listed here are changeable
under the design-freedom principle above.

![Package responsibilities for preparation and page trials](diagrams/design-packages.svg)

[Graphviz DOT source](diagrams/design-packages.dot)

| Package | Existing capability | Work still proposed here |
| --- | --- | --- |
| [`w3c-dom`](../../w3c-dom/doc/architecture.md) | Styled HTML trees, CSSOM registration, defaulted property lookup; property maps can depend on ancestors. | Template collection policy, separate source/style contexts, strict profile diagnostics and routing metadata. |
| [`css-box-layout`](../../css-box-layout/layout/README.md) | Unsized normalized boxes via builders; used-value resolution, ordinary layout and fixed layout. Typed styles and structural nodes are separate from output geometry. | Source-aware flow extraction integration, a geometry-only frame resolver, and a resumable block-fragment sizing boundary. |
| [`khipu`](../../khipu/doc/PREPARED_PARAGRAPHS.md) | `PrepareParagraph` yields immutable `Khipu` plus `Discovery`; `BindParagraph` supplies atomic advances; immutable `Tikray` trials, `Checkpoint`, first-fit continuation and `Freeze`. | No page scheduler, channel register or vertical box fragmentation is supplied by these operations. |
| [`typography/glyphing/paragraph`](../../typography/glyphing/paragraph/README.md) | `Prepare`, independent `Prepared.Trial` / `Trial.Assemble`, checkpoints, and `StartFlow` / `Continuation.Fit`; source styles and bidi context remain stable. | Compose these paragraph operations with block continuation and trial-specific atomic interiors across a page/register transaction. |
| [`tytui/documentlayout`](../../tytui/documentlayout/preparation.go) | Source extraction, font itemization and invocation-local prepared groups demonstrate the adapters. `LayoutFlow` serves a finite frame list for one paragraph group. | Reuse appropriate contracts; its invocation lifetime and bounded flow result are not a persistent register implementation. |

`khipu` already distinguishes an unbound atomic occurrence from a bound zero
advance. Widths belong to bindings; vertical dimensions and object payload stay
with the client. This directly supports preparing the parent paragraph before
sizing an inline-block. Preserve occurrence identity and source coordinates
through every trial.

Current paragraph continuation is bounded: `NoHyphenation`, whole-line prefix
fitting, and first-fit suffix search after a width change. `Continuation.Fit`
does not implement widows/orphans or general height optimization. Its existing
`StartFlow` path fixes other layout inputs, including atomic boxes. For atoms
whose geometry changes between frames, use the lower-level independent trial /
checkpoint design or extend the integration; do not assume that convenience
path already supports rebinding them. The
[`TrialCheckpoint`](../../typography/glyphing/paragraph/trial_checkpoint.go)
checks consumed atomic geometry and line-height inputs; the caller must also
preserve the matching atomic interiors.

The missing major service is a **resumable box-fragment fit operation**. Its
conceptual input is a prepared branch, a continuation, a containing width and
height budget, and break policy. Its output is placed fragments, candidate/legal
breaks, updated continuation and progress/overflow diagnostics. No exact Go
signature is proposed yet. `css-box-layout` should own box sizing and structural
fragment rules; typography/khipu own paragraph search and assembly; `pagelayout`
owns frame traversal, channel ordering and page selection/commit. First implement
only the simple block-stack profile rather than claiming a complete CSS paginator.

Keep the box core independent of khipu and DOM types by supplying paragraph
measurement/fitting services through an adapter. That adapter associates box
group identities with prepared paragraph handles. Today's `InlineLayouter`
returns a complete group's extent and atomic placements, not a partial block
continuation; the fragmentation boundary would be a new contract, not an
unannounced change to that callback's meaning.

## Opportunities to simplify through sibling changes

Review dated 2026-11-21. These are **recommendations for refinement**, not agreed
API changes or implemented capabilities. The two loading pipelines and page
trial/commit model remain useful. The main simplification is to move layout
mechanics into reusable sibling services, leaving `pagelayout` to coordinate
templates, channels and page choices.

### 1. Fit a prepared box sequence through one resumable operation

The largest opportunity is in `css-box-layout`. The process above describes a
recursive sizing walk, child positions, paragraph checkpoints, container edges
and pending margins. If `pagelayout` implements those details, it becomes a second
box-layout engine even when it calls existing sizing helpers.

Extend the proposed branch fitter to accept an **ordered sequence of prepared
box roots**, with an immutable, opaque continuation. A sequence is generic layout
input; the core need not know its channel name. It should preserve meaningful
containers and account for boundaries between roots as well as within them.
Fitting independent roots and adding their heights would leave inter-root margin
and break behavior in the page scheduler.

Conceptually, the new operation would be:

```text
fit(prepared sequence, cursor, receiving constraints, break policy)
    -> placed fragments, next cursor, fit status and diagnostics
```

The core owns open-container paths, pending edges, child positions and nested
paragraph continuation tokens. `pagelayout` retains a prepared sequence and a
committed cursor per channel; a trial replaces private cursors as it traverses
frames. The old cursor remains usable after every success or failure. Local
fragment coordinates become page coordinates only when composing the result.

This refines the current proposed branch boundary; it is **not** supplied by
[`FlowLayout`](../../css-box-layout/layout/layout.go) or
[`InlineLayouter`](../../css-box-layout/layout/interfaces.go).
Add a separate fragment-fitting service rather than changing the whole-group
callback's meaning. Paragraph fitting remains a supplied service, so the box core
does not acquire khipu types or page-selection policy. The new contract must
define progress, exhaustion, oversized material and unsupported input explicitly.
It simplifies the client, but still requires real fragmentation work upstream.

### 2. Unify paragraph fitting around existing prepared trials

The desired building blocks largely exist in `khipu` and
`typography/glyphing/paragraph`: immutable preparation, atomic binding, independent
trials and accepted-prefix checkpoints. Reuse them; another page-layout-specific
prepared paragraph representation would add synchronization and ownership work.

The useful upstream addition is a typography fitting operation that accepts a
prepared paragraph, an optional continuation, receiving dimensions, break policy
and **current atomic realizations**. It would perform binding, breaking, vertical
fitting and checkpoint creation together, returning a fragment and reusable
continuation. It could extend or complement `Continuation.Fit`; its exact API
should follow the box fitter's needs.

This removes the present choice between the convenient
[`StartFlow` path](../../typography/glyphing/paragraph/flow.go), whose atomic inputs
stay fixed, and manually composing
[`Trial` / `Assemble`](../../typography/glyphing/paragraph/trial.go) and
[`Checkpoint`](../../typography/glyphing/paragraph/trial_checkpoint.go).
Unconsumed atoms may be resized for a different frame width. Consumed geometry
must stay locked, and the adapter must retain the matching atomic interiors.
An immutable realization handle pairing metrics with interior output would make
that obligation easier to enforce than unrelated maps of dimensions and payloads.

Return trial-owned fragment handles with fit results. Today's inline callback
retains paragraph output in invocation-local storage keyed by group ID; one group
can now have several candidates and several placed fragments. A single mutable
"latest result" slot per group cannot represent that. The box core can carry
opaque handles while the typography adapter owns their rendering data.

Supporting hyphenated continuation or alternative suffix breakers would require
corresponding `khipu` work. Those are capabilities to develop, not reasons to make
`NoHyphenation` or first-fit a permanent page-layout restriction. Preserve the
accepted-prefix guarantee whichever breaker is selected.

### 3. Give styling a reusable program and detached effective inputs

Two related `w3c-dom` changes would simplify template compilation and content
preparation:

- **Separate reusable CSS from a styling session.** Compile ordered stylesheets
  and selectors once, then style each active template tree in an isolated session.
  Currently [`CSSOM.Style`](../../w3c-dom/style/cssom/cssom.go) registers inline
  declarations against HTML nodes in the rules tree, and selector caches belong
  to that tree. An explicit separation would allow shared compiled rules without
  retaining another template's inline declarations or node identities. Selector
  matching still runs in each template's own context; combining all active pages
  into one DOM would change sibling and structural selector behavior.
- **Expose an immutable effective-style result with diagnostics.** Capture
  inherited/defaulted values and source-dependent font context once. Preserve
  layout-dependent lengths as typed unresolved inputs for later used-value
  resolution. A copied local property map, or copied relative font-size text, is
  insufficient. Include source/property provenance and diagnostics for ignored
  or unsupported syntax; selector failures currently can be logged and skipped.

These changes replace repeated CSSOM registration, scattered property lookups
and a second page-layout CSS validation/parser layer. They could also remove the
need to retain styling ancestors *solely* for property lookup; source references,
text and borrowed font services still need explicit lifetimes. A fully detached
DOM is not a prerequisite for the first implementation.

Keep template selection, accepted stylesheet locations and strictness policy in
`pagelayout`. `w3c-dom` should expose general styling and diagnostic facilities,
not a page-template loader. Shorthand expansion, property defaults and typed CSS
value parsing belong there too; add missing support there instead of permanently
requiring template authors to spell out library-specific longhand workarounds.

### 4. Partition source-associated boxes inside the builder

Stage 2 currently asks the adapter to create a source-associated projection,
extract branches, preserve residual wrappers and then normalize each result.
That is a fragile place for two clients to implement the same tree surgery.

Extend `css-box-layout` construction with a generic extraction/partition hook or
annotations on projected source roots. `pagelayout` interprets `flow-into` and
supplies the partition assignments. The builder diverts assigned branches before
anonymous grouping, preserves source order and identities, and normalizes the
resulting roots and residual tree once. Its
[`fixed-root diversion`](../../css-box-layout/layout/build.go) already demonstrates
the useful ordering; named-flow extraction has different semantics and needs a
separate contract.

The result is channel-ready structural material without a second reconstruction
pass in `pagelayout`. Source effective styles must be captured before diversion:
[`RenderNode` defaults](../../css-box-layout/layout/render.go) can otherwise inherit
font size and white-space from the new build parent. Normalized IDs also need a
tree/preparation identity; a bare `BoxID` is not globally unique across builds.

This does not settle nested flow ownership, inline extraction or treatment of
empty residual containers. Those remain explicit semantics. Start with a clearly
declared profile and give unsupported partitions source diagnostics.

### 5. Keep direct fixed geometry, and share the integration adapter

The proposed `ResolveFixedGeometry` remains a worthwhile small addition. It
removes normalization scaffolding, synthetic empty-inline callbacks and fixed-root
identity matching from template compilation. Share its equations with ordinary
fixed layout; preserve the fallback below until the new operation exists.
Typed `box-sizing` and additional definite length forms could remove authoring
limits without making frame geometry depend on content.

There is also reusable integration work in
[`tytui/documentlayout`](../../tytui/documentlayout/preparation.go): style adaptation,
text/source extraction, font itemization, bidi mapping and paragraph preparation.
Factor the applicable parts into a reusable adapter used by both clients instead
of copying them into `pagelayout` or depending on a TUI package. Its location is
open; a typography integration package or a small bridge package are candidates.

The adapter should own mappings between box groups, prepared paragraphs and
atomic interiors, plus checked conversions between CSS pixels and typography
units. Preparation lifetime should be independent of a single layout invocation.
Core box layout still owns used dimensions; typography owns text metrics. This
needs a concrete second-client example before choosing a broad shared API, and
does not require a wholesale migration to one numeric geometry representation.

### 6. Avoid assembling unused paragraph output on every trial

This is an optimization opportunity after the fitting contracts are proven.
[`Trial.Assemble`](../../typography/glyphing/paragraph/trial.go) assembles the whole
paragraph; the current
[`changed-width continuation`](../../typography/glyphing/paragraph/flow_reflow.go)
freezes/assembles the complete path before selecting its tail. Competing page
templates can therefore repeat work for accepted prefixes and unused suffixes.

Typography could expose line metrics and legal breaks for fitting, then materialize
only the required fragment, reusing frozen accepted prefixes. `khipu` would need
a range-freezing operation if its present whole-plan freeze prevents that reuse.
The metrics and final fragment must come from the same captured plan and atomic
realizations: line height needs real font and object metrics, not an estimate
from advances. Paragraph-wide optimization may still inspect the entire suffix;
partial assembly does not imply bounded line-breaking work.

### Consequences for the proposal

The preferred direction for further discussion is **generic prepared sequences
and opaque fit continuations below the page scheduler**. Preparation stays eager,
source styles stay stable, and page commit remains an all-channel operation.
The scheduler need not know which container child or paragraph line a cursor
represents. This is the strongest reduction in page-layout-specific machinery.

Treat the restrictions in the earlier stages in three different ways:

| Kind | Examples | Treatment |
| --- | --- | --- |
| Established direction | Fixed template frames, original source styling, preparation before trials | Preserve through upstream changes. |
| Current capability or staging limit | No hyphenated continuation, fixed atomic inputs in the convenience flow path, zero-edge initial block stacks, missing CSS shorthand support | Record as implementation work or a temporary test profile; do not make it the desired final model. |
| Product/design choice | Default channel, nested extraction semantics, overlaps, repetition, template selection and break policy | Discuss explicitly; a more capable sibling API does not decide these. |

Refine the box-fit and paragraph-fit contracts together first, using one ordinary
paragraph, one nested box and one width-dependent atom as examples. Align source
partitioning and stable identities with that contract. The direct geometry service
and CSS diagnostics can proceed independently; reusable styling and shared adapter
extraction simplify loading. Optimize partial assembly once measurements show
where competing trials spend time. No sibling changes are made by this review.

## Template geometry contract retained from the earlier proposal

The changed process moves this work to eager catalog construction. It preserves
the principle that template geometry is independent of future content. TPL-03 now
implements the initial CSS validation and typed adaptation subset described in
[TEMPLATE-LOADING.md](TEMPLATE-LOADING.md#c-validate-source-declarations-and-effective-styles).
The geometry API and final rectangle policies below remain proposals:

| Area | Proposed restriction |
| --- | --- |
| Structure | One page DIV, direct fixed frame DIVs, initially empty receivers; indentation/comments are not content. |
| Page | Positive absolute dimensions, static block/flow box, zero margins/padding/effective borders; page-local origin. |
| Frames | Block/flow, fixed, content-box sizing, zero margins; positive content sizes determined by explicit dimensions or opposing-inset stretch. |
| Lengths | Start with `px`, `pt`, `in`, `cm`, `mm` and valid unitless zero; percentage/font-relative authoring deferred. |
| Insets | At least one definite inset per axis; auto size requires both. Reject content-dependent sizing and both-auto static-position fallback. |
| Edges and effects | Nonnegative absolute padding/effective border widths; initially none/hidden or solid borders. Explicit policies required for transforms, min/max sizing, clipping and unsupported effects. |
| Result checks | Finite positive receiving rectangles; propose rejecting out-of-page border boxes and positive-area overlaps. Touching edges are allowed; tolerance and bleed/overlay exceptions remain open. |

Source input validation complements effective-property checks: unsupported
declarations can otherwise be discarded by styling. DOM-02/03 now provide the
checked values, defaults and selected shorthand support, including the sketch's
`border: 0` and `background: #fff`. TPL-03 applies the source profile to all
declarations, then validates effective box roles, paint and content-independent
sizing prerequisites. Unsupported syntax is diagnosed rather than interpreted
as successful layout.

Adapt values to CSS pixels using checked conversion, e.g. the allowed absolute
units through [`dimen.ParseAbsolute`](../../w3c-dom/dimen/dimen.go), then divide
by `dimen.PX` before conversion to the core's float representation. Keep source
identities and useful diagnostics. Initialize `ComputedStyle` and `FixedStyle`
with their default constructors; their zero values mean explicit zero sizes or
insets rather than `auto`.

### Preferred service: proposed geometry-only API

There is **no public `ResolveFixedGeometry` API in the reviewed checkout**. The
preferred upstream addition remains this illustrative signature:

```go
// PROPOSED addition to css-box-layout/layout, not an existing operation.
func ResolveFixedGeometry(
    style ComputedStyle,
    viewport Rect,
    opts FixedLayoutOptions,
) (LayoutGeometry, error)
```

It would resolve one non-replaced fixed box without a DOM/layout tree, BoxID,
inline callback or intrinsic measurer. Return complete viewport-local border
and content rectangles, `RelativeTo == 0`, without mutating inputs. Accept only
constraints resolvable without content; distinguish a content-dependency error
from invalid values and unsupported static-position fallback. The intended typed
service can handle definite viewport percentages and `em` with supplied font size
even if our initial authoring profile is smaller.

Share the equations in [`fixed_sizing.go`](../../css-box-layout/layout/fixed_sizing.go)
and rectangle construction in
[`fixed_layout.go`](../../css-box-layout/layout/fixed_layout.go) with existing
`FixedLayout`. Merely exporting the current sizing helper would still require a
normalized tree and allow pending/content-dependent sizes. The refactor must
preserve the full operation's measurement, interior-layout and ownership behavior.
Typed `box-sizing` support would be a separate possible upstream improvement.

### Fallback using existing APIs

Project the page/frame boxes to typed `RenderNode`s and call `BuildLayoutTrees`.
Verify the fixed roots correspond exactly to the validated direct frames, in
source order; keep source `NodeID` and generated `BoxID` distinct. The normal
branch should contain only the page and empty normalization scaffolding.

Call `FixedLayout(trees.Fixed, viewport, emptyFrames, nil, options)`, with a
zero-origin page viewport. The checked `emptyFrames` inline adapter handles only
truly empty normalized groups, returning zero extent and no atomics. Explicit
or stretched geometry needs no intrinsic measurement. A nonempty group is a
projection error. This fallback is an integration alternative, **not** recovery
from a content-dependency error in the proposed direct resolver.

Both routes yield border-box `Frame` and receiving `Content` rectangles. Their
origins are already viewport-local: do not add a frame origin again to its
content origin. For a `300px` by `200px` viewport, right `20px`, top `10px`, content
size `80px` by `30px`, padding `3px` and border width `2px` yield border rectangle
`(190, 10, 90, 40)` and content rectangle `(195, 15, 80, 30)`. Add the external
page origin once when composing accepted output. Content descendants use their
own containing-box coordinates; make those transforms explicit.

Template geometry remains fixed when trial material is attached. Lay out the
content branches against the receiving rectangles; do not rerun fixed sizing
with attached material as a way to enlarge the frames. Paragraph APIs use
different dimension representations: convert CSS pixels to their DU/big-point
inputs explicitly (`1px = 0.75bp`), and convert results back at the adapter boundary.

## Open decisions

- **Residual content:** default channel or require explicit flow names? How is
  the default receiver declared, and which page types may omit it?
- **Routing:** what should nested flow declarations, inline extraction and
  non-rendering/empty source branches mean beyond the first restricted profile?
- **Prepared boxes:** which concrete tree representation and source-aware boxing
  hook should be shared, and where does the fragment continuation contract live?
- **Fragmentation:** container edges, margins at frame boundaries, kept blocks,
  widows/orphans, forced breaks, tables and floats need explicit supported policies.
- **Trials:** caller-driven candidate order versus scoring; look-ahead limits,
  page-to-page backtracking, first/facing-page rules, and required-channel progress.
  The initial proposal locks committed pages and does not backtrack across them.
- **Atomic content:** first supported sizing modes and alignment, intrinsic
  measurement, nested layouts, and compatibility of consumed versus suffix atoms.
- **Preparation lifetime:** eagerly preparing a large document has a memory cost;
  decide later whether staged preparation can preserve the same logical contracts.
- **Template profile:** CSS resource policy, geometry API types, numeric tolerances,
  decorative/overlapping frames, paint order, and repetition remain under discussion.

## Proposed first experiments, after design agreement

1. Compile two templates at load time, including chains with different frame
   widths. Verify source IDs, fixed geometry and bindings once; trials reuse them.
2. Style and box Cont-Doc with two non-nested named branches and residual main
   content. Verify channel order, exclusive ownership, and unchanged source styles.
   Exercise the proposed default-channel mode without silently dropping material.
3. Prepare several paragraphs and an inline-block with its own text. Confirm the
   parent has an unresolved atomic occurrence and nested text is reusable. For the
   simplest continuation test, use text-only `NoHyphenation` material first.
4. Trial both templates from identical register snapshots. Change frame fonts or
   paint styles and verify content styles remain unchanged. Change frame width
   and verify line breaking and trial atomic geometry can change while base
   preparation and the rejected candidate's registers remain intact.
5. Reject a candidate that advanced two channels provisionally; verify neither
   was consumed. Commit another, resume a paragraph at a different width on the
   next page, and check for missing/duplicated source ranges and altered prefixes.
6. Test the indivisible 150-unit box against two 100-unit frames, a missing
   receiver, an exhausted channel, and a non-fitting line/atom. Each must produce
   the intended explicit remainder/no-progress outcome rather than an endless
   page loop. Verify unit conversions and page/frame coordinate composition.

For upstream work, compare geometry-only results to `FixedLayout` and retain its
existing regression behavior. For preparation/trials, reuse existing paragraph
and khipu ownership/checkpoint tests as contract references. None of these planned
experiments has been implemented by this documentation revision.

## Diagram sources

Editable `.dot` files and rendered `.svg` previews live in `doc/diagrams/`.
Regenerate a changed preview from the repository root, for example:

```sh
dot -Tsvg doc/diagrams/design-pipeline.dot -o doc/diagrams/design-pipeline.svg
```

Use the same command with `design-packages` or `design-flows` for the other
diagrams. They illustrate the proposal and do not establish implemented APIs.
