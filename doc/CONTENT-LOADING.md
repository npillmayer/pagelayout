# Content loading and routing reference

`content.Load` turns a complete UTF-8 HTML document and optional author
stylesheets into immutable, source-associated box material. It publishes no
partial document on failure. The [package guide](../content/README.md) provides
client examples; the [fixtures](../content/testdata/README.md) specify observable
source records, normalized groups and diagnostics.

## Public boundary

`LoadOptions` supplies a diagnostic source name and ordered additional
`Stylesheet{SourceName, CSS}` values. Loading uses a reader and caller-provided
CSS text; there is no implicit file discovery, resource fetching or script
execution. Additional sheets follow head CSS with normal author cascade rules.

A `Document` exposes copied source and group records, independent branch
inspection and a persistent `RegisterBank[content.Box]`. A Box is an immutable
handle qualified by source snapshot, normalized tree and box identity. Box IDs
are local to a build; never compare bare IDs across trees or loaded documents.
No mutable DOM or core box pointers escape through a Box or register payload.

Loading establishes representable unsized material, not universal layout
feasibility. It performs no font lookup, shaping, intrinsic measurement, used
geometry or pagination. Preparation and fitting can reject a representable
input they cannot execute under their narrower policy.

## Loading stages

1. Parse the whole document, reject invalid UTF-8 and record original source
   identities, ancestry, attributes and decoded text ranges.
2. Inspect CSS and resource requirements, then style participating elements in
   original ancestry. Metadata, scripts and inert templates remain in source
   records and selector ancestry but do not participate in active styling.
3. Resolve source font/text semantics and checked effective box/paint values.
   Audit every retained declaration, including losing and nonmatching rules.
   Produce detached typed render input with unsized percentages and atomics.
4. Validate route roots before partitioning. Copy independent named branches
   and a residual tree; retain emptied residual ancestors and exclusive material
   ownership. The input projection stays unchanged.
5. Normalize each tree independently with `css-box-layout`, retain builder
   provenance and index actual inline groups, owners and atomic boundaries.
6. Publish the complete snapshot, register bank, source events and copied
   inspection records atomically.

Source resources are checked before display evaluation. `display:none` omits
material and effective subtree checks, but does not bypass authored CSS auditing
or resource checks. Suppressed, inert and metadata sources keep distinct
classifications and have no material ownership.

## Routing and register order

Residual material belongs to `main`, whose sole item is the residual root when
present. `Residual()` remains an inspection alias for that same tree and root;
it is not a second preparation target. A fully suppressed body retains its
residual TreeID but has no root or groups.

Named routes extract non-nested block branches under block ancestry. Repeated
channel names append branch handles in authoring order. Channels list `main`
first, then secondary channels in first-occurrence source order; global branch
inspection follows source order. Root, inline, atomic-interior and nested route
extractions are rejected with source identities. Validate all routes before
treating a valid `flow-into: main` as redundant, so it cannot hide a nested-route
error.

Registers are generic persistent queues. Deriving or consuming a bank leaves
older versions usable. Referenced payloads must stay immutable; no mutable
preparation session belongs in a payload. Queue order never changes to match
owning-link order. Placement relationships are validated by `pagelayout.Plan`,
separately from route ownership during loading.

## Supported elements, properties and defaults

The body is the render root; html supplies source context. Active supported
containers are `html`, `body`, `div`, `p`, `address`, `article`, `section`,
`aside`, `main`, `header`, `footer`, `nav`, `blockquote`, `h1`–`h6`, `hgroup`,
`figure`, `figcaption`, `pre`, `a`, `abbr`, `b`, `bdi`, `cite`, `code`, `data`,
`del`, `dfn`, `em`, `i`, `ins`, `kbd`, `mark`, `q`, `s`, `samp`, `small`,
`span`, `strong`, `sub`, `sup`, `time`, `u` and `var`.

This admits container/text participation, not browser decorations, generated
quotes or superscript sizing. Unknown/custom/foreign elements, replaced images,
lists/markers, tables, forms, `br` and `hr` lack supported projection contracts.
`pre` requires author `white-space:normal`; an active `hidden` attribute requires
explicit `display:none`.

| Properties | Accepted authored/effective values |
| --- | --- |
| `display` | None, or block/inline outer display with flow/flow-root inner display, including inline-block; no list-item flag. Body must be block flow/flow-root unless suppressed. |
| `position`, `float`, `visibility`, `box-sizing`, `writing-mode` | Static, none, visible, content-box, horizontal-tb respectively. |
| `flow-into`, `flow-from` | Checked decoded names or none for flow-into; only none for flow-from. Route eligibility is checked before extraction. |
| `break-before` | Auto or page; left, right, recto and verso normalize to page. Placement eligibility is checked by the pagination plan. |
| `width` | Auto, absolute lengths, percentages, em/rem. |
| `margin` and physical sides | Auto or signed absolute/percentage/em/rem lengths. |
| `padding` and physical sides | Nonnegative absolute/percentage/em/rem lengths. |
| `height`, `min-width`, `min-height`, physical insets | Auto only. |
| `max-width`, `max-height` | None only. |
| Border shorthands and longhands | Checked absolute/em/rem widths; none/hidden/solid styles; checked colors. None/hidden have zero effective width. |
| `color`, `background`, `background-color` | Checked sRGB/currentcolor; solid-color background shorthand. |
| `font-family`, `font-size`, `font-weight`, `font-style` | Shared textstyle grammar, including source-relative sizes/weights. Positive computed font size; no font selection. |
| `direction`, `unicode-bidi`, `hyphens` | LTR/RTL; normal/isolate; none/manual/auto respectively. |
| `white-space`, `line-height`, `word-spacing`, `letter-spacing`, `word-break`, `overflow-wrap` | Normal only. |

Properties outside the vocabulary in [profile.go](../content/profile.go) are
errors even if they would have no visible effect. Supported shorthands expand
through `w3c-dom` with resets. Initial, inherit and unset retain ordinary cascade
semantics; rollback keywords, deferred functions and custom properties are
unsupported. Reset-only background/border-image longhands must remain initial
and are not independently admitted authoring properties.

Ordinary inline boxes must have zero effective margin, padding and border;
blocks and inline-blocks retain edges. Length conversion checks finite float32
representation, including overflow and nonzero underflow. Explicit inheritance
copies computed source-parent lengths before detachment; percentages remain
percentages. Em/rem resolve against source fonts. Font-relative borders use
`css.GetBorderWithFontContext`, including original-ancestor resolution, effective
zero for none/hidden and the helper's DU rounding.

`w3c-dom/style/textstyle` owns font/text grammar and HTML lang/dir/bdi/emphasis
semantics. Defaults are generic serif, 16 CSS pixels, weight 400, normal slant,
black foreground, normal whitespace/line-height and manual hyphenation. Language
falls back to undetermined and direction to LTR. There is no browser stylesheet
adding heading sizes or paragraph margins. Loader admission of auto or mixed
hyphenation does not imply preparation support for those policies.

Floats remain unsupported. The intended image/table use case and unresolved
material-specific receivers are described in
[DESIGN.md](DESIGN.md#deferred-direction-floats-into-designated-template-frames).
Percentage widths are retained even when an execution operation cannot solve a
particular percentage dependency; loading does not invent a width to pass the
builder.

## Normalization, provenance and group identities

The core builder supplies normalized source-boundary provenance and structured
`layout.BuildError` values. RootID identifies the input branch, HasNode/NodeID
locate an offending source, and Unwrap preserves the original cause. Diagnostics
do not parse human-readable builder messages.

`layout.InlineGroups` identifies actual anonymous groups and their block owners.
Content validates source coverage and decoded-byte text ranges while retaining
these core relationships. Anonymous owner styles come from their enclosing
block, never the first text run. `GroupInfo.Atomics` lists boundary atomics,
excluding their interior groups; `ParentAtomic` links interiors to their nearest
atomic. All public inspection returns copied records and qualified Box handles.

`SourceInfo.Children` is original DOM ancestry, not a normalized paragraph
stream. Original decoded bytes, processed group bytes and structural positions
are distinct coordinate systems. Source-boundary provenance includes empty and
omitted endpoints; do not guess positions by sorting BoxIDs or choosing a nearby
text node.

## Source events and preparation handoff

`SourceEvents()` retains original target IDs, raw owning-anchor hrefs and effective
forced break facts. Excluded annotations remain available as inactive records.
Ordinary links do not infer ownership. `EventPositions()` resolves normalized
positions and visible/following/preceding/structural affinity; extracted group
input carries detached events and source coverage out of band.

`pagelayout.Plan` validates owning links and forced-break placement, builds the
main-rooted ownership graph and defines pagespans. See [SHIPOUT.md](SHIPOUT.md).
`content.Prepare` extracts groups from private normalized trees and publishes a
separate snapshot/session pair. See [PREPARATION.md](PREPARATION.md). Neither
operation changes the document, source styles or register payload type.

## Errors and validation

`LoadError` exposes ordered structured diagnostics with source, CSS location and
original causes when available. Source HTML failures retain original identities;
additional CSS failures use sheet labels without fabricated HTML nodes. Loading
stops at a failing stage without promising later diagnostics or partial material.

Fixtures cover routing and suppression, CSS overrides, decoded text, independent
normalization, group/atomic ownership, source event mapping and failed loads.
Contract tests exercise source/tree lifetime and persistent bank versions. The
route partition suite uses deterministic fixtures; a routing fuzz target is
outside its validation scope.

```sh
go test ./content -run '^Example' -v
go test ./content
```
