# pagelayout

A Go module for template-based page composition using HTML/CSS page definitions
and fixed receiving frames. The overall machinery is still being designed.

## Implemented: TPL-01 through TPL-03

Template loading lives in the [`template`](template/) subpackage, imported as
`github.com/npillmayer/pagelayout/template`. Its code, tests and fixtures are
encapsulated there; the root package is reserved for broader page composition.
Styling also lives there, and later compilation stages will use the same boundary.

The first slice provides private HTML collection parsing, definition discovery,
structural validation and a source model for later styling. It retains:

- The parsed html/head/body context and direct head style elements.
- Page definitions in document order, with exact, case-sensitive ID lookup.
- Each definition's page DIV and ordered, template-local frame identities.
- Source-node identities and paths in the parsed HTML tree.
- Structured, deterministic diagnostics, with original reader errors preserved.

TPL-02 inspects head CSS once and styles each definition in its own active
`html > body > page > frames` document. It copies collection attributes, preserves
source mappings, and creates a fresh `w3c-dom` CSSOM per definition. Stylesheets
retain source order; inline styles and inheritance use the existing cascade.
Storage templates and other definitions do not participate in selector matching.

CSS inspection retains losing declarations and reports syntax failures, invalid
selectors and unsupported at-rules, even in nonmatching rules. Inline failures
include the original template/node and CSS location. Conditional head sheets,
non-CSS sheet types and scoped sheets are rejected. Empty or `all` media and
empty or `text/css` types are accepted.

TPL-03 checks every source declaration, including losing and unmatched rules,
against the initial template CSS profile. It validates effective pages, frames
and their active ancestry, then produces detached page sizes, fixed constraints,
solid paint and flow bindings. It reuses checked `w3c-dom` values and
`css-box-layout` default constructors; absolute DU values become CSS pixels and
auto dimensions/insets remain distinct from zero.

Invalid input returns no partial collection. Source models remain read-only;
styled working trees own their nodes, attributes and styling state. Validated
inputs retain source identities but no DOM/CSSOM references. Public loading,
geometry resolution and immutable compiled catalogs are later milestones.

## Structural input profile

Page definitions are direct body children named
`<template id="..." data-type="page">`. IDs are nonempty and contain no HTML
ASCII whitespace. Each definition contains one HTML DIV for the page; its direct
DIV children are empty frames. Comments and formatting whitespace are accepted,
but nonbreaking spaces and other text are content. Classes are optional, and
zero-frame pages are allowed.

Other templates are inert storage and may be ignored; marked page templates
inside them still receive a location diagnostic. Other body content is rejected.
Head metadata (`title`/`meta`) and direct head styles are accepted. Element IDs
on html/body/page/frames, when present, must be valid and unique in each future
active document. Frame IDs/classes may recur across separate definitions.

HTML parser recovery applies. Source paths refer to the repaired tree, not byte
positions or source lines. TPL-01 retains CSS without parsing it. TPL-02 adds
inspection and the cascade. TPL-03 adds fixed-position/flow validation and the
[initial CSS profile](doc/TEMPLATE-LOADING.md#c-validate-source-declarations-and-effective-styles):
absolute lengths, horizontal LTR block/flow boxes, content-box sizing, zero
margins and solid paint. Page dimensions must be positive; auto frame sizes
require opposing insets. Percentages, font-relative units, images, transforms,
clipping, min/max sizing and other effects remain outside this profile.
Final receiving sizes, bounds and overlap checks belong to geometry compilation.

## CLI scaffold

[`pgtmpl`](pgtmpl/) provides the command-line shell for checking and previewing
collections, using the same Commando framework as `tytui/tycli`. Run
`go run ./pgtmpl --help` for usage. Its `load`, `serve` and `find` subcommands
are placeholders; template checking, browser previews and collection lookup
are not implemented yet.

## Dependencies and checks

Go **1.25.6** and `golang.org/x/net` **v0.49.0** match the checked-out `w3c-dom`
and `css-box-layout` baselines used for TPL-01. TPL-02 uses `w3c-dom`'s new
DOM-01 APIs, `cssom.InspectStylesheet` and `InspectInlineStyle`, with
`NewCSSOM`, `AddStylesForScope` and `Style`.

Upstream DOM-02 now provides checked box lengths, colors, flow names, keywords
and effective borders, plus value validation before cascade reduction. TPL-03
uses these APIs to enforce the template geometry/paint profile; TPL-02
already receives their invalid-value diagnostics through DOM-01 inspection.
DOM-03 adds border/side and solid-background shorthands with component resets.
The original collection from `doc/SKETCH.md` now passes styling and adaptation tests
with `border: 0` and `background: #ffffff` unchanged. Richer shorthand forms
remain diagnosed as unsupported.

Development requires sibling checkouts at `../w3c-dom`, `../css-box-layout`,
`../tree` and `../schuko`,
selected by `go.mod` replacements. The reviewed baselines are `w3c-dom` at
`ddac155`, `css-box-layout` at `66e420e`, `tree` at `167ff46c52db`, and
`schuko` at `287adb3427da`. The `w3c-dom` and `css-box-layout` `v0.0.0`
requirements are local development placeholders, not published releases.
Replace them with released/pinned versions
when the upstream changes are published. External checksums are in `go.sum`.

```sh
go test ./...
go vet ./...
go test -race ./...
go test ./template -run '^$' -fuzz '^FuzzTemplateCollection$' -fuzztime=10s -parallel=2
```

See [the loading package and PR plan](doc/TEMPLATE-LOADING.md),
[the broader design](doc/DESIGN.md), and
[the preparation phase](doc/PREPARATION.md).
