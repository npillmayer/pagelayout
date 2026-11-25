# Preparation examples and expected results

Load these HTML documents with `content.Load`, then pass the document and an
explicit backend to `content.Prepare`. The
[typography guide](../../../preparation/typography/README.md) shows font setup,
measurement and layout. No template collection is needed.

[`expected.json`](expected.json) describes raw decoded text, processed UTF-8,
source spans, group owners, atomic boundaries and expected preparation calls.
Use the tables below to interpret `PreparedSnapshot.Groups()` and `Input(group)`.

## Inputs

| Input | Group-owner order | Prepared groups | Behavior |
| --- | --- | --- | --- |
| [text-only.html](text-only.html) | text | 1 | `Café & tea U+FFFC NBSP ok.` with exact spacing in JSON. Whitespace collapses across a source-element boundary. Authored U+FFFC remains text, with no atomic occurrence. |
| [nested.html](nested.html) | outer, wide, inner | 3 | Outer: `Before U+FFFC after.`; wide: `Wide U+FFFC`; inner: `Inner.`. Atomic occurrences point to wide at byte 7 and inner at byte 5. Both inline-blocks have auto width. |
| [empty-atomic.html](empty-atomic.html) | empty, object, blank | 3 | An empty group, atomic-only group and empty atomic interior. Outer/object owner context is 20px; blank's own context is 12px. Only the structural occurrence inserts U+FFFC. |
| [branches.html](branches.html) | residual, a, note, b | 4 | Residual first, then global branch source order. Article contains a then b; notes contains note. Preparation leaves the bank unchanged. |
| [manual.html](manual.html) | manual | 1 | Preserves `co­operate`. Requires an explicit-marker hyphenator. With the marker behavior below, intrinsic measurement succeeds. |
| [manual-many.html](manual-many.html) | many | 1 | Preserves both soft hyphens. Preparation succeeds; intrinsic measurement with the marker behavior below reports unsupported policy for multiple eligible splits in one textbox. |
| [mixed_err.html](mixed_err.html) | mixed | 0 | Loading succeeds. A group mixes none and manual; typography preparation fails at #manual before font or shaping calls. |
| [auto_err.html](auto_err.html) | auto | 0 | Loading succeeds. Automatic hyphenation fails typography preparation at #auto before font or shaping calls. |

The `_err` suffix here denotes a **preparation-policy** failure, not a loading
error. Accepted cases require explicit font and shaping services. Manual policy
also requires a hyphenator even when a group has no U+00AD.

For the manual examples, supply a hyphenator whose U+00AD request returns one
permitted alternative: pre-break `-`, empty post-break and penalty 0. It does
not discover automatic word splits. The package does not supply this service
by default. Manual discovery and shaping of alternatives may occur lazily;
prepared-group counts are not counts of all later shaping calls.

## Read identities and source maps

Manifest labels use HTML IDs for readability, not numeric BoxID/TreeID values.
`tree: residual` selects the residual tree; other tree labels identify branch
roots. Group order is residual preorder, then branch trees in global source
order, with normalized preorder within each tree. `parent_atomic` is the nearest
containing inline-block, and `atomics` lists immediate boundaries only. Text
inside an inline-block does not enter its outer group's text.

Each `raw[owner]` array lists the element's immediate decoded text children in
source order. A span's `owner` and one-based `node` select one of those strings.
`source` and `processed` are half-open **UTF-8 byte** ranges, not HTML byte or
rune offsets:

| Mapping | Meaning |
| --- | --- |
| `identity` | Source bytes are copied into processed text. |
| `collapsed` | A whitespace run contributes one space. |
| `omitted` | Paragraph-edge whitespace maps to a zero-length processed interval. |

When a collapsed run crosses text nodes, both source fragments map to the same
output space. Its style, paint and policy come from the first contributing
source unit. Empty groups have no invented text span. The JSON `policy` field
is the **owner's** policy; source-span policies still apply to contributing text.

Every processed byte has text-source or atomic correspondence. Inserted atomic
markers occupy three UTF-8 bytes and have only atomic provenance. Authored
U+FFFC has an ordinary text span and no atomic occurrence. NBSP and soft-hyphen
bytes survive extraction.

## Reuse a preparation

For `nested.html`, load once and prepare the three groups once with the chosen
fonts. Measure and lay out the residual root at 120 and 240 CSS pixels using
the same snapshot/session pair and explicit `paragraph.FirstFit`. Both layout
results can coexist; neither changes the source boxes or register bank. See
[the two-width outline preview](../../../preparation/typography/testdata/README.md).

For `branches.html`, the same prepared document covers article, notes and default main (the residual tree)
content. Taking a branch from a derived bank does not require another preparation.
Use the returned whole-branch handle with the matching preparation pair.

A failed measurement or layout publishes no result and leaves the pair usable.
A failed `content.Prepare` publishes neither snapshot nor session and releases
completed handles in reverse order. A successful session can be closed while
retaining frozen output, provided the font resources remain valid. Two separate
preparations of one document have different preparation identities but share
the same source `Box` handles; do not mix their snapshot/session pairs.

The branches fixture routes its independent branches to `article` and `notes`.
All fixtures also expose default `main`; expected `branches` lists secondary
roots only, with the main tree covered by the existing `residual` group entries.
