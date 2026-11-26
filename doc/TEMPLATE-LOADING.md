# Template loading reference

The `template` package loads an HTML/CSS collection into an immutable catalog of
sized pages and receiving frames. It performs no content loading, font discovery,
resource fetching or pagination. See the [package guide](../template/README.md)
for client code and [SKETCH.md](SKETCH.md) for a complete collection.

## Entry points and file discovery

- `Load(reader, options)` loads caller-provided HTML. `SourceName` labels diagnostics.
- `LoadFS(fsys, name)` and `LoadFSWithOptions` load an explicit filesystem path.
- `LoadFile(name, hostIO)` and `LoadFileWithOptions` discover a named collection.
  A nil `IO` selects private OS I/O; callers can supply `UserConfigDir` and `DirFS`.

Relative discovery tries `UserConfigDir()/templates` before the current directory.
Absolute paths are used directly. Relative paths escaping through `..` use the
current directory only. Only a missing file allows fallback: an existing invalid
collection reports its failure. File APIs close opened files, preserve original
filesystem causes and report the selected source path. There are no config files,
search caches, implicit directory creation or system-wide repositories.

`LoadOptions.Stylesheets` is an ordered slice of `{SourceName, CSS}` values.
Additional sheets follow head CSS as author stylesheets; normal specificity,
inline precedence and importance rules apply. A later low-specificity declaration
does not override a stronger earlier one. Caller-provided CSS does not trigger
resource fetching.

## Collection structure

Each direct body child `<template id="…" data-type="page">` stores one definition.
Definition IDs are unique, nonempty and contain no whitespace. Each definition
contains one page DIV, whose direct DIV children are empty frames. Nested
receivers and flowing content inside a collection are outside the profile.
Collection structure is checked after HTML parsing, so identities describe the
parsed tree, including HTML repairs.

Each definition gets an isolated active document containing copied html/body
attributes, its page and its frames. The storage template and other definitions
are absent from selector ancestry. Original source mappings are retained, while
CSSOM state is fresh per definition. The input collection is never mutated.

## CSS inspection and cascade

Head CSS is inspected once. Inline styles and additional sheets are inspected
with source locations. Syntax, unsupported rules, selectors and declarations are
reported even when they lose the cascade or match no element. Validation uses
`w3c-dom` parsed declarations and checked values, not a second CSS parser.

The cascade supplies effective values for html/body, page and frames. Context
wrappers supply styles without adding geometry or paint. Supported shorthands
retain omitted-component resets, declaration order and `!important`. Reset-only
background and border-image longhands are checked internally; they do not expand
the authored property vocabulary.

## Supported HTML and CSS profile

| Area | Accepted values |
| --- | --- |
| Page | Static block/flow DIV; positive absolute width and height; content-box; zero margin, padding and effective border; auto insets. |
| Frames | Direct fixed block/flow DIVs; content-box; zero margins; visible receiving content. |
| Sizes and insets | Absolute lengths or unitless zero; explicit positive content size or auto size stretched by opposing insets; at least one definite inset per axis. |
| Edges | Nonnegative absolute padding and effective border widths; none, hidden or solid borders. |
| Paint | Solid colors and transparency, with checked initial/current-color behavior. |
| Bindings | One non-none, case-sensitive decoded `flow-from` name per frame; no receiving flow on page/context boxes. |
| Role keywords | Static/fixed positioning, block/flow display, content-box sizing, visible, float none, horizontal-tb, LTR. |

The exact property vocabulary is in
[`profileProperties`](../template/profile.go). It includes physical sizes/insets,
margin/padding, border longhands and supported shorthands, background/background-color,
color, box roles and named-flow properties. Explicit min/max dimensions, fonts,
radii, custom properties and image/effect longhands are rejected, even when set
to a harmless reset. Transforms, clipping, images and generated content are not
supported.

Absolute units are px, pt, in, cm, mm, pc and Q. Only sizes/insets may be auto;
margins must be zero. CSS-wide initial, inherit and unset are supported. Rollback,
math and variable functions are rejected, including in losing/nonmatching rules.
Static selector matching is owned by CSSOM; interactive pseudo-classes have no
active UI state.

No active template box may supply a `flow-into` channel. Nonmatching content-flow
declarations can be valid source metadata, but actual routing belongs in the
separate content document. A collection does not require a matching content
channel at load time. Use `flow-from: main` for the default content channel.

Percentages, font-relative template lengths, border-box sizing, bleed and
decorative overlap are outside this profile. The fixed-geometry service has a
broader typed domain than the HTML authoring profile.

## Geometry compilation

Validated inputs are detached values with no DOM or styling pointers. Default
core constructors preserve symbolic auto values. `layout.ResolveFixedGeometry`
resolves frames against the zero-origin page viewport without box normalization,
inline callbacks or intrinsic measurement. Content-dependent sizing is an error;
empty content is never a fallback for an unresolved dependency.

Border and receiving content sizes must be finite and strictly positive. Border
boxes must lie inside the page and may not overlap any other frame, including
frames of different channels. Touching edges and corners are allowed.

The comparison tolerance is the larger of one source DU in CSS pixels
(`1 / dimen.PX`, about 0.000020345 px) and eight float32 spacings immediately
below the larger page dimension. On each axis it is capped at half the smaller
compared span. Bounds are accepted within that tolerance; overlap is rejected
when penetration exceeds it on both axes. Endpoints are summed in float64.
Positivity is strict. Published geometry is never snapped or clipped.

Both border and content rectangles use page coordinates. A content origin already
includes frame position and edges: do not add the border origin again. Flow
chains filter frames in source order, without sorting by geometry or selector.

## Publication and diagnostics

A catalog is returned only when every definition compiles. Accessors return
independent handles or copied values, and lookups never recascade or rerun fixed
geometry. Read-only access is safe concurrently. Failed reloads and accessor
mutations cannot affect a retained catalog.

`LoadError` retains ordered diagnostics and original causes. HTML diagnostics
identify the source name, template, node and path; CSS diagnostics include
rule/declaration/line/column when known. Additional stylesheet diagnostics use
the sheet label and CSS location without inventing an HTML identity. Geometry
failures identify the source HTML box. Ordering follows HTML source order, then
additional sheet order; stable ties preserve check order.

## Examples and validation

[Executable examples](../template/example_test.go) cover reader, filesystem and
discovery entry points, geometry, chains, source inspection and errors. Tests
cover isolated cascades, losing declarations, combined CSS overrides, stretch
geometry, bounds/overlap tolerance, close failures and catalog lifetime.

```sh
go test ./template -run '^Example' -v
go test ./template
```
