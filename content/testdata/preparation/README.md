# PREP-00 acceptance inputs

Status: contract and fixtures pinned on 2026-11-23. All eight inputs load today.
Extraction, engine preparation and measurement results below are **expected
future results**, not claims that those operations have been implemented.
See [PREPARATION.md](../../../doc/PREPARATION.md#12-prep-00-decisions-and-deferred-extensions).

`expected.json` pins raw decoded text, processed UTF-8, text-source spans, owner
and atomic identities, group order and expected backend `PrepareGroup` counts.
`content/preparation_fixtures_test.go` checks current loading/group structure,
raw text and the internal byte-coordinate consistency of these expectations.
It does not implement a second extractor, invoke fonts or simulate preparation.
PREP-01 must compare actual extraction with this manifest; PREP-02/TYPE-PREP-01
must compare actual service counts and policy failures.

Run from the module root:

```sh
go test ./content -run '^TestPreparationAcceptanceInputs$' -v
```

## Inputs and future results

| Input | Group-owner order | Successful preparation calls | Future behavior |
| --- | --- | --- | --- |
| `text-only.html` | text | 1 | `Café & tea U+FFFC NBSP ok.` with the exact spacing in JSON. Collapse a whitespace run crossing a source-element boundary; preserve the authored U+FFFC as text, with **no** atomic anchor. |
| `nested.html` | outer, wide, inner | 3 | Outer: `Before U+FFFC after.`; wide: `Wide U+FFFC`; inner: `Inner.`. Anchors point to wide at byte 7 and inner at byte 5 in their respective groups. Both atomics remain auto-width. |
| `empty-atomic.html` | empty, object, blank | 3 | Empty group, atomic-only group and empty atomic interior all prepare once. Outer/object owner context is 20px; blank's own context is 12px. Only the structural occurrence inserts U+FFFC. |
| `branches.html` | residual, a, note, b | 4 | Residual first, then global branch source order. Main contains a then b, notes contains note; no bank operation during preparation. |
| `manual.html` | manual | 1 | Preserve `co\u00adoperate`; require an explicit-marker hyphenator. With the acceptance hyphenator below, bounded explicit intrinsic measurement succeeds. |
| `manual-many.html` | many | 1 | Preserve both authored soft hyphens. Preparation succeeds; intrinsic measurement with the acceptance hyphenator returns upstream unsupported-policy for multiple eligible splits in one textbox. Session and old outputs remain usable. |
| `mixed_err.html` | mixed | 0 | Loading/extraction succeed. Paragraph text mixes none and manual; whole-document policy preflight fails at #manual before any engine/font call. |
| `auto_err.html` | auto | 0 | Loading/extraction succeed. Automatic policy fails preflight at #auto before any engine/font call. |

The `_err` suffix denotes a **preparation policy** failure, not a loading error.
JSON `policy` is the paragraph **owner's** policy; source-span policies must also
be retained. A mixed-policy paragraph is not made uniform by using that field.
For accepted cases the call count assumes valid explicit services. Missing
resolver/shaper services, or a missing manual hyphenator, fail preflight with zero
calls even when there is no U+00AD. Defaults must never select a synthetic backend.

The future acceptance hyphenator is deterministic: a U+00AD request returns one
permitting alternative with pre-break `-`, empty post-break and penalty 0. It
does not find automatic word splits. This defines test policy only; the bridge
must not install it as a production default. Hyphenator queries/discretionary
shaping may be lazy; the count above is group preparation, not shaper-run count.

## Identity and source-map conventions

Manifest labels are HTML IDs solely for readable expectations. Tests resolve
them to original NodeIDs and then compare qualified `content.Box` handles; do
not freeze numeric BoxID/TreeID allocation. `tree: residual` selects the residual
tree, other tree labels identify a branch root. Group order is residual preorder,
then branch trees in global source order with each tree's normalized preorder.
`parent_atomic` is the nearest containing atomic, and `atomics` lists immediate
boundaries only. No text from an atomic interior enters its outer group.

Each `raw[owner]` array lists that element's immediate decoded text children in
source order. A span's `owner` and one-based `node` select one of those strings.
`source` and `processed` are half-open **UTF-8 byte** ranges, never HTML byte or
rune offsets. `identity` copies bytes, `collapsed` maps a whitespace run to one
space, and `omitted` maps paragraph-edge whitespace to a zero-length interval.
When a collapsed run crosses text nodes, both source fragments map to that same
output space. Its style/paint/policy comes from the first contributing source
unit. Empty groups have no invented textual span.

Every processed byte has text-source or atomic correspondence. Atomic markers
occupy three bytes and have only their atomic-source record, not fabricated
text-node bytes. Authored U+FFFC instead has an identity text span and no anchor.
The text-only fixture records both dropped edges and the shared collapsed space.
Raw NBSP and soft-hyphen bytes survive extraction; discovery and display policy
remain downstream. Keep these files free of trailing document whitespace so
changes to fixture source spans are intentional.

## Reuse and failure acceptance

For the future two-width trial, use `nested.html` unchanged: source `hyphens:none`
is explicit. Load once and prepare all three groups once. Measure/layout at 120px
and 240px using the same snapshot/session pair and explicit services. Repeated
trials add **zero** group preparation calls; target widths never enter Prepare.
Do not pin line counts without the later fake/real-font acceptance configuration.
Each candidate owns its geometry and paragraph results; rejecting one leaves the
other and the original bank unchanged.

Use `branches.html` for later-group failure injection: fail group b's fourth
PrepareGroup call after three successes. Release completed handles in reverse
order, publish neither snapshot nor session, retain the original bank and an
older preparation, and preserve the injected cause plus #b/group identity.
Retry uses a fresh working session, not the failed partial table. The current
tests do not claim to execute this future lifecycle contract.

Also use it for two coexisting preparations: identical source Box handles,
different preparation identities; foreign groups/owners and a mismatched session
are rejected before service calls. Close is idempotent and serialized with session
operations; frozen outputs keep stable font resources, not the closed session.
These counts and checks belong to PREP-02/TYPE-PREP-01/PREP-05.
