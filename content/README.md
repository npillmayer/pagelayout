# Loading content material

`content.Load` reads a complete HTML body, styles it in its original document
context, routes named block branches and normalizes each branch through
`css-box-layout`. The result is immutable, unsized box material with source
typography, paint and decoded text. Content loading through CONT-05 is complete;
preparation and page trials are separate phases.

## Executable examples

From the module root:

```sh
go test ./content -run '^Example' -v
go test ./content
```

The [examples](example_test.go) use only the public API. Go runs them in the
package directory, so their file paths refer to checked-in fixtures. The caller
opens and closes each HTML reader and reads any additional CSS explicitly.
There is no repository search, implicit CSS fetching or font-service setup.

| Example | Input and observable result |
| --- | --- |
| `ExampleLoad` | The existing [sketch content](../template/testdata/content/article.html) loads independently of its template collection: `heading-flow` has one branch/group, `article-flow` has one branch/two groups, and residual body retains its whitespace-only group. |
| `ExampleDocument` | [routing.html](testdata/routing.html) prints two channels, three branches, their source paths and six normalized group owners, including the separate atomic interior and two residual groups. |
| `ExampleDocument_Bank` | Consumes whole article branches in a derived bank, preserves notes and the original bank, and restores an unchanged whole item with `Prepend`. |
| `ExampleLoad_stylesheets` | [routing-override.css](testdata/routing-override.css) changes routing, source typography, paint and computed edges in a new snapshot; the original snapshot stays intact and percentage width stays unresolved. |
| `ExampleLoadError` | A nested named root fails with both original source paths and no published Document, including no earlier successful branch. |

The example-local `describeMaterial` function is a diagnostic report assembled
from `Channels`, `Branches`, `Residual`, `Groups`, `Box.Info` and `NodeSource`.
It is not a new public report/serialization API. HTML IDs are human labels;
source paths and branch order make ownership auditable. Group ordinals are
local to each printed tree, and `inside` identifies the nearest atomic ancestor.
`Box` equality, rather than these labels or a bare numeric BoxID, carries identity.

The report groups branches by channel, so branch 3 can print before branch 2
in another channel. `BranchInfo.Order` preserves their global source order.
`Document.Channels()` remains the original inventory after a derived bank drains.

## Supported loading contract

```go
doc, err := content.Load(reader, content.LoadOptions{
    SourceName: "article.html",
    Stylesheets: []content.Stylesheet{
        {SourceName: "reader.css", CSS: cssText},
    },
})
```

On success, `doc.Bank()` is a persistent `RegisterBank[content.Box]`. Each item
is the root of an entire routed branch, which can contain several paragraphs
and atomic interiors. `doc.Residual()` holds everything outside named branches;
there is no implicit `main` channel. Empty bodies are valid, and entirely
display-suppressed bodies have a residual TreeID with no root or groups.

| Concern | Implemented behavior |
| --- | --- |
| Input | Caller-owned UTF-8 reader; repaired HTML document, whole body scope; no content `LoadFS`/`LoadFile` or template repository search. |
| CSS | Original-document cascade; ordered additional author sheets; ordinary specificity, inline and importance rules. Losing/nonmatching declarations are audited too. |
| Profile | Supported ordinary block/inline flow and inline-block interiors, static positioning and content-box sizing; normal whitespace/line-height, positive source fonts, retained hyphenation metadata. The exact element/property list and defaults are in [section 6 of the plan](../doc/CONTENT-LOADING.md#6-adopted-initial-profile). |
| Routing | Non-nested block/flow-root branches under block ancestry; names are case-sensitive. Repeated names retain separate branch trees. No receiver/template validation occurs while loading. |
| Exclusion | `display:none`, inert contents and metadata remain classified in source records. Unsupported active resources and floats are diagnosed; no scripts run or resources are fetched. |
| Values | Source-font em/rem lengths are resolved; percentages and auto widths remain constraints. Box paint and paragraph source typography are retained separately from template paint. |
| Text | Raw decoded UTF-8 with byte ranges, including NBSP and soft hyphens. Omitted collapsible runs remain in the source ledger. No processed paragraph text, atomic text markers or shaping is produced. |
| Publication | All trees/indexes succeed before a Document is returned. `LoadError.Diagnostics()` supplies ordered copied source/CSS locations and related roots; `errors.Is/As` retain upstream causes. |
| Lifetime | Comparable Box handles retain the snapshot and qualify tree-local IDs. Inspection returns copies; normalized trees stay private. Independent reads and bank derivations are safe concurrently. |

`Take` consumes a whole branch from a derived bank. It does not fit that branch
into a frame or encode a partially consumed remainder. The initial bank and all
source styles remain unchanged by these queue operations.

## Handoff to preparation

Preparation starts from this published snapshot, not from another parse or
cascade. [PREP-00](../doc/PREPARATION.md#12-prep-00-decisions-and-deferred-extensions)
now pins session/service ownership and initial policy. Its
[fixtures](testdata/preparation/README.md) distinguish today's loaded structure
and extracted text from future preparation results. BOX-PREP-01's shared group
enumeration, PREP-01's detached extraction and PREP-02's neutral preparation
protocol are implemented. TYPE-PREP-01's optional
[typography backend](../preparation/typography/README.md) now supplies itemization
and paragraph preparation with explicit services. PREP-03's intrinsic integration
is next.

| Published material | Use at the preparation boundary |
| --- | --- |
| `Branches(channel)` and `Residual()` | Enumerate independent trees. Each tree's groups are prepared once; preparing does not consume the bank. |
| `Groups(tree)` | Deterministic normalized preorder, including empty groups. `Group` and `Owner` are qualified handles, not source-tag guesses. |
| `GroupInfo.Source` / `TextStyle` | Original paragraph-owner context, also for anonymous or object-only groups; never infer defaults from the first text run or a receiving frame. |
| `Atomics` / `ParentAtomic` | Immediate inline-block boundaries and separate interior groups. Outer extraction must not flatten interior text. |
| `NodeSource` / `TextSource` | Original provenance, source typography and raw decoded bytes for diagnostics/source mapping. DOM DU font sizes require explicit conversion at the typography bridge. |
| `Box.Info` | Copied unsized constraints and paint. Used sizes, atomic realization and accepted geometry remain later trial results. |

`SourceInfo.Children` describes original DOM ancestry, including routed or
excluded descendants. It is **not** a normalized paragraph stream. The public
inspection API intentionally exposes no mutable tree or general box-child
traversal. PREP-01's private extractor inside `content` reads processed groups from
the normalized trees, preserving text transformations and source maps.
The index already uses `layout.InlineGroups` for group ownership and atomic
relationships, with source-context validation remaining inside `content`.
The core's borrowed node pointers never escape through the public content API.

Extraction reuses `text.NormalizeNormal` and returns copied UTF-8 text, source
maps, DOM style spans, separate foreground paint, hyphenation policies, bidi
isolate scopes and qualified atomic occurrences. Collapsed whitespace uses its
first contributing source's style/policy while recording all source fragments.
Atomic interiors remain separate; authored U+FFFC stays text. Empty groups retain
the owner's defaults. Extraction changes no source, bank or used geometry and
admits no engine policy: even mixed/automatic hyphenation remains inspectable.
PREP-02 publishes these values as `GroupInput` to the explicit backend and through
prepared-snapshot inspection; the normalized tree remains private.

### Preparation protocol

```go
prepared, session, err := content.Prepare(doc, backend)
if err != nil {
    return err // Both results are nil; acquired handles have been released.
}
defer session.Close()
```

The caller-supplied `Preparer` implements `Preflight([]GroupInput) error` and
`PrepareGroup(GroupInput) (GroupPreparation, error)`. Content extracts and validates
all input before preflight. The backend validates its complete service/policy
requirements there, without invoking fonts, shaping or layout. It then prepares
each group once in residual-first/global-branch/preorder order, including empty
groups. Every callback gets independent input slices. This neutral protocol
chooses no font backend or hyphenation fallback; the optional typography
bridge must enforce its none/manual admission and explicit-service requirements.

Each `GroupPreparation` echoes its qualified Group/Owner and transfers an
exclusively owned `GroupHandle`, whose `Release()` method drops owned execution
state. A failed operation must clean its own in-progress work. Content also
defensively releases a nonnil handle returned with an error. Any failure returns
neither snapshot nor session and releases acquired handles in reverse order,
continuing after cleanup failures. `PreparationError` carries stage, group/owner,
source/range where known and the original cause; `errors.Is/As` reach cleanup
causes as well as the initiating error.

`PreparedSnapshot.ID()` identifies this successful preparation. `Groups()` and
`Input(group)` return copied metadata; `Covers(box)` checks whether a source box
belongs to the completely covered document. `Document()` returns a fresh handle
to the unchanged source. The snapshot contains no session or execution handles.
Two preparations of one document have the same source Boxes and distinct IDs.

`session.WithGroup(prepared, group, owner, callback)` checks the exact pair and
qualified identities before invoking a callback with its backend's `GroupHandle`.
The callback may use a typed backend API and retain frozen results, but must not
retain/release the borrowed handle or reenter the session. Execution failure
leaves the session usable. No geometry or paragraph-result type is added to the
neutral protocol; those belong to the later bridge/trial work.

Serialize all session operations and shared-service use, including `Close`.
Closing releases handles in reverse order, drops references even on failure and
never closes borrowed services. Repeated Close returns the same error without
repeating cleanup. Copies of the session wrapper share one lifecycle. Snapshot
inspection remains safe concurrently and valid after Close. These guarantees
assume backend methods honor the ownership contract; panics are not recovered.

See the [recording-backend example](prepare_example_test.go), executable with
`go test ./content -run '^ExamplePrepare$' -v`. The fake-engine tests prove the
neutral lifecycle. The typography bridge separately tests the real paragraph
pipeline with explicit fake fonts/streams; two-width layout remains later work.

The proposed two-width experiment should load once, extract/prepare once, then
use the same group and atomic identities for two independent layout results.
Choose `hyphens:none` explicitly in that experiment's source; the current fixtures
retain their `manual` policy. Do not silently alter it in the loader. Frame widths
are inputs to later measurement/layout, never to `Load` or the source cascade.

Content's immutable bank payloads remain boxes throughout. Prepared metadata and
effectful engine sessions belong to the separately specified preparation layer.
Float placement, residual binding, within-box continuation and page publication
remain later decisions. No pgtmpl content UI or visual proof is part of this
loading milestone.
