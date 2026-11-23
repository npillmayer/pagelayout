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
cascade. The [preparation plan](../doc/PREPARATION.md) remains proposed work;
PREP-00 still needs to settle its session/service and policy choices.

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
traversal. PREP-01's bridge inside `content` will extract processed groups from
the private normalized trees, preserving text transformations and source maps.
BOX-PREP-01 first supplies shared CSS-core group enumeration to replace the
current private indexing traversal.

The proposed two-width experiment should load once, extract/prepare once, then
use the same group and atomic identities for two independent layout results.
Choose `hyphens:none` explicitly in that experiment's source; the current fixtures
retain their `manual` policy. Do not silently alter it in the loader. Frame widths
are inputs to later measurement/layout, never to `Load` or the source cascade.

Content's immutable bank payloads remain boxes throughout. Prepared metadata and
effectful engine sessions belong to the separately proposed preparation layer.
Float placement, residual binding, within-box continuation and page publication
remain later decisions. No pgtmpl content UI or visual proof is part of this
loading milestone.
