# Shipout

---

[**REMARKS**]

TeX uses the term "shipout" for the subroutine of completing a document-page and
writing it out on file (in print format). The shipout procedure collects vertical
material until the page box is (more than) full, then freezes the material and
puts it to the `output routine`.


## Local vs global Optimization

TeX does global optimization for paragraph breaks, but not for pages. In modern
times, documents fit into memory easily and we may think about solutions for
document-wide (or chapter-wide) optimal page-breaks. "Shipout" for global optimization
would then mean the output of a series of pages at once.

We will leave the decision about local or global pagination open for now, but we
will collect arguments for and against it in this document.

### Types of Media

For belletristic books, single page decision for breaks is usually good enough, as 
there is not much variety from page to page in terms of typesetting. For our intended
template-based page-layout this is not necessarily the primary target. Layout templates
would lend themselves well for magazines or scientific papers.


## Pagination Strategy

We will call locations in the document where a forced page-break sits "synchronisation
points". Examples may be chapters or the document end.

[**Revised**]
"Global optimization" means optimizing the selection of page-templates. Placing 
flow-material on a page is not subject to global optimization. The task of
the page-breaker is as follows:

- Find sequences of page-templates for a pagespan, in a dynamic search tree
- Each branch of the search owns a local copy of the flow-material registers
- A branch may split if more than one template is feasible when looking for the next one
- A trial consumes the flow-material and fills a page with a local paradigm
- Page-templates will be tried out with page construction, possibly creating a candidate page
- If a candidate page is created, append it to a sequence of page-templates (of this search tree branch)

Synchronisation points will force a decision for the best pagespan-result.
The sequence of pages will be shipped out.

When a trial fills the frames of a single page, the decision where to cut of the flow-material
is local. The decision will depend only on the current quality of the constructed page.
Measuring the quality of trial-results will be called "minimizing a loss function".
It may regard (for example):

  - Fitting / filling of text to frames
  - Distance of float material referenced on the page and their final location
  - Sizing factor for images
  - Widows / clubs

Deciding the cut-off is independent of the decision on successive page-templates.
There is not global (multi-page) decision about flow-material cut-offs to reach some
kind of best-fill optimum.

Each single candidate-page therefore carries a loss-factor. The optimization tasks is
to find the sequence of candidate-pages will the minimum loss-sum (sum of loss-factors).

For local optimization (one page per trial run) the decision point is always the current
page. (Global optimization: For very long books without forced page-breaks at chapters, an
artificial intermediate decision point would need to be introduced, if too much memory would
be consumed. Disregarded for now.)

### Paragraph-Breaking vs Page-Breaking

Horizontal material gets broken into line-boxes.
Material gets collected until a line is full, then broken apart. Remainder (e.g. the suffix of a
discretionary) stays with the material queue. This is done until end-of-par is reached.
Global optimization: loss function over paragraph-wide penalties.

![Line breaking from horizontal material](diagrams/khipu-breaking.png)

Vertical material gets broken into page-boxes.
Material gets collected until a frame is full, then broken apart. Remainder (e.g., bottom halve of
a vbox) stays with the material queue. Inner loop continues until all frames stemming from a region
on a page are full. Outer loop continues until synchronisation point is reached (end-of-pagespan).
References may refer to material in other flow-channels and their synching is part of a loss-calculation.
Global optimization: loss function over pagespan-wide losses.

![Page breaking from vertical material](diagrams/page-breaking.png)

| | Paragraph | Page |
| --- | --- | --- |
| **Source** | khipu | prepared boxes in register |
| **Material** | boxes, glue, penalties | recursive CSS boxes (nothing prevents us from adopting glue + penalty) |
| **Sync.-point** | paragraph | forced page break (CSS: `break-before: page`) |
| **Splitting** | discretionary | vsplit |
| **Local strategy** | first-fit | single page shipout (like TeX) |
| **Global strategy** | Knuth+Plass | pagespan shipout - optimal range of selected templates |


## Template Selection

It is yet unclear how trial select templates for consideration. Strategies could
include:

  - Number of float boxes on the template
  - Other flow target referenced on the current page (aside, table, …)
  - Chapter / section headings
  - ToC or other special material pages


## Concurrency

[**Outdated**] Trial runs for sequences of pages offer a natural opportunity for concurrency.
Synchronisation points will collect goroutines and their trial-results. Then
the next bunch of trial-goroutines would start.

[**Revised**]
Line-breaking (K&P) is effectively a dynamic tree search. The same is true for searching a
globally optimal sequence of page-templates. We will postpone the idea of concurrency.
If anything, the pagespan optimization could be handled by a pool of workers.

---

## Design discussion: pagination and shipout

Discussion recorded on 2026-11-23 and revised on 2026-11-24 against the author's
updated remarks, which are preserved above. Global optimisation here means
choosing a sequence of page templates within a pagespan; flow-material cut-offs
are selected locally during page construction. The earlier recommendation to
expose cut-off alternatives to global search is superseded.

Agreed requirements are distinguished from recommendations and explicitly open
choices. The operation descriptions below are conceptual, not adopted Go APIs.

### Agreed requirements

- Synchronisation points delimit **pagespans**, the optimisation spans. Each
  point closes the span since the preceding point, or since the document start
  for the first span. Forced page breaks, authored as `break-before: page`, and
  document end supply these boundaries.
- **Forced page-break variants of `break-before` are equivalent to `page`**
  for this profile, including `recto` and `verso`. Each requests a forced page
  break and synchronisation point. Their page-side distinctions are disregarded;
  they impose no parity constraint and do not request blank pages.
- **Page construction selects flow-material cut-offs locally**, using the
  quality of the current page. It does not choose a cut-off because of a
  prospective successor template or a better later-page fill.
- **Global optimisation searches template sequences for a pagespan.** Each
  search branch owns independent flow-material state and extends its sequence
  with the page candidate produced by a local construction attempt. Alternative
  cut-offs for the same input state and template are not global search branches.
- Each page candidate carries a scalar **page loss**. The pagespan objective is
  to select a complete feasible sequence with the **minimum sum of page losses**.
- Reaching a synchronisation point in the main flow is insufficient on its own.
  **All related material in other flows must be drained** before the point is
  complete. This is a requirement, not an optional policy allowing that material
  to spill into the next span.
- The overall phase order is fixed: **box tree -> preparation -> render tree**.
  These phases do not run concurrently. All page attempts and pagination
  searches start with prepared flow registers; they do not wait for a producer
  to load or prepare more source material concurrently.
- Trying material on one page template and searching for a sequence of pages
  are distinct activities and need distinct names. The unqualified word
  "trial" currently obscures that distinction.
- **Concurrency is deferred.** A search branch is a logical candidate, not a
  goroutine. A worker pool may be considered later without overlapping the fixed
  overall phases.

The responsibility split below remains the basis for further discussion. The
choice of local or pagespan template selection, the search algorithm, template
eligibility and the page-loss formula remain **open**. The meaning of global
optimisation and its sum-of-page-losses objective are specified above.

"Prepared flow registers" describes the readiness of their material. It does
not require replacing register payloads: the existing `content.Box` handles
remain source identities, with prepared metadata and execution resources held
separately. Preparation completes before the render-tree phase begins. Fitting,
line breaking, atomic sizing and candidate page construction belong to that
render-tree phase; their dependence on receiving geometry remains unchanged.

### Responsibilities

| Responsibility | What it does |
| --- | --- |
| **Fragment fitting** | Supplies legal fragments and exact continuations to the local page constructor. |
| **Page construction** | Applies one template, selects cut-offs using current-page quality, and returns a page candidate, successor channel state and page loss. |
| **Pagination search** | Explores template sequences and selects a completed pagespan with minimum accumulated page loss. |
| **Shipout** | Delivers the selected, frozen pages to an output consumer. |

Under this decomposition, shipout starts with selected pages. It does not choose
breaks or mutate flow registers. Selection can deliver one page or a sequence;
output format and rendering backend need not determine the search strategy.

The recommended ownership boundary is that `css-box-layout` owns structural box
fitting and fragmentation, with paragraph fitting supplied by an adapter.
Typography/khipu own paragraph breaking and continuation. `pagelayout` owns
frame traversal, channel coordination, template choices and page-sequence
selection. This extends the existing division without introducing another box
layout engine in the scheduler.

### Proposed terminology

The following names are recommendations, not settled public API names:

| Term | Meaning |
| --- | --- |
| **Page attempt** | One local construction attempt for a given pagination state and template. It may examine several cut-offs internally and selects a result using current-page loss. |
| **Page candidate** | The selected result of a successful page attempt: page fragments, successor state and scalar page loss, optionally with a loss breakdown. |
| **Pagination search** | The operation that explores template sequences within a pagespan. |
| **Pagination candidate** | One competing, possibly incomplete page sequence, with its current state and accumulated sum of page losses. |

Thus a pagination search extends pagination candidates by making page attempts.
A page candidate can be attached to a sequence without being selected for
shipout. Its cut-offs are fixed within that candidate; the search may retain or
discard the candidate sequence, but does not reopen those cut-offs to improve
successor pages. Competing templates can still produce different cut-offs because
their receiving geometry differs.

Terminology remains **open**. These names describe responsibilities and results
without prescribing the deferred concurrency strategy.

### Pagination as a search over immutable states

The conceptual page-construction operation used by either template-selection
strategy is:

```text
pagination state + template + local construction policy
    -> chosen page candidate + successor state + page loss
    or infeasible / error
```

Cut-off selection is inside this operation. The pagination search invokes it
for eligible templates; it does not supply future-page objectives or request
alternative cut-offs as additional successors for one template.

**Recommendation, open:** make local selection deterministic, including ties,
so the same state, template and policy produce the same selected candidate.
Infeasibility under the local policy must remain distinguishable from an
operational error. The exact outcome types and handling rules are still open.

A pagination state needs more than a register bank. A branch may be only partly
placed, and later decisions can depend on page position. Conceptually it includes:

- A persistent bank state for the remaining whole branches.
- Per-channel continuation state for unfinished material, identifying the
  source branch and exact remaining position.
- The current optimisation span and progress towards its synchronisation point.
- Page number/parity and any other context that affects eligible templates or
  future costs, such as running-heading state when repetition is supported.

The candidate sequence retains its page results and accumulated loss alongside
that state. Pending references and their source positions may contribute state
when related material is placed later. The precise representation is open;
float placement remains outside the first fitting work.

The persistent registers already support independent derived banks sharing
immutable material. Copying a bank does not copy prepared trees or mean that a
partially consumed branch can be represented by an ordinary whole-box `Take`.
Continuations therefore remain separate from unchanged `content.Box` payloads.
The exact representation and ownership of those continuations must still be
specified.

An attempt reads one state and returns another. It does not modify the starting
state, source styles, compiled template geometry or earlier page candidates.
Rejecting a candidate leaves competing candidates usable. Selecting a sequence
publishes its pages and final channel state together, so consumption of a
heading or note cannot escape from a page sequence whose body placement fails.
This is compatible with the functional-data-structure discipline.

### Completing an optimisation span

The draining requirement applies to material **related to the closing span**,
not to every later item in every register. Since preparation precedes pagination,
registers may already contain material for subsequent chapters. That future
material must remain available for the next span.

Once the main flow reaches its boundary, the candidate must finish any remaining
related secondary material before admitting main-flow material from the next
span. Depending on permitted templates, that may require pages carrying only
secondary material. If the permitted templates cannot place it, the candidate
cannot complete the span; silently carrying it across the boundary or dropping
it would violate the draining requirement. At document end, all participating
material must be exhausted.

The remarks identify `break-before: page` as the authoring mechanism for a
synchronisation marker. **Decision (2026-11-24):** normalize all forced page-break
variants, including `recto` and `verso`, to the same effective `page` marker.
The rule applies to forced page-break values. Side-specific behaviour is deferred:
these values neither constrain the next page's side nor insert blank pages to
reach it. The resulting marker closes a pagespan and requires related material
in other flows to drain under the same rules as `break-before: page`.

The current [content CSS profile](../content/profile.go) does not admit
`break-before`. Admitting the property, applying this normalization and retaining
the effective marker through source styling, routing and preparation are
pagination prerequisites; recording the decision does not implement them.

**Open:** define the marker's typed representation and propagation, and assign
related material to pagespans. Routing by channel name alone cannot tell whether
a note belongs to the closing chapter or the next one. Marker scope, its position
relative to a routed branch, and association of secondary material need explicit
rules. These are metadata and source-association questions, independent of
implementing floats. The effective `page` meaning and normalization of forced
page-break variants are settled.

Reaching the same source boundary does not necessarily make two successor
states interchangeable. Sequences with different page counts may leave different
parity, numbering or template context for the next span. Draining secondary
material removes one source of cross-span dependencies but does not remove
these others. **Open:** define how boundary context becomes the next span's
starting state. The specified objective is pagespan-wide template optimisation;
it does not claim a document-wide optimum across those boundaries.

### Local cut-off selection and fragment fitting

Local means the **whole current page**, including its frame chains and related
channels. It need not mean independently taking the longest prefix that fits
each frame. A page constructor may compare legal cut-offs across that page and
select them using current-page loss, without evaluating successor templates or
later-page fill.

The fitter may expose legal fragment alternatives to that local constructor.
**Open:** choose the local selection algorithm and how fitting alternatives are
obtained, compared and bounded. This is an internal page-construction contract;
it is not an interface for branching over cut-offs in the global search. A local
policy's chosen cut-off remains fixed even if a different cut-off could have
improved a later page.

The main missing foundation is **finite-height, resumable box fitting**.
Whole-subtree layout at a width is available, but does not yet supply partial
box fragments or their exact continuations. Fitting must respect each actual
frame's dimensions and legal breaks; summed frame heights are insufficient.
Continuing into a different width must reflow only the unplaced suffix while
preserving accepted fragments and their matching atomic interiors.

Paragraph continuation facilities already provide useful building blocks.
The integration must also carry open-container and box-boundary state, retain
source identity, and associate paragraph fragments with their output. An opaque
continuation plus explicit progress/exhaustion outcomes is preferable to a bare
height or text offset. **Open:** specify these contracts and their initial
fragmentation profile before the scheduler relies on them. The remarks' `vsplit`
analogy requires an exact structural remainder, not a geometric clipping of an
already laid-out box.

### Feasibility, page loss and pagespan loss

Keep feasibility separate from loss. Lost or duplicated material, an illegal
break or an unsatisfied mandatory receiver cannot be made acceptable by assigning
a finite loss. A completed pagespan must also satisfy the draining requirement;
an incomplete search branch can still have pending related material. Eligibility
and legal-break constraints apply before loss is used to select among feasible
results.

The specified objective for a completed candidate sequence is:

```text
pagespan loss = sum(page loss for every page in the candidate sequence)
```

Aggregation across pages is therefore settled as summation. **Open:** define
the scalar page-loss function, its components and weights, tie-breaking, and
treatment of the final page. Loss may reflect unused space, undesirable breaks
and template preferences. Whether widows/orphans or keep rules are hard
requirements or weighted loss terms must be explicit. Any preference for fewer
pages needs to be represented in this model, for example by a per-page cost;
no such term is adopted yet.

**Recommendation, open:** retain a component breakdown alongside the scalar
loss so choices can be explained. The page constructor and pagespan evaluator
must use a consistent definition. The fitter should supply geometry and legal
fragments without embedding the complete scoring policy.

Within a pagespan, distance between a reference and its related material may
contribute loss. At its closing boundary, draining all related material is a
hard condition. These two uses of synchronisation must remain distinct.

Some distance terms cannot be known on the page containing the reference if
the referenced material is placed later. **Open:** specify when such loss is
charged and which pending-reference information is retained. One possible rule
is to record the reference position and charge its distance on the page that
places the material. Another is incremental delay loss while material remains
pending. These are accounting proposals, not adopted float policies. They can
preserve additive page losses and local cut-off decisions without revising an
earlier candidate's cut-offs using future-page results. Float placement and
image scaling remain deferred implementation work.

### Template search and the meaning of optimality

The global objective is the minimum-loss template sequence **under the chosen
local page-construction policy**. It does not include sequences that could only
be obtained by replacing that policy's cut-offs with globally preferred ones.
If a locally selected result eventually leads to an infeasible sequence, the
search can reject that sequence and try other templates; it does not reopen
the local cut-offs as another search dimension.

The same construction operation can support several template-selection
strategies, whose choice remains **open**:

- Local selection chooses a template extension after each page, retaining one
  path.
- Bounded look-ahead compares template sequences over several pages.
- A bounded set of promising candidates limits search size within a pagespan.
- Exhaustive search explores all permitted template sequences for small inputs.

Exact pagespan optimisation requires sufficient exploration of eligible template
sequences or sound pruning. A bounded or heuristic search may approximate that
objective without guaranteeing its minimum. **Open:** define the search
algorithm, bounds, pruning and state-equivalence rules. Partial sequences that
have consumed different amounts of material cannot be ranked fairly by page
fullness alone.

The paragraph-breaking analogy helps describe splitting, remainders and a
terminal synchronisation point. For page-template search, the state includes all
flow cursors and continuations, page context, and any pending references needed
for future eligibility or loss. Reaching the same page count or template does
not make two states equivalent. Merging states or discarding one as dominated
requires agreement on every future-relevant part of that state; the precise
equivalence contract remains open.

The distinction between selecting locally after each page and selecting a
completed pagespan remains available. It does not alter the fixed local
construction policy. Shipout scheduling and its output contract still need to
be specified for these modes; completed pagespan selection delivers the chosen
sequence, as described in the remarks.

Template selection should likewise remain a distinct policy over the catalog
and pagination state. Eligibility may depend on page parity, span position,
headings and pending channels; ordering may express author preference or search
heuristics. The existing proposal of a caller-supplied finite candidate list is
a possible starting point. **Open:** define eligibility, enumeration and required
channels; neither a final selection strategy nor a fixed candidate list is
adopted here.

Progress must be observable across channels. A page draining secondary material
can make progress even when the main flow has reached its boundary. Endless
empty pages must be rejected; deliberate blank pages need a bounded rule, such
as an explicit scheduler request. The normalized `break-before` variants do not
request them. A search that cannot place pending indivisible material should
report the material and receiving constraints rather than loop forever.

### Fixed phases and deferred concurrency

Concurrency is explicitly **deferred**, as stated in the revised remarks.
Sequential search can establish page construction, loss accounting and pagespan
selection without a worker/session-forking contract. Logical search branches
remain independent of how they might later be scheduled.

The fixed phase order establishes a preparation barrier: **every search begins
with prepared material**. There is no concurrent pipeline of box construction,
preparation and pagination. A later worker pool would operate within the
render-tree phase and would preserve this ordering.

Immutable source material, catalogs, bank states and completed results can be
shared. Logical independence of candidates nevertheless does not establish that
all execution resources are safe to share concurrently. The current
`PreparationSession` and borrowed font/shaping services require serialized use;
copying either its wrapper or a register bank does not create isolated execution
state. Geometry-dependent work and lazy discovery can still use mutable state
after base preparation has completed.

**Deferred/open:** if parallel evaluation is introduced, specify how completed
preparation is shared while worker execution state is isolated or shared services
are safely coordinated. That later integration must preserve the fixed phases
and must not require redoing source preparation for each page attempt. No session
forking API or concurrent-execution guarantee is implied by the current design.

Worker granularity, cancellation, cleanup and preservation of deterministic
selection are deferred questions. A synchronisation point is a document and
optimisation boundary; it does not imply a goroutine join or one worker per
search branch.

### Output ownership

Selected pages should retain frozen fragments, paint, source references and
stable rendering resources without retaining abandoned search workspaces.
Preparations and continuations remain live while any candidate can still resume
their material. Shipout consumes selected output, with its required font resources
kept valid, independently of the lifetime of discarded candidates. **Open:**
specify the frozen page representation, publication and output-consumer contract.
These lifetime requirements apply to sequential evaluation as well.

### Open decisions

The following choices remain open; none reopens the local cut-off boundary or
the sum-of-page-losses objective:

| Area | Decision still to make |
| --- | --- |
| Terminology | Adopt or replace page attempt, page candidate, pagination search and pagination candidate. |
| Synchronisation metadata | Implement the settled normalization of forced `break-before` page variants to `page`; define typed markers, scope, propagation and association of related material with pagespans. |
| Local page construction | Define the algorithm for choosing cut-offs within one page, fitting-alternative access, local tie-breaking and explicit success/infeasible/error outcomes. |
| Resumable fitting | Specify exact structural continuations, supported fragment boundaries, progress/exhaustion, oversized material and unsupported cases. |
| Residual content | Bind nonempty residual material to receiving frames explicitly; it must not disappear because it lacks a flow name. |
| Template eligibility | Define candidate enumeration, required-channel rules, secondary-material drain pages and bounded deliberate blank pages. |
| Page loss | Define components, weights, hard versus soft break rules, final-page treatment, any per-page cost and loss breakdowns. Summation across pages is already specified. |
| Related-material loss | Define pending-reference state and when distance/delay loss is charged, without using future pages to revise local cut-offs. Float placement remains deferred. |
| Search | Choose local or pagespan template selection, algorithm, exactness guarantees or approximation bounds, pruning and state equivalence. |
| Span boundaries | Define the parity, numbering and other context carried into the next pagespan. |
| Output | Define immutable page results, atomic publication with channel state, retained resources and the shipout consumer contract. |
| Concurrency — deferred | If later enabled, define worker/session ownership, safe preparation sharing, scheduling, cancellation and deterministic selection. |

The [proposed page-construction contract](#proposed-page-construction-contract)
below makes concrete recommendations for the inputs, exact remainder, scalar
loss, outcomes and ownership. It remains a proposal; the exact API, local
algorithm and other unresolved choices are explicitly marked open.

The existing [page-construction discussion](DESIGN.md#5-try-a-page-from-register-snapshots)
and [preparation ownership rules](PREPARATION.md#11-registers-prepared-snapshots-and-functional-ownership)
provide the related contracts. Their earlier single-page commit proposal must
be read alongside the distinction here between locally constructing a page
candidate and globally selecting a sequence of templates. The updated remarks
and requirements in this document define that optimisation boundary.

## Proposed page-construction contract

Proposal recorded on 2026-11-24. This section records recommendations for
discussion, not an implemented or adopted API. The established requirements
above still apply: cut-offs are local to a page, pagespan search chooses template
sequences, related material drains at synchronisation points, and preparation
precedes pagination. Additional contract choices below await agreement; remaining
design details are listed under [open contract decisions](#open-contract-decisions).

The recommended operation is **atomic page construction**: given prepared
material, one template and a fixed local policy, return one chosen page, its
exact successor state and its loss. The caller's input state is never advanced
in place.

```text
ConstructPage(context, prepared context, template, state, span plan, policy)
    -> Constructed(page, next state, loss, progress)
     | NoFit(reason)
     | SpanComplete
     | error
```

These are conceptual alternatives. **Open:** choose the Go signature and outcome
representation. The result should distinguish these alternatives unambiguously;
an error must not accompany a usable partial candidate or successor state.

### Proposed inputs

| Input | Concrete contents |
| --- | --- |
| Prepared context | The matching `content.PreparedSnapshot` and `PreparationSession`, plus the fragment-fitting backend. Preparation is already complete. |
| Template | One compiled `template.Template`, including frame geometry, paint and source-ordered flow chains. |
| Pagination state | Remaining registers, per-channel continuations, current span identity, next page number and relevant carried context. |
| Span plan | Validated synchronisation boundaries, association of secondary material with the span, and positions beyond which construction must not consume. |
| Page policy | Required channel participation, supported break rules, local cut-off selection, page-loss evaluation and deterministic tie-breaking. |
| Context | Cancellation and deadline. Cancellation produces no candidate or successor state. |

The span plan should resolve residual-content binding explicitly. Construction
receives an unambiguous collection of participating channels, including whichever
receives residual content. It does not silently discard that content or invent
an implicit default receiver.

Policy and prepared context remain fixed throughout a pagespan search. Changing
fonts, preparation or local construction policy creates a different search
context; existing continuations must not silently migrate into it. **Open:**
define how context and policy identity are represented and validated.

The constructor uses each frame's compiled content rectangle. Frames receiving
the same channel are visited in catalog order. A pending channel without a
receiver remains unchanged unless the page policy requires that channel to
participate on this page. Template geometry and source content styles remain
unchanged; attachment to a frame does not trigger another cascade.

### Proposed exact remainder

Represent the remainder as an immutable pagination state. Keep an unfinished
branch at its register head, with an opaque continuation beside it:

```text
Input:
    main register = [A, B, C]
    main cursor   = start of A

Page places:
    all of A, followed by a prefix of B

Successor:
    main register = [B, C]
    main cursor   = continuation within B
```

`B` remains the same `content.Box`; its cursor records the accepted prefix.
The proposed representation has these invariants:

- A channel has at most one unfinished branch, always its register head.
- An absent continuation means the beginning of the head. An empty register has
  no head continuation.
- A fully consumed branch is removed and its continuation cleared.
- Untouched channels retain their previous state.
- Register contents and continuations are published together. A raw bank alone
  is not the exact remainder.

Make the state opaque, with copied inspection views. Clients should not be able
to combine a continuation for `B` with a register headed by `C`. **Open:** define
the public inspection and initialisation API and the concrete continuation type.

A continuation must carry enough information to resume exactly: source and
preparation identity, open-container position, paragraph checkpoint, pending
boundary effects, and atomic realizations belonging to the accepted prefix.
Its structural and paragraph details belong below the page constructor, within
the ownership boundaries described below.

The central invariant is:

> Placed logical material followed by the remainder equals the input material,
> in order, without loss or duplication.

A height, text offset or clipped render tree cannot establish that invariant.
A different receiving width may change line breaks and atomic sizes in the
unplaced suffix. Accepted fragments and their matching atomic interiors remain
fixed. Earlier continuations remain reusable after a successful or failed
attempt, so competing candidates can start from the same state.

A synchronisation boundary can occur within a branch. Reaching it may leave
that branch at the register head, positioned for the next span. Completion is
therefore determined by the span plan and related-channel progress, not by an
empty bank alone. Material belonging to later spans remains untouched.

### Proposed successful output

Publish the following four results together, only after the whole attempt
succeeds:

| Output | Contract |
| --- | --- |
| Page | Immutable page dimensions, template identity, frame paint and ordered placed fragments, with page geometry in CSS pixels. |
| Next state | The exact remainder for every channel, updated carried context and next page number. It remains at the current span's boundary until the scheduler advances spans. |
| Page loss | One finite scalar plus an inspectable component breakdown. |
| Progress | Per-channel advancement and whether the current span is complete. Include the starting-state identity so the candidate's origin is unambiguous. |

This publication creates an independent candidate. It does not commit the
pagespan or mutate the scheduler's current state. The scheduler retains or
discards the candidate and later selects a sequence together with its final
channel state.

Each placed fragment needs its own identity. One source box may contribute
several fragments on the same page, so a map keyed only by `content.Box` is
insufficient. A fragment identifies its source box, receiving frame, geometry
and frozen output. **Open:** define fragment identity, ordering, output payloads
and the page inspection/rendering interface.

The page's renderable output and its successor state have different lifetimes.
Frozen output remains usable after preparation closes, provided its rendering
resources remain valid. Resuming the successor requires the matching live
execution context. A page must not retain abandoned alternative placements or
search workspaces merely to remain renderable.

### Proposed page-loss contract

Use a `float64` scalar constrained to finite, nonnegative values. Reject NaN,
infinity and negative loss as policy errors. Infeasibility has a typed outcome;
it is not represented by an infinite-loss sentinel. This numeric representation
is a recommendation awaiting agreement, not an existing type definition.

A practical initial loss shape is:

```text
page loss =
    page cost
    + weighted normalized underfill
    + soft break penalties
    + related-material penalties charged on this page
```

For ordinary vertical frames, normalized underfill can use the fitter's occupied
block extent, including relevant spacing and edges. Do not estimate occupancy
by summing independently measured box heights.

**Recommendation, open:** start with a positive per-page cost and configurable
underfill and break weights. Actual coefficients, normalization, final-page
treatment and treatment of intentionally empty frames remain open. Frame
participation and scoring rules should be established from the input and policy;
a candidate must not lower its loss merely by declaring an inconvenient frame
irrelevant.

The evaluator receives current-page facts and carried state. It does not examine
successor templates. A distance penalty involving previously referenced material
can be charged when that material is placed, with the reference position retained
in state. That accounting rule remains a proposal; the precise policy and pending
reference representation are **open**. Float implementation remains deferred.

Construction may compare several legal arrangements within the current page,
then returns one. **Recommendation, open:** use deterministic tie-breaking based
on a defined channel and breakpoint order. The pagespan search receives no
alternative cut-offs for the same state and template.

Pagespan aggregation remains the already specified sum of page losses. The
component breakdown supports inspection and explanation without introducing a
different objective for the page constructor and the pagespan evaluator.

### Proposed outcomes and failure reporting

| Outcome | Meaning | Published result |
| --- | --- | --- |
| `Constructed` | The local policy selected a feasible page making permitted progress. | Page, successor state, loss and progress together. |
| `NoFit` | This template cannot produce an acceptable page under the local policy: for example, no progress, a missing required receiver or blocked indivisible material. | Structured reason; no page or successor state. |
| `SpanComplete` | The input already reaches the boundary and all related material is drained. | No extra page or page loss; input remains unchanged. |
| Invalid-request error | Mismatched preparation, invalid state, closed session or inconsistent span/binding data. | No result. |
| Unsupported-feature error | Required fragmentation, CSS or paragraph behaviour is unavailable. | No result; must not masquerade as ordinary non-fit. |
| Execution error | Font/backend failure, cancellation, resource limit or invalid loss calculation. | No result; original cause preserved. |

`NoFit` describes this attempt; it is not proof that the material cannot fit any
permitted template. Conversely, an interrupted local search must not report
`NoFit` as though it had completed. A candidate whose page completes the span
is `Constructed` with span completion in its progress; `SpanComplete` is for an
input that already needs no further page.

Failures should carry template/frame, channel, source box and continuation
location when known, and preserve causes through `errors.Is`/`errors.As`.
**Open:** define diagnostic codes, concrete outcome/error types and how the
scheduler propagates operational failures. The distinction between ordinary
non-fit and an unsupported or failed execution must remain observable.

Ordinary successful pages must advance material. **Recommendation, open:**
exclude deliberate blank-page generation from the first contract and add it
later through an explicit bounded rule. Merely incrementing the page number
must never count as material progress. Pages draining secondary material can
make progress after the main flow has reached its synchronisation boundary.

### Proposed ownership boundaries

| Owner | Responsibility |
| --- | --- |
| `css-box-layout` | Structural fragmentation, legal box boundaries, container continuation, margins/edges and receiving constraints. No channel names, page search or font backend. |
| `content` | Access to private normalized trees, qualified source identities, preparation coverage and the adapter to core fitting. No mutable tree pointers escape. |
| Typography bridge | Paragraph fitting/checkpoints, atomic binding and frozen paragraph/interior output. |
| `pagelayout` page constructor | Frame traversal, channel coordination, local page selection, loss evaluation and atomic result publication. |
| Pagespan scheduler | Template enumeration, candidate sequences, accumulated loss and selection of the winning successor state. |
| Shipout consumer | Rendering or exporting selected frozen pages. |

Temporary geometry, alternative local placements and output collectors belong to
one attempt. Failure releases that work. Borrowed preparations, fonts and services
remain caller-owned; the constructor does not close them. Operations sharing a
preparation session remain serialized, with concurrency deferred.

Backend memoization may be warmed by an unsuccessful attempt, as permitted by
the existing preparation contract. It must not change observable content,
continuation or selection semantics. **Open:** define the new fitting interfaces
and how paragraph fragments are retained without putting backend payloads into
CSS source nodes or register items.

### Implementation prerequisites

This proposal requires a new resumable fitting path. Existing
[`content.Layout`](../content/layout.go) produces complete subtrees. The existing
[paragraph continuation](../../typography/glyphing/paragraph/flow.go) supplies
useful machinery, but its convenience path fixes atomic inputs and has a
restricted break profile. The constructor should depend on an explicit fitting
contract rather than treating either operation as a complete paginator.

The span plan also requires admission of forced page-break variants of
`break-before`, their normalization to `page`, propagation of the effective marker
and association of routed material with pagespans. `recto` and `verso` therefore
use the same marker and draining behaviour, without a page-side or blank-page
requirement. These prerequisites do not change the fixed phase order or permit
implicit re-preparation during an attempt.

### Open contract decisions

The section above proposes concrete defaults and invariants; recording them
does not settle their adoption. The following details remain explicitly open:

| Area | Recommendation supplied here | Decision still required |
| --- | --- | --- |
| Public operation | One atomic construction operation with distinguishable outcomes. | API location, signature, concrete types and names. |
| State and identity | Opaque state containing a persistent bank plus a continuation for each unfinished head. | Initialisation/inspection API and validation of source, preparation, span and policy identity. |
| Span plan | Explicit per-span consumption boundaries and association of related material; forced page-break variants normalize to `page` as a settled requirement. | Implement normalization; define marker propagation/scope, branch-interior boundaries, association rules and residual receiver binding. |
| Fitting | Structural continuations and exact logical remainders, with accepted prefixes fixed. | Fitting interface, initial fragmentation profile and treatment of edges, margins and paragraph policies. |
| Local selection | Return one locally chosen candidate, with deterministic ties. | Selection algorithm, alternative enumeration, bounds and tie-breaking order. |
| Page loss | Finite nonnegative `float64`, component breakdown, positive page cost and configurable underfill/break terms. | Confirm numeric representation and choose normalization, weights, hard/soft rules and final/empty-frame treatment. |
| Related-material accounting | Carry reference positions; charging distance on placement is one option. | Exact loss allocation and retained reference state; floats remain deferred. |
| Progress and blanks | Ordinary success advances material; exclude deliberate blank pages initially. | Initial-scope agreement, detailed progress reporting and any later bounded blank-page rule. |
| Diagnostics | Distinguish `NoFit`, already-complete spans and errors without partial publication. | Codes, outcome/error types, provenance detail and scheduler failure policy. |
| Output and lifetime | Immutable fragment occurrences, page geometry and frozen output; successor state remains resumable with its matching context. | Fragment IDs, typed output/inspection interfaces, resource retention and shipout consumer API. |

Template eligibility, exact versus bounded pagespan search, pruning and
cross-span context remain the separate [open design decisions](#open-decisions).
Concurrency is deferred and is not a prerequisite for this construction contract.
