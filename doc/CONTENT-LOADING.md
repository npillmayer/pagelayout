# Content loading and routing implementation plan

SHIP-01 update (2026-11-25): the historical residual policy below is superseded.
`main` is now the first channel and contains the existing residual root as one
item; `Residual()` aliases it. Valid explicit main directives are ignored only
after original routing validation. Secondary queues retain source order.
`SourceEvents()` exposes copied source ID/anchor/break facts; the break-before
profile admits auto and page/left/right/recto/verso, normalizing forced variants
to page. Ownership and span validation remain pagination-plan work.

Date: 2026-11-23. Status: CONT-00 adopted and complete. CONT-01 source parsing,
inspection and styling, CONT-02 typed projection/effective profile, and CONT-03
route ownership/partitioning are implemented. DOM-CONT-01's shared text-style
service, TY-CONT-01's tytui migration and DOM-CONT-02's source-font border helper
are implemented, as are BOX-CONT-01's structured builder-error/partition contracts
and BOX-CONT-02's generic persistent registers. CONT-04 implements normalization,
group indexing and the public immutable material loader. CONT-05 completes the
executable integration examples and preparation handoff. Content loading chunk 1
is complete. The separate restricted preparation phase is also implemented;
see PREPARATION.md for its snapshot/session and whole-tree layout APIs.
Floats remain explicitly deferred.

This plan develops the content-input phase of [DESIGN.md](DESIGN.md), following
the completed [template-loading milestone](TEMPLATE-LOADING.md). It ends with
source-associated, normalized, unsized box material partitioned into named
channels and residual content. [PREPARATION.md](PREPARATION.md) starts from those
groups and establishes reusable typography. Page trials come afterward.

The adopted approach reuses the architecture demonstrated by sibling
`tytui/htmlinput` and `tytui/documentlayout`, while moving shared CSS semantics
to reusable sibling services and keeping channel ownership in `pagelayout`.
Do not make the application module `tytui` a production dependency or copy its
complete document pipeline into this module.

Use the existing sibling [`registers`](../../css-box-layout/registers/doc.go)
package for channel material storage. `Register` and `RegisterBank` already
provide persistent queues and bank versions; they are not future APIs to invent.
CONT-00 adopts concrete `content.Box` handles as payloads. BOX-CONT-02 implements
the generic conversion of those queues, preserving their persistent behavior;
CONT-04 supplies the content handles and initial bank.

Section 11 records the adopted decisions separately from later open questions.
All content-loading slices are implemented. References describe checked-out
implementations; preparation has its own completed milestones, while page
composition remains separate design work.

### Implemented CONT-01 boundary

The private [`loadSource`](../content/load.go) pipeline returns a styled working
source or a structured error, never a public partial Document. Its input values
`LoadOptions` and `Stylesheet` feed the public CONT-04 loader; this private stage
does not publish material. [`source.go`](../content/source.go) reads caller-owned
UTF-8 input, keeps HTML recovery and indexes every source node in preorder with
ancestry, repaired-tree paths and decoded text. There is no HTML-hash identity.

[`style.go`](../content/style.go) retains all inspected declarations and applies
one fresh CSSOM to the original DOM, with ordered additional author sheets.
Syntax/inspection failures stop before styling. Template subtrees, scripts and
metadata are classified and excluded from active inspection/styled output; their
source records remain. Display:none nodes still participate in inspection and
styling; suppression is classified in CONT-02. Linked stylesheets, misplaced or
conditional style elements, external CSS at-rules and resource-bearing body
elements are diagnosed without fetching. Source resource checks occur before
display evaluation and apply outside inert boundaries. Script sources and
ordinary hyperlink/canonical-link destinations are never fetched or executed.

This uses the implemented sibling `CSSOM.StyleWithOptions` traversal boundary,
which preserves original selector ancestry and sibling positions. The styler
also now applies declarations to every included element, without a layout-tag
allowlist. CONT-02 consumes DOM-CONT-01's computed text-style service and
DOM-CONT-02's font-aware border helper in the projection described below.

Tests cover the fixtures' source/style assertions, reader ownership/failures,
UTF-8, CSS diagnostics and causes, inert boundaries, source immutability and
independent concurrent loads. No boxes, channel registers, fonts or used geometry
are created. The two `_err` fixtures still load through `loadSource`: profile
checking belongs to CONT-02 and nested route validation belongs to CONT-03.

### Implemented CONT-02 boundary

Private [`loadProjection` / `projectSource`](../content/project.go) audit the
authored CSS profile, classify participation, resolve source typography and
produce detached source records plus typed, unsized `layout.RenderNode`s.
[`profile.go`](../content/profile.go) checks losing, nonmatching and suppressed
declarations as well as the cascade winners. Section 6 records the exact
property/element profile. Unsupported values return source-associated diagnostics
and no partial projection; the original DOM and CSSOM remain unchanged.

The copied ledger retains every source node, original ancestry, attributes and
decoded text, including inert, metadata and display-suppressed sources. Active
element records retain text-style snapshots, checked flow names, typed box
constraints and paint. Render descriptors have no original DOM/CSSOM pointers;
all element styles are explicit, including empty paragraph owners and unsized
inline-block interiors. TextRefs cover each active text node's decoded UTF-8
bytes without whitespace or soft-hyphen transformation. A wholly suppressed
body has a nil render root and a complete source ledger.

Absolute values convert explicitly to CSS pixels; em/rem use source fonts and
percentages remain fractions. Explicitly inherited lengths retain their parent's
computed value across font changes. No receiving geometry or font service is
consulted. Tests cover fixture typography/edges/paint/text, nested atomics,
suppression, source detachment, diagnostics and independent concurrent loads.
Route-free builder contract tests verify anonymous-owner font context, but the
production projection does not normalize boxes. Routing/partitioning are implemented separately in CONT-03; public material
publication is implemented separately in CONT-04. `float_err.html` now
fails projection's authored profile check; `nested-flow_err.html` still projects
successfully at this boundary; CONT-03 rejects its route ownership.

### Implemented CONT-03 boundary

Private [`loadPartition` / `partitionSource`](../content/route.go) validate effective
named roots before copying any destination trees. Only non-nested block flow or
block flow-root branches under block ancestry can be extracted. Active html/body
roots, inline/inline-block roots, extraction within inline/atomic ancestry, and
nested names (including identical names or explicit inheritance) fail atomically.
Routing diagnostics carry the original source, effective flow name and, for
conflicts, a structured related source plus both paths in the message.

The private partition retains independent render trees for each branch and a
separate residual tree. Branches retain global source order; channels retain
first-occurrence order and ordered branch indices. Case-sensitive unmatched
names are preserved without a template catalog or an implicit main channel.
An ownership map assigns every active element/text source either to its branch
root source ID or to residual (zero). Suppressed/inert/metadata nodes stay in
the copied source ledger without material ownership. Coverage validation rejects
duplicate or missing material before returning a result.

Extraction preserves typed styles, paint in source records, decoded text ranges,
unsized atomic interiors and each root's own edges. Residual ancestors remain even
when emptied, while text on either side of a removed branch becomes adjacent in
source order. The input projection is unchanged and destination render nodes,
styles, text references and ledger slices are independently copied. No selectors,
normalization, font services, used geometry, registers or public API are involved.
Acceptance tests cover base/override channel membership, residual structure,
suppression, empty documents, diagnostics and concurrent partitions.

## 1. Deliverable and boundaries

Load a separate HTML/CSS content document and publish one read-only material
snapshot containing:

- Source node identities, ancestry, decoded text and diagnostic locators.
- Effective source typography, box constraints and paint, independent of frames.
- Ordered named channels, each owning an ordered sequence of extracted branches.
- An initial `registers.RegisterBank[Box]` containing read-only branch material for
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
groups, inherited language and preparation-time line-height/strut context.

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

**Adopted package:** `github.com/npillmayer/pagelayout/content`, parallel to
`template`. Keep source, styling, projection, routing, normalization and
diagnostic working types private. Root `pagelayout` later coordinates the two
loaded products; neither loader imports the root or the other loader.

The intended dependency direction is:

```text
page composition
  ├─ template catalog
  └─ content material
       ├─ w3c-dom: parsing, checked values and style/textstyle snapshots
       └─ css-box-layout: typed constraints, normalization, text references
                         and registers.RegisterBank[content.Box]
```

Keep computed text-style values in the new `w3c-dom/style/textstyle` service,
without typography/font-engine dependencies. It owns source CSS and HTML hint
semantics, including metadata absent from the current typography style type.
Map its values into `typography/style.Style` at the preparation boundary, with
explicit unit conversion. Do not add typography to the loader just for a style
record or create a second resolver in `pagelayout`.

The adopted reader and inspection API is:

```go
// Implemented reader and copied-inspection API (CONT-04).
func Load(r io.Reader, opts LoadOptions) (*Document, error)

type Stylesheet struct {
    SourceName string
    CSS        string
}

type LoadOptions struct {
    SourceName  string
    Stylesheets []Stylesheet // Named, caller-supplied author CSS texts.
}

func (d *Document) Bank() registers.RegisterBank[Box]    // Persistent value, immutable payloads.
func (d *Document) Channels() []string                  // First occurrence order.
func (d *Document) Branches(channel string) []BranchInfo // Source order.
func (d *Document) Residual() TreeInfo                  // Separate from channels.
func (d *Document) Groups(tree TreeID) []GroupInfo       // Normalized group order.
func (d *Document) NodeSource(id layout.NodeID) (SourceInfo, bool)
func (d *Document) TextSource(id text.TextSourceID) (TextInfo, bool)
```

Here `layout` and `text` refer to sibling `css-box-layout` packages. Inspection
records expose the following information; slices and nested records are copied:

| Record | Required inspection values |
| --- | --- |
| `BranchInfo` | Root `Box`, `TreeID`, source root locator and branch source order. |
| `TreeInfo` | Tree identity and root handle; the residual tree is separate from all channel branches. |
| `GroupInfo` | Group and nearest block-owner `Box` handles, source paragraph context and nested atomic associations. |
| `SourceInfo` | Source name, NodeID, parent identity, repaired-DOM path, element/text kind, effective text style and ownership/classification. |
| `TextInfo` | Source node identity and immutable decoded UTF-8 text referenced by `text.TextRef`; offsets are decoded bytes. |

Ownership/classification distinguishes named-branch material, residual material,
display suppression, inert contents and non-content metadata. Anonymous boxes
have no fabricated source node; their paragraph context comes from their owner.
Unknown source/text IDs return `false`. IDs passed to a Document are local to
that snapshot. Stable persisted numbering and a general public tree traversal
API are outside this surface.

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

The provisional sibling `FlowMaterial { GetBoxID() layout.BoxID }` interface
had no other client dependencies. **CONT-00 decision, implemented by BOX-CONT-02:**
remove it and convert the existing storage to `Register[T any]` and
`RegisterBank[T any]`. The queue/bank algorithms and persistence semantics are
preserved; the queue neither inspects payloads nor deduplicates box identities.
The registers package no longer imports layout. Consumers instantiate storage
with their concrete payload type; no compatibility wrapper is required.

The content loader instantiates that storage exclusively with concrete `Box`
values. One item is the root box of an entire routed branch, including its
subtree, not one item per paragraph. The handle is adopted as:

```go
// In content. All fields remain private.
type Box struct {
    owner *snapshot
    tree  TreeID
    id    layout.BoxID
}

func (b Box) Valid() bool
func (b Box) Info() (BoxInfo, bool)
```

`snapshot` retains normalized trees, source text, styles and provenance. A Box
keeps that material alive independently of the public Document variable. The
handle is comparable and qualifies its tree-local BoxID with tree and snapshot
ownership. Its zero value is invalid. `BoxInfo` returns copied kind, formatting
context, typed constraints, paint and source references; it exposes no mutable
`LayoutNode`, style pointer or child slice. An invalid handle returns `false`.

The generic queue avoids a dependency from `css-box-layout` into `pagelayout`;
`RegisterBank[Box]` statically fixes this consumer's payload type. The handle
protects source lifetime and read-only tree access rather than merely adapting
an existing GetBoxID interface. Mutable raw node pointers and bare BoxIDs do not
provide those guarantees.

Append box branches under `RegisterID(flowName)` in source order after
normalization. Chunk 1's items are unset box material, not shaped paragraphs.
Preparation later associates reusable prepared groups with these boxes without
mutating them. Register payloads do not mix boxes with arbitrary paragraph or
engine objects. Within-box remainder/continuation representation belongs to the
page-trial milestone and is not a field to invent in this loading handle.

The bank has no channel enumeration/order API, and removes exhausted registers.
Retain a separate channel/source inventory for deterministic inspection; it is
metadata, not another queue implementation. The generic bank keeps its permissive
string IDs and payload values; empty First/Take results return the zero value of
T with `false`. The content loader publishes only validated nonempty flow names
and valid Box handles. Keep residual content separate until its binding policy is
agreed; the bank's acceptance of an empty ID does not settle that policy.

Take/Prepend already support whole-item consumption and restoration. Applying
them to a partially consumed box requires a later continuation contract; do not
claim that a Box alone records a remainder. The bank does not measure or split
material, validate continuation identity, or make mutable payloads safe.
Versions can be derived concurrently, but typography preparation may still require serialized
operations. Publishing a shared current bank remains the coordinator's job.
Tests/examples of these existing mechanics are in
[`registers/example_test.go`](../../css-box-layout/registers/example_test.go).

Repeated Append copies each remaining queue, so bulk loading a large channel has
quadratic copying cost. Start with the existing API for the small milestone;
consider a bulk constructor only if realistic collection sizes justify it.

## 4. Acceptance fixtures and expected results

The canonical inputs and assertion tables now live alongside the content package
in [content/testdata](../content/testdata/README.md). They specify the tested
material-loader contract. CONT-01/02 tests exercise source/style, typed projection,
text ranges and participation classification. CONT-03 tests verify ordered channel
branches, exclusive ownership and residual projections. CONT-04 tests normalized
boxes/groups, source context, atomic associations, publication and bank lifetime.

| Fixture | Acceptance purpose |
| --- | --- |
| [routing.html](../content/testdata/routing.html) | Two `article` branches, one `notes` branch, residual material, original sibling/ancestor styles and an independent inline-block interior. |
| [routing-override.css](../content/testdata/routing-override.css) | The same HTML with changed channel names, source font sizes and paint; normal specificity, inline and importance precedence. |
| [residual-inline.html](../content/testdata/residual-inline.html) | Extraction joins residual inline runs before normalization; a decorated emptied ancestor remains. |
| [suppression.html](../content/testdata/suppression.html) | Display-suppressed, inert and metadata source remain classified without entering channels or groups. |
| [empty.html](../content/testdata/empty.html) | Empty body is valid; an empty group still has its owner's default context. |
| [nested-flow_err.html](../content/testdata/nested-flow_err.html) | Nested named roots fail atomically and identify both roots. |
| [float_err.html](../content/testdata/float_err.html) | Floats are diagnosed as outside the first profile. |

The expectation document pins source-root and channel order, ownership, group
owners, effective styles, retained constraints, raw decoded text and register
persistence. HTML IDs label source nodes for review; they do not replace NodeIDs
or freeze numeric TreeIDs/BoxIDs. Tests identify anonymous groups
through their source owners and qualified handles rather than hard-coded IDs.

CONT-05's `ExampleLoad` also uses the
[existing content sketch](../template/testdata/content/article.html) directly as
content, separately from templates. No pgtmpl invocation loads these fixtures
as content; a content CLI is outside this milestone.

## 5. Adopted loading stages

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

Validate UTF-8, following `tytui`, and reject invalid input bytes. Empty bodies
produce a valid empty-content result, unlike an empty template collection.

### B. Inspect CSS and style the original document once

Use the DOM inspection/reporting APIs for direct head styles, additional sheets
and active inline declarations. Retain source declarations, including losing and
nonmatching rules. Register additional sheets in supplied order after embedded
sheets as normal author CSS. Keep inline and importance precedence unchanged.

Apply the cascade to the original document, with no channel wrappers or template
frame nodes introduced. Resolve selector and inheritance context before any
projection changes. Keep this CSSOM private to the content snapshot/load.

Do not fetch resources or execute scripts. Diagnose linked CSS, `@import` and
unsupported active resource requirements, including images and font resources.
Ordinary hyperlink targets do not require fetching. Record inert template
contents, script contents and non-content metadata, but exclude them from box
projection, routing and active CSS inspection. Direct head style elements are
the explicit stylesheet exception. Inline declarations on display-suppressed
elements are still inspected; suppression is not an inert-content boundary.
Conditional/scoped styles and generated content are outside the first profile.

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
  line-height policy used by paragraph preparation.
- Effective text paint and supported box paint, independent of frame paint.
- Decoded text storage and source references; no flattening into paragraphs yet.

Use `css.GetFlowName(node, "flow-into")`; empty means no explicit channel. Names
are decoded and case-sensitive. `flow-into` is non-inherited by default, but
explicit `inherit` can create a nested named root and must be handled by the
routing validator. Branch ownership is inherited structurally after routing;
it must not be confused with CSS property inheritance.

Use checked `GetDisplay`, `GetBoxLength`, `GetBoxKeyword`, `GetColor` and
`GetBorderWithFontContext` (or absolute-only `GetBorder`). Convert absolute DU
values to CSS pixels explicitly. Keep percentages
as fractions in `layout.LenPercent`; defer resolution against receiving geometry.
Resolve source-relative font sizes before detaching a branch. Resolve `em` box
lengths against that element's computed font and `rem` against the source root
font. Font-relative border widths use DOM-CONT-02's implemented
`GetBorderWithFontContext` helper. No conversion may consult a frame's font or
duplicate CSS parsing here.

Text style computation uses the implemented shared resolver in section 8.
Preserve paragraph owner context even for empty/object-only groups. Do not use
the first text run as an owner's font default. Source snapshots may outlive the
DOM/CSSOM, following `tytui`; if some property still needs ancestry, retain it
explicitly rather than borrowing a mutable DOM accidentally.

### D. Discover and validate route roots

Traverse active content in source preorder. Identify explicit effective nonempty
flow names, classify their source boxes, and build a route table before building
normalized trees. For the adopted first profile, a route root must be an
in-flow, non-replaced block flow or block flow-root element.

Reject nested named roots, even if both name the same channel. Descendant
`flow-into:none` does not escape ownership of an already routed ancestor. Reject
inline/inline-block routing roots and routing from within an inline or atomic
ancestor in this first slice. Those restrictions avoid settling split inline
ownership and partially extracting an atomic interior here.

`display:none` suppresses its whole subtree and creates no channel branch, with
an explicit suppression classification available for inspection.
This differs deliberately from `tytui`'s blanket rejection of hidden content.
Do not equate `visibility:hidden` with display suppression: it still has layout
consequences, so reject hidden/collapse values in the initial profile.
Validate CSS sources even when selectors lose or target suppressed
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
from the serialized-use contract of prepared paragraph handles.

## 6. Adopted initial profile

These choices were adopted in CONT-00. They are staged scope, not permanent
restrictions inferred from sibling code. CONT-02 implements the audit below with
checked sibling parsers; CONT-03 implements the routing restrictions.

| Concern | Adopted first slice | Later extension or decision |
| --- | --- | --- |
| Input root | Whole body; style full document | Arbitrary selected subtree and out-of-scope material policy |
| Elements | Ordinary supported block/inline containers and text, including headings, article/section/aside | Replaced images, lists/markers, tables, forms and generated content require explicit handling, not silent generic DIV conversion |
| Display | Block/inline with flow/flow-root; display:none suppression | Flex/grid/table/ruby/contents unsupported until projection contracts exist |
| Flow roots | Non-nested block branches under block ancestry; repeated names concatenate branch references | Nested flows, inline extraction and extraction from inside atomics |
| Residual | Explicit residual tree; no automatic `main` name | Default-channel assignment and collision/interleaving policy |
| Position | Static in-flow material | Positioned content; template fixed frames are a separate domain |
| Floats | No floats in chunk 1; explicitly postponed by the author | Image/table floats may later flow into designated template frames, including margin-spanning illustration frames; see the deferred direction below |
| Box sizing | Content-box | Border-box adaptation |
| Dimensions | Auto/absolute/percentage widths and permitted margins/padding; em/rem resolved from source fonts; checked absolute or font-relative border widths | Viewport units, richer expressions and font-metric-dependent units need explicit contracts |
| Height/min/max | Auto height; default min/max behavior | Authored constrained block heights and fragmentation |
| Text | Positive computed font sizes, normal white-space/line-height; preserve language/direction/isolate and hyphens none/manual/auto metadata and soft-hyphen bytes | Other whitespace modes, line-height forms and zero font size need explicit representation/preparation support |
| Inline edges | Zero on ordinary inline boxes; preserve block/inline-block edges | General inline edge binding deferred as in PREPARATION.md |
| Paint | Checked foreground and solid background/border values retained | Rich backgrounds, decorations, effects and paint-only visibility policies |
| Resources | Caller-provided HTML and CSS; no implicit external fetching | Resource resolver and font configuration belong to later input/preparation work |

Initial typography defaults are generic serif, 16 CSS pixels (`medium`), weight
400, normal slant, black foreground, normal whitespace/line-height and manual
hyphenation. Language and direction come from source HTML/CSS context, falling
back to undetermined language and LTR. The shared resolver handles `lang`, `dir`,
`bdi` and emphasis defaults; author CSS retains precedence. This does not import
a browser stylesheet with implicit heading sizes or paragraph margins.

Scope decision (2026-11-22): postpone float support. The intended later use case
is HTML-authored image/table floats flowing into template frames suited to their
material, including illustrations extending beyond the body-text area into a
wide margin. [DESIGN.md records the direction and open questions](DESIGN.md#deferred-direction-floats-into-designated-template-frames).
For chunk 1, unsupported floats receive profile diagnostics; do not silently
ignore their float intent or treat this temporary limit as a permanent authoring
restriction. No float-to-channel mapping or frame-kind syntax is settled here.

Do not resolve a percentage width just to get through the builder. A later
layout operation may still reject a dependency it cannot solve; chunk 1 should
promise representability and source preservation, not universal trial support.
For example, current box layout has restrictions on percentage inline-block
sizing. Record that as integration work rather than discarding the percentage.

Positive font-size is the adopted first preparation-compatible profile: current
`typography/style` spans require it, and box `FontSizePx == 0` means inheritance
or fallback. Supporting an explicit CSS zero needs a distinct representation;
do not silently turn zero into an inherited nonzero size.

Do not silently lock hyphenation to NoHyphenation because `tytui` does. Preserve
the authored/effective policy and U+00AD source bytes, then let the preparation
profile choose or diagnose supported behavior. The shared DOM textstyle snapshot
carries these policies; no typography engine change is needed for chunk 1.

### Implemented CONT-02 property and element audit

The following authoring vocabulary is explicit. Properties outside it are errors,
even when they would currently have no visible effect. Supported shorthands are
expanded by `w3c-dom`, including their resets. CSS-wide `initial`, `inherit` and
`unset` retain ordinary cascade semantics; rollback keywords are unsupported.

| Properties | Accepted authored/effective values |
| --- | --- |
| `display` | `none`, or block/inline outer display with flow/flow-root inner display, including `inline-block`; no list-item flag. Body must be block flow/flow-root unless suppressed. |
| `position`, `float`, `visibility`, `box-sizing`, `writing-mode` | `static`, `none`, `visible`, `content-box`, `horizontal-tb`, respectively. |
| `flow-into`, `flow-from` | Checked decoded names or none for `flow-into`; only none for `flow-from`. Named-root eligibility is checked in CONT-03. |
| `width` | Auto, absolute lengths, percentages, em/rem. |
| `margin` and four physical sides | Auto or signed absolute/percentage/em/rem lengths. |
| `padding` and four physical sides | Nonnegative absolute/percentage/em/rem lengths. |
| `height`, `min-width`, `min-height`, four physical insets | Auto only. |
| `max-width`, `max-height` | None only. |
| `border`, four side shorthands, `border-width/style/color`, twelve side longhands | Checked absolute/em/rem widths; none/hidden/solid styles; checked colors. None/hidden have zero effective width. |
| `color`, `background`, `background-color` | Checked sRGB colors/currentcolor; background shorthand is solid-color only. Foreground and background resolve in source context. |
| `font-family`, `font-size`, `font-weight`, `font-style` | Shared textstyle grammar, including source-relative sizes and weights. Computed font size must be positive. No font selection occurs. |
| `direction`, `unicode-bidi`, `hyphens` | LTR/RTL; normal/isolate; none/manual/auto, respectively. HTML language/direction/emphasis semantics come from the shared resolver. |
| `white-space`, `line-height`, `word-spacing`, `letter-spacing`, `word-break`, `overflow-wrap` | Normal only. |

Background and border-image reset longhands introduced by supported shorthands
must remain at initial values. They are not independently accepted authoring
properties. Ordinary inline flow boxes must have zero effective margins, padding
and borders; blocks and inline-blocks retain their edges. Length conversion checks
finite float32 representation, including overflow and nonzero underflow. Explicit
inheritance copies computed source-parent lengths before detachment; inherited
percentages stay percentages. Font-relative borders use the shared checked
font-context helper, including its DU rounding.

Active supported HTML elements are `html`, `body`, `div`, `p`, `address`,
`article`, `section`, `aside`, `main`, `header`, `footer`, `nav`, `blockquote`,
`h1`–`h6`, `hgroup`, `figure`, `figcaption`, `pre`, `a`, `abbr`, `b`, `bdi`,
`cite`, `code`, `data`, `del`, `dfn`, `em`, `i`, `ins`, `kbd`, `mark`, `q`,
`s`, `samp`, `small`, `span`, `strong`, `sub`, `sup`, `time`, `u` and `var`.
This list grants container/text participation, not automatic browser decorations,
generated quotes or superscript sizing. Active unknown/custom/foreign elements,
replaced content, lists, tables, forms, `br` and `hr` require later explicit
contracts. `pre` needs author `white-space:normal` to override its HTML hint.
An active `hidden` attribute requires explicit `display:none` in this slice.

Display suppression skips effective box/element/text-style checks for the whole
subtree, retaining its provenance. It does not bypass authored CSS auditing or
CONT-01's earlier resource checks. Inert/metadata source records retain their own
classification. HTML is a source-style context; body is the render root. No
browser UA stylesheet is added.

## 7. Diagnostics and identity

Use a content-owned structured error, analogous to template `LoadError`, with
stable stage/code, source filename/path/node, optional CSS location, explanatory
message and wrapped original cause. Stages: input, css, source-style,
profile, routing, projection, normalization and publication.

Routing codes include `route.nested`, `route.root-display`,
`route.atomic-ancestor` and `route.document-root`. Source omissions not justified
by classification are internal invariant failures, not successful warnings.
Missing receivers are not load errors because a catalog is not an input.
Use `profile.value` for authored values excluded by the first profile, including
non-none floats; retain the property, value and CSS source location. These codes
are pinned in the fixture expectations without fixing diagnostic prose.

Preserve each error's true coordinate system: CSS line/column within its sheet,
DOM path in the repaired tree, decoded text byte range, and qualified normalized
box reference. A CSS location is not automatically an HTML line number.
No string parsing of upstream error messages to recover NodeIDs.

Within a failed phase, order diagnostics by source preorder and then additional
stylesheet order, following the existing template convention. A later phase need
not run on invalid earlier results. Explicit file wrappers, if added, close
their files and preserve read/close errors without partial publication.

## 8. Required sibling work and optional improvements

### DOM-CONT-01 — Shared checked source text-style resolution (implemented)

**Required by CONT-00; implemented.** `w3c-dom` supplies CSS
inspection, cascade, typed display, checked box values/colors and decoded flow
names, and now the detached text-style resolution previously performed in
`tytui/htmlinput.ResolvedStyles`.

CONT-01's `StyleWithOptions` traversal boundary and removal of the styler's
element allowlist are implemented prerequisites, not completion of this service.

Implemented `w3c-dom/style/textstyle`, a checked source text-style snapshot service with
explicit parent/root context. It must resolve source font sizes, relative weights,
foreground color and language/direction context without selecting fonts or
requiring containing geometry. Define its supported whitespace, line-height,
bidi and hyphenation metadata. Move relevant parsing to checked DOM value helpers;
do not add another parser in `pagelayout`.

Keep the result free of typography engine/font objects. Map it into existing
`typography/style.Style` at the content/preparation adapter boundary, with
explicit DU conversion. HTML semantics (`lang`, `dir`, `bdi`, emphasis defaults)
belong in this shared service. Preserve author CSS precedence over defaults and
presentational hints. Its snapshots carry line-height and hyphenation policy
metadata even where `typography/style.Style` has no corresponding fields.

Use `tytui` tests for relative sizes, root size, cascade keywords, currentcolor,
relative font weight, language and `dir=auto`/isolate boundaries as migration
fixtures. Retain whole source ancestry during directionality scans; routing must
not change the result. Stage any deliberately unsupported cases explicitly.

The previously proposed separate integration-package alternative is superseded
by CONT-00's DOM-side ownership decision. Tytui and pagelayout's CONT-02 projection
now use this service; neither should maintain a separate
implementation of the moved semantics.

The implemented API is `Resolve(nodes ...*styledtree.StyNode) ([]Style, error)`:
results follow input order and contain no DOM/font pointers. It computes original
ancestors once per call and does not visit unrelated branches. A failure returns
no partial slice. `ResolveWithContext(node, Context{Parent, RootFontSize})` and
`ResolveFontSize(value, parent, root)` expose explicit computed source context.
The `Error` type carries the original HTML node, property, value and wrapped
cause for adapter-owned diagnostics; errors, unlike successful styles, retain
that node. Read-only source calls can run concurrently without a shared cache.

The snapshot retains checked family/size/weight/slant, foreground spelling and
NRGBA, canonical language, resolved direction, whitespace/bidi, line-height and
hyphens. A line-height is normal, a unitless factor, or a computed DOM-DU length;
percentages/em/rem compute at the declaring element and inherit as lengths,
whereas factors inherit without multiplication. No source text is transformed.
The fixed print defaults match section 6; arbitrary CSSOM UA overrides are not
part of this text profile. Known whitespace/bidi modes and non-normal line-height
can be represented even when an application's layout profile rejects them.
Deferred math/substitutions, metric/viewport font units, fractional weights and
CSS cascade rollback remain explicitly unsupported. Full supported values and
HTML scan boundaries are documented in the sibling
[textstyle README](../../w3c-dom/style/textstyle/README.md).

CONT-02 consumes these snapshots in `content`; CONT-01 remains the source/cascade
boundary. The shared `ValidateDeclaration(property, value)` and
`ParseKeyword(property, value)` helpers support CONT-02's pre-cascade authored
audit without copying text CSS parsing into pagelayout. Declaration validation
does not invent font context for relative values; effective resolution still
checks their computed range. Applications impose their own narrower profile.
Tests cover relative/root sizes, CSS-wide keywords,
currentcolor, weights, HTML/CSS precedence, source-context direction scans,
line-height/hyphenation, failure identity and detached/concurrent snapshots.

### DOM-CONT-02 — Checked font-relative border resolution (implemented)

**Required by the adopted em/rem edge profile; implemented.**
`css.GetBorderWithFontContext(node, side, css.FontContext)` resolves absolute and
em/rem widths to the existing detached `css.Border` value. The context supplies
`RootFontSize dimen.DU` and `FontSize func(*styledtree.StyNode) (dimen.DU, error)`
from DOM-CONT-01 source snapshots. The lookup can receive an ancestor when a
width is explicitly inherited: compute there before inheriting, even when the
child has a different font size. Root-relative borders use the document
element's computed font size, including for borders on that element itself.

Parsing, none/hidden effective widths, inheritance and currentcolor stay in the
DOM module; `pagelayout` supplies source font context and converts DU to pixels.
`css.GetBorder` keeps its existing absolute-only behavior. Absolute/zero widths
and none/hidden need no font context; nonzero em consults the element lookup,
and rem consults the root size. Used fonts must be positive and below dimen.Fil.
Relative products round to nearest DU, half upward; overflow, nonzero unsupported
units and missing/invalid required context fail without a partial Border.
Font-lookup errors retain their causes. This does not implement font-metric units,
viewport units, percentages or deferred expressions for borders.

Sibling tests cover all physical sides, source/root sizes, inherited computed
widths across changed fonts, resets, none/hidden, currentcolor, cascade priority,
invalid input/context, range/rounding, source preservation and concurrent reads.
The integration test in [content/border_fonts_test.go](../content/border_fonts_test.go)
checks the actual acceptance fixture: story-b's border is 98304 DU (2px), or
117965 DU (approximately 2.4px) with the override sheet and its changed currentcolor.
It needs no frame geometry or local CSS parser. CONT-02 integrates this into the
typed content projection; no public material loader is introduced by this step.

### TY-CONT-01 — Migrate tytui to the shared resolver (implemented)

**Required companion work to DOM-CONT-01; implemented.** Preserves
`htmlinput.ResolvedStyles` as a compatibility facade returning its current
typography values. Replace moved parsing with the common implementation and
retain tytui's application profile checks. Run its typography/style, extraction,
display and document integration regressions before pagelayout relies on the
new shared service.

The facade still returns `map[int]typography/style.Style` for the selection and
its ancestors. It selects nodes, calls the shared resolver, maps source errors
to existing DOM paths, and explicitly converts DOM DU and direction values.
The moved parsing/HTML semantics are no longer implemented in tytui. Its layout
profile and whitespace/source/isolate extraction remain application-owned;
line-height/hyphens are separate from the compatibility typography struct.

Regression integration also aligns tytui with the DOM's current black initial
foreground and automatic minimum-size defaults. Checked invalid box/paint
declarations now fail before permissive cascade reduction can discard them;
the existing permissive display fallback remains deliberate application policy.

No change to tytui's CLI, image markers, font discovery, preparation lifetime,
single-group flow API or rendering is required for chunk 1. Moving the entire
`documentlayout` package is optional future integration work, not a prerequisite.

### BOX-CONT-01 — Structured construction errors and partition contract tests (implemented)

**Required to meet source-specific normalization diagnostics; implemented.**
Both builders now return `*layout.BuildError`: RootID identifies the supplied
branch root; HasNode/NodeID identify the offending source when one applies;
Err/Unwrap preserve `errors.Is` and typed `errors.As` causes. HasNode distinguishes
an actual zero source ID from a tree-wide failure. Display, style, positioning,
replaced-content and text errors retain their source through nested construction;
allocation-wide failures identify the input boundary without inventing a source
for an anonymous box. ReadText callback errors retain their causes and are not
mutated. Direct validators and geometry APIs retain their existing contracts.

The sibling's `layout/partition_test.go` verifies independently built, filtered
render projections: source IDs and decoded text ranges survive, input values
remain unchanged, explicit root font/whitespace snapshots survive detachment,
and residual inline adjacency/emptied decorated containers normalize correctly.
Repeated and concurrent builds keep separate value storage; a later failure
does not change earlier results. BoxIDs are scoped to each construction call
(including that call's fixed roots) and can repeat in independent builds.
These tests support the existing API rather than introducing channel semantics.
CONT-04 can map structured source IDs to its own diagnostics without parsing text.

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

### BOX-CONT-02 — Generic persistent registers (implemented)

`Register[T any]` and `RegisterBank[T any]` now replace `FlowMaterial` in the
existing implementation. Zero-value usability, FIFO order, front insertion,
structural sharing, concurrent derivation and caller-controlled publication are
preserved. Empty First/Take results return zero T and false; stored zero/nil
payloads return true. There is no GetBoxID or comparability constraint and no
dependency on layout or pagelayout. Payload values are copied as T; reachable
reference data is shared and must remain immutable while any version uses it.

Existing tests/examples now use concrete payloads. Additional tests cover zero
values, typed nils, slices/maps and noncomparable structs, shared pointer identity,
storage sharing, consumed-prefix copying and drained-storage release. Persistent
append/take/prepend, independent candidate banks, exhausted-name removal and
concurrent derivation retain their regression coverage. CONT-04 adds
content-specific assertions: valid Box payloads,
qualified identity, retained source lifetime and an unchanged initial bank after
deriving other versions. The channel/source inventory remains separate metadata.

### typography, khipu and other collaborators

No shaping, line-breaking or continuation engine changes are required for chunk
1. Preserve DOM-owned source snapshots and map supported values to existing
typography style contracts at preparation time. Any later typography value
extensions belong to that boundary; do not add `PrepareInline`,
`FitInline` or register cursors in this milestone. Those belong to preparation
and page trials, respectively.

The source text normalization helper in `css-box-layout/text` and the extraction
algorithm in tytui remain references for chunk 2. Source parsing, CSS semantics,
box normalization and typography should each keep one implementation owner.

## 9. PR sequence and exit criteria

### CONT-00 — Agree the boundary and executable fixture (complete)

Adopted on 2026-11-22. The reader/inspection surface, concrete Box handles and
generic registers, shared DOM textstyle ownership, input/routing/profile rules
and sibling work are recorded above and in section 11. The HTML/CSS fixtures
and [expected results](../content/testdata/README.md) are present. Float support
is explicitly deferred. Read each sibling's instructions when implementing its
work; no sibling source code is changed by this milestone.

Exit satisfied at the design level: reviewers can state which source material
each output owns and which operations must not occur during loading. Fixtures
were the design-stage inputs. CONT-01–05 now execute their loader assertions;
PREP-00–05 separately verify extraction, preparation and whole-tree layout.

### CONT-01 — Parse, inspect and style the content source (implemented)

Implemented in content's private source/load/style/diagnostic files. Private
source identities, decoded text, retained CSS inspection and one original-source
cascade are covered by source/style tests. Named additional sheets preserve
normal precedence. The sibling styling traversal option excludes inert/metadata
subtrees without DOM mutation. No template-package dependency was introduced.

Exit satisfied: source selectors/inheritance and errors are inspectable privately
without boxing; DOM identities/attributes/ancestry stay unchanged. Tests cover
original ancestor/sibling/nth-child selection, CSS overrides, reader errors,
malformed CSS and invalid UTF-8. A parse fuzz target checks source-index invariants.
No public partial source model escapes. Full profile and routing errors remain
separate from syntax/resource inspection as described in the boundary above.

### DOM-CONT-01 + TY-CONT-01 — Shared text-style snapshot service (implemented)

The common resolver and tytui facade are implemented and regression-tested, with
conversions and ownership documented in section 8 and the sibling README.

Exit: both consumers can obtain the same supported source typography without
font services; inherited values do not depend on later layout ancestry.

### DOM-CONT-02 — Source-font border values (implemented)

The checked font-context helper, sibling inheritance/effective-width tests and
acceptance-fixture integration check are implemented as described in section 8.
Content can project em/rem border widths from source styles without used values,
frame fonts or its own CSS parser. This CONT-02 prerequisite is complete.

### CONT-02 — Typed projection and effective profile (implemented)

Private `loadProjection`/`projectSource` in content's projection/profile files
produce detached source/style records and typed unsized `RenderNode`s. Checked
DOM getters and default constructors retain paint, edges, explicit source font
context and decoded text ranges. Authored and effective profile checks, element
participation and suppressed/inert/metadata classifications are implemented.
No source DOM/CSSOM pointers escape in successful values.

Exit satisfied: fixture assertions cover source context, nested unsized atomics,
absolute-unit conversion, source-font em/rem and inherited computed lengths,
percentage preservation, paint, decoded UTF-8 ranges and anonymous-owner context.
Independent concurrent projections and no-partial-result diagnostics are tested.
No frame/catalog/used-value input, normalization or route partition is introduced.

### CONT-03 — Route ownership and partition projections (implemented)

Private `loadPartition`/`partitionSource` discover eligible effective flow roots,
reject nested/unsupported routing, construct ordered channel branch sequences
and a residual projection, and verify exclusive source ownership. The source
context and residual ancestors remain intact; no normalization occurs here.

Exit satisfied: the principal fixture and override have the expected branches,
channel order and residual material. Repeated names preserve root order; hidden
content stays classified; unmatched channels and residual content are retained.
Tests cover invalid roots/ancestry, inherited and same-name nesting, no partial
results, emptied decorated ancestors, independent copies and concurrent calls.

### BOX-CONT-01 — Builder diagnostics and independent-tree contracts (implemented)

Structured `layout.BuildError` and the independent-projection regression tests
described above are implemented. Construction and geometry behavior are retained.

Exit satisfied: callers can map construction failures with `errors.As`, classify
original causes with `errors.Is`, and rely on documented build-local identities,
source/style/text preservation, input immutability and atomic failure results.

### BOX-CONT-02 — Typed register storage (implemented)

The existing persistent queues and examples are converted and regression-tested
as described above. Exit satisfied: `RegisterBank[T]` retains the existing
semantics with no payload interface or layout dependency. CONT-04 instantiates
it with concrete content.Box values; content publication is not part of this step.

### CONT-04 — Normalize, index and publish (implemented)

[`content.Load`](../content/load.go) builds each partition through
`layout.BuildLayoutTree` with validated decoded-byte text lookup. Private
[`normalize.go`](../content/normalize.go) checks source ownership/channel order,
full text coverage and normalized references before publishing the complete
snapshot. Builder failures map `BuildError` identities to public `LoadError`
diagnostics while retaining original causes. Later-branch failure returns nil.

[`document.go`](../content/document.go) implements the section 3 reader/inspection
contract and comparable Box handles retaining snapshot/tree/box identity. The
initial `RegisterBank[Box]` contains branch roots in channel source order.
`BranchInfo.Order` is one-based global branch order; TreeIDs are snapshot-local
and zero is invalid. Residual content has a separate tree identity; a fully
display-suppressed body has an invalid residual root and no groups.

`GroupInfo` enumerates actual anonymous inline groups in normalized preorder,
including empty groups. `Owner` is the nearest block box; `Source`/`TextStyle`
retain its original paragraph context, including anonymous owners. `Atomics`
lists immediate inline-block boundaries without flattening interiors;
`ParentAtomic` links each interior group to its nearest containing atomic.
`Box.Info` returns copied unsized constraints, box paint and source context.
`NodeSource` retains classification, ownership, original ancestry and copied
attributes; `TextSource` includes raw decoded text even for excluded sources.

Exit satisfied: fixture tests verify groups/owners/atomics, residual adjacency,
empty/decorated owners, source typography/paint and unresolved percentages/auto
sizes. Tests cover nil/unknown handles, copied inspection, independent loads,
concurrent reads/derivations, retained box lifetime, malformed text references,
ownership corruption and later-branch failure with source-aware wrapped causes.
No fonts, used-value resolution, preparation or layout calls are introduced.
CONT-05 supplies the executable integration examples and preparation handoff.

### CONT-05 — Integration examples and handoff (implemented)

[`content/example_test.go`](../content/example_test.go) contains five executable
examples using only the public API: the original sketch content, a routing and
group report, persistent bank operations, additional author stylesheets and
source-aware load errors. Run `go test ./content -run '^Example' -v` from the
module root. The inputs are checked-in fixtures; callers explicitly own I/O.

`ExampleDocument` prints two channels, three branches with global source ordinals,
source paths and all six normalized group owners, including residual groups and the
atomic interior. Its example-local report helper uses copied inspection records;
it adds no public serialization or mutable tree API. Output avoids pinning raw
TreeID/BoxID numbers. `ExampleLoad` reads the existing sketch fixture without
duplicating it or loading it as a template collection.

The [content guide](../content/README.md) documents the exact loading boundary,
diagnostic report and handoff to PREP-00/BOX-PREP-01/PREP-01. It distinguishes raw
source ancestry/text from normalized group extraction, and branch payloads from
paragraph preparations. No font, shaping, measurement or layout service is used.

Exit satisfied: the examples' output assertions cover the complete public
loading/routing inspection path, unchanged banks/snapshots and atomic failure.
Content loading chunk 1 is complete. No pgtmpl content UI, visual page proof,
preparation API or sibling-module change is introduced by CONT-05.

### Dependencies and review order

```text
CONT-00 -> CONT-01 ------------------------------------> CONT-02
CONT-00 -> DOM-CONT-01 -> TY-CONT-01 -------------------> CONT-02
                       DOM-CONT-01 -> DOM-CONT-02 ------> CONT-02
CONT-02 -> CONT-03 ------------------------------------> CONT-04 -> CONT-05
CONT-00 -> BOX-CONT-01 --------------------------------> CONT-04
CONT-00 -> BOX-CONT-02 --------------------------------> CONT-04
```

The diagram expresses dependencies, not authorization to delegate work.
Shared resolver semantics must be reviewed with the tytui migration before
CONT-02 depends on them; CONT-02 also needs DOM-CONT-02. CONT-04 depends on
CONT-03, structured builder errors and generic registers.
Do not claim BOX-CONT-01 completes the later preparation driver.

## 10. Validation matrix

| Scenario | Required observation |
| --- | --- |
| Ancestor, adjacent-sibling and nth-child selectors | Styles reflect original DOM positions after branches are routed elsewhere. |
| Same HTML, different additional CSS | Fresh source/style/routing snapshot; no HTML-hash-only cache reuse. |
| Two roots naming the same channel | Ordered separate branches; no duplicate residual material or synthetic merged parent. |
| Initial RegisterBank[Box] | Take visits branch-root handles in source order; deriving a bank or prepending an unchanged whole item leaves the initial bank unchanged. Handles retain qualified identity and source lifetime. No within-box remainder is claimed. |
| Nested named roots, including explicit CSS inherit | Deterministic route error with both source roots; no partial material. |
| Inline/atomic routing roots or ancestors | Profile error; do not silently blockify or split atomic ownership. |
| No flow declarations | All active material remains residual; zero named channels is valid. |
| Unused flow name | Branch remains available; no receiver check during loading. |
| Extracted block between residual inline runs | Normalize the filtered projection; correct residual group adjacency and source mapping. |
| Empty decorated ancestor after extraction | Retained residual box; its source material is not duplicated into branches. |
| Nested inline-block | Outer group contains an atomic boundary; interior groups remain independently indexed and unsized. |
| Source font em/rem/% and inherited language/direction | Same effective source context before/after projection; no frame defaults consulted. |
| Font-relative border widths | Checked source-font resolution, including inheritance, none/hidden and currentcolor; no local CSS parser or receiving geometry. |
| Percentage box width and auto atomic width | Remain typed constraints; no used-value or intrinsic measurement occurs. |
| Entities, multibyte UTF-8, NBSP and collapsible spaces | Valid decoded-byte ranges; no invented HTML offsets or unexplained content loss. |
| Anonymous/split boxes | Qualified BoxIDs, original NodeIDs where available, valid owner context. |
| Display suppression and inert content | Explicit classification; no channels created from suppressed subtrees. |
| Empty body or emptied decorated ancestor | Valid residual tree and indexed empty horizontal group with owner context; no invented flowing text or discarded decoration. |
| Deferred floats | Profile diagnostic, never ignored or automatically routed into a new kind of frame. |
| Later branch build failure | No published document; wrapped cause and source locator retained. |
| Missing CSS/source read failure | Structured error; no styling/boxing of partial input. |
| Inspection mutation and repeat loads | Returned slices do not mutate snapshots; independent loads never share mutable current state. |
| No preparation dependency | Test fakes/call boundaries show zero font lookup, shaping, measurement, layout or continuation operations. |

Check ownership before normalization; afterward check valid references and
documented suppression rather than assuming a one-to-one source-node/box mapping.
Decision (2026-11-23): the proposed routing fuzz target was omitted and will not
be added to complete CONT-03; this is not outstanding work. Deterministic tests
remain the validation basis for exclusive ownership, ordering, text ranges and
failure atomicity. Use normal Go formatting/tests in each touched module and
integration regression tests in tytui when its shared facade changes. A later
render proof may help inspection, but must not become a hidden layout dependency
of content loading.

## 11. Adopted decisions and later open questions

The following CONT-00 choices were adopted on 2026-11-22 and delivered by the
content-loading slices above. Deferred work remains identified separately.

| Decision | Adopted choice | Delivery |
| --- | --- | --- |
| Package/API and tree access | Separate `content` package, reader API and copied inspection values; private normalized trees and read-only Box handles | CONT-01/04 |
| Shared text-style resolver | `w3c-dom/style/textstyle`, including HTML semantics; tytui compatibility facade | DOM-CONT-01 + TY-CONT-01 |
| Typography dependency | Keep DOM-owned style/policy values through loading; map to typography at preparation time | CONT-02; preparation later |
| Source scope | Whole body, full original-document cascade; reader remains caller-owned; empty body is valid | CONT-01 |
| Encoding/resources | Reject invalid UTF-8; no fetching or script execution; diagnose active unsupported resource requirements; classify inert/script/metadata contents | CONT-01 |
| CSS/cascade | Adopt section 6 and its defaults; ordered additional author sheets with ordinary specificity/inline/importance; audit losing/nonmatching declarations | CONT-01/02 |
| Relative lengths | Resolve em/rem from source fonts, including checked border helpers; retain percentages unsized | DOM-CONT-02 + CONT-02 |
| Display/visibility | Suppress display:none with recorded classification; reject hidden/collapse visibility in this profile | CONT-02/03 |
| Route eligibility | Non-nested static block/flow-root under block ancestry; reject document-root, inline and atomic extraction | CONT-03 |
| Residual/ancestors | Preserve residual separately, including emptied decorated containers; no automatic main; routed roots keep own boxes and ancestor-derived styles, not ancestor boxes | CONT-03 |
| Identity | Snapshot + tree + BoxID; comparable Box handles retain ownership; no raw global BoxID keys or HTML-only semantic identity | CONT-02/04 |
| Register payload | Concrete `content.Box` branch-root handles in `RegisterBank[Box]`; convert existing queues to generics and remove FlowMaterial/GetBoxID | BOX-CONT-02 + CONT-04 |
| Text policy | Positive font size; normal whitespace/line-height; preserve hyphens none/manual/auto and soft-hyphen bytes for preparation | DOM-CONT-01 + CONT-02 |
| Diagnostics/publication | Structured source-aware errors and causes, deterministic ordering, no partial document/bank on failure | BOX-CONT-01 + CONT-01/04 |
| Floats | Explicitly deferred beyond chunk 1; unsupported float values are profile errors | CONT-02; future float design |

Still open beyond CONT-00:

| Later question | Boundary already fixed | Decide when |
| --- | --- | --- |
| Branch-root margins and containing blocks | Preserve constraints; do not zero edges or resolve against a guessed frame | Page trials, before chunk 3 |
| Residual binding and missing receivers | Retain all material now; explicit residual binding and errors for material with no permitted destination are the later recommendation | Composition/page trials |
| Within-box continuation and page publication | Reuse persistent banks and immutable boxes; no mutable cursor or accepted geometry in the source snapshot | Page trials |
| Richer profile and float placement | Current limits are staged; the recorded image/table-to-frame direction remains deferred | Dedicated later designs |

The channel inventory supplements the bank with source/order metadata. Detailed
diagnostic prose and implementation helper signatures may evolve without
reopening these ownership and scope decisions. Preparation was adopted and implemented in its own milestones; completing
CONT-00 did not by itself adopt page-trial APIs.

## 12. Handoff to preparation

Chunk 1 supplies normalized groups, stable source typography/text, qualified
owner identities, branch order and atomic interior associations. Chunk 2 adds
reusable preparations and independent retained trial results, as described in
[PREPARATION.md](PREPARATION.md). The implemented two-width preparation experiment
consumes this material without rebuilding or restyling it. The
[content guide's handoff](../content/README.md#handoff-to-preparation) maps the
published API to that boundary. `SourceInfo.Children` retains source ancestry;
it must not be used as a substitute for normalized paragraph extraction.

PREP-00 has pinned service/session ownership and initial policy, with
[preparation acceptance fixtures](../content/testdata/preparation/README.md).
BOX-PREP-01 supplies shared core group enumeration, now used by content's index
with its existing source-context validation and qualified handles. PREP-01 now
extracts detached processed text, source/style/policy mappings, isolate scopes
and atomic occurrences from content's private normalized trees. PREP-02 publishes
the detached GroupInput values through an explicit Preparer protocol, with an
immutable PreparedSnapshot and separate serialized PreparationSession. The public bank
retains immutable branch-root Box handles throughout. TYPE-PREP-01 and PREP-03–05
add the optional typography bridge, intrinsic measurement, independent whole-tree
layout, real-font proofs and GC lifetime checks. BOX-PREP-02 distinguishes
`RealizedAtomic` geometry from reusable paragraph preparation.

Do not require general fragmentation before completing content loading. Equally,
do not call this material prepared paragraphs: no fonts have been selected,
base shaping performed or paragraph continuation tokens created yet.
