# Pagination and shipout implementation plan

Recorded on 2026-11-25 against the current implementation and the decisions in
[SHIPOUT.md](SHIPOUT.md). This is a plan, not a report of implemented pagination.
**SHIP-00 is complete as a contract-and-fixture slice; DOM-SHIP-01 is implemented
in w3c-dom; SHIP-01 is implemented in content; BOX-SHIP-01 is implemented
in css-box-layout; SHIP-02 source planning, SHIP-03 state/loss/outcome contracts,
BOX-SHIP-02 structural finite-height fitting, TYPE-SHIP-01 paragraph-prefix
fitting for `none`, KHIPU-SHIP-01 manual discretionary continuation and
TYPE-SHIP-02 manual paragraph-prefix fitting and BOX-SHIP-03 used fragment
geometry/atomics/paint facts, SHIP-04 content/typography fitting, SHIP-05
single-channel Page attempts, SHIP-06 multi-channel arrangements, SHIP-07
deterministic pagespan beam search and SHIP-08 span transitions/shipout handoff are
implemented.** Page construction includes placement-dependent activation,
FIFO queues and transitive draining, with exact remainders, frozen pages and
factual loss. Search selects one complete span with explicit truncation and
interruption reporting. Document pagination preserves earlier selections on later
failure and separately delivers only selected frozen pages. SHIP-09's broader
end-to-end and search-quality proofs remain planned.
Scoring and numerical execution limits remain working hypotheses.

The author's initial remarks, scattered remarks and closing answers remain in
SHIPOUT.md, unchanged. That document remains the design authority. This plan
preserves its distinction between **settled decisions**, **adopted working
hypotheses**, **open details** and **deferred work**. Scheduling a hypothesis for
implementation does not promote it to a settled design decision.

## 1. Intended result and boundaries

The first complete milestone takes a compiled template catalog and fully
prepared content, constructs immutable pages, chooses a template sequence for
each pagespan, and delivers the selected frozen pages to a consumer. Every
selected sequence carries its exact remainder and an inspectable loss breakdown.
Source styles, prepared material, competing candidates and earlier output remain
unchanged by successful or failed attempts.

The responsibility split is:

| Owner | Responsibility |
| --- | --- |
| `w3c-dom` | Checked CSS values, defaults and cascade; no pagination policy. |
| `css-box-layout` | Normalization provenance, structural fitting, legal box cuts, used edges, margins and exact structural continuation. |
| `khipu` | Paragraph break decisions, discretionary carry and certified accepted-prefix continuation. |
| `typography` | Paragraph fitting alternatives, matching atomic bindings and frozen renderable paragraph fragments. |
| `pagelayout/content` | Private normalized-tree access, qualified source positions and preparation identity; neutral adapter to core fitting. |
| `pagelayout/preparation/typography` | Connect prepared paragraphs and atomic interiors to the neutral fitting protocol. |
| Root `pagelayout` | Span/ownership plan, immutable channel state, page construction, loss, template-sequence search and delivery of selected pages. |
| Output consumer | Paint/export frozen pages, without choosing breaks or advancing registers. |

Use the existing generic registers with `content.Box` payloads. Prepared
snapshots and execution sessions stay separate. An unfinished branch stays at
the register head with a separate continuation; neither a prepared wrapper nor
a clipped box tree replaces that payload. No register redesign is required.

The first fitting profile covers ordinary block sequences, splits between
paragraph lines and indivisible inline atomics. The adopted paragraph breaking
policy follows the existing explicit FirstFit bridge, normal line height and
bottom-margin-edge atomic alignment; page construction enumerates legal cuts in the resulting
paragraph layout, not every possible paragraph line-breaking solution.
Preserve the existing preparation distinction between uniform `none` and uniform
`manual` hyphenation. An early `none`-only proof is useful, but does not complete
the manual-continuation work identified below. Existing explicitly diagnosed
preparation limitations remain visible; pagination must not silently change
source policies to evade them.

Out of scope are floats and their special frames, automatic hyphenation,
repeated/generated running material, arbitrary table fragmentation, recto/verso
parity or blank-page insertion, concurrent attempts, incremental preparation,
state merging, remaining-cost heuristics, automatic beam widening, and a new
PDF/browser/CLI product. A small rendering proof is part of validation; a new
general output backend is not required.

## 2. Existing foundations and actual gaps

This table records the starting inventory when the plan was written. The slice
statuses in sections 4–5 describe which of these gaps are now implemented.

| Existing foundation | Reuse | Missing capability |
| --- | --- | --- |
| [Template catalog](../template/catalog.go) | Immutable compiled geometry/paint, definition order and per-channel frame order. | Page instances and placed content; no new cascade or template compiler is needed. |
| [Content routing](../content/route.go), [normalization](../content/normalize.go) and [Document](../content/document.go) | Detached styles, source identities, branch order and persistent banks. | Residual-to-`main` integration; valid explicit `main` must stop being extracted after routing validation. |
| [Content profile](../content/profile.go) | Source declaration auditing and effective-value checks. | Admission of `break-before`, effective forced-break metadata and owning-anchor positions. |
| [Group extraction](../content/extract.go) | Processed UTF-8 text, source maps, policy spans and atomic occurrences. | A complete mapping for structural and zero-width events through normalization and preparation. |
| [Preparation](../content/prepare.go) | Eager preparation barrier, immutable snapshot, separately owned serialized session. | Fitting operations that reuse the pair and retain exact continuations. |
| [Whole-tree layout](../content/layout.go) and [typography adapter](../preparation/typography/layout.go) | Used values, atomic interiors, source/paint copying and frozen output ownership. | Finite-height partial results, multiple alternatives/fragments of one group, and resumable collection of output. |
| [CSS layout core](../../css-box-layout/layout/flow.go) and [structural fitting](../../css-box-layout/layout/fit.go) | Structural flow, margins and whole-tree geometry; finite-height zero-edge block fitting with neutral callbacks and open-container continuation. | Used fragment geometry, edge omission, margin collapse and atomic integration in the finite-height fitter. |
| [Paragraph trials](../../typography/glyphing/paragraph/trial.go), [checkpoints](../../typography/glyphing/paragraph/trial_checkpoint.go) and [prefix fitting](../../typography/glyphing/paragraph/prefix_fit.go) | All fitting whole-line prefixes for `none` and `manual`, selected fragment/checkpoint/atomic associations, logical/cluster/replacement coverage and changed-width suffixes. | The content fitting bridge. |
| [Paragraph flow convenience API](../../typography/glyphing/paragraph/flow.go) | Useful reference for whole-line continuation. | It fixes atomic inputs and currently restricts hyphenation; it is not the complete page-fitting backend. |
| [Khipu checkpoints](../../khipu/checkpoint.go) | Immutable accepted-prefix certificates, exact manual replacement carry and suffix atomic rebinding. | Automatic continuation and recursive replacement-tail splitting remain unsupported. |

Do not treat `content.Layout` followed by geometric clipping as fragmentation.
Do not treat a longest-prefix paragraph convenience call as enumeration of all
local page choices. Neither can supply the required structural remainder alone.

## 3. Rules every slice must preserve

1. **Fixed phases:** load/box, then prepare all material, then paginate. Width-
   dependent line breaking and atomic realization occur during pagination;
   source preparation is not repeated for each page. Lazy manual alternatives
   may still use the explicitly supplied services.
2. **Authoring order:** validate routing before ignoring redundant
   `flow-into: main`. Never reorder secondary queues by owning references or
   bypass their heads. Differences between anchor order and queue order are not
   malformed input by themselves.
3. **Two kinds of coordination:** explicit `data-flow-anchor` relationships
   permit deferral and do not close pagespans. Forced page-break variants of
   `break-before` normalize to `page`; those boundaries and document end require
   all related secondary material to drain, transitively. Future-span material
   remains untouched. Markers are out-of-band.
4. **Exact remainder:** placed logical material followed by the remainder is
   the input material in order, without loss or duplication. At most one partial
   branch exists per channel, at its head. Logical positions support inspection;
   the opaque continuation additionally retains structural and paragraph state.
5. **Fragment edges:** a split box retains top edges only on its first fragment,
   bottom edges only on its last, and side edges throughout. Unsplit descendants
   keep their own edges. The adopted margin contract uses core collapse rules
   inside a frame and no collapse across frames.
6. **One result per template:** a Page attempt selects its cut-offs from current-
   page facts only. Local alternatives never become extra global successors.
   Whole-page construction may need shorter prefixes than a greedy frame fill.
7. **Deterministic ties:** within a template, compare loss, then further logical
   `main` progress, then secondary progress in first-occurrence source order,
   then stable arrangement order. Between equal-loss templates prefer catalog
   order; equal-loss sequences compare template ordinals lexicographically.
8. **Loss:** retain components and a scalar; nonnegative finite values precede
   a distinct absorbing blocked value. Invalid arithmetic is an error, not a
   blocked assessment. Charge each pending target once per carrying page,
   starting on its owning-anchor page if it remains incomplete.
9. **Search:** default 32 unfinished candidates, complete page-depth rounds,
   accumulated loss then lexicographic template sequence, separate best complete
   incumbent, no merging or guessed remaining cost. Do not stop at the first
   completion. A valid minimum-next-page-cost bound prunes only on strict
   greater-than; equality can still improve the sequence tie.
10. **Functional publication:** an attempt returns a complete candidate and
    successor together or neither. Rejection cannot advance shared banks.
    Selected span pages and their final state are committed together. Frozen
    output has a different lifetime from resumable execution state.

## 4. Implementation order

Each row is a separately reviewable slice with its own acceptance checks.
Dependencies indicate completion prerequisites, not permission to spread one
change across all repositories. Contract work and test doubles can precede the
real backend, as noted in the detailed steps.

| Order | Slice | Owner | Depends on | Deliverable |
| --- | --- | --- | --- | --- |
| 1 | SHIP-00 — complete | pagelayout, with sibling contract review | Existing loading/preparation | Adopted contracts and checked-in fixture expectations; runtime implementation remains pending. |
| 2 | DOM-SHIP-01 — complete | w3c-dom | SHIP-00 | Checked break-before metadata, parsing and cascaded resolution. |
| 3 | SHIP-01 — complete | pagelayout/content | SHIP-00, DOM-SHIP-01 | Default main and detached source events. |
| 4 | BOX-SHIP-01 — complete | css-box-layout | SHIP-00 | Normalized source-boundary provenance. |
| 5 | SHIP-02 — complete | pagelayout/content and root | SHIP-01, BOX-SHIP-01 | Validated ownership/span plan and mapped positions. |
| 6 | SHIP-03 — complete | pagelayout root | SHIP-00, SHIP-02 | Immutable state, loss and outcome contracts. |
| 7 | BOX-SHIP-02 — complete | css-box-layout | SHIP-00, BOX-SHIP-01 | Structural finite-height fitting with neutral paragraph callbacks. |
| 8 | TYPE-SHIP-01 — complete | typography | SHIP-00 | Enumerated paragraph prefixes and frozen fragments for `none`. |
| 9 | KHIPU-SHIP-01 — complete | khipu | SHIP-00 | Manual discretionary checkpoint/carry support. |
| 10 | TYPE-SHIP-02 — complete | typography | TYPE-SHIP-01, KHIPU-SHIP-01 | Manual paragraph continuation through the same API. |
| 11 | BOX-SHIP-03 — complete | css-box-layout | BOX-SHIP-02 | Used-value, margin, atomic and fragment-paint integration. |
| 12 | SHIP-04 — complete | pagelayout/content and preparation/typography | SHIP-03, BOX-SHIP-03, TYPE-SHIP-02 | Real resumable fitting bridge. |
| 13 | SHIP-05 — complete | pagelayout root | SHIP-02, SHIP-03, SHIP-04 | Single-channel Page attempt and two-template proof. |
| 14 | SHIP-06 — complete | pagelayout root/content, typography adapter; css-box-layout composition support | SHIP-05 | Multi-channel local arrangements, anchors and draining. |
| 15 | SHIP-07 — complete | pagelayout root | SHIP-03, SHIP-06 | Deterministic beam search, eligibility and search outcomes. |
| 16 | SHIP-08 — complete | pagelayout root/content | SHIP-07 | Span transitions, selected output and shipout handoff. |
| 17 | SHIP-09 | pagelayout, sibling regression checks | SHIP-08 | Real-font, lifetime, search-quality and end-to-end proofs. |

BOX-SHIP-02 and TYPE-SHIP-01 use the contracts pinned in SHIP-00 and test doubles;
they must not import one another. The SHIP-07 search algorithm can likewise be
developed against a fake Page attempt after SHIP-03, but is not complete until
integrated with SHIP-06. SHIP-04 supports `none`, `manual` and atomics; SHIP-05 may start with a smaller
page-construction fixture;
their final acceptance includes the later sibling dependencies in the table.

## 5. Detailed slices

### SHIP-00 — Pin contracts and acceptance fixtures

**Status: complete (2026-11-25), design and fixtures only.** The user adopted the
contract walkthrough, retaining scoring and numerical limits as working
hypotheses. The [adopted contract](SHIPOUT.md#adopted-page-construction-contract-ship-00)
is authoritative; no new loading/fitting/pagination runtime is implied.

| Contract | Adopted choice |
| --- | --- |
| Public operation | Immutable content-derived plan; serialized execution object binds catalog, preparation/session, neutral fitter and fixed options. Explicit input states; no mutable current position. Plan/NewPagination, InitialState, PageAttempt and one-span PaginationSearch form the agreed API shape. |
| Identity | Opaque source/preparation/execution identities, plan-local branch ordinals and logical positions, separate page-local fragment occurrences. Reject foreign cursors/context before execution; copied inspection only. |
| Main integration | Existing residual root becomes the sole main item when present. Channels lists main first; Branches(main) and Residual inspect the same tree. No duplicate preparation; original nested-route validation precedes ignoring valid explicit main. |
| Anchors | One active explicit owner per active secondary branch; unique same-document branch-root target and acyclic ownership to main. Visible links activate once from rendered source coverage. Empty links attach forward within their block, otherwise backward, otherwise structurally. Suppressed/inert owners are inactive; active/suppressed mismatches are diagnosed. |
| Breaks | Auto plus page/left/right/recto/verso, normalized to page; initial/inherit/unset resolved normally. Main block-level locations outside atomics only. First-child propagation, coalescing and terminal drain; no leading blank page. Other values, rollback and legacy aliases unsupported initially. |
| Fitting | Existing auto-height profile; block/line cuts and indivisible atomics. Actual receiving widths, no cross-frame margin collapse, adopted edge omission. FirstFit with none/manual policies and explicit unsupported cases. |
| Enumeration | Synchronous callback with exhausted/stopped/error completion; retained immutable alternatives. Main then source-ordered channels, chain-ordered frames, further-progress prefixes first and legal empty placements. Validate owning eligibility against the complete arrangement. |
| Loss representation | Tagged blocked/uint64 units, one million units per point, one nearest-even rounding per weighted component, checked sums, exact comparisons and explicit arithmetic errors. |
| Reporting/publication | Constructed/NoFit/already SpanComplete; errors publish no page. Interrupted search may report a completed unselected incumbent. Atomic complete-span selection and exact transition; separate delivery with progress/error reporting; frozen output survives session close while borrowed resources remain valid. |

**Working scoring hypotheses:** page 1, underfill up to 1, one-line split-fragment
10, carry 1 per pending target/page and missing main receiver 1, in cost points.
Underfill participation is fixed from attempt-entry state after immediately due
events: main if material remains, secondary channels if already pending. Newly
activated targets pay carry immediately when incomplete but enter underfill on
the next attempt. Use area-weighted squared unused-height fractions, zero for an
empty participating set, and the fitter's allocated extent including surviving
edges. Exempt the finishing and later main frames at span end while retaining
the original area denominator; earlier gaps remain. Count one-line fragments per
paragraph/frame, coalescing internal subdivisions; unsplit one-line paragraphs
are exempt. Secondary missing receivers add no charge beyond carry.

**Working numerical limits:** 100,000 local enumeration steps (including partial
arrangements) per attempt; 1,000,000 attempts and 10,000 depth rounds per span.
Configurable, with no implicit wall-clock deadline or automatic retry/widening.
Limits/cancellation are explicit interruption, never NoFit or proven infeasibility.
These caps do not change the settled configurable default-32 unfinished beam.

**Fixtures and expected results:** [testdata/pagination](../testdata/pagination/README.md)
contains separate content/template HTML, an additional stylesheet, and six
machine-readable tables:

- `source-expected.json`: routing, marker mapping, affinity, ownership, suppression
  and source/effective profile failures, with stable HTML-ID labels.
- `template-expected.json`: exact catalog definition order, chains and content rectangles.
- `fitting-expected.json`: declared synthetic inputs for exact remainders, edge
  capacities, changed widths/atomics, manual carry, local ties and draining.
- `scoring-expected.json`: explicit provisional page facts/costs plus settled
  arithmetic boundary cases.
- `search-expected.json`: complete finite fake Page-attempt graphs, expected
  sequences/outcomes, pruning and interruption observations.
- `protocol-expected.json`: identity, atomic publication, lifetime, enumeration
  interruption and delivery expectations with explicit scenarios.

The synthetic tables isolate contracts from unspecified font metrics. Their
expectations are independently authored, not generated from a paginator. Positive
source cases describe the adopted future profile; the current loader does not
admit break-before yet. Later slices implement executable consumers and extend
these fixtures with real-font and lifetime proofs; the fixture presence does not
claim that those later acceptance gates pass.

**Remaining implementation gates:** exact supporting Go declarations/private
cursor representation and diagnostics belong to SHIP-03/04 and sibling fitting
slices; source-to-normalized/prepared mappings to SHIP-01/02 and BOX-SHIP-01;
eligibility predicate declarations to SHIP-07; frozen consumer payload types to
SHIP-08. The recorded behavior is fixed except for explicitly provisional scoring,
limits and the retained eligibility baseline. Calibration does not reopen settled
source ordering, remainder, tie or beam rules.

### DOM-SHIP-01 — Checked `break-before` support

**Status: implemented.** w3c-dom supplies the non-inherited `auto` default,
`style.ParseBoxKeyword("break-before", value)` and
`css.GetBoxKeyword(node, "break-before")` through the existing shared declaration
normalization and cascade. Its full Level 3 vocabulary preserves page-side,
avoidance, column and region values. The narrower pagination profile and
page-value normalization are implemented by SHIP-01. Checked resolution supports
initial/inherit/unset and reports unsupported rollback/deferred values. Tests
cover malformed declarations through inline, inspected and legacy stylesheet
paths, cascade precedence and retention of losing/nonmatching source declarations.

**Work:** add the property metadata/default and shared checked value support
needed by the agreed profile. Reuse w3c-dom's declaration parsing and computed-
value resolution conventions, including explicit inheritance and CSS-wide
keywords. Keep source-audit grammar and effective-value grammar consistent.
An extension to the existing keyword helper may suffice; avoid a second parser
in pagelayout.

w3c-dom returns checked property meaning. It must not know that this client
treats `recto` and `verso` as `page`, create pagespans or route material.

**Acceptance:** tests cover defaults, valid forced values, malformed values,
CSS-wide resolution and normal cascade precedence. Losing/nonmatching
declarations remain available for pagelayout's profile audit. Existing template
and text-style behavior is unchanged. No font or paginator dependency is added.

### SHIP-01 — Default main and source event capture

**Status: implemented (2026-11-25).** Default main aliases the residual tree;
`SourceEvents()` returns copied original-node ID/anchor/break records and
`SourceInfo.BreakBefore` retains normalized effective policy. Excluded IDs and
anchors remain inactive records. Ownership validation and content integration
of normalized/prepared positions are implemented in SHIP-02 using BOX-SHIP-01
normalization provenance. Tests cover main equivalence,
route validation, preparation once per group, cascade and declaration auditing.

**Work:** adapt routing in two stages: validate the original effective flow
directives, then treat valid `main` directives as redundant for extraction.
Keep residual content in source order and expose it as the main material without
duplicating its tree or preparation groups. Implement the public inspection
migration agreed in SHIP-00 and update the relevant executable examples.

Admit/audit `break-before` through the checked sibling API; normalize admitted
forced page variants to one effective page event. Capture original source
identity and the effective value before partitioning changes the working tree.
Collect owning-link presence, `href`, target IDs and source positions as detached
metadata. Do not infer an owner from ordinary xrefs or target source proximity.

Generic content loading preserves the facts needed by pagination. The whole
ownership graph and span validation belong to the pagination plan in SHIP-02;
ordinary standalone content users should not need a template catalog merely to
load styled material. Apply the SHIP-00 split: content preserves source facts;
root Plan validates the ownership graph and span plan.

**Acceptance:** residual-only and redundant-main documents have identical main
order/styles. Nested routes still fail, including two `main` declarations.
Named queues keep source order. All prepared groups are covered once. Forced
variants produce identical events without parity effects; losing declarations,
additional stylesheets and original diagnostic sources remain correctly audited.
Metadata introduces no text, fake boxes or changed widths.

### BOX-SHIP-01 — Normalization boundary provenance

**Status: implemented (2026-11-25).** The opt-in sibling API
`BuildLayoutTreesWithProvenance` returns the same trees plus a detached
`NormalizationProvenance`. `Sources()` exposes source before/after mappings
and retained occurrences; `Boundaries()` exposes per-root structural enter/exit
order and anonymous-group identities. Split/hoist tracing stays inside the
builder. Empty, suppressed and omitted endpoints are explicit, without guessed
positions. Inputs require unique source IDs, including suppressed descendants;
zero is allowed. TextRef ranges remain unchanged. Copies, independent builds,
fixed roots, omitted whitespace and empty-inline fixtures are tested.
Content integration and consumer affinity/prepared-position mapping are implemented
by SHIP-02;
structural positions alone do not establish legal fragmentation opportunities.

**Work:** expose enough neutral normalization provenance to map requested source
boundaries into structural/group positions. Reuse existing source IDs and text
ranges where sufficient; add detached boundary records for cases they cannot
represent reliably. Establish this with empty-inline, split-inline, hoisted-block,
anonymous-group and omitted-whitespace fixtures before choosing the smallest
API extension.

The builder is the authority on its normalization. The content adapter must not
replicate that algorithm or guess a position by numeric BoxID order. If source
boundary requests are passed into the builder, keep them in a separate input
collection and return copied mapping data. Do not add layout-affecting marker
nodes. Records may contain neutral source/box/group identities and before/after
positions, but no anchor, flow-channel, page or font semantics.

Define the ordered structural boundary vocabulary used by BOX-SHIP-02. Preserve
the distinction between source identity, generated box identity and multiple
normalized occurrences of the same source. An omitted/suppressed position needs
an explicit mapping disposition, not a silently fabricated nearest box.

**Acceptance:** a provenance map survives each normalization fixture with
unchanged geometry/text and independent builds. It explains empty/omitted cases,
does not publish mutable nodes, and keeps IDs local to their build. Together
with content's tree/snapshot identity it supports unambiguous inspection.

### SHIP-02 — Ownership graph, span plan and position propagation

**Status: implemented (2026-11-25).** `pagelayout.Plan(document)` publishes an
immutable `SpanPlan` with copied branches, anchors, span boundaries and transitive
target membership. `PlanError` retains diagnostic codes and related sources.
Source identity is opaque; plans preserve original FIFO queues and never activate
or consume material. Main-only block breaks propagate through actual first
normalized children, coalesce and retain initial/terminal semantics.

Content stores BOX-SHIP-01 provenance, exposes `EventPositions`, and carries
copied `GroupInput.Events` through extraction/preparation and the typography
bridge. Logical source positions, processed bytes and original decoded ranges
remain separate. Empty/whitespace anchors use following/preceding/structural
affinity within their original containing block. Visible coverage remains one
source activation even across fragments; actual placement/once-only activation
is runtime work in subsequent slices. Glyph/cluster mapping belongs to paragraph
fitting. Source-table fixtures, preparation lifetime/copy tests and independent
plan inspection tests cover this slice; no finite-height fitting is implemented.

**Work:** compile the detached source events and normalized mappings into an
immutable plan tied to the content snapshot. Carry inline events through group
extraction/preparation as metadata alongside processed text. Never insert a
glyph, U+FFFC atomic marker or khipu penalty to represent a page/anchor event.
Handle a marker in text collapsed to zero bytes according to the explicit
SHIP-00 affinity rule. Do not assume source byte offsets equal prepared offsets.

Implement the adopted ownership contract: one explicit same-document owner
per independently scheduled secondary branch, targeting its unique branch-root
ID; diagnose unresolved/duplicate/inactive targets, duplicate ownership, cycles
and chains not rooted in main. Ordinary xref cycles are irrelevant. Suppressed
owner/target cases use the pinned source policy and retain useful diagnostics.

Assign secondary branches transitively to main spans, independently of queue
order. Activation remains placement-dependent: visible anchors activate with
their first placed content fragment, once; empty anchors use their mapped
boundary. A future target is not pending just because it is prepared. A target
owned from a secondary branch becomes eligible only when that anchor is placed.

Implement the adopted main-only forced-boundary contract; diagnose
active secondary forced breaks as unsupported. Coalesce identical boundaries,
handle an initial boundary without a blank page, and add a terminal draining
boundary. The plan must stop within a main branch as well as between branches.

**Acceptance:** copied plan inspection identifies source, branch and logical
position. Fixtures prove visible, empty, multiline and transitive anchors;
suppression; coincident/terminal boundaries; and a span ending inside one main
branch. `[A, B]` remains `[A, B]` when B's owner is first. Cross-span queue
obstructions remain observable and are not repaired or mislabeled as bad HTML.

Review update (2026-11-25): DOM-SHIP-01, SHIP-01, BOX-SHIP-01 and SHIP-02 were
checked against their source/normalization contracts. SHIP-02 fixes now cover
retained empty blocks in affinity, atomic interiors as indivisible from outside,
and canonical ordering of group-byte positions against nested structural
positions. Regressions include following-span assignment, outer-group preparation
metadata and ordering consistency. Invalid zero plans no longer match zero source
handles. Repeated tree indexing/event copying was removed from the mapping path.
Actual placement, fragment/cluster activation and page execution remain in their
scheduled later slices, not claimed by this source review.

### SHIP-03 — Immutable state, loss and outcome contracts

**Status: implemented (2026-11-25).** `NewPagination(PaginationConfig)` binds a
copied plan/catalog, matching prepared snapshot/borrowed session and fixed
policy/limits/beam settings. `InitialState` and `Validate` provide opaque
context-qualified immutable state and copied head/activation/pending inspection.
Private tested transitions retain unfinished heads and their paired positions,
clear completed heads/cursors together, and validate draining before span
advancement. State retains no session or backend. `PreparationSession.Check`
validates availability/identity without backend calls.

`Loss` is private tagged finite/blocked uint64 units with checked sums and
ties-to-even component quantization. `AssessPage` consumes detached factual
assessments, using exact rational underfill intermediates and exact integer
coefficients/counts. It preserves component contributions and entry participation;
the validated positive page weight supplies the search lower bound.
Scoring weights, participation rules and numerical limits remain **working
hypotheses**, not newly settled calibration decisions.

Page/search result contracts, typed cause-preserving diagnostics, separate local
and sequence comparators, and cooperative work budgets are implemented and tested.
Publication helpers reject interrupted/incomplete attempts and retain only
completed, explicitly unselected interrupted incumbents. All scoring fixtures are
executable; state, arithmetic, identity, cursor, comparator and publication tests
pin this slice's protocol boundary.

Private synthetic advances test the original state contract. SHIP-04/05/06 now
supply actual opaque checkpoints and multi-channel page construction; SHIP-07
supplies pagespan search. Initial state preserves source registers without
inferring fit or manufacturing metadata-only pages. Candidates pair completed
page data, loss and exact successors; their separate frozen Page handles retain
rendering without execution. SHIP-08 below supplies general selected-page delivery.

**Work:** introduce root-level pagination initialization and opaque immutable
state. Bind the bank, per-head continuation, pending/activation facts, current
span, next page number and fixed policy/preparation identity. Return copied
inspection records. Validate empty-bank/head/cursor combinations and reject
foreign source or preparation identities before execution.

Use a logical position comparison independent of widths, line counts, heights
and numeric BoxIDs. Comparing inspection positions is not permission to exchange
continuations or merge candidates. Continuations hold the structural/checkpoint
facts needed by later fitting; state never puts execution services into boxes.

Implement tagged blocked/finite loss and checked component aggregation using
the numeric contract pinned in SHIP-00. Keep factual page assessment separate
from configurable weights. Test the provisional score from synthetic page facts
before depending on real fitting. Retain a positive lower bound valid under the
fixed policy for search pruning; do not hard-code the default one-point value
when policy configuration can change it.

Implement the adopted page outcomes matching SHIPOUT: Constructed, NoFit, already
SpanComplete, and typed invalid-request/unsupported/execution errors. Interrupted enumeration
cannot publish its current best as a completed Page attempt. Search result
types must separately represent completion, proven failure under the policies,
heuristic exhaustion, cancellation and limits. Expose a completed incumbent after
interruption only as explicitly unselected, without presenting it as a successful
optimum or automatically committing it.
Diagnostics identify the template/frame, channel, qualified source box and
logical continuation position where known, preserving original causes through
`errors.Is`/`errors.As` rather than requiring clients to parse messages.

**Acceptance:** state copies/persistent banks are independent; mismatched cursors
fail early; finite/blocked ordering, quantization boundaries and overflow are
pinned. Errors retain causes. NoFit publishes no successor. Tests establish the
local and sequence comparators separately, including main/secondary progress
ties and lexicographic sequences of different lengths.

### BOX-SHIP-02 — Pure finite-height structural fitting

**Status: implemented (2026-11-25).** `layout.NewStructuralFitter` snapshots
normalized block structure and binds a neutral `ParagraphFitter`.
`Enumerate(ctx, FitRequest, visitor)` returns immutable `FitAlternative` values
with paragraph outputs, occupied CSS-pixel extents, first/last container facts,
logical end positions and exact opaque continuations. Requests carry finite
width/height, optional qualified continuation/stop, optional empty choice and
an explicit local work cap. Only `FitExhausted` certifies complete enumeration;
`FitStopped` is interruption, while cancellation, limits and invalid/unsupported
input or callback failures are errors with box identity and preserved causes.

BOX-SHIP-02 initially supported static auto-width blocks with zero edges, nested
block sequences and ordinary inline/text groups. BOX-SHIP-03 now adds actual
used widths/edges, margin collapse and indivisible atomics. Pending opening/
closing obligations remain preserved with open ancestors and next-child indices,
including an unopened container reached by an explicit stop. Results expose
fragment geometry and retained edge values separately from unchanged source styles.

Paragraph services provide all legal prefixes and exact opaque
checkpoint/output associations. Core freezes cloneable payloads on receipt,
inspection and resumption, filtering oversized choices as non-fit. Per-group
choices are collected before depth-first traversal: complete choices first,
then descending logical ends, stable in provider order for ties; extensions
precede their shorter block prefix. Caller stops and work caps never certify
the current best choice. Logical comparison is independent of width or height
and never authorizes exchange of continuations between fitter instances.

Sibling tests cover every supported structural/line cut, repeated and
changed-width resumption, zero-height advancing material, root/group stops,
foreign cursors and unsupported inline cuts, exact-height/oversized non-fit,
copied inspection, callback scratch reuse, rejection, late errors and concurrent
independent runs. The executable example retains a selected prefix while fitting
its suffix at a narrower width. No typography, register, channel or page policy
enters this API. TYPE-SHIP-01 supplies real paragraph choices; BOX-SHIP-03 supplies
used fragment geometry; SHIP-04 connects the content/preparation bridge.

**Work:** add a structural fitter over an immutable normalized tree, receiving
constraints, optional opaque continuation and an optional stop boundary. Return
legal fragment alternatives, their logical end positions, exact continuations,
occupied extents and completion/progress facts. The API may enumerate rather
than materialize every alternative, but enumeration order and early termination
must be explicit. A work limit is not exhaustion of the legal alternatives.

Begin with block sequences and nested containers, using deterministic synthetic
paragraph results. Carry open containers, next child/group position and pending
edge/margin facts. Remove a branch from a queue only above this layer when the
fitter reports its completion. The core knows nothing about registers or spans;
a caller-supplied stop position is just a structural bound.

Expose a neutral paragraph-fitting callback with value inputs and opaque
continuation/output associations. It must allow shorter legal line prefixes and
arbitrary selected continuation, not only the longest prefix. Keep font objects,
typography types and page loss out of the core. No result may depend on a mutable
callback workspace after the operation returns.

**Acceptance:** split/resume at every supported block/line boundary conserves
ordered material, including zero-height advancing material. Changed receiving
width leaves the accepted prefix fixed. Invalid continuations and unsupported
cuts are errors; oversized indivisible material is non-fit. Repeating, rejecting
or failing an alternative leaves the source and prior alternatives usable.

### TYPE-SHIP-01 — Paragraph prefix alternatives for `none`

**Status: implemented (2026-11-26).** Typography's
`Trial.FitPrefixes(height)` assembles the captured trial once and describes all
legal whole-line prefixes of its unaccepted suffix, using authoritative line
bottoms in fractional big points. `PrefixFit.Prefixes()` returns detached choices
in farther-progress-first order, including an explicit zero-line NoProgress
choice. Empty/already-completed input has one zero-line Complete choice.
Zero-height lines advance; horizontal overflow is reported, never clipped.

`PrefixFit.Select(localLineCount)` returns an immutable selected fragment with
its exact `TrialCheckpoint`, accepted atomic boxes and matching local placements.
The set's private trial supplies the association; descriptor edits and unrelated
fragments cannot create checkpoint authority. Selection needs no service calls.
Resumed trials preserve the inherited accepted-line count and never offer those
lines again. Changed widths and unconsumed atomic dimensions are supported;
consumed atomic geometry and line-height remain protected by existing checkpoint
validation. Rendering identities/interiors remain adapter-owned for SHIP-04.

`LogicalPosition` carries preparation-qualified byte/knot coordinates for
comparison. `SourceCoverage` separately reports glyph clusters, retained spaces
and atomic anchors in logical source order. A nonempty source range overlapping
a ligature belongs to the fragment containing that whole cluster. Trimmed spaces
and fill have no rendered coverage. Original HTML-to-processed mapping and empty
marker affinity remain with the content bridge; no visual bidi order or synthetic
markers are used as logical progress.

Tests cover every line cut, exact-height/zero-height/empty cases, repeated and
changed-width continuation without base reshaping, bidi/source coverage, suffix
atomic rebinding, foreign checkpoints, invalid selections and failed/retried
assembly. Weak-pointer tests prove selected output releases trials, preparation,
discovery and shape callbacks; extracting the fragment also releases checkpoints
and material. A real OpenType ligature/rendering proof and executable client
example exercise retained output after changed-width completion. Typography
remains synchronous and imports neither CSS core nor pagelayout; dependency
audits enforce this boundary. No khipu or CSS-core production changes were required.

TYPE-SHIP-01 established the `NoHyphenation` profile. KHIPU-SHIP-01 and
TYPE-SHIP-02 now extend the same fitting surface to manual carry. BOX-SHIP-03
supplies CSS fragment geometry and SHIP-04 connects the content/preparation
adapter. SHIP-05/06/07 supply page construction and pagespan search; scoring and
limits remain working hypotheses.

**Work:** extend the existing Prepared.Trial/checkpoint/fragment facilities to
enumerate supported whole-line prefixes that fit a finite height and freeze a
chosen prefix with its exact continuation. Reuse authoritative line bottoms and
break plans; do not estimate a prefix from average height or clip glyph output.
The caller can choose a shorter prefix to optimize the complete current page.

Make logical source/processed positions available for mapping events and
comparing progress. Define behavior where a source boundary meets a cluster or
ligature; glyph order in bidi text is not logical progression. Return enough
information to preserve the marker affinity fixed in SHIP-00.

Continue using the retained preparation. A changed-width suffix may change its
line breaks and unconsumed atomic geometry; accepted lines and consumed atomic
bindings must remain fixed. Certify the association of frozen paragraph output
and its accepted atomic placements. Do not rely on the convenience Flow API's
fixed-atomic restriction as the desired final contract.

**Acceptance:** all legal fitting prefix lengths, exact-height and zero-height
cases, NoProgress, changed-width continuation, empty paragraphs, bidi/source
positions and altered suffix atomics are covered. Invalid prefix bindings fail.
Earlier frozen fragments remain renderable; callbacks, discovery and trial
scratch are not retained by output. No base reshaping occurs on repeated trials.

### KHIPU-SHIP-01 — Manual discretionary continuation

**Status: implemented (2026-11-26).** Khipu checkpoints accept
`ExplicitHyphenation` alongside `NoHyphenation`. First-fit continuation restores
the exact selected post-break core before scanning later original knots and
allows later authored discretionary discovery in the suffix. Prefix widths,
skips, policy and selected alternatives remain fixed; accepted text is not
reprepared or rediscovered. Empty/silent carries preserve original source
coverage and trimming semantics. Automatic continuation remains unsupported.

The checkpoint owns copied prefix decisions and selected replacement arenas,
without retaining discovery, the originating trial, a binding or older
checkpoints. Only unconsumed atomic advances can change. Lazy suffix services
may fail; errors return no plan and preserve checkpoints/completed batches for
retry. Frozen output retains no working handles. TYPE-SHIP-02 now integrates
manual source/fragment coverage through typography's checkpoint/prefix API. No CSS, font implementation or page-policy dependency
was added. The existing first-fit profile still does not split a replacement
tail recursively.

Carry the exact selected alternative and its remaining material across a frame
boundary. Preserve source conservation when alternatives emit no bytes or
replace a range; do not replay a discretionary already accepted in the prefix.
Allow rebinding only unconsumed atomics and retain immutable prefix certificates.
Keep the API independent of fonts, DOM, CSS boxes and page policy.

**Acceptance: implemented** in
[khipu manual continuation tests](../../khipu/linebreak/firstfit/manual_continue_test.go)
and [synthetic replacement/lifetime tests](../../khipu/manual_checkpoint_integration_test.go).
They cover cuts before/at/after selected alternatives, zero/full prefixes,
nonempty/empty/silent replacement tails, UTF-8 source partitions, changed widths,
multiple resumes, FF/KP prefix preservation, suffix atomic rebinding and
consumed-atomic/incarnation rejection. Failed lazy discovery and warm concurrent
retries preserve earlier output. Weak-pointer checks prove that checkpoints
release trial/discovery and frozen results release all working handles.
Synthetic empty-spelling pairs exercise the neutral carry contract; ordinary
authored SHY discovery still retains the original letters on both sides.
These are paragraph-level proofs, not page-runtime fixture results.

### TYPE-SHIP-02 — Manual fitting through the paragraph API

**Status: implemented (2026-11-26).** `Trial.Checkpoint` and `Trial.FitPrefixes`
admit `ExplicitHyphenation` alongside `NoHyphenation`; selection and continuation
use the same `FittedPrefix`/`TrialRequest.Prefix` surface. Accepted frozen glyphs,
line-height, source splits and atomic geometry remain fixed. First-fit resumes
with the exact selected replacement tail, including zero-width material, and may
discover later authored alternatives lazily. No base paragraph is re-prepared.

`LogicalPosition.Offset` uses the original split and `NextKnot` identifies the
next base knot, while the checkpoint carries pending replacement authority.
`ReplacementCoverage` reports each rendered half as one indivisible original
source partition; generated cluster offsets stay local to the replacement and
are never presented as original source positions or a per-character edit map.
Empty/silent output has no rendered coverage; logical consumption/carry still
preserves its provenance. HTML mapping, empty-marker affinity and application
atomic interiors remain with SHIP-04's adapter.

Provider/shaper failures return no trial, preserving previous fragments,
checkpoints and complete discovery batches for retry. Existing unsupported
policies remain stage/cause errors, never successful NoProgress. Automatic
continuation and recursively splitting carried tails are unsupported. The
separate `StartFlow` convenience API retains its NoHyphenation restriction.

**Acceptance: implemented** in
[manual prefix tests](../../typography/glyphing/paragraph/prefix_manual_test.go),
[coverage tests](../../typography/glyphing/paragraph/prefix_coverage_test.go),
[lifetime tests](../../typography/glyphing/paragraph/prefix_manual_lifetime_test.go)
and a [real-font rendering proof](../../typography/integration/prefix_manual_render_test.go).
They cover every cut, exact heights/NoProgress, UTF-8/bidi source partitions,
replacement-adjacent owning ranges, equal/changed widths and repeated resumes,
full/empty checkpoints, lazy provider/shaper failure/retry, suffix atomic rebinding
and consumed geometry rejection. Selected output releases trial/discovery/services;
retaining only fragments releases checkpoints/material too. Real-font output
renders the selected hyphen once after changed-width completion and preparation
collection. Atomic interiors remain client-owned; stable IDs and exact accepted
box/placement values preserve their association. An
[executable example](../../typography/glyphing/paragraph/prefix_manual_example_test.go)
demonstrates manual selection/resumption. No sibling production changes or page
runtime were needed; working scoring/limits remain unchanged.

**Work:** integrate the KHIPU-SHIP-01 contract into the same prefix/fragment API
as TYPE-SHIP-01. Retain matching glyph output, source ranges and carried
replacement material. Manual alternatives may use explicit services lazily,
but continuation must not reprepare the base paragraph or turn manual into none.

Keep existing explicit profile failures distinguishable from a paragraph that
simply cannot fit the receiving space. Do not broaden this slice into automatic
hyphenation or unrelated paragraph optimization changes.

**Acceptance:** manual paragraphs split at discretionary breaks resume at equal
and different widths with the correct suffix and frozen prefix. Injected service
failures publish no fragment/continuation pair and leave previous output usable.
Exercise the source-position/anchor mapping near replacement material and prove
that accepted atomics retain their matching interiors.

### BOX-SHIP-03 — Used geometry, atomics and fragment paint

**Status: implemented (2026-11-26).** `StructuralFitter` snapshots normalized
structure and styles, resolves each receiving width through core used values,
and replays frozen selected paragraphs on pruned fragment topology through
`FlowLayout`. Only crossed containers omit top/bottom margins, padding and
borders; contained children retain their edges. Final edges must fit together
with final content. Signed/empty/nested margin collapse uses existing core
rules within a receiver, with no collapse across receivers. Copied `Geometry`,
`UsedValues` and `ContainerFragment` facts support painting without changing
source ComputedStyle. Physical overflow is non-fit; invalid geometry is an error.

`ParagraphFitRequest` now carries the resolved owner width, a conservative height
ceiling, borrowed used values and source-ordered `ParagraphAtomic` values.
Interiors are realized before outer fitting. Output placements must consume a
source-order prefix of remaining atomics, while physical order may differ.
Consumed items retain their exact interior output/geometry across continuation;
only suffix items may be re-realized. `AtomicFragment` exposes immutable handles
with copied geometry/used values and cloned paragraph payloads. Auto atomics use
`NewStructuralFitterWithOptions` and an optional intrinsic measurer. Definite
percentage/em widths use existing core resolution; unknown-auto percentage
bases still fail. Atomic interiors must produce complete paragraph alternatives.

Neutral sibling tests pin the SHIP-00 first/middle/last edge examples, closing
capacity, contained children, local percentage/font contexts, signed/empty
margins, changed-width atomic reuse, nested/auto/replaced/zero-sized atomics,
oversized retry, protocol errors, interruption and copied output lifetime.
No typography imports or page policy were added. SHIP-04 now connects the bridge;
scoring and numerical limits remain working hypotheses.

**Work:** connect structural fitting to existing used-value, intrinsic and
flow-layout machinery. Resolve each actual receiving width; percentages,
margins, borders and padding must use the correct local context. Do not replace
the core sizing rules with summed measured heights in the page constructor.

Apply top/bottom edge omission only to containers crossed by the split. Account
for the last fragment's retained bottom edges before accepting a final fit;
first/middle fragments can have different capacity. Keep source ComputedStyle
unchanged and expose fragment-specific geometry/paint facts separately. Use the
adopted within-frame margin rules and no cross-frame collapse, including nested
and empty blocks and signed margins admitted by the content profile.

Inline atomics remain indivisible. Resolve their interiors before the enclosing
paragraph needs their geometry. On continuation, re-realize only geometry allowed
to vary in the suffix; retain the accepted prefix's exact interior results.
Use neutral test paragraph callbacks to enforce those associations without
introducing a typography dependency into the core.

**Acceptance:** first/middle/last nested-fragment geometry and paint obey edge
rules; contained children keep all edges. Margin/percentage fixtures have exact
expected extents. An atomic too large for one frame can fit a later configuration
without corrupting the original continuation. Invalid geometry/unsupported
fragmentation is an error, never silently clipped or classified as ordinary fit.

### SHIP-04 — Content and typography fitting bridge

**Status: implemented (2026-11-26).** `content.NewFitter` checks complete covered
subtree preparation and snapshots a core fitting program. `content.Fit` reserves
the exact live preparation/session for the whole invocation, qualifies borrowed
group/atomic requests and structural stops, and returns a candidate set only on
exhaustion. Foreign cursors, closed/reentrant sessions, cancellation, work caps
and late failures return no partial bundle. The reusable fitter retains no
execution services between calls. Original banks and source trees stay unchanged.
Whole-tree Layout and MeasureIntrinsic retain their existing APIs.

`preparation/typography.Fit` supplies FirstFit paragraph trials from retained
preparation for `none` and `manual`. Structural and paragraph checkpoints keep
matching consumed atomic interiors; suffix geometry can change with width.
Receivers use checked CSS-pixel/big-point conversion, with width rounded inward
to the DU grid and height tested without upward quantization. Each published
paragraph occurrence has a distinct `content.FragmentID`, retaining its source
group identity. Frozen `Fragment` bundles expose copied used geometry, retained
container edge/paint facts, source paint/text mappings and immutable paragraph
fragments. Extracting a bundle drops continuation authority; neither bundles
nor continuations retain a fitter, session, collector or preparation handle.

Rendered coverage maps whole clusters and original manual replacement partitions
to decoded source bytes. Owning-anchor associations use logical coverage and
following/preceding/empty-group affinity; per-group continuation prevents replay.
Cross-group deduplication and structural block/forced-break event execution remain
page-layer responsibilities. `Fitter.BoundaryAt` accepts structural source-plan
stops; text coordinates cannot manufacture a stop or checkpoint.

Tests cover changed-width continuation, repeated occurrences, nested/auto and
percentage atomics, zero-height groups, foreign/closed sessions, reentrancy,
late errors/retry, strict units and source/marker affinity. Real-font ligature
and bidi tests pin logical coverage. GC proofs retain rendering with both
hyphenation policies after execution handles and fitters are collected, even
while retaining a continuation. An executable `ExampleFit` demonstrates two
receivers and unchanged banks. No sibling changes were needed. SHIP-05 now
uses this bridge for pages, extended to multiple channels by SHIP-06; scoring and numerical limits remain
working hypotheses.

**Work:** add the neutral content fitting entry point beside whole-tree Layout.
It owns private-tree access, complete preparation coverage checks, source/tree
qualification, callback borrowing and serialized session use. Keep existing
whole-tree measurement/layout available. Do not expose raw normalized nodes or
add typography types to `content`.

Extend `preparation/typography` with the resumable adapter. Pair structural
continuations with paragraph checkpoints and matching atomic interiors. Allocate
fresh used-value and output collection state for each invocation/alternative.
The current whole-tree collector's one-result-per-group assumption is inadequate:
one group can contribute multiple fragments at different widths and locations.
Key output by fragment occurrence while retaining source/group identity.

Convert CSS-pixel geometry and typography physical units through checked
conversions. Preserve source/paint and marker mapping in frozen results. No page
result may retain a borrowed callback view, mutable collector or abandoned
alternative just to remain renderable. Resumption still requires the matching
live prepared context; rendering selected output does not.

**Acceptance:** end-to-end frame fitting at two widths reuses one preparation
and yields exact logical continuations, matching atomics and source mapping.
Test `none` and supported `manual`, multiple fragments of one group, foreign and
closed sessions, reentrancy, late errors and retry. Failed attempts publish no
partial bundle and leave banks, old cursors and frozen results unchanged.

### SHIP-05 — Single-channel Page attempt

**Status: implemented (2026-11-26).** `PaginationConfig.Fitter` supplies a
neutral `content.GroupFitter`; typography clients use `FittingAdapter()` and
recover typed frame output with `InspectFragment`. `PageAttempt` uses compiled
frame chains/rectangles and paint, enumerates whole-frame prefixes and unused
frames, then selects one complete arrangement by `CompareLocal`. It never
commits a greedy frame choice. One reusable content fitter per branch keeps
exact checkpoints paired across invocations and widths. Root production code
has no typography/font dependency.

Success returns a Candidate with frozen `Page`, loss facts/components, progress
and an immutable successor. Partial main branches keep their original bank head
plus an opaque checkpoint; completed branches leave only the derived bank.
`FitPosition.Logical()` maps inspection positions through normalization provenance
and processed source spans, retaining within-offset discretionary progress.
Forced-break stops work inside a head and across paragraph continuations.
Already-complete spans return SpanComplete; no material progress returns NoFit.
Empty groups can make genuine zero-height progress without a marker-only page.

`content.ParagraphOutput.LineCount` supplies scoring facts without a typography
type. Tests pin one-line split penalties, entry participation and final-main-tail
exemption, including the original area denominator and earlier gaps. Scoring
and numerical limits remain **working hypotheses**. The attempt-wide work budget
counts visited arrangements, receiver invocations, backend measurements/paragraph
calls and yielded paragraph choices; each core invocation also caps internal
structural steps at the remaining allowance. Cancellation and caps are checked
around synchronous work. No interrupted or failed call publishes a provisional
winner. Errors retain frame/template/source context and original causes.

Synthetic tests exercise the adopted two-template fixture, shorter-prefix wins,
progress/enumeration ties, changed widths, missing/unused receivers, forced stops,
late errors, retry, cancellation, limits and session/identity checks. Real Gentium
proofs cover none/manual, percentage atomics, source conservation and rendering
after execution/session collection while a successor is retained. No sibling
changes were necessary for SHIP-05. SHIP-06 adds secondary activation, FIFO
placement and draining below. SHIP-07 supplies search; SHIP-08 supplies public
span driving. Initial main is the existing sole residual-tree bank item;
no new queue or content representation was introduced.

**Work:** implement one-template page construction over main and its ordered
frame chain. Use compiled content rectangles and paint directly. Enumerate legal
complete-page arrangements, evaluate page facts and choose exactly one by the
settled local comparator. Do not greedily commit the longest prefix per frame.

Stop at the span boundary, including a boundary within the current head branch.
Preserve partially consumed heads and advance fully consumed branches only in
derived states. Enforce material progress, handle already-complete spans without
a new page, and distinguish genuine zero-height consumption from a page number
increment or marker-only empty page. Implement the pinned final-main-tail loss
treatment and attach fragment occurrences, loss components and progress together.

**Acceptance:** use a small prepared document and two templates with different
frame widths/heights. Try both from the identical state; inspect fragments,
exact remainder and loss; discard either without changing the other. Include a
case where the best current page uses a shorter prefix, a same-loss progress tie,
different-width continuation across frames and a split ending at a main marker.
An interrupted local enumeration is an explicit error/limit, not NoFit or a
supposedly final candidate.

### SHIP-06 — Multi-channel arrangements, activation and draining

**Status: implemented (2026-11-26).** `PageAttempt` enumerates complete
arrangements across main and source-ordered secondary channels, preserving each
frame chain and register queue. It may fit current-span targets speculatively
when a later-visited channel could activate them; fixed earlier-channel material
prunes unreachable owners. Final whole-page activation is a transitive fixed
point over actual selected owner events. Thus a target can appear above its
owner, or in an earlier-visited channel, without being placed on an earlier page.

Text owners use `content.ParagraphEvents.PlacedEvents`, supplied by the typography
adapter from its frozen cluster/replacement/affinity associations. Events must
match the prepared group's detached inputs. Logical byte consumption alone is
not rendered placement. Structural owners use `FitFragment.ContainsBoundary`;
opening a suspended container is not enough. Activation is deduplicated by target
and retained across pages. Ordinary references do not activate targets.

`PageFrame.Fragments()` exposes ordered fragments from several independent
branch roots, including a final partial root. `PageFrame.Extent()` gives composed
occupancy. `content.ComposeFragments` replays frozen root border boxes through
core `FlowLayout` for signed sibling-margin collapse and translates copied
geometry/paragraph origins. Roots remain containment boundaries for their already
fitted interiors. The sibling `layout.FitRequest.DeferVerticalBounds` option
allows intermediate negative/oversized root margins; final composition must
pass all physical bounds. Horizontal checks, finite geometry and paragraph
height ceilings remain enforced. Default fitting behavior is unchanged.

Only fully consumed heads leave derived banks; one exact checkpoint remains
beside each partial original head. Inactive heads are never skipped, and
future-span heads are never fitted. NoFit's copied `Obstructions()` preserve
entry-state awaiting-owner, future-span and missing-receiver facts, including
pending items blocked behind a head. They do not claim global infeasibility.
At a main boundary only related secondary material can progress. Entry frame
participation remains fixed; newly pending targets incur one carry each, absent
secondary receivers add no duplicate penalty, and missing-main is charged only
while main remains. All channels publish one successor together. A late error,
cancellation or cap discards every provisional activation and page.

`page_channels_test.go` exercises the adopted transitive fixture with exact
losses, reverse channel dependencies, authored `[A,B]` order with B owned first,
multiple/partial heads in one frame, changed-width continuation, split/ordinary
and empty owners, secondary-only pages, cross-span blocking, signed margins and
late failure/retry. Real Gentium tests activate through ligature coverage and
manual continuation and retain frozen rendering after session closure/GC.
Content/core tests pin copied composition and deferred-bounds validation.
Scoring and numerical execution limits remain **working hypotheses**. SHIP-07
supplies search; SHIP-08 supplies public span driving/delivery.

**Work:** extend local enumeration to whole-page combinations across channels.
Preserve each channel's frame-chain and queue order. Main/secondary progress,
anchor activation and transitive target placement must describe the same final
arrangement. Roll back all provisional activations with a discarded alternative.

Do not let cross-channel frame visitation accidentally prohibit a valid layout:
a target may be above its owning anchor on the same page, even if its frame
appears earlier in template order. Determine same-page eligibility from placed
anchor material while preserving the ordered chain within each channel. An
anchor outside the selected prefix activates nothing. Transitive anchors become
active only when their owning secondary fragments are placed.

Enforce FIFO heads. If B is pending behind A whose anchor has not yet been
reached, B waits; do not place A early or skip to B. A future-span head may prevent
the current span from draining. Preserve that reason for failure instead of
repairing the authored order.

Track target activation/completion and charge each pending target once on the
anchor page and each later carrying page, including partial placements. Ordinary
references do not activate or charge. Apply the working missing-main charge;
secondary missing receivers use carry alone. Use the fixed frame-participation
convention from SHIP-00 across all alternatives, including those that reach
different anchors.

After main reaches its boundary, admit only related secondary material until
the transitive draining obligation is satisfied. Permit progressing pages with
no main receiver. Missing receivers alone are not a mandatory-channel failure;
no material progress still means no successful page.

**Acceptance:** same-page/above-anchor placement, repeated ordinary references,
split anchors, partial targets, transitive ownership, absent receivers and
secondary-only drain pages have exact expectations. Out-of-order `[A, B]`
fixtures retain authored order and correct carry costs. Future-span material is
untouched. A late failure in any channel leaves all channel inputs unchanged.

### SHIP-07 — Deterministic pagespan beam search

**Status: implemented (2026-11-26).** `Pagination.PaginationSearch(ctx, state)`
selects one complete current pagespan using SHIP-06's one-candidate-per-template
Page attempt. The default beam retains 32 unfinished paths. Every depth round
extends all retained paths through eligible templates in catalog order before
globally ranking successors by accumulated loss and lexicographic catalog
ordinals. Completed candidates are kept separately; the search continues after
the first completion. It neither merges equal states nor adds progress/future-cost
heuristics. Immutable page predecessors share output without retaining cumulative
rank vectors from every ancestor.

`PaginationConfig.Eligible` is an optional deterministic
`func(State, *template.Template) (bool, error)`. Nil admits all templates; false
filters a state/template edge without changing its ordinal. The callback receives
immutable state and an independent catalog handle. Errors abort the invocation.
A whole-search execution guard rejects reentry from eligibility/backend callbacks.
Explicit PageAttempt calls remain unfiltered. Already-complete input returns
zero pages and zero loss without calling eligibility or the fitting backend.

After each complete round, the positive policy page-cost lower bound safely
removes only unfinished paths whose bound is strictly greater than the incumbent
loss. Equality survives. Checked arithmetic failures abort search. Width
truncation is separate and reported even if a completion is found.
`SearchReport.Stats()` copies completed-round, attempted-page, NoFit, completion,
safe-pruning and width-pruning counts. Attempts include failed calls; eligibility
rejections are not attempts. A partial interrupted round is not a completed round.

NoFit drops one edge; cancellation, local/depth/attempt limits and operational
errors abort with no selection. A previously completed incumbent may be retained
explicitly unselected. Exhaustion without width truncation proves infeasibility
under the fixed eligibility/local policies; after truncation it is heuristic
exhaustion. Successful reports select an immutable Sequence with loss, frozen
pages and exact final state at the current span boundary. Inspection returns
independent handles. No automatic retry, widening, span transition or output
delivery is introduced. Scoring and numerical limits remain **working hypotheses**.

`search_graph_test.go` executes all twelve SHIP-00 graph cases, their retained
frontiers/counters and requested widths, with an independent exhaustive DFS.
Additional tests cover catalog-vs-name order, invalid/overflowing costs,
post-callback cancellation and exact attempt caps. `search_test.go` compares
actual finite page fixtures against exhaustive PageAttempt traversal, including
changed widths, transitive draining and cross-span FIFO obstruction. It verifies
eligibility/parity, reentry, explicit attempts, all public outcome categories,
completed-incumbent isolation, state identity, retry and output lifetime.
Real Gentium none/manual tests retain selected rendering after session close/GC.
No sibling changes were needed. SHIP-08 below supplies public transitions/delivery.

**Work:** implement the settled search policy independently of fitting details.
Start with a fake deterministic Page attempt and known graphs, then integrate
SHIP-06. Enumerate eligible templates in catalog order, using all templates by
default and the adopted optional deterministic filter without reordering.

For each page-depth round, extend every retained unfinished candidate through
every eligible template. Each template contributes at most its one selected page.
Retain complete candidates separately; rank unfinished successors globally after
the full round by summed loss and lexicographic template sequence. Keep at most
the configured positive width, 32 by default. No per-parent quota, progress-based
beam tie, state merging, percentile pruning or estimated remaining cost is added.

Retain the best complete incumbent outside the beam. Continue after the first
completion. Prune an unfinished branch by the safe minimum-next-page-cost bound
only when it is strictly greater than the incumbent's loss. Count safe pruning
separately from width truncation. An already complete input returns no extra page.

NoFit drops a template edge; operational failure aborts by the adopted contract.
Report completion with whether heuristic truncation occurred. No completion after
fully explored, uninterrupted, unpruned alternatives establishes infeasibility
under these local/template policies; after width truncation it is search
exhaustion. Apply explicit page/attempt/enumeration limits and cancellation using
the SHIP-00 contract; no automatic widening or retry. Cancellation checks belong
at orchestration/work boundaries and must respect synchronous sibling APIs.

**Acceptance:** small exhaustive reference graphs prove ranking, delayed better
completion, equal-loss lexicographic preference, strict bound equality, separate
completion storage, globally capped rounds and every outcome category. Compare
the real finite fixtures against exhaustive search where no beam truncation
occurs. The search must not claim global cut-off optimization or document-wide
optimality across committed pagespans.

### SHIP-08 — Span transitions and shipout handoff

**Status: implemented (2026-11-26).** `Pagination.Paginate(ctx, state)` selects
successive complete spans and returns a `DocumentResult`. `Spans()` contains only
the successful selections made by this invocation; `LastSearch()` preserves the
last search report, including an explicitly unselected interrupted incumbent.
`Complete()` distinguishes whole-document completion from later-span failure,
infeasibility or interruption. The returned `State()` is the final drained state
on completion or the unsuccessful span's unchanged entry on failure. Retrying
that state is explicit and cannot reconsider earlier selections. Limits restart
for each span, and the complete orchestration rejects callback reentry.
Interruption between searches can retain the preceding drained boundary and
its successful `LastSearch`; resuming still advances without reselection.

`AdvanceSpan` validates complete transitive draining, session/context identity
and exact fitting-checkpoint authority. It retains the main root and opaque
continuation, future queues and next page number. `State.DocumentComplete()`
provides terminal-draining inspection without live execution. Passing an already
drained boundary to `Paginate` advances without selecting it again. Search keeps
the immutable page predecessor sharing introduced in SHIP-07.

At search entry, a certified empty main remainder advances through the existing
structural fitter without a physical page or page cost. `PreparedSnapshot.EmptyRange`
conservatively checks the relevant source interval for text, atomics, paint,
spacing and nonzero definite widths; it never manufactures a continuation. The
zero-extent structural fit carries empty owning annotations and honors the local
work cap. Targets activate before terminal draining, including with notes-only
receivers. Nonempty or unproved material still uses normal Page attempts. Direct
PageAttempt remains the explicit receiver-fitting operation; this empty-span
normalization belongs to search/document orchestration.

`Sequence.Shipout(ctx, PageConsumer)` delivers independent frozen `Page` handles
in order. Only successful selections are accepted; unselected incumbents and zero
sequences are rejected before any consumer call. `DeliveryReport` records successful
page calls, a consumer's failing page and the next undelivered page. `DeliveryError`
matches `ErrDelivery` and preserves the original cause. Cancellation is checked
before/after synchronous calls, so a cancellation following the last successful
call can coexist with delivery `Complete() == true`. Failed calls may have partial
external effects. There is no rollback or automatic retry; a fresh explicit Shipout
starts the selected span again. Delivery needs no live execution session.

Evidence: `document_test.go` selects the two transitive fixture spans, proves
exact boundary continuation and page numbers 1–3, preserves future material and
earlier selections on later failure, and covers explicit retry and empty spans.
`shipout_test.go` exercises the delivery-failure protocol (page 2 fails, only page 1
completed), unselected incumbents, cancellation, copied handles and closed execution.
`content/empty_test.go` checks conservative, source-qualified range certification.
Gentium none/manual tests in `page_realfont_test.go` deliver and render selected
output after session close and GC. No sibling changes were required. Scoring,
eligibility policy and numerical caps remain **working hypotheses**; the broader
visual, lifetime and search-quality gate remains SHIP-09.

**Implemented scope:** add document orchestration around completed pagespan search. Select a
whole span and publish its frozen sequence with its final exact state. Advance
to the next span using that state and next page number; do not reset numbering
or revisit the selected span for later parity. A main branch may continue across
this boundary with the same source handle.

Use immutable predecessor sharing for retained page sequences. This shares
output prefixes, not equivalent search states. Selected output carries template,
page/frame geometry, fragment occurrences, source/paint, frozen paragraph/atomic
output and loss facts, independently of discarded execution workspaces.

Expose a small consumer handoff over selected frozen pages or a selected span,
with ordering and error propagation fixed in SHIP-00. Consumers never choose
breaks or receive mutable banks. Do not send speculative pages to irreversible
output. Keep span selection atomic even if an external consumer can fail after
writing some output; document delivery progress separately rather than promising
rollback of external effects.

**Acceptance:** two spans prove draining, future-material preservation, exact
numbering and no prior-span reselection. An empty document/initial empty span
creates no page. Later-span failure preserves earlier selected results with
explicit completion status. Closing preparation prevents further resumption but
does not invalidate already frozen pages while borrowed rendering resources
remain valid. Consumer errors preserve their causes and do not alter the plan.

### SHIP-09 — Real-font, lifetime and search-quality proofs

**Work:** extend the existing preparation real-font approach using pinned local
test fixtures and test-only concrete font/render adapters. Produce a small
multi-page visual proof showing template/frame boundaries, fragments and source
identities, accompanied by machine-checked page sequences and loss breakdowns.
Visual inspection complements conservation/geometry assertions; it does not
replace them.

Prove two-width paragraph continuation, changed suffix atomics, manual carry,
source-style preservation, owning-anchor deferral and span draining through the
complete public API. Verify that accepted prefixes and interiors do not change
when competing candidates are tried or fail. Repeat attempts with service counts
to establish base-preparation reuse; allow legitimate lazy manual-alternative
work rather than claiming all service calls cease after preparation.

Add lifetime evidence retaining chosen pages while releasing rejected branches,
session handles, trials, collectors and search frontiers. Borrowed fonts remain
caller-owned. Test that page output does not retain preparation factories or
abandoned alternatives through accidental references. Retained resumable states
may keep the immutable data they need; distinguish that intentional ownership
from output-only retention.

Measure widths 8, 32 and 128 against exhaustive small cases. Record attempts,
local alternatives, peak retained candidates, bound cuts, heuristic truncation,
completed loss and repeated breakpoint vectors. Those observations may motivate
future merging but must not implement it or assert equivalence from positions
alone. Use work counts and quality comparisons, not fragile wall-clock thresholds.

**Acceptance:** all section 7 cases pass, real-font output is usable after
session release, and dependency audits remain clean. Add executable usage
examples and usage-only README updates. Update implementation status in design
documents and AGENTS only for delivered behavior, preserving author remarks and
remaining hypotheses. No milestone is complete merely because the happy-path
visual proof looks correct.

## 6. Sibling-module change summary

| Module | Required work | Boundary and regression obligation |
| --- | --- | --- |
| `w3c-dom` | DOM-SHIP-01: checked page-break values/defaults and matching declaration/effective resolution. | No pagespan or typography policy; preserve existing styling and diagnostics. |
| `css-box-layout` | BOX-SHIP-01 through BOX-SHIP-03: boundary provenance and pure resumable structural fitting with fragment geometry/paint. SHIP-06 adds opt-in intermediate vertical bounds deferral for sibling-root composition. | No channels, scheduler, fonts or typography imports; persistent registers stay as they are. Final composed bounds remain the caller's responsibility. |
| `khipu` | KHIPU-SHIP-01: manual discretionary prefix/carry continuation. | Neutral paragraph material and immutable certificates; no CSS/DOM/page coupling. |
| `typography` | TYPE-SHIP-01/02: legal prefix enumeration, chosen frozen fragments, source positions and checkpoint/atomic integration for the admitted policies. | Reuse abstract font/service contracts; no concrete OpenType, pagelayout or css-box-layout dependency. |
| `tytui` | No new production feature required. | Run existing consumer regressions when shared APIs change. Prefer additive APIs; if a shared interface must change, include the smallest explicit consumer migration in that sibling slice. |
| `opentype`, `cords`, `uax`, `tree`, `schuko` | No required production changes identified. | Reuse existing services and test fixtures; any newly demonstrated upstream gap needs its own scoped dependency, not an assumed rewrite. |

In particular, keep font abstraction in the existing typography/khipu service
contracts and the optional bridge. A module-graph dependency is not a reason to
import a concrete font implementation into content, the CSS core or page search.
The current test-only OpenType adapters are sufficient for the planned proofs.

## 7. Acceptance fixture matrix

The SHIP-00 [fixture tables](../testdata/pagination/README.md) supply initial
inputs and explicit expected results for these families. Their owning slices add
executable checks, real-font cases and instrumentation. Expectations include
logical source coverage and remainder, not only page counts or screenshots.

| Family | Required observation | Main acceptance slice |
| --- | --- | --- |
| Default main | Residual/redundant-main equivalence; nested route still rejected; one preparation per group. | SHIP-01 |
| Marker provenance | Empty/multiline anchors, collapsed whitespace, inline splits, anonymous boxes, suppression and terminal affinity. | BOX-SHIP-01, SHIP-02 |
| Forced breaks | All admitted forced variants equivalent; main-only rule; coalescing; no leading blank; boundary inside a branch. | SHIP-02, SHIP-05 |
| Ownership errors | Missing/duplicate owner or target, inactive target, cycle, non-main-rooted chain; ordinary xrefs excluded. | SHIP-02 |
| Structural splits | Exact order through nested first/middle/last fragments; margins/edges and zero-height progress. | BOX-SHIP-02/03 |
| Paragraph prefixes | All legal fitting cuts, exact height, NoProgress, changed width, bidi and source-boundary mapping. | TYPE-SHIP-01 |
| Manual carry | Discretionary replacement tails, repeated continuation, source conservation and explicit service failure. | KHIPU-SHIP-01, TYPE-SHIP-02 |
| Inline atomics | Consumed interiors fixed; suffix interiors vary with width; oversized atomic without endless pages. | BOX-SHIP-03, SHIP-04 |
| Local choice | A shorter prefix wins current-page loss; ties prefer logical main/secondary progress then stable enumeration. | SHIP-05/06 |
| Owning placement | Same page above anchor allowed; earlier page disallowed; split anchor activates once; ordinary links add no charge. | SHIP-06 |
| Authored FIFO | B owned before A, queue `[A, B]`; pending charges and blocked heads; no sorting or skipping. | SHIP-06 |
| Transitive drain | Main → note → further target; drain-only page; future-span material retained; impossible drain reported. | SHIP-06/08 |
| Loss arithmetic | Blocked/finite ordering, one-time component quantization, checked overflow and positive page lower bound. | SHIP-03 |
| Scoring facts | Fixed participation, newly active channels, empty frame sets, split-paragraph counts, final-main tail and missing receivers. | SHIP-03, SHIP-06 |
| Search reference | Better completion after first; beam-wide ranking; width cap excludes completions; strict-bound tie; no merging. | SHIP-07 |
| Failure reporting | NoFit versus unsupported/backend error; local limit; search infeasible versus heuristic exhaustion; cancellation with/without incumbent. | SHIP-03/07 |
| Transactions | Late frame/channel failure leaves every input unchanged; selected span publishes pages and final state together. | SHIP-04/06/08 |
| Lifetime | Source/preparation mismatch; session close; chosen output survives rejected states and released execution handles. | SHIP-04/09 |
| Real fonts/quality | Two widths, atomics and manual carry rendered; beam 8/32/128 compared with an exhaustive reference. | SHIP-09 |

Use deterministic tables and generated finite combinations where they clarify
conservation and search behavior. This plan does not reopen the deliberately
omitted CONT-03 fuzz test.

## 8. Validation and completion checkpoints

For implementation slices, follow each repository's instructions, format Go
changes, and run focused package tests first. Then run the relevant module's
ordinary tests, race tests, build and vet checks for public boundary changes.
Exercise pagelayout integration after each sibling handoff; run typography and
tytui consumers when their shared paragraph/core interfaces change. Race checks
do not imply a new concurrent-session guarantee. Check package import boundaries
as well as runtime results. Documentation-only planning needs no Go test run.

Useful completion checkpoints are:

1. **Source contract ready:** SHIP-00 through SHIP-03 plus DOM-SHIP-01 and
   BOX-SHIP-01. Main, markers, ownership, state and loss can be inspected and
   tested without a real paginator.
2. **Exact fitting ready:** BOX-SHIP-02/03, TYPE-SHIP-01/02, KHIPU-SHIP-01 and
   SHIP-04. Structural and paragraph fragments resume across widths without
   source mutation, duplication or loss.
3. **Page construction ready:** SHIP-05/06. One-template attempts choose one
   deterministic complete page, including multiple channels and draining.
4. **Pagination ready:** SHIP-07/08. Bounded search selects spans, preserves
   exact state and delivers only selected frozen output.
5. **First milestone complete:** SHIP-09 and all preceding acceptance gates,
   including manual continuation within the declared profile, failure reporting
   and lifetime proofs. Record any still-provisional scoring calibration openly.

The next implementation task is **SHIP-09**: real-font, lifetime, search-quality
and end-to-end proofs.
SHIP-00, DOM-SHIP-01, SHIP-01,
BOX-SHIP-01, SHIP-02, SHIP-03, BOX-SHIP-02, TYPE-SHIP-01, KHIPU-SHIP-01,
TYPE-SHIP-02, BOX-SHIP-03, SHIP-04, SHIP-05, SHIP-06, SHIP-07 and SHIP-08 are complete.
Public span transitions and delivery are implemented; the broader proof gate remains planned.
