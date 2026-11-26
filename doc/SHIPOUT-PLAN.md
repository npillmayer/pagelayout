# Pagination validation and open work

The pagination APIs are available from source planning through selected-page
delivery. [SHIPOUT.md](SHIPOUT.md) specifies their current contracts; this guide
maps those contracts to executable evidence and identifies extensions that still
need design or calibration. Scoring coefficients and numerical limits remain
working hypotheses.

## Validation boundaries

| Boundary | Required observations | Evidence |
| --- | --- | --- |
| Checked breaks | Source declarations are validated even when losing/nonmatching; page-side variants normalize to page. | `w3c-dom` checked-break tests; [source tests](../content/ship_source_test.go). |
| Default main and routing | One residual/main item; redundant main does not hide route errors; FIFO and first-occurrence channel order. | [source tests](../content/ship_source_test.go), [routing tests](../content/route_test.go). |
| Source provenance | Original/processed/structural coordinates remain distinct; empty, suppressed and normalized boundaries retain provenance. | CSS core provenance tests; [position tests](../content/positions_test.go), [extraction tests](../content/extract_test.go). |
| Plan | Exactly one active owner per active secondary branch, unique branch-root target, main-rooted acyclic graph, forced-break propagation and transitive spans. | [plan tests](../plan_test.go), [source expectations](../testdata/pagination/source-expected.json). |
| State and loss | Context-qualified persistent state, exact head pairing, checked loss arithmetic, separate local and sequence ties. | [state tests](../state_test.go), [loss tests](../loss_test.go), [outcome tests](../outcomes_test.go). |
| Structural fitting | Every admitted cut conserves ordered material; exact changed-width resumption; margin/edge composition; stable consumed atomics. | CSS core fitting/geometry/composition suites; [content fitting](../content/fit_test.go) and [composition tests](../content/compose_test.go). |
| Paragraph fitting | Whole-line alternatives, precise source/cluster coverage, shorter choices, manual carry and atomic rebinding. | Typography prefix/manual suites; khipu manual continuation tests; [bridge fitting tests](../preparation/typography/fit_test.go). |
| Page attempt | Full local enumeration, deterministic quality/progress ties, unchanged input, no partial candidate on failure. | [page tests](../page_test.go), [fitting expectations](../testdata/pagination/fitting-expected.json). |
| Multiple channels | Same-page activation, targets before owners, transitive drain, authored FIFO, missing receivers and exact pending charges. | [channel tests](../page_channels_test.go), [real-font page tests](../page_realfont_test.go). |
| Search | Complete depth rounds, catalog-order ties, separate completion, strict safe bounds, explicit heuristic truncation, no merging/retry. | [graph tests](../search_graph_test.go), [search tests](../search_test.go), [quality tests](../search_quality_test.go). |
| Document and delivery | Exact span transition, retry state, retained prior selections, selected-only delivery, explicit external failure progress. | [document tests](../document_test.go), [shipout tests](../shipout_test.go). |
| Lifetime and services | One base preparation per group; permitted lazy manual shaping; frozen output survives cleanup without retaining execution owners. | [document proofs](../shipout_realfont_test.go), [bridge lifetime tests](../preparation/typography/fit_lifetime_test.go), sibling private-lifetime suites. |
| Dependencies and use | No concrete font imports in production bridge; neutral packages stay independent of typography; executable full workflow. | [dependency tests](../preparation/typography/dependencies_test.go), [delivery example](../shipout_example_test.go). |

The [fixture reference](PAGINATION-FIXTURES.md) describes six independent
contract tables, synthetic input units, error scenarios, source identities and
the relationship between paragraph, core and page-level evidence. Fixture
expectations are not regenerated from the implementation under test.

## Real-font and output-lifetime evidence

The two [document proofs](../testdata/pagination/README.md#real-font-document-proofs)
use a pinned local Gentium Plus fixture through public loading, preparation,
planning, attempts, pagination and delivery. Each selects five pages in two
spans. Explicit eligibility produces a narrow opening, changed-width continuation
and secondary drain pages; it is a test constraint rather than an optimal
eligibility policy.

Assertions cover continuous source-byte consumption, source paint, owning-event
activation, transitive drain, unchanged accepted prefixes and an oversized atomic
that terminates infeasibly. The first 50px atomic interior stays fixed while a
suffix atomic resolves to 80px. Manual `co-` / `operate` retains its shaped
right-half carry. Base preparation counts remain one per group, while the manual
case permits its two legitimate lazy replacement shapes.

Only frozen pages leave the output-lifetime helper. Weak references check release
of the paginator, session, factory/hyphenator, backend adapter, group wrappers,
streams and rejected shaped paragraphs. Additional tests cover search paths and
local collectors; sibling private tests cover internal preparations, trials and
checkpoints. Borrowed fonts intentionally remain live. Retaining a resumable
state is a separate ownership case, not evidence of an output leak.

The [SVG proof](../testdata/pagination/shipout-proof.svg) and
[metric baseline](../testdata/pagination/proof-expected.json) complement
independent conservation and geometry assertions. The test-only renderer does
no layout or shaping; it is not a public export API.

## Search-quality evidence and interpretation

[Recorded work counts](../testdata/pagination/search-quality.json) compare beams
8, 32 and 128 against independent exhaustive traversal. A finite late-cheap-path
graph yields loss 10 at widths 8/32 versus loss 2 at width 128/exhaustive. The
actual-page case yields the same loss 3 for all widths, with 90/234/258 attempts
and 132/276/300 local alternatives versus exhaustive 258/300.

Repeated logical breakpoint vectors are measured without merging. They do not
prove equivalent behavior or exchangeable continuations. Peak counts include
complete round construction before pruning and are not heap-size estimates.
Safe bound cuts and heuristic width cuts are counted separately. These cases
show both useful pruning and quality loss; they do not establish representative
performance or settle scoring, eligibility or work-cap calibration.

## Open work

1. **Calibrate scoring and limits.** Collect representative documents with
   explicit expected tradeoffs. Compare component losses, page sequences and
   work counts across configured weights/limits. Keep final-page underfill and
   numerical caps visibly provisional until evidence supports a policy change.
2. **Evaluate search extensions.** Measure repeated states and quality losses
   before designing merging or widening. A merging key must cover exact
   continuation behavior, activation, span facts and policy context; logical
   positions alone are insufficient. Any retry/widening policy must be explicit.
3. **Design material-specific float receivers.** Define matching, author anchors,
   source order, text exclusion/overlap, fitting and deferral for image/table
   material, including receivers extending into page margins. No float syntax or
   routing behavior is implied by the current APIs.
4. **Extend the admitted content/fitting profile.** Tables, replaced content,
   running material, mandatory per-page channels and richer inline/height/text
   policies need source, preparation, fragmentation and paint contracts together.
5. **Design concurrent execution only with service guarantees.** Separate immutable
   outputs from mutable discovery/session state; borrowed services must support
   the chosen sharing model before execution becomes concurrent.
6. **Supply rendering/export integrations as clients.** Consume frozen geometry,
   paragraphs, source paint and font resources without reflow or double-applying
   origins. Define external delivery retry semantics at the integration boundary.

These are open design/calibration tasks, not missing parts of the supported
pagination contract. Changes involving sibling modules should start at the
responsible boundary: CSS parsing/semantics in w3c-dom, structural geometry in
the CSS core, paragraph/discretionary behavior in typography/khipu, orchestration
and selection here. Do not duplicate sibling algorithms to extend the profile.

## Running the checks

From the module directory:

```sh
go test ./...
go test . -run '^TestSearchQualityWidthsAndExhaustiveReference$' -count=1
go test . -run 'TestShipout.*Real|TestSelectedPagesReleaseSearchWorkspaces' -count=1
```

Race checks validate tested ownership patterns without promising concurrent
session use. When changing a shared contract, run the affected sibling suites as
well. Golden refresh switches are explicit and documented in the fixture reference;
ordinary tests never rewrite expected output.
