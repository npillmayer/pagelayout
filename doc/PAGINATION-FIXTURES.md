# Pagination fixture reference

This reference describes contract fixtures, test inputs and expected results.
For client-facing sample inputs and rendered output, use the
[sample guide](../testdata/pagination/README.md). The broader contract matrix and
open calibration work are in [SHIPOUT-PLAN.md](SHIPOUT-PLAN.md).

Commands below run from the module root. Six independent contract tables describe
source planning, geometry, fitting, scoring, search and failure protocols. Their
expectations are not regenerated from the implementation. Pinned-font output and
search-quality observations have separate explicit refresh controls.

## Reproduce the real-font proof

```sh
go test . -run 'TestShipout.*Real|TestSelectedPagesReleaseSearchWorkspaces' -count=1 -v
PAGELAYOUT_SHIP_PROOF=testdata/pagination/shipout-proof.svg go test . -run '^TestShipoutRealFontDocumentProof$' -count=1
```

The font is pinned by SHA-256 in the proof tests. Independent assertions check
source conservation, geometry, activation and carry charges; the detailed
[JSON baseline](../testdata/pagination/proof-expected.json) records metrics and
losses. Only frozen pages leave the output-lifetime helper. Weak references cover
execution owners, factories, adapters, streams and rejected shaped paragraphs;
borrowed rendering resources remain intentionally live.

To intentionally refresh the JSON after reviewing changed expectations, set
`PAGELAYOUT_UPDATE_SHIP_PROOF=1` for the focused test. Normal tests never rewrite it.
Use the SVG for visual inspection alongside the independent assertions.

## Choose a fixture table

| Table | Input boundary | What to compare |
| --- | --- | --- |
| [template-expected.json](../testdata/pagination/template-expected.json) | Load [receivers.html](../testdata/pagination/templates/receivers.html). | Definition order, frame-chain order and compiled content rectangles. |
| [source-expected.json](../testdata/pagination/source-expected.json) | Load the named content file, apply listed additional CSS, then compile its plan. | Main aliases, source order, affinity/propagation, ownership/span membership and required diagnostics. |
| [fitting-expected.json](../testdata/pagination/fitting-expected.json) | Supply the declared synthetic structural/paragraph inputs, or the declared complete arrangement. | Exact fragments/remainders, geometry, atomic reuse, manual carry, local comparisons and span transitions. |
| [scoring-expected.json](../testdata/pagination/scoring-expected.json) | Supply the stated page facts directly to the evaluator. | Participating frames, components, exact integer loss and arithmetic failures. Scoring cases are working hypotheses. |
| [search-expected.json](../testdata/pagination/search-expected.json) | Supply the finite graph as a fake Page attempt. | Selected sequence, loss, retained frontier, pruning, completion/failure and interrupted-incumbent behavior. |
| [protocol-expected.json](../testdata/pagination/protocol-expected.json) | Construct the stated identity, session, failure or delivery scenario. | Atomic publication, error category/cause, state stability and output lifetime. |

## Beam-width quality and work counts

`TestSearchQualityWidthsAndExhaustiveReference` compares widths 8, 32 and 128 with
an independent depth-first reference that performs no beam/bound pruning or
merging. [search-quality.json](../testdata/pagination/search-quality.json) records the full observations:

| Case | Widths 8 / 32 / 128: attempts | Exhaustive attempts | Completed loss: 8 / 32 / 128 / exhaustive |
| --- | --- | ---: | --- |
| Late cheap completion | 360 / 1,320 / 1,640 | 1,640 | 10 / 10 / 2 / 2 |
| Repeated vectors | 36 / 39 / 39 | 39 | 3 / 3 / 3 / 3 |
| Strict lower bound | 6 / 6 / 6 | 9 | 3 / 3 / 3 / 3 |
| Actual page enumeration | 90 / 234 / 258 | 258 | 3 / 3 / 3 / 3 |

The actual-page case enumerates six equal-geometry templates over five synthetic
lines, with two-line receivers. It evaluates 132 / 276 / 300 local arrangements
(300 exhaustively), observes 40 repeated unfinished breakpoint vectors, and
discards 28 / 4 / 0 paths by width. It has a peak of 42 round-retained paths even
with a beam of 8: the complete next round is formed before pruning. Public
`PaginationSearch` outcomes and work counters are cross-checked against the
instrumented scheduler. No additional instrumentation API is added to production.

Metric definitions:

- `LocalAlternatives`: complete, activation-valid arrangements assessed locally,
  including losers; zero for finite graphs, which have no box fitter.
- `PeakFrontier`: unfinished paths retained after a round's pruning (initial path
  included); `PeakPending`: unfinished successors before that pruning.
- `PeakRetained`: parent frontier plus pending successors plus the best completion
  during a round. It excludes transient local alternatives and shared ancestor
  output prefixes, and is not a heap-byte estimate.
- `RepeatedVectors`: repeated logical breakpoint vectors among generated unfinished
  successors in one round, before pruning. It is not continuation equivalence.
- `SafeCuts` and `WidthCuts` distinguish proof-preserving bounds from heuristic
  truncation. Width zero denotes the exhaustive report only; its frontier/repetition
  metrics are not applicable and stored as zero.

Run `go test . -run '^TestSearchQualityWidthsAndExhaustiveReference$' -count=1`.
`PAGELAYOUT_UPDATE_SEARCH_QUALITY=1` explicitly refreshes the observed report;
the independent optimality/quality assertions still run. These finite observations
demonstrate both successful pruning and a quality loss under truncation. They do
not calibrate the default beam, establish representative document performance,
justify merging, or settle scoring/eligibility/limit hypotheses. No timing threshold
is used and no merging, widening or automatic retry is implemented.

## Acceptance evidence map

The contract matrix is exercised at these boundaries. Sibling suites cover
shared semantics and private ownership that root tests cannot inspect.

| Matrix families | Executable evidence |
| --- | --- |
| Default main; marker provenance; forced breaks; ownership errors | `content/ship_source_test.go`, `content/positions_test.go`, root `plan_test.go`; core `provenance_test.go` and DOM checked-break tests. |
| Structural splits; geometry/margins; atomics | Core `fit_test.go`, `fit_geometry_test.go`, `fit_composition_test.go`; bridge `fit_test.go`; root real-font atomic and oversized cases. |
| Paragraph prefixes; manual carry; bidi/source mapping | Typography `prefix_fit_test.go`, `prefix_manual_test.go`, integration render proofs; khipu manual checkpoint/continuation tests; bridge `fit_lifetime_test.go`; root manual proof. |
| Local choice; owning placement; authored FIFO; transitive drain | Root `page_test.go`, `page_channels_test.go`, `document_test.go`, and the two real-font documents. |
| Loss arithmetic; scoring facts | Root `loss_test.go`, page fixtures and the detailed proof-loss baseline. |
| Search reference; failure reporting; transactions | Root `search_graph_test.go`, `search_test.go`, `search_quality_test.go`, `document_test.go`, `shipout_test.go`; bridge failure/retry tests. |
| Lifetime; real fonts/quality | Root `shipout_realfont_test.go`, `page_realfont_test.go`, `search_quality_test.go`; bridge `lifetime_test.go`, `fit_lifetime_test.go`; typography and khipu private-lifetime checks. |
| Dependency boundaries; usage | `preparation/typography/dependencies_test.go`; executable `ExamplePagination_Paginate`. |

## Document pagination and delivery evidence

`document_test.go` runs `transitive.html` with `receivers.html` through `Paginate`:
span 1 selects `[note-first, figures-only]` (3,840,278 units), then span 2 selects
`[note-first]` (1,000,000 units). Pages are numbered 1, 2, 3. The main root and
exact opaque continuation survive the public `AdvanceSpan` transition; later
searches never revisit earlier selections. These costs retain their working-
hypothesis status.

The `later-span-failure` protocol is exercised with backend failure, infeasibility,
cancellation and limits: earlier selected reports remain available, completion is
false, and the unsuccessful span's entry permits an explicit retry. Empty documents
and initial empty spans produce no page. Certified empty main ranges still process
structural owners before terminal draining, including a notes-only receiver.
Painted/spaced boxes, definite-width boxes and atomics are not treated as empty.

`shipout_test.go` exercises `delivery-failure`: pages 1–3 are selected, page 2's
consumer fails, completed deliveries are `[1]`, failed/next page is 2 and page 3
is not called. The original error and selection survive. No external rollback is
claimed. It also checks rejection of unselected incumbents, cancellation before/
after calls, independent handles, zero-page delivery and closed-session output.
Gentium none/manual cases in `page_realfont_test.go` paginate and deliver selected
output after session close and GC, comparing frozen render proofs. The broader
multi-page visual, lifetime and search-quality evidence is documented in the [sample guide](../testdata/pagination/README.md).

## Multi-channel page evidence

`page_channels_test.go` runs `transitive.html` with `receivers.html` through
`PageAttempt`, using one 10px line per group. Its selected pages match the
`transitive-drain-and-transition` fitting case:

| Template / starting point | Placed material | Loss units |
| --- | --- | ---: |
| `note-first`, span 1 | Main through its boundary and the note; figure pending. | 2,000,000 |
| `figures-only`, draining span 1 | Figure; no missing-main charge. | 1,840,278 |
| `note-first`, after private span transition | Next main and its note; future material was untouched earlier. | 1,000,000 |

Additional tests retain FIFO `[A,B]` when B is owned first, keep B pending behind
inactive A, and place both in one frame once A is reached. They cover several
heads followed by a partial head and narrower continuation; reverse channel
dependencies and targets above owners; split/ordinary/empty owners; secondary-only
progress; fixed entry participation and per-target carry; signed root margins;
late backend failure with unchanged state/output and identical retry.
`cross-span-fifo.html` leaves a current target behind a future-span head and
reports copied queue obstructions without skipping or fitting that head.

`page_realfont_test.go` verifies transitive activation from Gentium ligature
coverage and manual hyphenation, including a narrow-to-wide main chain. Frozen
rendering survives session close and collection. `content/compose_test.go` and
the sibling core's `fit_composition_test.go` separately check immutable sibling
placement and the explicit intermediate-bounds option. Document driving and
delivery are exercised separately above. Numerical loss expectations
remain working hypotheses.

## Single-channel page evidence

`page_test.go` runs `two-templates-same-start` from the fitting table through the
actual `PageAttempt` API with a deterministic paragraph backend. `wide` consumes
eight lines; `narrow-chain` consumes four in each frame. Both cost 1,000,000 units
and leave the original state unchanged. Selecting between templates is a separate
search concern, covered by the search suites.

Additional synthetic cases in that test file pin these working scoring facts:

| Input | Selected result |
| --- | --- |
| Four 10px lines; main frames 100×30 and 100×20 | 2+2 lines, loss 1,066,667; the 3+1 arrangement incurs a one-line split penalty. |
| Two-line paragraph followed by four-line paragraph; 100×30 frame | Stop after the first paragraph, loss 1,111,111; taking one line of the second paragraph costs more. |
| Equal-loss arrangements with sufficient total space | Further logical progress wins; equal progress keeps longest-first stable arrangement order. |
| First receiver 100×1, then two 100×20 receivers; two-line material | First frame unused; finishing and later frames exempt, with all main areas retained in the denominator. |
| Main paragraph split before a forced marker | Exact continuation stays at the unchanged head; marker completion finishes the span without consuming the next block. |

`page_realfont_test.go` uses the checked-in Gentium fixture with the same compiled
receiver collection. It checks none/manual source conservation, percentage
atomics, multiple widths, immutable outline proofs and collection of the execution
object/session while frozen pages and a partial successor remain retained.
No installed fonts, network resources or new renderer are needed.

All tables except the catalog table contain `cases`, each with a stable `id`.
`expect` is the independently specified result, never a value to regenerate from
the implementation under test. Table/case `status` distinguishes settled contracts
from provisional scoring or numerical limits. Cost facts used to test search
ordering are supplied values, not additional default scoring policies.

## Source inputs and identities

Paths in JSON are relative to this directory. `stylesheets` lists additional
author sheets in order. `additional_css` supplies one further author sheet after
those files and the document head, with normal cascade precedence. It is test
input, not a request to fetch resources. No content case references remote assets.

Source labels are HTML IDs without `#`; `body` identifies the existing residual
root that becomes main. They do not prescribe numeric NodeIDs, BoxIDs, tree IDs
or runtime identity tokens. Duplicate `n` in `duplicate-target_err.html` is
intentional: diagnostics must identify both source occurrences by provenance.
`source_ids` identifies relevant locations; `required_diagnostics` uses semantic
categories, not frozen Go constant names or diagnostic prose. Do not require a
particular count/order of consequential diagnostics that the contract does not
specify. Error fixtures must not publish the invalid plan/document indicated.

The source cases exercise loading and plan validation at separate boundaries.
Source tests distinguish declaration-profile rejection from unsupported break
locations diagnosed by the plan. The `gate` field selects the relevant test layer.
`_err.html` names a deliberately invalid routing/ownership/break-location input;
the extra-CSS cases supply further deliberate profile failures without duplicating
the HTML files.

| Content family | Inputs and intended observation |
| --- | --- |
| Main | [main-default](../testdata/pagination/content/main-default.html) and [main-explicit](../testdata/pagination/content/main-explicit.html) have the same material/order, one residual/main tree and no duplicate preparation. |
| Affinity | [anchors](../testdata/pagination/content/anchors.html) supplies start, whitespace-only middle, trailing, empty-block, multiline and bidi owners, plus an ordinary reference. The trailing marker stays in its paragraph. |
| Boundary affinity | [boundary-anchors](../testdata/pagination/content/boundary-anchors.html) keeps a trailing owner in the closing span, a leading owner in the new span and a terminal owner before the final drain completes. |
| Normalization | [normalization](../testdata/pagination/content/normalization.html) puts a marker in an inline source split around a block. It attaches to following material in the original containing block; generated groups must retain provenance. |
| Breaks | [breaks](../testdata/pagination/content/breaks.html) supplies all five forced page values, an initial break, first-child propagation and terminal drain. [Additional CSS](../testdata/pagination/content/break-override.css) removes the left boundary through the cascade. |
| Transitive drain | [transitive](../testdata/pagination/content/transitive.html) has main → note → figure in span 1 and a later note in span 2. The later main/note material must remain untouched while the figure drains. |
| FIFO | [fifo](../testdata/pagination/content/fifo.html) owns B before A but queues A before B. B waits; A cannot be placed before its own anchor. |
| Cross-span obstruction | [cross-span-fifo](../testdata/pagination/content/cross-span-fifo.html) has a future-span A ahead of current-span B. The plan is valid, but exhaustive search cannot drain span 1 under the policies. Do not sort, skip or consume A early. |
| Edges | [fragments](../testdata/pagination/content/fragments.html) supplies asymmetric split-box edges and a contained child whose own edges survive. |
| Paragraphs/atomics | [atomics-manual](../testdata/pagination/content/atomics-manual.html) preserves two percentage-width atomics and explicit soft-hyphen source material. Supply explicit services for manual preparation. |
| Exclusion/empty | [suppression](../testdata/pagination/content/suppression.html) has inactive owners/targets and an inert template; [empty](../testdata/pagination/content/empty.html) requires no physical page. |

The source table specifies visible-link activation by source coverage. The exact
line wrapping in `anchors.html` is intentionally unspecified: inject a split at
the paragraph boundary under test and verify activation once. Bidi visual order
does not change ownership. The whitespace-only marker uses the adopted affinity
rule even if its whitespace contributes a collapsed space.

## Synthetic fitting inputs

All dimensions in the fitting table are CSS pixels. Synthetic paragraph line
heights, source ranges and width-dependent breaks are supplied facts; they make
the fitting contract independent of installed fonts or a particular shaper.
They are not claims about the natural rendering of the HTML text. Real-font
execution and lifetime are checked separately by the real-font proof suites.

`source_ranges` are half-open UTF-8 byte intervals in the stated processed text.
In `changed-width-lines`, `[0,6)` remains frozen at width 100; the suffix divides
into `[6,9)` and `[9,11)` at width 50. No line count or height substitutes for that
logical coverage. The atomic case keeps the accepted first interior unchanged
while the second atomic changes from its 50%-width rule at the new width.

The split-edge arithmetic is independent of HTML line breaking: five declared
10-pixel lines, top edges 8 and bottom edges 12 yield fragments of 28, 20 and 22
pixels. The last fragment does not fit a 21-pixel receiver. The contained-child
case separately verifies that the parent's split does not remove the child's
3-pixel top/bottom edges. These are direct neutral fitter inputs, not a second
interpretation of the entire `fragments.html` subtree.

Manual-discretionary cases supply synthetic khipu material explicitly. The
nonempty carry renders `ab-` then `cd` from unbroken `abcd`; the hyphen has
replacement provenance and the carried `c` must appear exactly once. An empty
post-break string still retains its discretionary identity. These cases do not
invent automatic hyphenation or prescribe a production hyphenator's dictionary.

`transitive-drain-and-transition` checks the specified three-page sequence with
declared 10-pixel block heights and the actual catalog rectangles. It does not
claim that a search has selected that sequence. Its first span costs 3,840,278
units: 2,000,000 on the anchor page and 1,840,278 on the figure-drain page. The
second span costs 1,000,000. Numerical costs follow the working scoring policy.

In local comparator cases, `positions` are abstract ordered logical ranks in
`[main, notes, figures]`, not runtime BoxIDs or line counts. A complete arrangement
is already known feasible. `same-later` loses only on stable enumeration order.
The shorter-prefix case supplies all needed current-page costs and must not
inspect a successor template to override that local result.

## Scoring and arithmetic

The table's `entry` is the state after immediately due events at attempt entry;
`pending_after` is the distinct set still pending after the candidate placement.
`target_channels` maps those target identities to channels. Frame indexes are
zero-based source/catalog order. Unless supplied, no paragraph is a one-line
split, main does not finish in a frame, and absent component costs are zero.

Fix participation from entry: main if it remains, secondary channels with an
already pending target. An activated-and-completed target incurs no carry;
an activated-and-incomplete target incurs carry now but does not change this
attempt's participating frames. The next attempt includes its channel.

For participating frames, use:

```text
area_i = width_i * height_i
unused_i = (height_i - occupied_i) / height_i
underfill = sum(area_i * unused_i^2) / sum(area_i)
```

Exempt finishing/later main frames contribute zero to the numerator and retain
their area in the denominator. No participating frames means zero underfill.
Impossible occupancy is rejected before evaluation. The empty-frame-set case
tests the evaluator only and does not itself establish a progressing page.
Internal consecutive paragraph pieces in the same frame are coalesced before
counting one-line split fragments.

Round each weighted component once, nearest unit with ties to even. The exact
ratios `1/12` and `121/144` give 83,333 and 840,278 underfill units respectively.
The separate arithmetic cases are settled contracts: half-unit values are exact
decimal test inputs; large uint64 endpoints are decimal strings to prevent a
JSON consumer from rounding them through float64. Parse them as integers. `NaN`
is an invalid-value test token, not a valid JSON number. Overflow has no finite
or blocked result. Carry timing and loss algebra are contracts independent of provisional
coefficient calibration.

## Finite search graphs

`search_graph_test.go` executes all twelve cases through the same private
scheduler used by `PaginationSearch`. It checks retained round frontiers,
incumbents, exact losses, attempt/pruning counts, all outcome categories and the
requested explicit widths. An independent depth-first traversal checks finite
graphs without beam or bound pruning. Additional cases cover nonalphabetical
catalog order, checked cost overflow, post-callback cancellation and attempt caps.

`search_test.go` compares actual PageAttempt traversal with exhaustive search for
changed-width paragraphs, `transitive.html` and `cross-span-fifo.html`. The
transitive fixture selects `note-first`, `figures-only` at 3,840,278 units and
leaves later-span material untouched. Cross-span FIFO blocking is infeasible
under the current policies. Eligibility/parity, width truncation vs proof,
late failures, copied selected/incumbent handles, identity, limits and retry are
also checked. `page_realfont_test.go` retains selected Gentium none/manual
rendering after session close and GC. The search tests select one pagespan and do
not claim optimization over alternate local cuts; document delivery is covered by the document and delivery tests.

Each graph declares template order, initial state, completed states, positive
minimum page cost and one edge per feasible state/template pair. Missing edges
are NoFit; entries in `errors` are operational failures. An edge already
represents the single locally selected Page candidate, not a cut-off choice for
the global search. Costs are integer units.

Compare sequences lexicographically by indexes in `templates`, not alphabetically
by template strings. The `width` applies only to unfinished candidates after a
complete depth round. Track the best completion separately. Safe lower-bound
pruning requires strict greater-than and is distinct from beam truncation.
`round_N_unfinished_sequences` describes the retained frontier after pruning.

`compare_widths` requests the same finite case at the additional explicit widths.
`exhaustive_reference` gives the unpruned result for the case that deliberately
loses its feasible path at width 1. There is no automatic retry. Repeated graph
state `X` deliberately models equal logical position vectors: retain both
candidate paths and make both sets of attempts; do not merge them.

`interrupt.after_round` injects cancellation or a depth limit after that complete
round and before another extension. The incumbent is complete but unselected;
no successor is committed and nothing is shipped automatically. An operational
error similarly aborts the search instead of dropping only the failed edge.

## Use protocol scenarios

Construct the stated `given` conditions with recording/failing adapters and
assert the `expect` observations. These are behavioral scripts, not serializable
runtime states. Foreign contexts/preparations and closed sessions fail before
backend calls. A late channel error cannot publish a partially advanced bank.
Stopping enumeration or reaching its configured cap cannot masquerade as NoFit.

Retain selected frozen pages while releasing rejected alternatives and closing
preparation; rendering remains possible while borrowed fonts stay live, whereas
resumption fails. Delivery failure reports completed deliveries and the failing
page; that page may already have external partial effects. A later-span failure
does not invalidate earlier selected spans or imply a complete document.

Only the numerical work-cap cases are marked as limit hypotheses. The default
32 unfinished beam, explicit interruption, no automatic retry and absence of
state merging are fixed policies. Routing fuzz tests are outside this suite.

## State and scoring contract tests

`loss_test.go` consumes all scoring cases with explicit synthetic factual
assessments. Arithmetic tests cover ties-to-even, adjacent quantization values,
maximum finite cost, absorbing blocked values, overflow and nonfinite rejection.
The state/outcome tests exercise context/preparation/session mismatches, exact
head/cursor pairing, A-complete/B-partial queue suffixes, draining, atomic
publication, interruption and separate tie comparators. These tests use no
real fitter; frozen-output and delivery proofs are separate checks.

## Structural fitting evidence

Core synthetic fitting tests live in
[css-box-layout/layout/fit_test.go](../../css-box-layout/layout/fit_test.go),
beside an [executable client example](../../css-box-layout/layout/fit_example_test.go).
They exercise nested auto-width, zero-edge blocks and deterministic paragraph
prefixes: every supported cut conserves ordered material, selected checkpoints
resume at changed widths, empty/zero-height material advances, and rejected or
failed alternatives leave earlier output usable. Enumeration distinguishes
exhaustion from stops, errors, cancellation and limits. Payload cloning is checked
with a backend that mutates scratch immediately after yielding.

These tests do not consume `fitting-expected.json` wholesale. Nonzero edges,
margins and atomics are covered by core geometry/composition tests and the bridge;
manual carry is covered by paragraph tests. Complete page arrangements and span
transitions are checked at the root API. Structural checks alone do not establish
page-level scoring, search or rendering.

## Paragraph prefix evidence

The tests in
[typography/glyphing/paragraph/prefix_fit_test.go](../../typography/glyphing/paragraph/prefix_fit_test.go)
exercise every supported whole-line prefix under `none`, including shorter
choices, exact/zero heights, NoProgress, repeated changed-width resumes and
consumed-versus-suffix atomic bindings. Logical positions and rendered cluster
coverage are tested separately from source HTML mapping. Weak-pointer tests
check execution-state release; the
[real-font proof](../../typography/integration/prefix_render_test.go) covers
an owning range inside a ligature and retained rendering after narrower-width
completion. The [client example](../../typography/glyphing/paragraph/prefix_example_test.go)
demonstrates selecting a shorter prefix.

These paragraph-boundary proofs complement core geometry/edge tests and
content/event integration checks. Working scoring and limit expectations apply
at the page boundary.

## Manual continuation evidence

The tests in
[khipu/linebreak/firstfit/manual_continue_test.go](../../khipu/linebreak/firstfit/manual_continue_test.go)
and [khipu/manual_checkpoint_integration_test.go](../../khipu/manual_checkpoint_integration_test.go)
prove exact manual replacement carry, including synthetic empty/silent halves
with nonempty original provenance. Accepted FF/KP prefixes, changed-width and
repeated resumes, source conservation, suffix atomic rebinding and rejection of
changed consumed atomics are checked. Lazy-service failure publishes no plan;
retry and concurrent reuse preserve old output. Lifetime checks keep frozen
replacement output usable after collecting working handles.

The [manual prefix tests](../../typography/glyphing/paragraph/prefix_manual_test.go),
[lifetime tests](../../typography/glyphing/paragraph/prefix_manual_lifetime_test.go)
and [real-font proof](../../typography/integration/prefix_manual_render_test.go)
exercise the typography selection/checkpoint API. Original source partitions
and replacement-local clusters stay separate; rendered halves supply indivisible
ReplacementCoverage, while silent output retains logical carry without rendered
coverage. Repeated width changes, source-adjacent owning ranges, provider/shaper
failures, retained atomic geometry and rendering after preparation collection
are checked. The [example](../../typography/glyphing/paragraph/prefix_manual_example_test.go)
shows manual resumption. These proofs do not establish page fitting, page loss
or span search; those are checked through the content/typography bridge and
root pagination APIs.
