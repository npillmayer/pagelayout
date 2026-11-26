# Page layout architecture

Pagelayout composes prepared HTML content into frames from a separate HTML/CSS
page-template collection. Loading, preparation, pagination and delivery are
separate phases. Content keeps its source styles throughout; a receiving frame
supplies geometry, never a replacement CSS ancestry.

![Loading, preparation, pagination and delivery](diagrams/design-pipeline.svg)

## Responsibilities

| Package or module | Responsibility |
| --- | --- |
| `pagelayout/template` | Inspect and validate collections; compile immutable page/frame geometry, paint and channel bindings. |
| `pagelayout/content` | Style the original content document, validate its profile, partition and normalize boxes, retain source mappings, and mediate preparation and fitting access. |
| `pagelayout/preparation/typography` | Adapt explicit typography services to preparation, intrinsic measurement, whole-tree layout and resumable fitting. |
| `pagelayout` | Validate ownership and forced breaks; construct pages, search template sequences, transition spans and deliver selected output. |
| `w3c-dom` | HTML/CSS inspection and cascade, checked values, source text styles and semantics. |
| `css-box-layout` | Box normalization and provenance, persistent generic registers, used values, geometry, structural fitting and fragment edges. |
| `typography` and `khipu` | Font-independent service protocols, reusable paragraphs, line breaking, discretionary continuations and frozen paragraph output. |
| Caller | Font discovery and concrete font/shaping adapters, service lifetime, template eligibility, rendering and export. |

![Package boundaries](diagrams/design-packages.svg)

The root, content, template and CSS core packages do not import the typography
bridge. The bridge uses abstract font and shaping services; concrete OpenType
adapters live in tests and client code. A dependency in `go.mod` does not imply a
production package import. Sibling modules can be adapted when a shared contract
improves the layout process; reuse their services instead of duplicating layout,
CSS parsing, shaping or line breaking here.

## Data and ownership

| Value | Meaning and lifetime |
| --- | --- |
| Template catalog | Fully sized immutable page/frame values, in source order. Selection performs no styling or geometry work. |
| Content document | Immutable source metadata and private unsized normalized trees. Qualified `content.Box` handles retain their source snapshot. |
| Register bank | Persistent queues of `content.Box`, main first and secondary channels in first-occurrence order. Each queue retains authoring order. |
| Prepared snapshot | Copied group input and opaque source/preparation identity. Register payloads remain unchanged. |
| Preparation session | Serialized access to effectful backend handles and borrowed services. It is not stored in a box or register. |
| Pagination state | Immutable bank version, exact head continuations, activation and span/page facts, qualified by one pagination context. |
| Page candidate | A frozen page, its assessed loss and exact successor state. Rejecting it consumes nothing. |
| Selected sequence | Complete pagespan output and final state; eligible for explicit delivery. |

Publication is atomic at each boundary. Invalid collections, failed content loads
and failed preparations expose no partial results. Page-attempt errors expose no
candidate. Search or document pagination can report completed work without
silently selecting an interrupted incumbent. Already selected spans survive a
failure in a subsequent span.

Immutable output may share immutable source data and borrowed font resources.
It retains no preparation session, mutable trial or discovery factory. Closing a
session prevents further execution without invalidating frozen output; the
caller keeps borrowed rendering resources valid for as long as needed.

## Loading and preparation

Template frames are direct fixed-position children of a page DIV. The page is
their viewport. Fixed geometry is compiled before any content fitting. Border
and receiving rectangles, paint and source identities are values in the catalog.
The [template reference](TEMPLATE-LOADING.md) defines the accepted profile.

Content is styled once in its original HTML ancestry. Non-nested named block
branches are extracted before normalization. Residual content is the sole `main`
item; a valid `flow-into: main` is redundant after routing validation. Suppressed
and inert sources retain provenance without owning material. The
[content reference](CONTENT-LOADING.md) describes admission and source identities.

Preparation extracts normalized horizontal groups and resolves their fonts and
base shaping through caller-supplied services. It does not fix line breaks or
inline-block dimensions. Used values, line breaking and atomic interiors depend
on the receiving width. Whole-tree layout and finite-height fitting reuse the
same preparation/session pair; [PREPARATION.md](PREPARATION.md) specifies this
boundary and its policy limits.

## Page construction and pagination

![Immutable state branching and selection](diagrams/design-flows.svg)

A **Page attempt** fits one template from one immutable state and selects its
best admissible arrangement using current-page loss. Its frames are visited in
channel-chain order. Exact opaque checkpoints carry unfinished material between
frames, including frames of different widths. Summed frame heights are not a
substitute for fitting each receiver.

A **Pagination search** explores sequences of these locally selected pages for
one **pagespan**. Forced page breaks delimit spans. Explicit owning links activate
secondary targets as placement relationships; they permit penalized deferral
without closing a span. All transitively related material must drain before a
span closes. Queues never reorder material to match anchor order.

Search retains a configurable beam of unfinished candidates, with deterministic
ties and a separate best completion. It does not merge states or estimate
remaining cost. Selected spans are not revisited to improve subsequent spans.
[SHIPOUT.md](SHIPOUT.md) specifies exact remainders, loss, outcomes and delivery.
[SHIPOUT-PLAN.md](SHIPOUT-PLAN.md) maps the contracts to validation and open work.

## Deferred direction: floats into designated template frames

HTML-authored image or table floats may need receivers appropriate to their
material. An illustration frame can extend from the body-text area into a wide
margin, allowing the template designer to control its placement while the source
author expresses float intent.

Float support is deferred. Frame-kind syntax, material matching, source anchors,
text exclusion/overlap, fitting and deferral rules remain open. The current
content profile rejects floats and replaced images; it does not silently turn
float intent into ordinary in-flow material.

## Open extensions

The current restricted profile does not provide general tables, replaced
content, running headings, arbitrary positioned content, rich backgrounds or
inline decoration fragmentation. Broader whitespace, line-height and hyphenation
policies require corresponding preparation and fitting contracts. Execution is
serialized; concurrency requires explicit guarantees from every shared service.

Scoring coefficients, final-page underfill treatment and numerical work caps are
working hypotheses. Search-quality evidence informs their tuning and any future
state-merging or widening policy. These are distinct from the implemented
ordering, exact-continuation and immutable-publication contracts.
