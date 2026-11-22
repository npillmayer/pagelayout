# Content loading and routing implementation plan

Date: 2026-11-22. Status: CONT-00 adopted and complete; CONT-01 private source
parsing, inspection and styling implemented. The public material loader,
normalization/routing and DOM-CONT/TY-CONT/BOX-CONT work remain pending.
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
CONT-00 adopts concrete `content.Box` handles as payloads and a generic conversion
of those existing queues, preserving their persistent behavior.

Section 11 records the adopted decisions separately from later open questions.
CONT-02 onward and the DOM-CONT, TY-CONT and BOX-CONT items remain implementation
work. References describe checked-out implementations; code sketches describe
the adopted target API unless explicitly identified as implemented.

### Implemented CONT-01 boundary

The private [`loadSource`](../content/load.go) pipeline returns a styled working
source or a structured error, never a public partial Document. Its input values
`LoadOptions` and `Stylesheet` exist; public `Load`, Document, Box and inspection
records await CONT-04. [`source.go`](../content/source.go) reads caller-owned
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
allowlist. These narrow CSSOM changes do not implement DOM-CONT-01's computed
text-style service or DOM-CONT-02's font-aware border helper.

Tests cover the fixtures' source/style assertions, reader ownership/failures,
UTF-8, CSS diagnostics and causes, inert boundaries, source immutability and
independent concurrent loads. No boxes, channel registers, fonts or used geometry
are created. Full authored/effective profile checking (including float, CSS
paint-resource/effect and typography restrictions) remains CONT-02; nested route
validation remains CONT-03. Thus the two `_err` fixtures currently load as private
styled source and still must fail when those later phases are implemented.

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
// Adopted target API. Input structs exist in CONT-01; Load/Document await CONT-04.
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

The current sibling payload interface is
`FlowMaterial { GetBoxID() layout.BoxID }`. The author confirms that it is
provisional and has no other client dependencies. **CONT-00 decision:** remove
that interface and convert the existing storage to `Register[T any]` and
`RegisterBank[T any]` in BOX-CONT-02. Keep the queue/bank algorithms and persistence
semantics; the queue does not inspect payloads or deduplicate box identities.

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
in [content/testdata](../content/testdata/README.md). They specify required future
material-loader results. CONT-01 tests exercise their source/style portions;
channel, box, group and publication assertions await the subsequent phases.

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
or freeze numeric TreeIDs/BoxIDs. Future tests must identify anonymous groups
through their source owners and qualified handles rather than hard-coded IDs.

Also reuse the [existing content sketch](../template/testdata/content/article.html)
in CONT-05 as content, never as templates. No pgtmpl invocation can load these
fixtures as content yet.

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
Resolve source-relative font sizes before detaching a branch. Resolve `em` box
lengths against that element's computed font and `rem` against the source root
font. Font-relative border widths require DOM-CONT-02's checked context-aware
helper. No conversion may consult a frame's font or duplicate CSS parsing here.

Text style computation currently needs the shared resolver work in section 8.
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
from the serialized-use contract of future prepared paragraph handles.

## 6. Adopted initial profile

These choices were adopted in CONT-00. They are staged scope, not permanent
restrictions inferred from sibling code. The property-by-property implementation
audit in CONT-02 must follow this profile and checked sibling parsers.

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

### DOM-CONT-01 — Shared checked source text-style resolution

**Required by CONT-00.** `w3c-dom` already supplies CSS
inspection, cascade, typed display, checked box values/colors and decoded flow
names. It does not currently provide the complete detached text-style resolution
performed in `tytui/htmlinput.ResolvedStyles`.

CONT-01's `StyleWithOptions` traversal boundary and removal of the styler's
element allowlist are implemented prerequisites, not completion of this service.

Add `w3c-dom/style/textstyle`, a checked source text-style snapshot service with
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
by CONT-00's DOM-side ownership decision. Both tytui and pagelayout use this
service; neither maintains a separate implementation of the moved semantics.

### DOM-CONT-02 — Checked font-relative border resolution

**Required by the adopted em/rem edge profile.** The current `css.GetBorder`
rejects nonzero font-relative widths. Add a checked context-aware variant/helper
using element and root computed font sizes from DOM-CONT-01. Keep CSS parsing,
effective none/hidden widths, inheritance and currentcolor semantics in the DOM
module; `pagelayout` supplies the source font context and converts DU to pixels.
Retain the existing absolute-only API's behavior for existing callers.

Test em/rem widths, inherited computed widths when parent/child font sizes differ,
none/hidden and currentcolor. The acceptance fixture's relative border must
resolve without frame geometry or local border parsing. The precise helper
signature is an implementation detail, not a reason to narrow the agreed units.

### TY-CONT-01 — Migrate tytui to the shared resolver

**Required companion work to DOM-CONT-01.** Preserve
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

### BOX-CONT-02 — Generic persistent registers

Replace `FlowMaterial` with type parameters on the existing Register and
RegisterBank; do not introduce another queue implementation. Preserve zero-value
usability, source/FIFO order, front insertion, structural sharing, concurrent
derivation and caller-controlled publication. Empty First/Take results return
the zero T and false; if a caller stores zero T, the boolean distinguishes it
from an empty queue. Storage does not impose a GetBoxID constraint or call into
pagelayout. Update existing register tests/examples to concrete payload types.

Test persistent append/take/prepend, independent candidate banks and removal of
exhausted names. CONT-04 adds content-specific assertions: valid Box payloads,
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

## 9. Proposed PR sequence and exit criteria

### CONT-00 — Agree the boundary and executable fixture (complete)

Adopted on 2026-11-22. The reader/inspection surface, concrete Box handles and
generic registers, shared DOM textstyle ownership, input/routing/profile rules
and sibling work are recorded above and in section 11. The HTML/CSS fixtures
and [expected results](../content/testdata/README.md) are present. Float support
is explicitly deferred. Read each sibling's instructions when implementing its
work; no sibling source code is changed by this milestone.

Exit satisfied at the design level: reviewers can state which source material
each output owns and which operations must not occur during loading. Fixtures
are inputs to future executable tests, not evidence of a working content loader.

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

### DOM-CONT-01 + TY-CONT-01 — Shared text-style snapshot service

Implement and regression-test the agreed common resolver, migrate the tytui
facade, and document conversions and ownership. This can proceed independently
of private content parsing once the required context contract is agreed.

Exit: both consumers can obtain the same supported source typography without
font services; inherited values do not depend on later layout ancestry.

### DOM-CONT-02 — Source-font border values

Add the checked font-context helper and its inheritance/effective-width tests.
Exit: content can project em/rem border widths from source styles without used
values, frame fonts or its own CSS parser. This is a CONT-02 prerequisite.

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

### BOX-CONT-02 — Typed register storage

Convert and regression-test the existing persistent queues as described above.
Exit: `RegisterBank[T]` retains the existing semantics with no payload interface;
CONT-04 can instantiate it with concrete content.Box values.

### CONT-04 — Normalize, index and publish

Build each projected tree through CSS core, supply validated text lookup, index
groups/owners/nested atomic interiors, and populate an initial RegisterBank[Box]
with valid branch-root handles in channel source order. Publish it with the
complete material snapshot only after all branches succeed.
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
Add a small routing fuzz target after deterministic cases: any successful result
must satisfy exclusive ownership, ordering and range invariants; failures must
publish nothing. Use normal Go formatting/tests in each touched module and
integration regression tests in tytui when its shared facade changes. A later
render proof may help inspection, but must not become a hidden layout dependency
of content loading.

## 11. Adopted decisions and later open questions

The following CONT-00 choices were adopted on 2026-11-22. Adoption establishes
the implementation target; it does not mean the APIs have been implemented.

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
reopening these ownership and scope decisions. Broader PREPARATION.md and page
trial proposals are not adopted by completing CONT-00.

## 12. Handoff to preparation

Chunk 1 supplies normalized groups, stable source typography/text, qualified
owner identities, branch order and atomic interior associations. Chunk 2 adds
reusable preparations and independent retained trial results, as described in
PREPARATION.md. The first two-width preparation experiment should consume this
material without rebuilding or restyling it.

Do not require general fragmentation before completing content loading. Equally,
do not call this material prepared paragraphs: no fonts have been selected,
base shaping performed or paragraph continuation tokens created yet.
