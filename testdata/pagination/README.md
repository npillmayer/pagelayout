# Pagination contract fixtures

Use these inputs and independently authored expectations to exercise the adopted
[SHIP-00 contracts](../../doc/SHIPOUT.md#adopted-page-construction-contract-ship-00).
They are independently authored expectations, not generated paginator output.
The source table is exercised by SHIP-01/02 tests against content loading and
`pagelayout.Plan`; the scoring table is exercised by SHIP-03. State/outcome protocol cases are
checked at their contract boundary; fitting, search execution and rendering/
delivery cases remain later runtime gates.

Keep template and content documents separate. The template collection can already
be checked with:

```sh
go run ./pgtmpl load ./testdata/pagination/templates/receivers.html
```

Run that command from the module root. It checks the receiving geometry only.
The six JSON tables below supply expected results at separate boundaries; a
passing catalog load does not prove content routing, fitting or pagination.

## Choose a fixture table

| Table | Input boundary | What to compare |
| --- | --- | --- |
| [template-expected.json](template-expected.json) | Load [receivers.html](templates/receivers.html). | Definition order, frame-chain order and compiled content rectangles. |
| [source-expected.json](source-expected.json) | Load the named content file, apply listed additional CSS, then compile its plan. | Main aliases, source order, affinity/propagation, ownership/span membership and required diagnostics. |
| [fitting-expected.json](fitting-expected.json) | Supply the declared synthetic structural/paragraph inputs, or the declared complete arrangement. | Exact fragments/remainders, geometry, atomic reuse, manual carry, local comparisons and span transitions. |
| [scoring-expected.json](scoring-expected.json) | Supply the stated page facts directly to the evaluator. | Participating frames, components, exact integer loss and arithmetic failures. Scoring cases are working hypotheses. |
| [search-expected.json](search-expected.json) | Supply the finite graph as a fake Page attempt. | Selected sequence, loss, retained frontier, pruning, completion/failure and interrupted-incumbent behavior. |
| [protocol-expected.json](protocol-expected.json) | Construct the stated identity, session, failure or delivery scenario. | Atomic publication, error category/cause, state stability and output lifetime. |

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

An explicit `gate` names the slice responsible for the expectation. Otherwise
source-to-main/plan cases are consumed by SHIP-01/02. Source tests distinguish declaration-profile rejection from unsupported break
locations diagnosed by the plan.
`_err.html` names a deliberately invalid routing/ownership/break-location input;
the extra-CSS cases supply further deliberate profile failures without duplicating
the HTML files.

| Content family | Inputs and intended observation |
| --- | --- |
| Main | [main-default](content/main-default.html) and [main-explicit](content/main-explicit.html) have the same material/order, one residual/main tree and no duplicate preparation. |
| Affinity | [anchors](content/anchors.html) supplies start, whitespace-only middle, trailing, empty-block, multiline and bidi owners, plus an ordinary reference. The trailing marker stays in its paragraph. |
| Boundary affinity | [boundary-anchors](content/boundary-anchors.html) keeps a trailing owner in the closing span, a leading owner in the new span and a terminal owner before the final drain completes. |
| Normalization | [normalization](content/normalization.html) puts a marker in an inline source split around a block. It attaches to following material in the original containing block; generated groups must retain provenance. |
| Breaks | [breaks](content/breaks.html) supplies all five forced page values, an initial break, first-child propagation and terminal drain. [Additional CSS](content/break-override.css) removes the left boundary through the cascade. |
| Transitive drain | [transitive](content/transitive.html) has main → note → figure in span 1 and a later note in span 2. The later main/note material must remain untouched while the figure drains. |
| FIFO | [fifo](content/fifo.html) owns B before A but queues A before B. B waits; A cannot be placed before its own anchor. |
| Cross-span obstruction | [cross-span-fifo](content/cross-span-fifo.html) has a future-span A ahead of current-span B. The plan is valid, but exhaustive search cannot drain span 1 under the policies. Do not sort, skip or consume A early. |
| Edges | [fragments](content/fragments.html) supplies asymmetric split-box edges and a contained child whose own edges survive. |
| Paragraphs/atomics | [atomics-manual](content/atomics-manual.html) preserves two percentage-width atomics and explicit soft-hyphen source material. Supply explicit services for manual preparation. |
| Exclusion/empty | [suppression](content/suppression.html) has inactive owners/targets and an inert template; [empty](content/empty.html) requires no physical page. |

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
execution and lifetime instrumentation are added by their later owning slices.

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
or blocked result. Previously settled carry timing and loss algebra do not
become provisional merely because coefficient calibration is provisional.

## Finite search graphs

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
state merging remain settled. This fixture set does not reopen the omitted
CONT-03 fuzz test.

## State and scoring contract tests

SHIP-03 tests consume all scoring cases with explicit synthetic factual
assessments. Arithmetic tests cover ties-to-even, adjacent quantization values,
maximum finite cost, absorbing blocked values, overflow and nonfinite rejection.
The state/outcome tests exercise context/preparation/session mismatches, exact
head/cursor pairing, A-complete/B-partial queue suffixes, draining, atomic
publication, interruption and separate tie comparators. These tests use no
real fitter and do not claim the later frozen-output or delivery proofs.

## Structural fitting evidence

BOX-SHIP-02's synthetic tests live in
[css-box-layout/layout/fit_test.go](../../../css-box-layout/layout/fit_test.go),
beside an [executable client example](../../../css-box-layout/layout/fit_example_test.go).
They exercise nested auto-width, zero-edge blocks and deterministic paragraph
prefixes: every supported cut conserves ordered material, selected checkpoints
resume at changed widths, empty/zero-height material advances, and rejected or
failed alternatives leave earlier output usable. Enumeration distinguishes
exhaustion from stops, errors, cancellation and limits. Payload cloning is checked
with a backend that mutates scratch immediately after yielding.

These tests do not consume `fitting-expected.json` wholesale. Its nonzero-edge,
margin, atomic and manual-carry cases remain the BOX-SHIP-03 and paragraph bridge
gates; complete page arrangements and span transitions remain page-runtime gates.
The structural tests do not establish page-level scoring, search or rendering.

## Paragraph prefix evidence

TYPE-SHIP-01's tests in
[typography/glyphing/paragraph/prefix_fit_test.go](../../../typography/glyphing/paragraph/prefix_fit_test.go)
exercise every supported whole-line prefix under `none`, including shorter
choices, exact/zero heights, NoProgress, repeated changed-width resumes and
consumed-versus-suffix atomic bindings. Logical positions and rendered cluster
coverage are tested separately from source HTML mapping. Weak-pointer tests
check execution-state release; the
[real-font proof](../../../typography/integration/prefix_render_test.go) covers
an owning range inside a ligature and retained rendering after narrower-width
completion. The [client example](../../../typography/glyphing/paragraph/prefix_example_test.go)
demonstrates selecting a shorter prefix.

These are paragraph-boundary proofs, not whole-page fixture results. CSS fragment
edges/geometry and content/event integration remain later gates; working scoring
and limit expectations are unchanged.

## Manual continuation evidence

KHIPU-SHIP-01 tests in
[khipu/linebreak/firstfit/manual_continue_test.go](../../../khipu/linebreak/firstfit/manual_continue_test.go)
and [khipu/manual_checkpoint_integration_test.go](../../../khipu/manual_checkpoint_integration_test.go)
prove exact manual replacement carry, including synthetic empty/silent halves
with nonempty original provenance. Accepted FF/KP prefixes, changed-width and
repeated resumes, source conservation, suffix atomic rebinding and rejection of
changed consumed atomics are checked. Lazy-service failure publishes no plan;
retry and concurrent reuse preserve old output. Lifetime checks keep frozen
replacement output usable after collecting working handles.

TYPE-SHIP-02
[manual prefix tests](../../../typography/glyphing/paragraph/prefix_manual_test.go),
[lifetime tests](../../../typography/glyphing/paragraph/prefix_manual_lifetime_test.go)
and [real-font proof](../../../typography/integration/prefix_manual_render_test.go)
now exercise the typography selection/checkpoint API. Original source partitions
and replacement-local clusters stay separate; rendered halves supply indivisible
ReplacementCoverage, while silent output retains logical carry without rendered
coverage. Repeated width changes, source-adjacent owning ranges, provider/shaper
failures, retained atomic geometry and rendering after preparation collection
are checked. The [example](../../../typography/glyphing/paragraph/prefix_manual_example_test.go)
shows manual resumption. These proofs do not establish page fitting, page loss
or span search; the content/event/interior bridge remains SHIP-04.
