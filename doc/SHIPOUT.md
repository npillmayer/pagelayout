# Shipout

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

For a global optimization there will be a couple of trials running. These are
conceptually independent from one another. Each one owns a local copy of the 
flow-material registers. A trial consumes the flow-material and tries out different
page templates, filling the frames with the material. Synchronisation points
will force a decision for the best trial-result. The trial-result is then
transformed to a sequence of pages, which may be shipped out.

Measureing the quality of trial-results may require a penalty model, similar to
the evaluation of horizontal material (see TeX or module `khipu`). Penalties may
measure (for example):

  - Fitting / filling of text to frames
  - Distance of float material referenced on the page and their final location
  - Sizing factor for images
  - Widows / clubs

For local optimization (one page per trial run) the decision point is always the current
page. (For very long books without forced page-breaks at chapters, an artificial
intermediate decision point would need to be introduced, if too much memory would
be consumed. Disregarded for now.)


## Template Selection

It is yet unclear how trial select templates for consideration. Strategies could
include:

  - Number of float boxes on the template
  - Other flow target referenced on the current page (aside, table, …)
  - Chapter / section headings
  - ToC or other special material pages


## Concurrency

Trial runs for sequences of pages offer a natural opportunity for concurrency.
Synchronisation points will collect goroutines and their trial-results. Then
the next bunch of trial-goroutines would start.

---

## Design discussion: pagination and shipout

Discussion recorded on 2026-11-23. The original notes above are preserved.
This section combines the subsequent findings and author clarifications.
Agreed requirements are distinguished from recommendations and open choices;
the operation descriptions below are conceptual, not adopted Go APIs.

### Agreed requirements

- Synchronisation points delimit **optimisation spans**. Each point closes the
  span since the preceding point, or since the document start for the first
  span. It is a pagination boundary and a decision boundary for that span.
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
- The responsibility split below is the basis for further discussion. Local
  versus multi-page optimisation, scoring and template-selection policy remain
  open.

"Prepared flow registers" describes the readiness of their material. It does
not require replacing register payloads: the existing `content.Box` handles
remain source identities, with prepared metadata and execution resources held
separately. Preparation completes before the render-tree phase begins. Fitting,
line breaking, atomic sizing and candidate page construction belong to that
render-tree phase; their dependence on receiving geometry remains unchanged.

### Responsibilities

| Responsibility | What it does |
| --- | --- |
| **Fragment fitting** | Fits material into receiving dimensions and returns placed fragments, legal break choices and exact continuations. |
| **Page construction** | Applies one template, fills its frame chains and produces a candidate page plus the resulting channel state. |
| **Pagination search** | Explores template and break choices, evaluates competing sequences and selects one. |
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
| **Page attempt** | One attempt to place material from a given pagination state onto one template. It may examine several legal frame-break choices. |
| **Page candidate** | A concrete result of a page attempt: page fragments, successor state and quality information. |
| **Pagination search** | The operation that explores possible page sequences within an optimisation span. |
| **Pagination candidate** | One competing, possibly incomplete page sequence, with its current state and accumulated quality information. |

Thus a pagination search extends pagination candidates by making page attempts.
A page candidate can be attached to a sequence without being selected for
shipout. Completion of one candidate page does not force a durable decision
about that page while competing sequences remain under consideration.

These terms also distinguish a logical search candidate from a goroutine: a
worker may evaluate several candidates, and a candidate may be evaluated
sequentially. Naming need not encode the eventual concurrency strategy.

### Pagination as a search over immutable states

The recommended common model for local and multi-page optimisation is:

```text
pagination state + template + break choices
    -> page candidate + successor state + quality information
```

A pagination state needs more than a register bank. A branch may be only partly
placed, and later decisions can depend on page position. Conceptually it includes:

- A persistent bank state for the remaining whole branches.
- Per-channel continuation state for unfinished material, identifying the
  source branch and exact remaining position.
- The current optimisation span and progress towards its synchronisation point.
- Page number/parity and any other context that affects eligible templates or
  future costs, such as running-heading state when repetition is supported.

The candidate sequence retains its page results and accumulated quality
information alongside that state. Deferred material may add further state when
floats are introduced; float placement remains outside the first fitting work.

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

We still need a source-level representation for synchronisation markers and a
rule assigning related material to spans. Routing by channel name alone cannot
tell whether a note belongs to the closing chapter or the next one. This is a
metadata and source-association question, independent of implementing floats.

Reaching the same source boundary does not necessarily make two successor
states interchangeable. Sequences with different page counts may leave different
parity, numbering or template context for the next span. Draining secondary
material removes one source of cross-span dependencies but does not remove
these others. The treatment of such boundary context remains open: selecting
the best sequence for one span is not automatically a document-wide optimum.

### Break alternatives as well as template alternatives

Searching only over templates would leave an important choice hidden. Two
attempts using the same template can end at different legal breaks. A page with
more unused space can enable a better following page, so always returning only
the longest fitting prefix would exclude those sequences before scoring them.

The fitting contract should therefore permit access to legal alternatives,
either as a bounded set or through an enumeration mechanism. An initial local
strategy may choose just one, but the boundary should not permanently impose
greedy fitting on every search strategy. The exact enumeration and its bounds
remain to be designed.

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
height or text offset. These contracts need refinement before the scheduler can
rely on them.

### Feasibility, quality and search policy

Keep feasibility separate from quality. Lost or duplicated material, an illegal
break, an unsatisfied mandatory receiver, or failure to drain a span cannot be
made acceptable by assigning a finite penalty. A candidate must first satisfy
the required constraints.

Quality information can then describe unused space, undesirable breaks,
template preferences and other tradeoffs. Widows/orphans or keep rules may be
hard requirements or weighted preferences according to the supported policy;
that choice should be explicit. Float-reference distance and image scaling fit
this model later without bringing float placement into the first implementation.

Retain separate quality components and let an evaluation policy combine them.
This makes selection explainable and avoids fixing one penalty formula in the
fitter. The aggregation rules, page-count effects, boundary costs and deterministic
tie-breaking remain open. Partial sequences that have consumed different amounts
of material cannot be ranked fairly by page fullness alone.

The same page-construction machinery can support several strategies:

- Local optimisation selects one extension after each page, retaining one path.
- Bounded look-ahead compares several pages before choosing an extension.
- A bounded set of promising candidates limits search size within a span.
- Exhaustive search can explore all permitted sequences for small inputs.

Maintaining several candidates is multi-page search, not by itself a guarantee
of a global optimum. Such a guarantee requires adequate coverage of legal
choices and sound pruning. Local and global policy remain open; a useful first
implementation can be deterministic and sequential while preserving the shared
state and result contracts. One-page decision granularity does not itself
require immediate output; selection and shipout scheduling are separate concerns.

Template selection should likewise remain a distinct policy over the catalog
and pagination state. Eligibility may depend on page parity, span position,
headings and pending channels; ordering may express author preference or search
heuristics. The existing proposal of a caller-supplied finite candidate list is
a possible starting point, not an adopted final selection strategy.

Progress must be observable across channels. A page draining secondary material
can make progress even when the main flow has reached its boundary. Endless
empty pages must be rejected; deliberate blank pages need a bounded rule, such
as a parity transition. A search that cannot place pending indivisible material
should report the material and receiving constraints rather than loop forever.

### Fixed phases and concurrency within pagination

The fixed phase order does not require all page attempts to run sequentially.
It establishes a preparation barrier: **every search begins with prepared
material**, and any parallel search happens within the render-tree phase.
There is no concurrent pipeline of box construction, preparation and pagination.

Immutable source material, catalogs, bank states and completed results can be
shared. Logical independence of candidates nevertheless does not establish that
all execution resources are safe to share concurrently. The current
`PreparationSession` and borrowed font/shaping services require serialized use;
copying either its wrapper or a register bank does not create isolated execution
state. Geometry-dependent work and lazy discovery can still use mutable state
after base preparation has completed.

Parallel pagination therefore needs an explicit contract for sharing completed
preparation while isolating worker execution state or safely coordinating shared
services. This is an integration requirement to solve, not a reason to overlap
the fixed phases or redo source preparation for each page attempt. No session
forking API or concurrent-execution guarantee is currently implied.

The recommended first step is deterministic sequential evaluation, with the
logical candidate state already independent of workers. Parallel evaluation can
then preserve the same selection semantics, including deterministic tie-breaking,
cancellation and release of abandoned candidates. A synchronisation point is a
document/optimisation boundary whether or not goroutines are used.

Selected pages should retain frozen fragments, paint, source references and
stable rendering resources without retaining abandoned search workspaces.
Preparations and continuations remain live while any candidate can still resume
their material. Shipout consumes selected output, with its required font resources
kept valid, independently of the lifetime of discarded candidates.

### Questions for the next design discussion

1. Adopt or replace the proposed names: page attempt, page candidate, pagination
   search and pagination candidate.
2. Define synchronisation markers and how routed material is associated with an
   optimisation span, including material spanning a source boundary.
3. Specify the resumable box-fitting contract and how it exposes alternative
   legal breaks, progress, oversized material and unsupported cases.
4. Decide how residual content is bound to a receiving channel before pagination;
   it must not disappear merely because it has no explicit flow name.
5. Define template eligibility, required-channel rules and how secondary material
   is drained after the main flow reaches a boundary.
6. Specify boundary context, quality components and the first search strategy,
   without confusing span optimisation with guaranteed document-wide optimality.
7. Define worker/session ownership before enabling concurrent attempts, and the
   frozen page/output contract used by shipout.

The existing [page-construction discussion](DESIGN.md#5-try-a-page-from-register-snapshots-proposed)
and [preparation ownership rules](PREPARATION.md#11-registers-prepared-snapshots-and-functional-ownership)
provide the related contracts. Their earlier single-page commit proposal must
be read alongside the distinction here between building a candidate page and
selecting a complete candidate sequence.
