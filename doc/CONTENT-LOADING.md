# Content loading and routing implementation plan

Date: 2026-11-22. Status: proposal for chunk 1; not implemented or approved API.

This plan develops the content-input phase of [DESIGN.md](DESIGN.md), following
the completed [template-loading milestone](TEMPLATE-LOADING.md). It ends with
source-associated, normalized, unsized box material partitioned into named
channels and residual content. [PREPARATION.md](PREPARATION.md) starts from those
groups and establishes reusable typography. Page trials come afterward.

The recommendation is to reuse the architecture demonstrated by sibling
`tytui/htmlinput` and `tytui/documentlayout`, while moving shared CSS semantics
to reusable sibling services and keeping channel ownership in `pagelayout`.
Do not make the application module `tytui` a production dependency or copy its
complete document pipeline into this module.

Use the existing sibling [`registers`](../../css-box-layout/registers/doc.go)
package for channel material storage. `Register` and `RegisterBank` already
provide persistent queues and bank versions; they are not future APIs to invent.
The source routing and concrete material payload contracts remain proposed.

All CONT, DOM-CONT and BOX-CONT items below are proposed work. References describe
the checked-out implementations, not a promise that the whole proposed process
already exists. This plan authorizes no sibling edits by itself.

## 1. Deliverable and boundaries

Load a separate HTML/CSS content document and publish one read-only material
snapshot containing:

- Source node identities, ancestry, decoded text and diagnostic locators.
- Effective source typography, box constraints and paint, independent of frames.
- Ordered named channels, each owning an ordered sequence of extracted branches.
- An initial `registers.RegisterBank` containing read-only branch material for
  those channels, built only after all branches normalize successfully.
- A residual source-associated tree, explicitly separate from named channels.
- Normalized trees and an index of their actual horizontal groups, including
  groups inside inline-blocks.
- Provenance connecting trees, boxes, groups and text ranges to source nodes.

Loading either succeeds completely or returns diagnostics with no partial
material. Neither a template catalog nor receiver dimensions are loader inputs.
Unmatched channel names are retained; receiver availability is a later
composition concern. A collection reload or failed content load must not alter
an already published template or content snapshot.

Chunk 1 does not select fonts, shape text, prepare khipus, measure intrinsic
widths, resolve used values, break lines, fit frames, create register cursors,
commit pages or render output. It preserves the information those operations
will need. A computed font size is source information; a chosen font face and
the dimensions of an auto-width inline-block are not computed here.

The acceptance example has two channels (`article`, `notes`), two separate
branches feeding `article`, and retained residual content. An inline-block has
an independent nested horizontal group. This is a source/routing proof, not a
page-layout demonstration.

## 2. What tytui teaches us

### Existing contracts worth reusing

| Existing implementation | Evidence and lesson | Treatment in this plan |
| --- | --- | --- |
| [htmlinput.ParseWithCSS](../../tytui/htmlinput/document.go) | Parses and styles the full document before selecting content; adds author CSS after embedded sheets; assigns document-local node locators. | Preserve original selector ancestry and stylesheet order. Use newer DOM inspection APIs rather than repeating its parser/validation code. |
| [htmlinput.ResolvedStyles](../../tytui/htmlinput/styles.go) | Resolves source typography through ancestors, including relative font sizes and HTML language/direction context, without font services. | Share these semantics through a checked DOM-side snapshot service; avoid a second independent resolver in pagelayout. |
| [documentlayout.Build](../../tytui/documentlayout/document.go) | Snapshots source tables, constructs typed `RenderNode` input, invokes the box builder, then indexes normalized groups and their owners. It retains no original `htmlinput.Document`. | Use this order with an additional partition stage between projection and normalization. |
| [documentlayout.Locator and NodeSource](../../tytui/documentlayout/document.go) | Source identity differs from normalized `BoxID`; one source element may generate multiple boxes after split/hoist. | Keep source identity, branch/tree identity and box identity distinct. |
| [documentlayout.Extract](../../tytui/documentlayout/extract.go) | Extracts an actual normalized group; stops at inline-block boundaries, emits an atomic occurrence and leaves nested groups independent. It maps processed bytes back to decoded DOM bytes. | Preserve enough source data and group ownership to reuse this model in preparation. Do not prepare individual DOM paragraphs now. |
| [documentlayout preparation](../../tytui/documentlayout/preparation.go) | Checks both group pointer and owner identity, and reuses extracted input and prepared paragraphs within one invocation. | Retain this identity discipline. Extending preparation lifetime belongs to chunk 2. |
| [documentlayout policy](../../tytui/documentlayout/policy.go) | Enforces an explicit supported subset and reports unsupported cases. | Adopt explicit failure, not its complete policy: pixel-only box parsing, hidden-content rejection and NoHyphenation are application choices. |
| [documentlayout lifetime tests](../../tytui/documentlayout/lifetime_test.go) | Exercise detached source/results and later reuse. | Port relevant contract scenarios, not whole implementation files. |
| [css-box-layout registers](../../css-box-layout/registers/doc.go) | Persistent queues and named banks with Append, First, Take and Prepend; whole-item consumption preserves earlier versions. | Use for channel material; do not build a competing queue/cursor container. |

`tytui` is especially useful because it separates **source context** from
**normalized layout structure**. Its group context comes from the owning block,
not from the first text run. That matters for empty paragraphs, object-only
groups, inherited language and future line-height/strut calculation.

### Where direct reuse would be wrong

`documentlayout.Build` combines projection, normalization, group indexing and
extraction validation for one selected subtree. It has no named-channel
partition API. Calling it separately on arbitrary DOM elements after reparenting
would lose selector context; cutting its normalized tree afterward would be too
late to preserve extraction boundaries reliably.

Its source hash is based on HTML bytes. With added CSS, identical HTML can have
different styles and routing. A hash of HTML alone must not identify prepared
material or serve as a style-cache key here. Snapshot ownership is the lifetime
identity; a future persistent cache would include all relevant inputs and policy.

Its box adapter uses local `pixelLength` parsing even though checked
`w3c-dom` box-value APIs now exist. Reuse those checked APIs instead. Likewise,
`htmlinput`'s older stylesheet parsing/validation path is not the model for a new
diagnostics implementation: our template loader already uses
`InspectStylesheet` and `InspectInlineStyle` and retains losing declarations.

The existing [LayoutFlow integration](../../tytui/documentlayout/flow.go) is not
a register implementation: it serves a restricted single-group flow request
with invocation-local preparation. It does not supply arbitrary multi-branch
channels, whole-page transactions or general block fragmentation.

## 3. Ownership and package boundary

**Proposed package:** `github.com/npillmayer/pagelayout/content`, parallel to
`template`. Keep source, styling, projection, routing, normalization and
diagnostic working types private. Root `pagelayout` later coordinates the two
loaded products; neither loader imports the root or the other loader.

The intended dependency direction is:

```text
page composition
  ├─ template catalog
  └─ content material
       ├─ w3c-dom: parsing/style access and checked source values
       ├─ css-box-layout: typed constraints, normalization and text references
       └─ typography/style: shared value vocabulary, when adopted
```

Depending on typography style values does not require opening fonts or invoking
paragraph preparation. Inspect its module dependency cost before adoption; a
DOM-owned value snapshot mapped into typography at the preparation boundary is
an alternative. Avoid recreating typography's style semantics in a local type.

A small candidate reader API is:

```go
// PROPOSED names and signatures; settle in CONT-00.
func Load(r io.Reader, opts LoadOptions) (*Document, error)

type LoadOptions struct {
    SourceName  string
    Stylesheets []Stylesheet // Named, caller-supplied author CSS texts.
}

func (d *Document) Bank() registers.RegisterBank        // Persistent value, immutable payloads.
func (d *Document) Channels() []string                  // First occurrence order.
func (d *Document) Branches(channel string) []BranchInfo // Source order.
func (d *Document) Residual() TreeInfo                  // Separate from channels.
func (d *Document) Groups(tree TreeID) []GroupInfo       // Normalized group order.
```

`BranchInfo` identifies a routed root and its `TreeID`; `GroupInfo` identifies
its tree-local group and owner boxes and source paragraph context. These are
inspection values, not mutable box pointers. Avoid publishing all of the
internal graph merely to demonstrate routing. The exact source/text inspection
accessors should be driven by the acceptance example.

Keep normalized trees private initially. The later preparation bridge can live
in `content` and invoke a box-owned preparation driver without exposing writable
trees. If external adapters need direct access, decide on an explicitly borrowed
read-only tree view before adding it; returning `*LayoutNode` does not enforce
immutability. `tytui.Root()` is a useful contract reference, not automatic approval
of that public surface.

Start with the reader API. An explicit `LoadFS` convenience can follow; template
repository discovery and `find` do not automatically apply to content files.
The caller owns reader lifetime. Additional CSS should match the established
template-loader semantics without making `content` depend on `template` just to
share an options struct. A common stylesheet input type is a packaging decision,
not a prerequisite for a shared cascade implementation.

### Flow channels and existing registers

A **flow channel** is the logical named stream connected by `flow-into` and
`flow-from`. Its queued material lives in a **Register**, and a **RegisterBank**
holds those named registers. These terms describe different aspects of the same
mechanism, not competing designs.

[`Register`](../../css-box-layout/registers/register.go) supports persistent FIFO
Append/Take, First and front insertion with Prepend. The
[`bank`](../../css-box-layout/registers/bank.go) derives a new version on update,
sharing unchanged queues and payloads. A trial can retain its starting bank,
derive a candidate bank, and publish the chosen candidate only after success.
That checkpoint mechanism already exists; no additional whole-item cursor layer
is required just to advance or reject a trial.

The current payload interface is `FlowMaterial { GetBoxID() layout.BoxID }`.
**Author clarification (2026-11-22): this definition is provisional and has no
other client dependencies.** Its purpose was to leave the concrete item type
open; the working expectation is now that register items will always be boxes.
Do not treat GetBoxID or interface-based storage as a compatibility requirement.
Sibling changes, including changes to the register types, are allowed and
encouraged when useful to the page-layout design.

Choose the box/material representation before introducing an adapter solely to
satisfy FlowMaterial. Evaluate direct box storage or a box-oriented handle that
retains the source/material owner, tree identity and future preparation/result
associations. A raw mutable LayoutNode pointer is not automatically the right
answer: persistent bank versions still need safe shared ownership, and BoxIDs
remain tree-scoped. The current bank neither deduplicates nor interprets box IDs.

Append box branches under `RegisterID(flowName)` in source order after
normalization. Chunk 1's items are unset box material, not shaped paragraphs.
Preparation later supplies reusable associations or a new bank of prepared box
handles without mutating an earlier bank's items. The exact payload API and any
necessary sibling changes are part of CONT-00, not fixed by this plan.

The bank has no channel enumeration/order API, and removes exhausted registers.
Retain a separate channel/source inventory for deterministic inspection; it is
metadata, not another queue implementation. The bank accepts every string ID and
nil payloads, but the content adapter should publish only validated flow names and
non-nil branch items. Keep residual content separate until its binding policy is
agreed; the bank's acceptance of an empty ID does not settle that policy.

Later fragmentation can Take a whole item and Prepend a new immutable remainder
to the candidate bank. The bank does not measure or split material, validate
continuation identity, or make mutable payloads safe. Queue versions can be
derived concurrently, but typography preparation may still require serialized
operations. Publishing a shared current bank remains the coordinator's job.
Tests/examples of these existing mechanics are in
[`registers/example_test.go`](../../css-box-layout/registers/example_test.go).

Repeated Append copies each remaining queue, so bulk loading a large channel has
quadratic copying cost. Start with the existing API for the small milestone;
consider a bulk constructor only if realistic collection sizes justify it.

## 4. First executable input

The principal fixture should resemble this complete content document:

```html
<!doctype html>
<html lang="en">
<head>
  <style>
    body { font-size:16px; color:#203040; }
    .source { font-size:20px; }
    .source > .story { flow-into:article; }
    .source > .note { flow-into:notes; }
    .story + .note { color:#884400; }
    .story > p { margin:0; }
    .story .tag { display:inline-block; width:auto; }
  </style>
</head>
<body>
  <p>Residual introduction.</p>
  <section class="source">
    <article class="story">
      <p>First article with an <span class="tag">atomic interior</span>.</p>
    </article>
    <aside class="note">A source-styled note.</aside>
    <article class="story"><p>Second article.</p></article>
    <p>Residual section ending.</p>
  </section>
</body>
</html>
```

Expected result:

| Destination | Owned material |
| --- | --- |
| `article` | First article branch, then second article branch; not their common section ancestor. |
| `notes` | The aside branch, still with the color selected by its original preceding sibling. |
| Residual | Body and section structure containing the introduction and section ending, with the routed branches excluded. |

The article and note retain the 20px source font context. The sibling selector
is evaluated before partitioning. The inline-block's interior is a separate
normalized group, without a chosen width or height. Channel order is `article`,
then `notes`; branch IDs are assigned by root source order independently of
channel grouping. The residual tree has its own identity.

Add a second fixture with unadorned inline text around an extracted block inside
a block container. After extraction the residual inline runs may normalize into
one group. This demonstrates why routing must precede final normalization.
Also reuse the [existing content sketch](../template/testdata/content/article.html)
as an integration input; it must be loaded as content, never as templates.

## 5. Proposed loading stages

### A. Parse and identify the complete source document

Parse with `w3c-dom`/`x/net/html` infrastructure and retain HTML recovery behavior.
Assign document-local preorder IDs before styling and routing. Preserve original
ancestry and repaired-tree paths; decoded text-node offsets are distinct from
original HTML byte positions. Comments/metadata have identities for diagnostics
but do not become content boxes.

For the initial API, use body as the content root and keep html/body style
context. Do not offer arbitrary selector-based subtree loading until selection
scope and residual ownership are specified. Reject a named routing root on html
or body for this first profile; that avoids an ambiguous residual/root contract.

Recommend UTF-8 validation, following `tytui`, rather than silently changing bad
input bytes. This is an explicit input-policy choice for CONT-00. Empty bodies
should produce a valid empty-content result, unlike an empty template collection.

### B. Inspect CSS and style the original document once

Use the DOM inspection/reporting APIs for direct head styles, additional sheets
and active inline declarations. Retain source declarations, including losing and
nonmatching rules. Register additional sheets in supplied order after embedded
sheets as normal author CSS. Keep inline and importance precedence unchanged.

Apply the cascade to the original document, with no channel wrappers or template
frame nodes introduced. Resolve selector and inheritance context before any
projection changes. Keep this CSSOM private to the content snapshot/load.

Adopt explicit no-resource-loading behavior: no scripts, linked CSS, `@import`,
network images or fonts. Recommend diagnosing active resource requirements rather
than loading a visually incomplete document silently. Inert template contents and
non-rendered metadata need a documented traversal boundary. Conditional/scoped
styles and generated content are outside the proposed first profile.

Distinguish three validation layers: malformed CSS anywhere in an inspected
sheet; unsupported authored properties/values in the content profile; and
unsupported effective box participation. Do not reuse the template property's
allowlist: flowing content legitimately has typography and different geometry.
Explicitly audit defaults rather than treating absence of a declaration as an
unsupported style. Diagnostics must not depend on whether an invalid rule wins.

### C. Resolve and snapshot source context

Walk source ancestry before detaching anything. Capture:

- Checked display, position, visibility, flow name and box-sizing values.
- Typed unsized widths/edges and source-computed font size/white-space.
- Font family/weight/style, language, direction, supported bidi context and
  line-height policy needed by future paragraph preparation.
- Effective text paint and supported box paint, independent of frame paint.
- Decoded text storage and source references; no flattening into paragraphs yet.

Use `css.GetFlowName(node, "flow-into")`; empty means no explicit channel. Names
are decoded and case-sensitive. `flow-into` is non-inherited by default, but
explicit `inherit` can create a nested named root and must be handled by the
routing validator. Branch ownership is inherited structurally after routing;
it must not be confused with CSS property inheritance.

Use checked `GetDisplay`, `GetBoxLength`, `GetBoxKeyword`, `GetColor` and
`GetBorder`. Convert absolute DU values to CSS pixels explicitly. Keep percentages
as fractions in `layout.LenPercent`; defer resolution against receiving geometry.
Resolve source-relative font sizes before detaching a branch. Ordinary `em` box
lengths must use that source font context, not a frame's font.

Text style computation currently needs the shared resolver work in section 8.
Preserve paragraph owner context even for empty/object-only groups. Do not use
the first text run as an owner's font default. Source snapshots may outlive the
DOM/CSSOM, following `tytui`; if some property still needs ancestry, retain it
explicitly rather than borrowing a mutable DOM accidentally.

### D. Discover and validate route roots

Traverse active content in source preorder. Identify explicit effective nonempty
flow names, classify their source boxes, and build a route table before building
normalized trees. For the proposed first profile, a route root must be an
in-flow, non-replaced block flow or block flow-root element.

Reject nested named roots, even if both name the same channel. Descendant
`flow-into:none` does not escape ownership of an already routed ancestor. Reject
inline/inline-block routing roots and routing from within an inline or atomic
ancestor in this first slice. Those restrictions avoid settling split inline
ownership and partially extracting an atomic interior here.

Recommend that `display:none` suppress its whole subtree and creates no channel
branch, with an explicit suppression classification available for inspection.
This differs deliberately from `tytui`'s blanket rejection of hidden content.
Do not equate `visibility:hidden` with display suppression: it still has layout
consequences, so reject it for the initial profile unless its paint contract is
implemented. Validate CSS sources even when selectors lose or target suppressed
content; do not validate nonexistent boxes as active receivers/material.

When checking source coverage, distinguish authored non-content/inert nodes,
display-suppressed subtrees and layout material. A hidden subtree must never be
reported as an unexplained missing branch. Structural route errors should name
both conflicting roots and their channels.

### E. Partition a detached render projection

Create typed `layout.RenderNode` projections from the snapshots. Each routed
branch owns its complete eligible subtree, except display-suppressed content.
The residual projection includes all remaining active body material and retains
unextracted container nodes, their edges and paint.

Do not modify the source DOM or rerun selectors. Projected HTML nodes can be
minimal element/text descriptors as in `tytui`; `NodeID` and the source tables
carry correspondence. Use explicit effective typed styles on element projections
so a detached root cannot inherit default font or whitespace from a new parent.

An extracted branch keeps its own root box, margins, padding, borders and
flow-root status. It does not acquire copies of ancestor layout boxes. Its
ancestor-derived source styles remain captured. What its root margins mean at a
frame boundary, and how its percentages bind to a frame, are later trial rules.
Do not solve those by copying ancestor geometry or zeroing edges now.

Retain authored residual containers even if extraction empties them: an empty
decorated container may still matter. Do not flatten multiple channel branches
into one artificial source block; that would prematurely choose margin and
formatting-context behavior between branches. A channel is an ordered sequence
of branch trees, not an enclosing DOM element.

Maintain an ownership ledger: each eligible source element/text range belongs
to exactly one routed branch or the residual projection. Source ancestors may
remain referenced as context without owning duplicated layout material. Repeated
BoxIDs after normalization are not the measure of source ownership.

### F. Normalize each projected tree and index horizontal groups

Call the existing `BuildLayoutTree` with a validated `BuildOptions.ReadText`
callback on each detached block root. Let `css-box-layout` implement anonymous
wrappers, inline split/hoist, whitespace participation and flow-root structure.
Do not duplicate its normalization algorithm in the router.

Each independent build has its own BoxID allocation domain. Use a reference such
as `(snapshot, TreeID, BoxID)` for groups and owners; `(snapshot, NodeID)` for
source nodes. Never use a raw BoxID as a global channel/preparation key. Keep a
deterministic tree order, with residual identity separate from routed branch
order. The exact numbering is an inspection convention, not a persisted ID API.

Index actual `BoxAnonymousInline` groups and their nearest block owners, walking
through inline-block interiors to find their own groups. Ordinary inline nodes
are not independent paragraphs. Group order follows normalized traversal within
each tree; channel consumption order follows branch order, not accidental BoxID
sort order. Anonymous wrappers obtain paragraph context through their owners.

Retain raw decoded text and `TextRef` ranges now. The builder may omit empty or
collapsible-only runs according to its whitespace rules. Such omissions must be
explainable by the source ledger and supported whitespace policy. Do not collapse
all source text before routing: new branch boundaries can change which text is
adjacent. Processed paragraph text and detailed transformation spans belong to
the preparation extraction bridge, following `documentlayout.Extract`.

No calls to `ResolveUsedValues`, `FlowLayout`, typography measurement or shaping
occur during this stage. An inline-block is a structural atomic boundary with
an unsized interior, not a realized object with width and height.

### G. Publish the material snapshot atomically

Validate channel references, ownership coverage, source ranges, tree/group
references and paragraph context before returning. A failure in a later branch
must not expose earlier successful branches. Do not reuse mutable builder state
across independent loads.

Public inspection returns values/copies. Internally retain normalized structure
read-only and retain source/style/text data for the full preparation lifetime.
No committed cursor, receiver, current width or cached accepted result belongs
to this snapshot. Publish its initial persistent register bank with private,
read-only branch payloads; callers may derive independent banks without changing
the loaded document. Concurrent read-only inspection may be supported separately
from the serialized-use contract of future prepared paragraph handles.

## 6. Initial profile proposed for agreement

These are staged choices, not permanent restrictions inferred from sibling code.

| Concern | Proposed first slice | Later extension or decision |
| --- | --- | --- |
| Input root | Whole body; style full document | Arbitrary selected subtree and out-of-scope material policy |
| Elements | Ordinary supported block/inline containers and text, including headings, article/section/aside | Replaced images, lists/markers, tables, forms and generated content require explicit handling, not silent generic DIV conversion |
| Display | Block/inline with flow/flow-root; display:none suppression | Flex/grid/table/ruby/contents unsupported until projection contracts exist |
| Flow roots | Non-nested block branches under block ancestry; repeated names concatenate branch references | Nested flows, inline extraction and extraction from inside atomics |
| Residual | Explicit residual tree; no automatic `main` name | Default-channel assignment and collision/interleaving policy |
| Position | Static in-flow material | Floats and positioned content; template fixed frames are a separate domain |
| Box sizing | Content-box | Border-box adaptation |
| Dimensions | Auto/absolute/percentage widths and permitted edges retained unsized; effective positive source font context for em | Viewport units, richer expressions and font-metric-dependent units need explicit contracts |
| Height/min/max | Auto height; default min/max behavior | Authored constrained block heights and fragmentation |
| Text | Normal white-space and normal line-height initially; preserve language/direction/isolate metadata | Other whitespace modes and line-height representations need shared extraction/preparation support |
| Inline edges | Zero on ordinary inline boxes; preserve block/inline-block edges | General inline edge binding deferred as in PREPARATION.md |
| Paint | Checked foreground and solid background/border values retained | Rich backgrounds, decorations, effects and paint-only visibility policies |
| Resources | Caller-provided HTML and CSS; no implicit external fetching | Resource resolver and font configuration belong to later input/preparation work |

Do not resolve a percentage width just to get through the builder. A later
layout operation may still reject a dependency it cannot solve; chunk 1 should
promise representability and source preservation, not universal trial support.
For example, current box layout has restrictions on percentage inline-block
sizing. Record that as integration work rather than discarding the percentage.

Positive font-size is a proposed first preparation-compatible profile: current
`typography/style` spans require it, and box `FontSizePx == 0` means inheritance
or fallback. Supporting an explicit CSS zero needs a distinct representation;
do not silently turn zero into an inherited nonzero size.

Do not silently lock hyphenation to NoHyphenation because `tytui` does. Preserve
the authored/effective policy and U+00AD source bytes, then let the preparation
profile choose or diagnose supported behavior. The exact metadata carrier for
hyphens and future line-height data is an explicit sibling decision below.

## 7. Diagnostics and identity

Use a content-owned structured error, analogous to template `LoadError`, with
stable stage/code, source filename/path/node, optional CSS location, explanatory
message and wrapped original cause. Candidate stages: input, css, source-style,
profile, routing, projection, normalization and publication.

Suggested routing codes include `route.nested`, `route.root-display`,
`route.atomic-ancestor` and `route.document-root`. Source omissions not justified
by classification are internal invariant failures, not successful warnings.
Missing receivers are not load errors because a catalog is not an input.

Preserve each error's true coordinate system: CSS line/column within its sheet,
DOM path in the repaired tree, decoded text byte range, and qualified normalized
box reference. A CSS location is not automatically an HTML line number.
No string parsing of upstream error messages to recover NodeIDs.

Within a failed phase, order diagnostics by source preorder and then additional
stylesheet order, following the existing template convention. A later phase need
not run on invalid earlier results. Explicit file wrappers, if added, close
their files and preserve read/close errors without partial publication.

## 8. Required sibling work and optional improvements

### DOM-CONT-01 — Shared checked source text-style resolution

**Required for the recommended reuse strategy.** `w3c-dom` already supplies CSS
inspection, cascade, typed display, checked box values/colors and decoded flow
names. It does not currently provide the complete detached text-style resolution
performed in `tytui/htmlinput.ResolvedStyles`.

Add a checked, DOM-side text-style snapshot facility, with explicit parent/root
context where needed. It must resolve source font sizes, relative weights,
foreground color and language/direction context without selecting fonts or
requiring containing geometry. Define its supported whitespace, line-height,
bidi and hyphenation metadata. Move relevant parsing to checked DOM value helpers;
do not add another parser in `pagelayout`.

Keep the result free of typography engine/font objects. Map it into existing
`typography/style.Style` at the content/preparation adapter boundary, with
explicit DU conversion. Whether HTML hints (`lang`, `dir`, `bdi`, emphasis tags)
are handled by this facility or an adjacent reusable HTML adapter must be decided
before extraction. Preserve author CSS precedence over presentational hints.

Use `tytui` tests for relative sizes, root size, cascade keywords, currentcolor,
relative font weight, language and `dir=auto`/isolate boundaries as migration
fixtures. Retain whole source ancestry during directionality scans; routing must
not change the result. Stage any deliberately unsupported cases explicitly.

**Alternative requiring a decision:** promote the resolver into a separate
integration package instead. This may be preferable if keeping HTML semantics
outside the DOM styling package is important. It still must be shared by tytui
and pagelayout and must not import an application module. This is a packaging
choice, not permission to maintain two drifting implementations.

### TY-CONT-01 — Migrate tytui to the shared resolver

**Required companion work if DOM-CONT-01 extracts its semantics.** Preserve
`htmlinput.ResolvedStyles` as a compatibility facade returning its current
typography values. Replace moved parsing with the common implementation and
retain tytui's application profile checks. Run its typography/style, extraction,
display and document integration regressions before pagelayout relies on the
new shared service.

No change to tytui's CLI, image markers, font discovery, preparation lifetime,
single-group flow API or rendering is required for chunk 1. Moving the entire
`documentlayout` package is optional future integration work, not a prerequisite.

### BOX-CONT-01 — Structured construction errors and partition contract tests

**Required to meet source-specific normalization diagnostics.** The current
builder wraps several errors with a textual `node N` prefix and error sentinels.
Expose a structured construction error carrying the relevant source NodeID and
underlying cause. Preserve `errors.Is` compatibility. Update display, style,
text-range and normalization paths consistently; if no single node applies,
identify the branch/tree boundary without inventing a node.

Add regression tests confirming that independently built, filtered render
projections preserve their source IDs/text ranges and do not mutate the input;
explicit root font/whitespace snapshots must survive changed projection ancestry.
Document BoxID scope across independent builds. These tests support an existing
construction API rather than introducing channel semantics to the engine.

**No required named-flow callback or forest API for the first profile.**
The existing `registers` package already covers channel material storage;
this statement concerns additional box-construction APIs, not queue availability.
`RenderNode.ChildrenNodes` is already adapter-owned, and source styles can be
snapshotted before filtering. That makes detached projection partitioning a viable
implementation using `BuildLayoutTree` today. `BuildLayoutTrees`' fixed-root
diversion is a useful precedent for partition-before-normalization, but fixed
positioning and named channel ownership are different policies.

A generic partition/batch-build API becomes useful if a later profile needs
source-preserving inline extraction or one allocation domain across branches.
Introduce it only when a concrete normalization case requires it. Until then,
qualify BoxIDs by tree identity rather than promising global uniqueness.

### typography, khipu and other collaborators

No shaping, line-breaking or continuation engine changes are required for chunk
1. Reuse existing style value contracts and text storage concepts. If richer
line-height/hyphenation source metadata belongs in `typography/style`, propose a
small value-contract extension with mapping tests; do not add `PrepareInline`,
`FitInline` or register cursors in this milestone. Those belong to preparation
and page trials, respectively.

The source text normalization helper in `css-box-layout/text` and the extraction
algorithm in tytui remain references for chunk 2. Source parsing, CSS semantics,
box normalization and typography should each keep one implementation owner.

## 9. Proposed PR sequence and exit criteria

### CONT-00 — Agree the boundary and executable fixture

Resolve the blocking decisions in section 11. Specify the value inspection
surface and source/profile vocabulary, add the small fixture and an expectation
document, and pin the acceptance assertions. Choose shared resolver ownership
and verify sibling instructions before changing their repositories.

Exit: reviewers can state exactly which source material each output owns and
which operations must not occur during loading. This plan's illustrative API is
either accepted or revised before a public package is introduced.

### CONT-01 — Parse, inspect and style the content source

Add private source identities, decoded text storage, CSS inspection and a single
source-document cascade. Accept named additional stylesheet texts; preserve
normal cascade precedence. Define active/inert/resource boundaries and structured
diagnostics. Reuse DOM helpers and template-loader conventions without exposing
template internals or importing the template package into content.

Exit: source selectors/inheritance and errors are inspectable without boxing;
the DOM remains unchanged. Include sibling/ancestor selector and CSS override
cases, reader errors, malformed CSS and invalid UTF-8 according to the agreed
policy. No public partial source model escapes.

### DOM-CONT-01 + TY-CONT-01 — Shared text-style snapshot service

Implement and regression-test the agreed common resolver, migrate the tytui
facade, and document conversions and ownership. This can proceed independently
of private content parsing once the required context contract is agreed.

Exit: both consumers can obtain the same supported source typography without
font services; inherited values do not depend on later layout ancestry.

### CONT-02 — Typed projection and effective profile

Adapt styled nodes into detached source/style records and typed unsized
`RenderNode`s. Use checked getters and default constructors; retain block paint,
edges, explicit font context and original text ranges. Validate participation,
positioning and the initial content profile. Record suppressed/inert nodes.

Exit: projection matches source context, includes nested unsized atomics, and
does not need any frame/catalog/used-value input. Absolute-unit conversions,
percentage preservation and anonymous-owner context have concrete assertions.

### CONT-03 — Route ownership and partition projections

Discover eligible effective flow roots, reject nested/unsupported routing,
construct ordered channel branch sequences and a residual projection, and check
exclusive ownership. Retain untouched source context and residual ancestors.
Implement routing before invoking any box normalization.

Exit: the principal fixture has the exact expected branches and residual
material; repeated flow names preserve root order; hidden content is classified;
unmatched channels and residual content are retained without inventing cursors.

### BOX-CONT-01 — Builder diagnostics and independent-tree contracts

Deliver structured source errors and the regression coverage described above.
Keep behavior of existing builders and tytui geometry unchanged.

Exit: callers can map normalization failures to source identities using
`errors.As`, and independent builds have documented identity/ownership behavior.

### CONT-04 — Normalize, index and publish

Build each projected tree through CSS core, supply validated text lookup, index
groups/owners/nested atomic interiors, and populate an initial RegisterBank with
read-only branch payloads in channel source order. Publish it with the complete
material snapshot only after all branches succeed.
Add the agreed minimal public reader/inspection API. Validate no duplicate or
missing ownership and no partially published trees on later-branch failure.

Exit: chunk 2 has stable normalized groups, style/text lookup and atomic subtree
associations without any used dimensions or typography preparation. Returned
inspection values are independent copies; old snapshots survive later loads.

### CONT-05 — Integration examples and handoff

Add executable loading/routing examples and a diagnostic description of channels,
branches, source paths and normalized group owners. Use the existing sketch
content and the richer routing fixture. Document exact supported behavior and
link the preparation experiment to the published source/material contract.

Exit: an example loads two named channels plus residual material and prints
auditable ownership/group results without opening fonts or running layout.
No pgtmpl content UI or visual page proof is required to complete this milestone.

### Dependencies and review order

```text
CONT-00 -> CONT-01 ----------------------┐
CONT-00 -> DOM-CONT-01 -> TY-CONT-01 ----> CONT-02 -> CONT-03 --┐
CONT-00 -> BOX-CONT-01 -------------------------------------> CONT-04 -> CONT-05
```

The diagram expresses dependencies, not authorization to delegate work.
Shared resolver semantics must be reviewed with the tytui migration before
CONT-02 depends on them; CONT-04 depends on CONT-03 and structured builder errors.
Do not claim BOX-CONT-01 completes the later preparation driver.

## 10. Validation matrix

| Scenario | Required observation |
| --- | --- |
| Ancestor, adjacent-sibling and nth-child selectors | Styles reflect original DOM positions after branches are routed elsewhere. |
| Same HTML, different additional CSS | Fresh source/style/routing snapshot; no HTML-hash-only cache reuse. |
| Two roots naming the same channel | Ordered separate branches; no duplicate residual material or synthetic merged parent. |
| Initial RegisterBank | Take visits the channel's branch items in source order; deriving a bank or prepending a remainder leaves the initial bank unchanged. Payloads retain qualified source/tree identities. |
| Nested named roots, including explicit CSS inherit | Deterministic route error with both source roots; no partial material. |
| Inline/atomic routing roots or ancestors | Profile error; do not silently blockify or split atomic ownership. |
| No flow declarations | All active material remains residual; zero named channels is valid. |
| Unused flow name | Branch remains available; no receiver check during loading. |
| Extracted block between residual inline runs | Normalize the filtered projection; correct residual group adjacency and source mapping. |
| Empty decorated ancestor after extraction | Retained residual box; its source material is not duplicated into branches. |
| Nested inline-block | Outer group contains an atomic boundary; interior groups remain independently indexed and unsized. |
| Source font em/rem/% and inherited language/direction | Same effective source context before/after projection; no frame defaults consulted. |
| Percentage box width and auto atomic width | Remain typed constraints; no used-value or intrinsic measurement occurs. |
| Entities, multibyte UTF-8, NBSP and collapsible spaces | Valid decoded-byte ranges; no invented HTML offsets or unexplained content loss. |
| Anonymous/split boxes | Qualified BoxIDs, original NodeIDs where available, valid owner context. |
| Display suppression and inert content | Explicit classification; no channels created from suppressed subtrees. |
| Later branch build failure | No published document; wrapped cause and source locator retained. |
| Missing CSS/source read failure | Structured error; no styling/boxing of partial input. |
| Inspection mutation and repeat loads | Returned slices do not mutate snapshots; independent loads never share mutable current state. |
| No preparation dependency | Test fakes/call boundaries show zero font lookup, shaping, measurement, layout or continuation operations. |

Check ownership before normalization; afterward check valid references and
documented suppression rather than assuming a one-to-one source-node/box mapping.
Add a small routing fuzz target after deterministic cases: any successful result
must satisfy exclusive ownership, ordering and range invariants; failures must
publish nothing. Use normal Go formatting/tests in each touched module and
integration regression tests in tytui when its shared facade changes. A later
render proof may help inspection, but must not become a hidden layout dependency
of content loading.

## 11. Decisions still required

| Decision | Recommendation | Needed before |
| --- | --- | --- |
| Package/API and tree access | Separate `content` package; private trees and copied inspection values; preparation bridge accesses internals later | CONT-01/public surface in CONT-04 |
| Shared text-style resolver home | DOM-side checked snapshot service plus tytui compatibility facade; integration-package alternative if HTML hints need separation | DOM-CONT-01 |
| Typography value dependency | Reuse shared style values or map from DOM snapshots; explicitly carry policy fields absent from current Style | CONT-02 |
| Source input scope | Whole body, full original document cascade; defer selector-based roots | CONT-01 |
| Invalid UTF-8 and external resources | Reject invalid encoding and unsupported active resource requirements; never fetch implicitly | CONT-01 |
| Content CSS profile/defaults | Approve section 6 and a property-by-property source audit; do not copy the template allowlist | CONT-02 |
| Display:none vs visibility | Suppress display:none with recorded classification; diagnose unsupported visibility behavior | CONT-02/03 |
| Route eligibility | Non-nested block/flow-root under block ancestry; reject root/inline/atomic extraction initially | CONT-03 |
| Residual handling | Preserve separately; do not auto-bind to `main` in chunk 1 | CONT-03 |
| Extracted-root ancestor boxes | Retain source style context, exclude ancestor layout boxes from the routed branch | CONT-03 |
| Identity | Snapshot + tree + BoxID; no raw BoxID global keys or HTML-only semantic identity | CONT-02/04 |
| Register payload API | Treat FlowMaterial/GetBoxID as provisional with no other clients; choose box storage/handles and adapt sibling registers as needed, preserving ownership guarantees | CONT-00 |
| Zero font size and deferred text features | Explicit initial profile error for zero; preserve future policy metadata without choosing a breaker | CONT-02 |
| Branch-root margins/containing block | Preserve constraints now; decide frame-edge and fragmentation rules in trials | Before chunk 3, not a reason to size in chunk 1 |
| Default channel and missing receivers | Decide binding/order/collision policy when integrating existing registers with trials; never drop source material now | Before chunk 3 |

After agreement, update the status of individual choices rather than marking the
entire proposal settled. Register/RegisterBank storage and persistent bank
versions already exist. What remains to design is the concrete material payload,
within-item continuation, residual binding and page-level publication policy.
The channel inventory supplements the bank with source/order metadata.

## 12. Handoff to preparation

Chunk 1 supplies normalized groups, stable source typography/text, qualified
owner identities, branch order and atomic interior associations. Chunk 2 adds
reusable preparations and independent retained trial results, as described in
PREPARATION.md. The first two-width preparation experiment should consume this
material without rebuilding or restyling it.

Do not require general fragmentation before completing content loading. Equally,
do not call this material prepared paragraphs: no fonts have been selected,
base shaping performed or paragraph continuation tokens created yet.
