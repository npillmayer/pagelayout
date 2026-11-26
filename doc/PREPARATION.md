# Preparation between CSS box construction and layout

Date: 2026-11-23. Status: PREP-00 contract/fixtures, BOX-PREP-01 shared enumeration
and PREP-01 extraction complete. PREP-02's neutral preparation protocol and
session lifecycle are implemented. TYPE-PREP-01 connects the real typography
pipeline with explicit fake-font/stream integration tests. PREP-03 reuses that
preparation for intrinsic measurement. PREP-04 now publishes independent whole-tree
layout results. PREP-05 proves real-font reuse, bank persistence and release of
working state. BOX-PREP-02 completes the realized-atomic terminology migration.
The restricted preparation slice is complete. BOX-SHIP-02/03 and SHIP-04 now
supply finite-height fitting and continuation; page construction remains later work.

This note develops the preparation phase introduced in [DESIGN.md](DESIGN.md).
Preparing reusable content before page trials is established direction. The
first-slice ownership and policy decisions in sections 11–12 are now pinned by
PREP-00. BOX-PREP-01 implements shared structural enumeration; PREP-01 implements
detached extraction. PREP-02 publishes the neutral protocol and isolated sessions;
TYPE-PREP-01 adds the typography bridge; PREP-03 adds intrinsic measurement.
PREP-04 adds whole-tree layout integration; PREP-05 adds real-font/lifetime proofs
and an executable handoff. Wider policies and page composition remain unimplemented. Earlier
conceptual sketches yield to those decisions. The sibling modules are under our control;
their current limitations identify integration work rather than permanent limits
on this design.

The preceding content-input phase is implemented as documented in
[CONTENT-LOADING.md](CONTENT-LOADING.md): source styling, named-branch routing,
residual preservation and normalized group indexing. Its output is unsized box
material with source context, not prepared paragraphs. CONT-00 has adopted that
loading contract and its fixtures; CONT-01/02/03 implement private source loading,
typed projection and route partitions. **Content loading chunk 1 is complete:**
BOX-CONT-01's builder diagnostics/independent-tree contracts and BOX-CONT-02's
generic registers are implemented. CONT-04 now supplies normalization/group
indexing and public immutable material. CONT-05 supplies
[executable examples](../content/example_test.go) and the
[published material handoff](../content/README.md#handoff-to-preparation).
Sections 11–14 specify the register/session model, scoped extensions, PR sequence
and acceptance tests. PREP-00 establishes the implementation contract; it does
not add a preparer, font service or layout operation. These sections
refine the earlier callback/driver sketch where functional ownership requires it.

## Implemented placement in the pipeline

The implementation provides an explicit preparation stage **after box normalization and before
used-value resolution**. Reuse the result for both intrinsic measurement and
layout. Keep group enumeration in the CSS core and effectful preparation in an
explicit orchestrator/session, as detailed in sections 11–13.

This establishes a prepared-material lifetime spanning many layout trials.
The box core identifies horizontal groups; the typography adapter prepares them;
`pagelayout` decides when to prepare flow material and which trials consume it.
Registers keep their immutable content.Box payloads throughout.

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
`paragraph.Prepare`, caching the result for that layout invocation. Pagelayout
now makes preparation explicit and extends its lifetime beyond one invocation
through `content.Prepare` and the separate snapshot/session pair.
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
frames and commits only a selected complete page. CSS-level fragment fitting and
the content/typography bridge are implemented by BOX-SHIP-02/03 and SHIP-04;
page composition remains subsequent work in [SHIPOUT-PLAN.md](SHIPOUT-PLAN.md).

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

## 4. Explicit preparation and execution handles

The earlier sketch proposed a `PreparedInline` object with measurement and layout
methods stored in `PreparedBoxes`. Review of the current functional and typography
contracts refines this: executable handles belong to a separate session, not an
immutable bank payload or structural tree. See section 11 for the lifetime model.

A neutral preparer receives detached group input and returns an executable engine
handle plus immutable metadata to the orchestrator. Its request contains source
text/styles, paragraph owner context and atomic identities, but no receiver width,
height budget, used-value table or realized atomic dimensions. The optional typography bridge
wraps `paragraph.Prepared` privately; the core and loader need no khipu or
concrete font types. PREP-02's public protocol is documented in section 13.

The operations remain distinct:

- Preparation establishes reusable horizontal material and source correspondence.
- Measurement uses that material with explicit atomic contribution ranges.
- Layout uses it with definite width and realized atomic geometry, returning an
  independent candidate rather than replacing a current result on the handle.

The existing `InlineLayouter` and `IntrinsicMeasurer` interfaces can remain.
Each trial supplies fresh callback adapters that look up the same session's
prepared group and collect typed paragraph results privately. Publish those
results alongside CSS geometry after whole-invocation success. There is no need
to put an opaque paragraph payload in core `InlineResult` or geometry tables.

Whole-group layout and partial fitting remain separate contracts. Typography's
`Prepared.Trial`, `Trial.Assemble`, checkpoints and `Continuation.Fit` already
supply paragraph-level pieces. `content.NewFitter`/`content.Fit` and
`preparation/typography.Fit` now connect structural fragments and exact paragraph
continuations. `Pagination.PageAttempt` composes transactional multi-channel pages,
and `PaginationSearch` selects one complete pagespan. Public span transitions and
delivery remain in the shipout plan.

## 5. Group enumeration and the preparation driver

The structural traversal lives in `css-box-layout`: `layout.InlineGroups`
enumerates normalized horizontal groups, owners and atomic boundaries
deterministically, including interior groups. It is a pure operation with no
shaping or geometry work. Content's normalized group index now uses it.

The effectful driver is `content.Prepare`, with an explicit `content.Preparer`:

```text
layout.InlineGroups(root) -> iter.Seq[layout.InlineGroup]  // implemented
content.Prepare(document, preparer)
    -> immutable prepared snapshot + separate serialized execution session
```

The snapshot shares the existing immutable source trees and stores copied side
data. The session owns executable preparation handles. Qualified group identity
includes the content snapshot and tree; a bare BoxID is not unique across branches.
No pass adds readiness, glyphs, selected geometry or execution handles to nodes.

Each trial receives its own used-value tables, atomic realizations, paragraph
choices and placements. Preparation provides reusable data without choosing
geometry. This is a refinement of the earlier proposal to put the entire
`PrepareBoxes` driver in css-box-layout: the core retains group semantics, while
external services and their caches stay outside the pure passes.

![Requests, returned handles and reuse between css-box-layout and typography during preparation and trials](diagrams/preparation-interactions.svg)

[Graphviz DOT source](diagrams/preparation-interactions.dot)

The diagram shows the implemented snapshot/session split and bridge callbacks.
Executable handles stay in the session; only copied metadata is published in the
snapshot. Measurement/layout recursively consume atomic bounds/realizations.
The separately marked box-fit step remains future work.

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

Enumeration can therefore walk top down, as envisaged in the author's remarks.
Outer-group extraction stops at the atomic boundary; enumeration continues into
the atomic subtree so the orchestrator prepares its separate groups.
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

BOX-PREP-02 names this trial work explicitly: `layoutAtomicInteriors` lays out
atomic interiors and returns `RealizedAtomic` border dimensions through a
`RealizedAtomicsView`. These CSS-pixel values belong to one layout invocation;
reusable horizontal paragraph preparation remains separate. Margins and the
outer paragraph's final atomic placement are separate from those dimensions.
See [the current atomic operation](../../css-box-layout/layout/atomic.go).

## 7. Intrinsic measurement remains separate

A text-only paragraph might have reusable intrinsic bounds. A paragraph
containing objects needs the objects' intrinsic contributions, including
applicable horizontal edges. Those contributions are not automatically known
when the outer paragraph is prepared.

Preparation therefore produces **material that can be measured**, without
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

PREP-00 ownership rules for the new phase (with the split in section 11):

- A successful pass publishes a complete snapshot/session pair for the document.
  Failure publishes no partially ready set. Internal completed caches need not
  be erased merely because a later operation fails.
- Prepared snapshots contain no execution state. Sessions contain no mutable
  current width, selected page or accepted register cursor.
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

## 10. Completed contract experiment and remaining decisions

Start from one `content.Load` snapshot, following the
[CONT-05 handoff](../content/README.md#handoff-to-preparation). Its `Groups(tree)`
identifies normalized groups, owner context and atomic boundaries. Source-child
lists describe original DOM ancestry, not extracted paragraphs; PREP-01's private
extractor reads the normalized structure. Keep bank payloads and
source styles unchanged, and use `hyphens:none` explicitly in the experiment's
input rather than overriding retained policy during preparation.

Prepare an outer paragraph and a nested auto-width inline-block, then lay them
out at two receiving widths. All groups are prepared before any
measurement or layout callback. Both candidates coexist; rejecting either
must leave published material unchanged and the session reusable, although complete
memoized discoveries may persist. Base text is not reshaped for
the second width, while explicitly deferred alternatives remain permitted.

PREP-03–05 execute this experiment, including a nested inner atomic, real-font
outline proofs and GC lifetime checks. SHIP-04 separately implements finite-height
fragments and exact changed-width continuation through the same preparation.
Empty template frames use the geometry-only resolver and have no horizontal
content to prepare.

PREP-00 pins the initial choices in section 12. Implementation must preserve:

- The prepared-handle interface and compatibility with the existing callbacks.
- The representation and ownership of retained paragraph trial results.
- Driver scope, tree/group identities and failure publication behavior.
- Deferred inline contributions and the supported initial text/box profile.
- Keep partial fitting separate from this whole-group experiment; the later
  `content.Fit` and typography bridge implement it under SHIP-04.

Sections 11–13 establish the first-slice choices and sequence implementation.
Wider policies and page-level fitting remain separate decisions. Completing
PREP-00 does not implement or authorize all subsequent sibling changes at once.

## 11. Registers, prepared snapshots and functional ownership

**PREP-00 decision (2026-11-23): keep the register payload
`content.Box` throughout preparation.** That is the adopted content-loading
payload, not a new mixed interface or a box with a mutable readiness flag.
BOX-CONT-02 implements generic registers; CONT-04 now supplies content.Box and
the initial branch bank, with copied group/source inspection.

A register stores immutable branch-root handles. A branch may own many horizontal
groups, including groups inside atomics; it is not interchangeable with one
paragraph. Preparing those groups does not change the branch's identity,
structure, source styles or position in its channel. Publish a separate prepared
snapshot keyed by qualified group identity. The same bank can then be consumed
with either of two independently prepared snapshots of the same content.

```text
Content Document D ─── immutable normalized trees + source/group indices
       │
       └── Bank B: RegisterBank[content.Box] ─── references D only

Prepare(D, explicit services and policy)
       ├── PreparedSnapshot P: D + immutable group/atomic/source metadata
       │                      + preparation identity and coverage manifest
       └── PreparationSession S: matching execution handles/services
                                  (serialized use; never a register payload)

Trial(B, P, S, receiver constraints)
       └── independent candidate output + derived bank B′
           choosing the candidate publishes B′; rejecting it keeps B
```

The last line describes a future page coordinator, not a preparation API that
already implements page fitting. Within-box continuations remain separate from
whole-item `Take`; neither preparation nor measuring consumes a bank.

Use `(content snapshot, TreeID, BoxID)` for group identity and a new
`PreparationID` for each successful preparation session/snapshot pair. Bind the
session to exactly that snapshot and reject cross-document or cross-preparation
calls before invoking services. Atomic occurrence IDs are local to a group;
associate them with qualified box handles, not raw BoxIDs across trees.

Prepare the whole document in the first implementation: residual tree first,
then branch trees in global source order, visiting groups in deterministic tree
preorder. This includes empty and atomic-only groups. Validate complete coverage
before publishing either result. A bank assembled from another document is not
implicitly compatible. Selected-subset and incremental preparation are deferred.

`PreparedSnapshot` does not point to its session. It can retain immutable source,
processed-text/source-map/paint values and stable resource identity, but no
mutable discovery cache, stream, current result, used-value table or cursor.
`PreparationSession` owns the adapter's `paragraph.Prepared` handles, which retain
immutable Khipu plus mutable Discovery. Dropping the session releases working
preparation without invalidating source material or previously frozen results.
The snapshot alone is an inspection/coverage value, not an executable paragraph.
Do not reopen fonts or silently reprepare if the matching session is absent.

This fits the current sibling API rather than assuming an immutable
`paragraph.Prepared`: its [contract](../../typography/glyphing/paragraph/prepared.go)
requires serialized operations and retains Discovery. Khipu already separates
[immutable material, binding and plans from discovery](../../khipu/doc/PREPARED_PARAGRAPHS.md).
The first bridge can use those APIs unchanged; exporting the private base Khipu
from paragraph.Prepared is not required just to create a public manifest.

This deliberately avoids placing `*paragraph.Prepared` in a register or a field
reachable through `content.Box`: persistent queues share payloads, not deep
copies. Calling such a handle could update discoveries visible through old bank
versions. A private mutable field would not become functional merely because it
is unexported. See the [register contract](../../css-box-layout/registers/doc.go).

The split follows [css-box-layout's functional rules](../../css-box-layout/doc/Functional.md):

- Structural trees, source records, register versions and published geometry are
  immutable values. Preparation adds side data without decorating existing nodes.
- Core group enumeration and geometry transforms are deterministic with explicit
  inputs. Font selection/shaping and memoized discovery live in an explicit
  orchestration/session layer, not hidden core state.
- Each measurement/layout invocation has fresh scratch, used values, atomic
  realizations and a private result collector. Publish copied results only after
  complete success. A rejected trial never overwrites another candidate.
- Session calls, including calls sharing a resolver or services across sessions,
  are serialized initially. Persistent-bank concurrency does not imply concurrent
  typography execution. Failed calls may leave complete memoized discoveries,
  but cannot mutate published material or earlier results.

Strict referential transparency of every font-service call is not claimed by
this contract. The functional guarantee applies to material, banks and published
results; the effectful service layer is explicit. Requiring discovery itself to
be a persistent value would be additional khipu work, not necessary for this
milestone or a reason to weaken the register payload contract.

## 12. PREP-00 decisions and deferred extensions

The following choices are the implementation target for the first preparation
slice. They do not claim that the APIs already exist. Exact method/type spelling
can be finalized in the assigned PR without reopening ownership or policy.

| Decision | Pinned first implementation | Deferred boundary |
| --- | --- | --- |
| Register payload | Keep `RegisterBank[content.Box]`; readiness belongs to a separate snapshot/session pair | No mixed raw/prepared queue items or destructive conversion |
| Preparation scope | Eager, whole-document, atomic publication, including residual and nested groups | Partial/lazy preparation and bounded working sets later |
| Tree access and package boundary | A backend-neutral `content.Prepare` entry point owns private tree access and extraction; the optional `pagelayout/preparation/typography` bridge implements its preparer protocol | No mutable LayoutNodes in public inspection and no typography import in content/core; PREP-02 publishes the protocol |
| Core traversal | Shared group enumeration in css-box-layout; effectful driver in pagelayout orchestration | Refines the earlier proposal to put the entire preparation driver in the box core |
| Font selection | Caller supplies preloaded selection/resolver and matching shaper services; reuse `khipu/contracts.FontFace` and `FontInstance` inside the bridge | Discovery/configuration remain application concerns; no new font wrapper or default OpenType backend |
| Trial result ownership | Retain typed paragraph trials/results in a fresh adapter invocation collector; return them alongside fresh CSS geometry | No `any` paragraph payload in core geometry; no last-result map on a prepared group |
| Initial text/box profile | Normal whitespace and CSS normal line height; zero ordinary-inline edges; nested auto/fixed-width atomics in the trial experiment | Loading/preparation retain percentages; unsupported intrinsic dependencies fail explicitly in the trial, not by rewriting material |
| Breaker and alignment | Start the experiment with explicit FirstFit, zero side skips, CSS line boxes and bottom-margin-edge atomic alignment | Additional breaker policies can follow without changing bank payloads |
| Hyphenation | Experiment uses source `hyphens:none`; bridge admits uniform none and uniform manual with an injected explicit-marker hyphenator. Mixed/auto groups fail whole-document preflight | Manual intrinsic measurement can still reject multiple eligible splits/variants; retain that upstream failure and do not silently downgrade policy |
| Partial fitting | SHIP-04 connects core structural fitting to retained paragraphs | Whole-page transactions remain separate; use exact qualified continuations and frozen fragment bundles |
| Concurrency/lifetime | Caller serializes session and shared-service operations, including Close; session owns executable handles and has idempotent Close; frozen output retains stable resources independently | No concurrent-trial promise, internal global lock, implicit reprepare or permanent cache |

The hyphenation boundary is substantive. Content loading preserves none/manual/
auto per source span, with manual as its default. Khipu currently fixes one
hyphenation mode per preparation; its intrinsic measurement supports none and
bounded explicit alternatives, but not automatic or arbitrary multiple explicit
splits per textbox. The bridge must retain all original policies and return
source/group-associated unsupported-policy errors where necessary. Do not split
one paragraph into separate paragraphs to simulate span-level policy. Broader
admission requires a separately scoped KHIPU-PREP extension and matching
paragraph tests; this is an integration limitation, not a permanent CSS rule.

### Policy admission and preflight

Extract and validate every group before any backend preparation/font work.
Determine group policy from nonempty processed **text** spans, using the source
of the first contributing byte for a collapsed space. Omitted whitespace does
not create a policy conflict. Structural atomic markers do not import interior
policy; interiors are checked as independent groups. With no text spans, use the
paragraph owner's policy. Owner strut style is always resolved separately.
Retain all source policies in the extracted records, including rejected inputs.

- Uniform `none` maps to `khipu.NoHyphenation`. Preserve soft-hyphen source bytes;
  disabling discretionary discovery is not permission to delete source mappings.
- Uniform `manual` maps to `khipu.ExplicitHyphenation` and requires an explicitly
  supplied hyphenator, even if this group has no markers. Its U+00AD request
  follows the sibling's [explicit-marker protocol](../../khipu/hyphenation.go).
  There is no implicit ASCII hyphen or dictionary.
- Mixed none/manual/auto within a paragraph, and uniform auto, fail before any
  group's engine preparation. Do not split paragraphs or silently choose none.
  Uniform policies may differ between independent groups.
- Manual preparation does not promise arbitrary intrinsic measurement. The
  sibling [MeasureIntrinsic contract](../../typography/glyphing/paragraph/measure.go)
  supports bounded explicit alternatives; multiple eligible positions/variants
  can fail later at measurement with the original unsupported-policy cause.
  Do not run intrinsic measurement during preparation to test eligibility.

Other deterministic preflight checks include supported inline edges/text policy,
valid UTF-8 source maps/style/isolate ranges, qualified group/owner/atomic
correspondence, complete group coverage, and explicit required service inputs.
Use the first failure in residual-first/global-branch/preorder traversal, then
processed-span order, for deterministic diagnostics. Validate all groups before
starting the first effectful group operation. A policy error in the last group
therefore has zero backend preparation calls.

### Services, failure publication and cleanup

`Prepare` targets a complete immutable snapshot plus a separate execution
session. The session retains the snapshot; the snapshot never retains the
session. Give each successful pair a new opaque preparation identity. Existing
content Boxes, source trees and banks remain unchanged. Reject a foreign source,
group/owner, preparation ID or closed session before service work. A retry of a
failed whole-document preparation starts fresh working state; there is no
published partial manifest or reusable half-built session.

The neutral preparer protocol has an all-input preflight followed by one
PrepareGroup operation per ordered group, including empty groups. Returned
execution capabilities belong privately to the session. Backend-specific typed
results stay in the optional bridge; no `any` paragraph payload enters core
geometry or a bank. PREP-02 implements the neutral protocol with fake services;
TYPE-PREP-01 adds the typography adapter. No target width, used-value table,
frame style, receiver height or live cursor is a preparation input.

Caller-supplied selected fonts/resolver, coverage, shaper factory and supported
hyphenator are borrowed and remain stable. Never select paragraph.Prepare's
synthetic zero-config backend implicitly. The caller retains ownership of font
resources needed by frozen output; session closure must not close shared faces,
resources, the resolver or factories. No new font representation is introduced:
the optional bridge reuses `contracts.FontFace`/`FontInstance` and typography's
selected-resource contracts, without concrete OpenType types in neutral APIs.

Preparation failures return neither snapshot nor session. The backend cleans up
its in-progress operation and returns no live handle on error. The orchestrator
releases successful owned handles in reverse creation order, including when
final coverage validation fails. Cleanup proceeds after a cleanup error; retain
the initiating error and any cleanup causes with `errors.Is/As` (for example,
through `errors.Join`). No callback panic recovery is promised. Borrowed services
may retain complete memoized work; rollback of their caches is not required.

Session `Close` is serialized and idempotent, releases owned handles once and
drops session/service references even if release reports an error. Repeated Close
does not repeat cleanup; subsequent execution fails as closed. Source inspection,
prepared metadata and earlier frozen outputs remain valid. The current
`paragraph.Prepared` has no Close method: its adapter release drops references.
Streams are operation-owned and obey `StreamingShaper.Finish`, not a newly
invented font/paragraph close API. Factories return fresh exclusive streams;
factory errors own their cleanup, and successful streams are finished after
producer/consumer completion, including unused preflight failures. Reuse
[the existing stream contract](../../khipu/contracts/shaping_stream.go) and
[StreamingConfig ownership](../../khipu/config.go).

Errors carry preparation stage (preflight, extraction, itemization/font,
preparation, measurement, layout or release as applicable), qualified group and
owner handles, source locator and the original cause. Use the offending span's
source/decoded range when available; otherwise use the owner's source context.
Anonymous groups get no fabricated source NodeID. Lifecycle/identity errors
without a valid source report that absence. Trial failure returns no candidate
and leaves previous results and the session reusable; a closed session is not
implicitly recreated. PREP-02 publishes `PreparationError` and `PreparationStage`;
backends can add precise stage/source details while preserving the cause.

### Acceptance fixtures and scoped follow-up

The [PREP-00 fixture contract](../content/testdata/preparation/README.md) and
[`expected.json`](../content/testdata/preparation/expected.json) pin eight inputs:
text-only, nested auto-width atomics, empty/atomic-only groups, two branches in
one channel plus another channel and residual, manual markers, multiple manual
markers, mixed-policy rejection and auto rejection. Expected processed text,
source-byte mapping, atomic offsets, preparation order and service-call counts
are explicit. Current tests verify loading/structure, manifest consistency and
actual PREP-01 extraction against the unchanged expectations. PREP-02 tests now
verify fake-engine call counts, policy preflight, failure cleanup, identity
isolation and snapshot/session lifetime. TYPE-PREP-01 checks actual typography
preparation with injected fake fonts/streams, retaining lazy manual measurement
restrictions. PREP-03 exercises public measurement, nested atomic contributions,
signed margins, request-sensitive answers and failure/retry without base reshaping.
PREP-04 verifies two-width results, exact atomic geometry and late assembly
failure/retry using the same synthetic services. PREP-05 adds explicitly selected
real fonts, two-width outline proofs, persistent bank versions, changed fonts/styles/
policies and GC checks while retaining frozen output and source material.

The restricted preparation slice through PREP-05 is implemented.
Wider span-level/automatic hyphenation and intrinsic support require KHIPU-PREP
work; ordinary-inline edges,
percentage-atomic sizing cycles, font discovery, concurrent sessions sharing
services, incremental preparation, residual receiver binding, floats, partial
box fitting and page transactions remain scoped extensions. None narrows the
loader's accepted unsized source material. PREP-00 changes no sibling APIs.

No w3c-dom change is currently required: its source-style snapshots already carry
font sizes, language, bidi and hyphenation policy. Convert DOM DU explicitly to
typography/khipu DU. Keep line-height and hyphenation beside typography.Style,
whose fields do not represent both. The bridge reuses
`glyphing.NewStyleMapper`/`ItemizeInline` and `paragraph.Prepare`; the application
may choose typography's OpenType adapter. No concrete `*ot.Font` enters
pagelayout's public/core types. Typography bundles adapters in its Go module,
so an indirect module-graph dependency is still possible even without OpenType
package imports; separate adapter modules would be independent packaging work.

## 13. Step-by-step PR plan

PREP-00 is complete as a contract/fixture milestone; BOX-PREP-01's shared
enumeration/content-index migration, PREP-01's extraction and PREP-02's neutral
protocol/session lifecycle, TYPE-PREP-01's typography bridge and PREP-03's
intrinsic integration, PREP-04's whole-tree results and PREP-05's real-font/lifetime
proofs are implemented. BOX-PREP-02's terminology migration is also complete;
wider extensions remain separate work. Cross-module changes must follow each sibling's
instructions. Each PR has a narrow acceptance
boundary; do not label the full phase complete after the first preparation call.

### PREP-00 — Close prerequisites and pin the contract (complete)

**Owners:** pagelayout; prerequisite css-box-layout content work.

BOX-CONT-01 (structured builder errors) and BOX-CONT-02 (generic registers) are
complete, as are CONT-04 (normalization, group indexing, immutable public material)
and CONT-05 (executable handoff). Section 12 now pins snapshot/session ownership,
admitted hyphenation, service failure/cleanup and source error mapping. The
public content.Box remains the branch payload; preparation cannot publish a
partial pair or mutate source material.

Eight acceptance inputs and machine-readable expected results live under
`content/testdata/preparation`. Run
`go test ./content -run '^TestPreparationAcceptanceInputs$' -v` for current
loading/group and coordinate checks. The nested experiment explicitly uses
`hyphens:none`.

**Exit satisfied:** prerequisites pass; first-slice choices are pinned or scoped
out. The fixtures now drive extraction, preparation and layout tests in later
completed slices. PREP-00 itself added no runtime or sibling API.
The intentionally omitted CONT-03 routing fuzz target is not reopened.

### BOX-PREP-01 — Enumerate normalized groups without sizing (implemented)

**Owner:** css-box-layout, with consumer migration in pagelayout/content.

`layout.InlineGroups(root) iter.Seq[layout.InlineGroup]` yields descriptors with
`Group`, `Owner`, `Atomics` and `ParentAtomic`. It identifies actual
BoxAnonymousInline groups and their nearest block-container owners in structural
preorder, including anonymous owners and empty groups. Immediate atomic lists
stop at inline-block boundaries; nested interiors are independent later groups.
Replaced leaves are atomic boundaries without interior groups.

Node pointers borrow the normalized input tree and must remain read-only,
including styles/child slices. Each Atomics slice is caller-owned and independent
of other descriptors/iterations. BoxIDs remain tree-local. Nil yields nothing;
subtrees ignore external ancestors, and an inline-block root encloses its own
interiors. The iterator is repeatable after early termination and supports
concurrent reads. It is not a malformed-tree/cycle validator. No text processing,
measurement, shaping, style resolution or geometry operation occurs.

CONT-04's content index consumes this shared traversal for group ownership and
atomic relationships. Its separate source/identity validation walk remains;
paragraph source context is still resolved by content, including anonymous
owners. Public metadata retains copied values and qualified immutable Box
handles, with no borrowed raw nodes exposed.

**Exit satisfied:** core tests cover anonymous/split structure, empty owners,
nested atomics, replaced leaves, unchanged input and stable preorder. Independent
trees with colliding local IDs, descriptor slice ownership, subtree boundaries,
early termination and concurrent reads are covered. Existing content tests retain
their group/source-context results. No typography, DOM or font imports are added
by the iterator. PREP-01 extraction is implemented separately below; effectful
preparation is implemented in PREP-02 and TYPE-PREP-01.

### PREP-01 — Extract detached group input and source mappings (implemented)

**Owner:** pagelayout/content, consuming BOX-PREP-01.

The private `snapshot.extractGroup(tree, index)` uses normalized trees and source
snapshots to build copied `GroupInput` records (exported by PREP-02):
processed UTF-8 text, source spans, computed text styles, separate paint, paragraph
owner context, bidi isolates, hyphenation policy spans and atomic occurrences.
It reuses `css-box-layout/text.NormalizeNormal` and follows tytui's established
extraction contract without importing that application. Collapsed/omitted bytes,
NBSP, soft hyphens and entities map back to decoded source ranges. One U+FFFC is
inserted for each structural atomic occurrence; authored U+FFFC remains ordinary
text. Occurrence IDs are one-based and local to a group, with qualified Box
mappings and independent interior group identities. Atomic percentages remain
unresolved. Existing source owner context remains separate from text-run styles.

Style, separate foreground paint, policy and isolate ranges all address the same
processed UTF-8 bytes. A collapsed space belongs to its first contributing source
unit; source maps retain every contributor, including omitted source policies.
Processed policy spans contain text only, excluding structural markers and
omitted bytes. Empty/object-only groups retain the owner's policy for later
preflight. Mixed/auto source policies are extracted unchanged, without admission
or font calls. Isolate scopes follow normalized structural postorder; empty
scopes add no markup, and atomic interiors do not leak into outer scopes.

All slices are independently owned; source references/styles are detached values
and boxes retain immutable snapshots. No raw tree or mutable source pointer
escapes. Private extraction errors retain group/owner, source/range where known
and original causes, returning no partial group. PREP-02 now wraps them in public
PreparationError diagnostics; the extractor itself remains private.

**Exit satisfied:** actual extraction matches all eight acceptance manifests,
including exact processed bytes, source ranges and atomic offsets. Tests cover
routing adjacency, split/anonymous owners, empty/suppressed groups, nested and
collapsed-space isolate scopes, policy provenance, nonzero source-range origins,
checked failures, independent snapshots and concurrent detached extraction.
Inputs, banks and prior outputs remain unchanged; no sibling-module changes or
font/typography dependencies are required.

### PREP-02 — Publish preparation snapshots and isolated sessions (implemented)

**Owner:** pagelayout/content; fake engine only at first.

`content.Prepare(document, preparer)` returns `(*PreparedSnapshot,
*PreparationSession, error)`. It extracts and validates every GroupInput before
calling the backend's `Preflight([]GroupInput) error` once. Preflight owns
backend-specific service and policy admission and must make no font/shaping/layout
calls. `PrepareGroup(GroupInput) (GroupPreparation, error)` then runs once per
group in residual-first/global-branch/preorder order. Each callback receives
independent slices; backend mutation cannot change the manifest or later input.

GroupPreparation echoes qualified Group/Owner handles and transfers an exclusive
GroupHandle with `Release() error`. Nil handles and mismatched identities fail;
even a handle returned with an error is defensively released. Nothing is published
until complete coverage succeeds. All acquired handles are released in reverse
creation order on failure, continuing after errors and joining original causes.
Neither bank operations nor receiving geometry are involved.

PreparedSnapshot retains source and immutable input/coverage data, with a fresh
opaque comparable PreparationID. Its Groups/Input methods return copies, Covers
checks source identity, and Document returns an independent source handle. The
snapshot never references its session, backend or executable handles.

PreparationSession keeps the private handle table. WithGroup checks the exact
snapshot/session pair and group/owner before borrowing the backend handle for
one callback. Callers retain typed/frozen results, never the handle itself.
Backend execution errors preserve the session for retry. Serialized operations
are required; reentrancy is rejected without promising concurrent execution.
Close releases in reverse order, clears owned references even after cleanup
failure, and returns the same error on repeated calls. Wrapper copies share the
lifecycle. Borrowed fonts/resolvers/services are never closed.

PreparationError retains stage, qualified group/owner, source/range where known
and the original cause. Backend errors can supply precise itemization/layout
stages. Invalid identity/lifecycle errors without a valid source report its
absence; errors.Is/As preserve initiating and cleanup causes.

**Exit satisfied:** fake-engine tests prove exact fixture call counts, complete
preflight before all preparation, deterministic order, unchanged material/banks,
late failure and retry, reverse cleanup despite errors, detached backend inputs,
two coexisting pairs, foreign/colliding-ID rejection before callbacks, repeatable
Close, and concurrent immutable inspection after closure. A public executable
[recording-backend example](../content/prepare_example_test.go) exercises the API.
The fake policy models none/manual admission and rejects mixed/auto; real font
service validation and typography preparation are implemented separately by
TYPE-PREP-01. No typography or font imports enter content; no public tree accessor
or mutable register payload is introduced.

### TYPE-PREP-01 — Implement the typography preparation bridge (implemented)

**Owner:** optional pagelayout/preparation/typography package; typography/khipu
changes only if an identified contract gap requires them.

`preparation/typography.New(Config)` returns a content.Preparer backend requiring
an already constructed glyphing.StyleMapper and fresh exclusive-stream factory.
The caller builds the resolver with glyphing.NewStyleMapper and selected opaque
font faces/coverage. Config optionally supplies an explicit-marker Hyphenator;
source manual policy requires it even when no marker is present. No implicit
backend, discovery or new font wrapper is introduced. No sibling changes were
needed; local module replacements wire the existing typography/khipu/cords/uax
graph while keeping concrete adapter packages out of production imports.

Whole-document preflight validates source policies, finite dimension conversion,
style/anchor coordinates and bidi markup without font resolution, coverage,
scaling, stream creation or shaping. Mixed/auto groups fail before any group
preparation. Uniform none preserves soft-hyphen bytes without discovery; manual
uses khipu.ExplicitHyphenation and the explicit U+00AD protocol. Omitted whitespace
and structural markers do not select policy; owners supply empty/object-only
defaults. Source ranges and original causes are preserved in PreparationError.

The bridge resolves the owner separately, converts DOM DU using the physical BP
ratio with a finite-range check, and retains paint outside shaping styles.
ItemizeInline's returned styled text is passed unchanged to paragraph.Prepare
exactly once per group, together with zero-byte bidi markup and atomic anchors.
Private session handles retain only the prepared paragraph and copied source,
atomic, font-binding and normal line-height metadata. No receiving width, atomic
geometry, vertical metrics or line breaking is involved in preparation.

The upstream bidi callback represents one opening and one closing per offset.
Coincident nested openings/closings therefore fail preflight explicitly; adjacent
close/open pairs are supported. Broader coincidence support is later uax/paragraph
integration work, not a permanent content-loading restriction. No text characters
are inserted to work around it.

Release drops handle references without closing borrowed services or fonts.
The existing paragraph/khipu lifecycle finishes streams, including unused ones
after acceptance failure; factories retain cleanup responsibility on failure.
Itemization/preparation causes and upstream paragraph stages remain available.
Engine errors without structured source offsets fall back to owner context.
Base preparation is reused; lazy manual alternative shaping may still occur.
Unsupported intrinsic alternatives remain upstream errors without fallback.

**Exit satisfied:** all eight fixtures run through the real typography pipeline
with explicit fake fonts/streams and exact group/base-preparation counts. Tests
cover source/atomic mappings, paint bypass, owner struts, nested interiors,
manual admission/lazy discovery, auto/mixed/service rejection, late itemization/
factory/acceptance/shaping failures, stream finishing, cleanup, retry and retained
output after Close. Production dependency tests exclude OpenType/fontfind/tytui
from the bridge and typography/khipu from content/template/CSS core. Public
measurement is implemented in PREP-03 and whole-tree trial integration in PREP-04;
PREP-05 adds real-font visual proof. See the [bridge guide](../preparation/typography/README.md).

### PREP-03 — Consume preparation during intrinsic measurement (implemented)

**Owners:** pagelayout content orchestration/typography bridge; css-box-layout
standalone access to its existing recursive intrinsic pass.

`preparation/typography.MeasureIntrinsic(prepared, session, root, context)` measures
a covered block container, returning CSS-pixel border-box bounds excluding the
root's own margins. The context provides a definite containing width for root
percentages and a fallback font size, not a paragraph line width. Definite root
widths contribute their prescribed width plus padding/borders; auto widths recurse.

The missing core entry point is now `layout.MeasureIntrinsic(root, context,
measurer)`. It runs the existing intrinsic pass with fresh scratch and the normal
atomic-width profile, producing no used-value table or geometry. No typography
change was necessary. The neutral `content.MeasureIntrinsic` wrapper reserves the
exact snapshot/session pair for the entire invocation, including definite boxes
that skip callbacks. It qualifies core group/owner identities and calls an
explicit `GroupMeasurer` with the retained handle and value-returning edge/atomic
views. Mutable tree nodes never escape. Missing coverage and foreign/closed pairs
fail without preparation or service work.

The typography adapter supplies ordered atomic ranges from core-provided border
bounds plus horizontal margins exactly once. Each CSS-pixel primitive is converted
to physical typography DU, signed sums use int64, and both input and summary
rounding are checked against the finite domain. Padding/borders are not added
twice, negative advances are not clamped prematurely, and owner/vertical edges do
not enter the paragraph measurement. Retained `Prepared.MeasureIntrinsic` performs
no final heights, line breaking, vertical metrics or geometry publication.

**Exit satisfied:** repeated measurements reuse base preparation; changed atomic
requests and root percentage bases produce fresh answers. Requests are consumed
without retention. Tests cover nested auto-width dependencies, zero/empty input,
signed/fractional margins, range failures, qualified tree/session coverage,
reentrancy, late failure with zero published output and unchanged source/banks.
Unsupported percentage cycles and manual alternatives preserve causes and leave
the session/previous outputs usable; failed lazy discovery retries and complete
batches are reused. Source-aware failures use StageMeasurement and retain upstream
paragraph stages. Typography still imports no concrete font backend; content/CSS
core import no typography. PREP-04 adds whole-tree used sizing/layout separately.

### PREP-04 — Produce independent whole-tree trial results (implemented)

**Owner:** pagelayout orchestration and typography bridge.

`preparation/typography.Layout(prepared, session, root, LayoutRequest)` runs a
covered block container/subtree at a CSS-pixel containing width. FirstFit must be
selected explicitly; zero width is valid. The initial profile uses CSS normal
line height, zero side skips and bottom-margin-edge atomic alignment. Percentage
dependencies and manual hyphenation retain the existing engine restrictions.

Neutral `content.Layout` reserves the exact pair, checks every requested group's
coverage before services, then runs existing ResolveUsedValues/FlowLayout.
GroupLayouter combines measurement and layout against the same retained handles.
Callbacks receive qualified group/owner Boxes and borrowed value views, never raw
tree nodes. Each call owns fresh used values, callback tracking and a collector;
repeated final callbacks are rejected within that invocation, including failures.
Nothing is added to source trees, registers or core InlineResult payloads.

The bridge calls retained Prepared.Trial followed by Trial.Assemble. Core supplies
completed atomic border dimensions; margins are converted independently, with
checked units/ranges and no duplicate padding/border contribution. Fractional
paragraph placements remain big points in the typed result; only the CSS summary
is narrowed. No preparation or receiving style cascade is repeated.

Success returns a Result with Root/Groups/Group/Geometry accessors. Geometry is
keyed by qualified content.Box; RelativeTo is a tree-local BoxID. GroupResult
contains copied source/paint/atomic metadata and frozen paragraph output, whose
coordinates are relative to the owner's content origin. Maps, slices and mutable
envelopes are copied on inspection; frozen paragraphs/VLists remain shared
immutable values. Results retain source/font resources but no session,
preparation, discovery or trial workspace. Borrowed fonts must stay valid.
Only complete subtree success publishes the bundle; failure returns nil and
preserves source-associated PreparationError and upstream paragraph causes/stages.
No page fitting or bank advancement is implied. No sibling API change was needed.

**Exit satisfied:** the unchanged nested fixture produces coexisting results at
120px and 240px without second base shaping; later success/failure and session
Close preserve them. Tests cover source-tree/bank/manifest stability, subtree and
session identity, reentrancy, incomplete coverage, callback rejection, exact
atomic borders/margins/coordinates, zero width/empty groups, request and finite
range validation, percentage-cycle failure, and late vertical-metric failure
followed by successful retry. PREP-05 adds real-font proofs, executable handoff
and GC lifetime evidence; BOX-PREP-02 completes the independent terminology cleanup.

### BOX-PREP-02 — Clarify names for realized atomics (implemented)

**Owner:** css-box-layout, with tytui and pagelayout consumer updates.

CSS core now uses private `layoutAtomicInteriors` and public `RealizedAtomic` /
`RealizedAtomicsView`. They replace `prepareAtomics`, `PreparedAtomic` and
`PreparedAtomicsView` without compatibility aliases. The view returns copied
border-box dimensions in CSS pixels for the current layout invocation, including
padding/borders and excluding margins. The outer paragraph supplies placement
separately through the unchanged `AtomicPlacement` contract.

Pagelayout's `content.InlineLayoutRequest` and both consumers' adapters/tests use
the new names. Core and consumer guides, pseudocode and diagrams describe atomic
interior layout explicitly. Reusable `paragraph.Prepared`, group/session handles,
intrinsic bounds and their ownership remain unchanged. This is a source-level
API rename with unchanged geometry, callback ordering and publication semantics.

**Exit satisfied:** core and both consumer regressions pass, including nested
atomics, dimensions/margins/placements, reuse, failures and retained results.
The terminology migration is complete and needs no subsequent alias removal.

### PREP-05 — Prove lifetime, bank persistence and the handoff (implemented)

**Owner:** pagelayout; cross-module regression checks for touched boundaries.

The [executable two-width example](../preparation/typography/example_test.go)
selects a checked-in Gentium Plus font and concrete services explicitly, prepares
once, lays out one branch at 120px and 240px, and retains output after Close.
Its four prepared groups include the empty residual group. The unchanged nested
acceptance fixture separately produces three groups and the
[two-width outline proof](../preparation/typography/testdata/two-widths.svg).
The proof renders retained glyph paths and CSS geometry; it performs no layout.
See [the visual example and font provenance](../preparation/typography/testdata/README.md).

Real-font integration tests verify repeated measurement/layout without base
reshaping under `hyphens:none`, discarded and later successful candidates,
original/derived bank persistence, late preparation failure with no partial
publication, and new preparations of the same boxes with different font resources.
A sibling capability-reduced Gentium derivative changes font identity without
changing the ASCII source; resource identity cannot be cached by HTML alone.
Separate reloads with changed source size/paint or manual policy preserve old
metadata/results. Their identical structure deliberately collides in local
BoxIDs/TreeIDs, proving that snapshot-qualified groups cannot alias.

GC tests retain both two-width results, the manifest and source bank. They check
that closed group handles, paragraph preparations, backend, session wrapper (when
dropped), shaping streams and per-call collector become collectible. Both a
retained closed wrapper and a dropped wrapper are exercised. Manual policy first
populates lazy discovery; a weak factory/hyphenator canary proves release of its
transitive recipe/discovery ownership without exposing sibling internals. The
sibling paragraph lifetime suite checks raw discovery/trial/workspace pointers.
After GC, a fresh explicit outline adapter still renders glyphs from the retained
font resources. No finalizer timing, unsafe access or production GC hooks are used.

Concrete OpenType/render imports are confined to tests and the executable example.
Production dependency checks still enforce neutral content/core and no concrete
font backend in the optional bridge. The indirect OpenType module dependency
supports those test adapters. No production or sibling API changes were required.
Session and shared-service operations remain caller-serialized, including Close;
manual alternatives may invoke services lazily. Initial policy/atomic limits in
section 12 and the [bridge guide](../preparation/typography/README.md) still apply.

**Exit satisfied:** the acceptance matrix below is covered by the combined
PREP-00 through PREP-05 tests. No automatic frame assignment,
page-template selection, bank commit, within-box remainder or general FitInline
is implied. Preparation is complete for its explicitly admitted profile; page
trials and wider policy support have their own milestones.

### Dependencies and optional extensions

```text
BOX-CONT-01 + BOX-CONT-02 -> CONT-04 -> CONT-05 -> PREP-00
PREP-00 -> BOX-PREP-01 -> PREP-01 -> PREP-02 -> TYPE-PREP-01
TYPE-PREP-01 -> PREP-03 -> PREP-04 -> PREP-05
BOX-PREP-02: independent terminology migration, implemented
```

No new khipu or w3c-dom feature is a prerequisite for the restricted experiment.
For wider hyphenation admission beyond PREP-00's restricted profile, schedule KHIPU-PREP-01 for
per-span policy and intrinsic-measurement support, with typography facade tests,
before claiming that wider preparation profile. BOX-SHIP-03 supplies percentage
atomic layout with definite bases and SHIP-04 connects CSS-level continuation.
Page construction remains separate shipout work; do not narrow the loader's
accepted unsized material to hide remaining integration limits.

## 14. Preparation acceptance matrix

| Test | Required observation |
| --- | --- |
| Load then prepare | No fonts during loading; no geometry during preparation; existing bank and source inspection unchanged |
| Two branches with colliding local BoxIDs (identically structured source snapshots) | Qualified group identity keeps preparations and results distinct; branches within one snapshot are also tested |
| Two preparations of one content snapshot | Same content.Box payloads, distinct preparation identities; session mixing rejected |
| Nested auto-width atomic | Outer preparation contains only occurrence; inner group prepared separately before any measurement |
| Empty/atomic-only paragraph | Owner source typography supplies strut context; first text run is never the default |
| Whitespace, NBSP, soft hyphen, literal U+FFFC, isolates | Exact processed-byte/source correspondence; supported policy preserved, unsupported policy diagnosed |
| Failure in last group | Neither prepared snapshot nor session published; original document/bank and previous preparation usable |
| Repeated measurement and two-width layout | Base shaping count unchanged; atomic contributions/realizations are explicit request inputs |
| Reject trial or fail a later callback | Earlier candidate, preparation metadata and bank version unchanged; no partial new output |
| Change fonts, source style or preparation policy | New preparation/session; old results remain stable; no HTML-only cache key |
| Release session while retaining frozen output | No preparation/discovery retention through results or bank; needed font resources remain usable |
| Dependency and concurrency boundaries | Neutral content/core imports; no hidden OpenType defaults; serialized session use documented and tested |

Run focused tests for each PR and the relevant full/race suites on changed
boundaries; test races under the supported serialization protocol rather than
asserting concurrent execution is supported. Add build/vet and the tytui
regressions for shared core/typography changes. Documentation-only planning does
not require running Go tests.

## Diagram sources

Editable Graphviz `.dot` files and their rendered `.svg` previews live in
`doc/diagrams/`. From the repository root, regenerate the previews with:

```sh
dot -Tsvg doc/diagrams/preparation-pipeline.dot -o doc/diagrams/preparation-pipeline.svg
dot -Tsvg doc/diagrams/preparation-interactions.dot -o doc/diagrams/preparation-interactions.svg
```

The diagrams distinguish reusable loading/preparation from per-receiver geometry.
Finite-height fitting is now implemented through SHIP-04; page-level transactions
remain planned in SHIPOUT-PLAN.md.
