# Pagination and shipout contracts

Pagination consumes immutable prepared flow registers and a compiled template
catalog. It constructs pages under receiving constraints, selects complete
pagespan sequences, then delivers selected frozen pages separately. Loading,
preparation and pagination are fixed phases; execution is serialized.

See the [root usage guide](../README.md) for the public workflow and
[validation guide](SHIPOUT-PLAN.md) for contract tests, proofs and open work.

## Terms and responsibilities

| Term | Contract |
| --- | --- |
| Frame | A compiled receiving rectangle bound to one flow channel. |
| Region chain | A channel's frames in template source order. |
| Page attempt | Construct and select one locally best arrangement for one template and input state. |
| Page candidate | Frozen page output, assessed loss and exact successor state. |
| Pagespan | Main material between synchronization boundaries, including all transitively related secondary material. |
| Pagination search | Explore template sequences for one pagespan. |
| Pagination candidate | A partial or complete page sequence with accumulated loss and exact state. |
| Shipout | Explicit ordered delivery of an already selected sequence. |

| Component | Owns |
| --- | --- |
| `content` | Immutable boxes and source provenance; prepared source maps; qualified fitter/continuation bridge. |
| CSS core | Pure structural fitting, used values, margins, fragment edges and atomic geometry. |
| Typography bridge | Prepared paragraph prefixes, manual carry, frozen paragraphs and actual event coverage. |
| `pagelayout.Plan` | Owning-link validation, activation graph and synchronization span boundaries. |
| Page attempt | Multi-channel arrangements, factual activation, local loss and atomic successor publication. |
| Pagination search | Deterministic template-sequence exploration, beam limits and selection. |
| Caller | Eligibility, borrowed services, rendering/export and external delivery effects. |

## Immutable inputs and exact remainder

`Plan(document)` returns a content-derived immutable SpanPlan without fitting.
`NewPagination` binds that plan, a catalog, the matching prepared snapshot/session,
a fitting backend, eligibility and fixed scoring/limit configuration. It performs
no font or fitting work. The execution object has no mutable current position.
Each public operation receives an explicit State.

A state retains persistent register versions, a cursor for each partly consumed
head, activation/pending facts, span and page numbering, and opaque context
identity. It retains no session or mutable fitting workspace. Completed items
leave their queue; a partially consumed item stays at the unchanged head with
its exact opaque checkpoint. Logical positions and copied progress vectors are
inspection/comparison data, never authority to recreate or exchange checkpoints.

States from another load, preparation or pagination context are invalid.
Validation precedes backend calls and checks the borrowed session. A rejected
alternative, failed operation or interruption cannot consume shared input.
Returning a bank version does not authorize replacing the state's internal
continuations with a caller-derived queue.

Residual material is the sole `main` item when present. Channels use main-first,
then first-occurrence source order. Valid explicit `flow-into: main` is ignored
only after route validation. Every queue retains source authoring order: no
sorting by anchors and no skipping an earlier unconsumed item.

## Owning links and source events

An owning link is an explicit source annotation:

```html
<p>The passage introducing a note <a href="#note-17" data-flow-anchor>17</a>.</p>
<aside id="note-17" style="flow-into: notes">The note text.</aside>
```

Ordinary cross-references establish no placement relationship. Every active
secondary branch requires exactly one active owning link. Its same-document
fragment must resolve uniquely to an active secondary branch root. Suppressed,
inert, ambiguous and interior targets are invalid. Ownership must form an
acyclic forest rooted in main; links inside a secondary branch may own another
branch transitively. Source proximity does not infer ownership.

An owning anchor is a placement relationship, not a forced page break or span
boundary. A target may be deferred with loss. Queue order still prevails when
target source order differs from anchor order. A future-span queue head can
block a target needed now; that is valid authoring with possible infeasibility,
not a plan error to repair by reordering.

Annotations remain out of band. Visible anchors attach to their first placed
content through source/processed/cluster coverage. A shaped cluster that covers
a source byte carries its event as a whole; no splitting of a ligature is
invented. Atomic coverage attaches to the atomic occurrence, not an arbitrary
interior paragraph. Silent discretionary progress alone is not placed coverage.
An empty link uses following content in its block, then preceding content, then
an explicit structural boundary when the block has no content.

A secondary branch is eligible when its owner is already placed or is actually
placed on the candidate page. Arrangement enumeration can tentatively visit a
target before its owner; final closure validation requires factual owner placement.
Activation alone does not close a span. Pending targets remain pending until fully
consumed, including targets that have already produced a partial fragment.

## Synchronization and span boundaries

Effective `break-before: page`, left, right, recto and verso all mean a forced
page break. They impose no parity constraint and insert no blank page. Supported
breaks belong to ordinary main-flow blocks outside atomics; active inline and
secondary breaks are rejected. Suppressed content contributes no active boundary.

Builder provenance and extracted source maps propagate a first-child break to
opening ancestor edges where necessary. Coincident boundaries coalesce. An
initial boundary does not produce a blank page, and terminal exhaustion supplies
a final boundary. Source, structural and prepared coordinates remain distinct.

A span closes only when its main boundary is reached **and all transitively
related secondary material is drained**. Future-span main and secondary material
remain untouched. A drained span is both a synchronization boundary and the end
of an optimization span; selected pages are not revisited for subsequent spans.

`AdvanceSpan` requires the exact drained state and a live matching session. It
preserves page numbering, queue roots and the main continuation while entering
the next span. Terminal, foreign and incomplete requests fail unchanged.
Certified empty ranges can drain without a page; metadata advancement is never
published as a page with no placed material.

## Fitting and fragment geometry

Normal auto-height blocks can split between sibling material and paragraph lines.
Inline atomics are indivisible. Fitting uses explicit FirstFit and normal CSS
line height. Exact paragraph and structural continuations retain discretionary
carry, edge state and consumed atomic interiors across changed receiver widths.
Unconsumed suffix atomics may resolve at the new width.

A frame supplies its receiving content rectangle. Content styles remain those of
the source document. Within-frame block margin behavior follows the CSS core;
margins do not collapse across frames. Fragment top edges occur only on the first
piece, bottom edges only on the last and left/right edges throughout. Wholly
contained children retain all their edges. Closing edges fit with final content;
there is no detached trailing-edge fragment.

Intermediate fragment composition may defer vertical bounds, but final admission
requires Fits and valid finite geometry. Horizontal constraints and structural
validity remain checked. Consumed prefixes and their geometry never change when
a suffix is fitted elsewhere.

Fitting enumerates alternatives synchronously with explicit exhausted/stopped/error
outcomes and obeys a false callback return. An interrupted enumeration does not
prove NoFit. Page construction includes leaving frame space unused, explores
admissible channel arrangements and publishes only after complete local selection.
The selected page must place material; metadata-only advancement is not progress.

## Local selection and ties

Within one template, compare current-page loss, then:

1. Further logical progress in main.
2. Further progress in secondary channels in first-occurrence source order.
3. Earlier stable arrangement enumeration.

Progress compares completed whole queue items before the position in the
unchanged head. It is not a heuristic for future page cost. Equal-loss choices
between templates prefer earlier catalog ordinals. Equal-loss page sequences
compare their ordinal sequences lexicographically.

## Loss algebra

`Loss` is either a finite uint64 count of units or a tagged absorbing blocked
value. One point is 1,000,000 units. Every finite value ranks ahead of blocked;
blocked is not a raw floating-point infinity. Inputs and coefficients must be
finite and nonnegative. Weighted components round to units with nearest-even
rounding; sums and conversion overflow are errors. Components remain inspectable
beside the total.

Every feasible page has a strictly positive page cost. Underfill, one-line split,
carry-over and missing-main costs use explicit assessment facts. Eligibility and
hard fit failures do not become attractive numerical costs.

### Scoring working hypotheses

The formulas, coefficients and final-page treatment below are configurable
working hypotheses, not claims of typographic optimality.

| Component | Default weight in points |
| --- | ---: |
| Page | 1 |
| Underfill | 1 |
| One-line split piece | 10 |
| Pending target per carrying page | 1 |
| Missing main receiver | 1 |

Participation is determined at page entry: remaining main and already pending
secondary channels participate in underfill. A newly activated target can incur
carry on its anchor page but enters underfill participation on the following
page. Underfill is an area-weighted squared unused-height fraction; no
participating receivers means zero underfill. Measurements are checked, not
silently clamped.

When main finishes, its finishing frame and later main frames are exempt from
underfill in the numerator; all participating frame areas remain in the
denominator. This final-page treatment is provisional.

A one-line piece is penalized only when its paragraph is genuinely split.
Adjacent pieces in the same frame coalesce before counting; an unsplit one-line
paragraph is exempt. Carry is charged once per distinct pending target on its
owning-anchor page and each carrying page; a partial target remains pending.
Ordinary references add no charge. Missing secondary receivers use carry alone;
there is no duplicate missing-receiver cost. Missing-main cost applies only
while main remains.

## Deterministic pagespan search

Search starts with a default beam width of **32 unfinished candidates**;
configuration can change the width. Every retained parent is extended through all
eligible templates in catalog order before pruning that page-depth round.
PageAttempt itself is an explicit one-template operation and does not apply the
search eligibility filter.

Rank unfinished candidates by accumulated loss, then lexicographic template
sequence. Do not use local progress to break beam ties. Keep the best completion
separately; it consumes no beam slot, and the first completion does not stop
search. A valid minimum-next-page-cost bound permits pruning only on strict
greater-than, retaining equal-loss contenders for deterministic ties.

There is no remaining-cost estimate, state merging, automatic widening or retry.
Breakpoint vectors compare all channels; equal positions alone do not establish
behavioral equivalence or interchangeable continuations. Width cuts are heuristic
and reported separately from safe bound cuts. Without heuristic truncation,
optimality is relative to configured eligibility and the single local winner
returned for each template, not all possible global arrangements.

### Limit working hypotheses

Defaults are 100,000 local work steps, 1,000,000 page attempts per span and 10,000
depth rounds per span. These positive configurable work-unit caps are not timeouts.
Cancellation is explicit through context. Document pagination restarts span
limits for each new span. Measurements may motivate different caps; they do not
justify hidden retries or silent truncation.

## Outcomes and failure publication

| Operation/result | Meaning |
| --- | --- |
| Page Constructed | Frozen page, exact successor, score and progress. |
| Page NoFit | Complete local enumeration found no admissible page for this template; obstruction facts do not prove global infeasibility. |
| Page SpanComplete | Input span is already drained; no page is produced. |
| Search complete | A complete span sequence is selected. |
| Search infeasible | The configured search space was exhausted without a completion and without heuristic truncation. |
| Search exhausted after truncation | No retained completion; heuristic cuts prevent an infeasibility claim. |
| Cancellation, limit or backend failure | Explicit error; any complete incumbent is retained as unselected, never delivered automatically. |

`PaginationError` exposes category, available template/frame/channel/source
location and original cause. Invalid requests fail before fitting. Page-attempt
errors expose no candidate or successor. Search interruption may retain only a
fully completed incumbent, marked unselected; partial pages are not shipout input.

`Paginate` selects successive spans without delivering them. `DocumentResult`
retains successful selected spans and the exact retry/completion state. Failure
in a new span leaves its entry state; interruption between searches can leave the
preceding drained boundary. LastSearch is the most recent actual search. Check
Complete as well as error: infeasibility is an incomplete normal result.

## Frozen output and delivery

Pages retain compiled page/frame descriptions and independently frozen content
fragments. CSS geometry uses page-local or explicitly receiver-local CSS pixels;
paragraph output uses local big points. Renderers apply the documented receiver
and paragraph origins once and do not restack already positioned lines. Source
box identity is distinct from each fragment occurrence.

`Sequence.Shipout` accepts selected sequences only and synchronously calls a
PageConsumer in page order with independent page handles. It performs no layout,
shaping or session access. Consumer failures preserve their original causes and
report completed calls, failed page and next page. Cancellation after the final
successful callback can report complete delivery together with an error.

External effects cannot be rolled back. There is no automatic retry or hidden
resume cursor; explicitly calling Shipout again redelivers the sequence. Closing
preparation prevents resumption but does not invalidate frozen pages, provided
borrowed rendering resources remain valid.

## Open extensions

General floats and material-specific receivers, tables/replaced content, mandatory
per-page secondary channels, running material and richer fragmentation policies
remain open. Page-side parity is deliberately absent. Concurrent execution,
state merging, remaining-cost heuristics and automatic widening require new
contracts and evidence. Scoring/limit hypotheses need representative documents;
small proof cases do not settle them.
