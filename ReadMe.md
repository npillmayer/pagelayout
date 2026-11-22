# pagelayout

A Go module for template-based page composition using HTML/CSS page definitions
and fixed receiving frames. The overall machinery is still being designed.

## Implemented: TPL-01 and TPL-02

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

Invalid input returns no partial collection. Source models remain read-only;
styled working trees own their nodes, attributes and styling state. Public
loading, the complete geometry/paint CSS profile, geometry compilation and
immutable compiled catalogs are later milestones.

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
inspection and the cascade; it does not yet validate fixed positioning, used
sizes, flow bindings or the complete supported property/value profile.

## Dependencies and checks

Go **1.25.6** and `golang.org/x/net` **v0.49.0** match the checked-out `w3c-dom`
and `css-box-layout` baselines used for TPL-01. TPL-02 uses `w3c-dom`'s new
DOM-01 APIs, `cssom.InspectStylesheet` and `InspectInlineStyle`, with
`NewCSSOM`, `AddStylesForScope` and `Style`.

Development requires sibling checkouts at `../w3c-dom`, `../tree` and `../schuko`,
selected by `go.mod` replacements. The reviewed baselines are `w3c-dom` at
`c326be477fc6` plus this work's DOM-01 changes, `tree` at `167ff46c52db`, and
`schuko` at `287adb3427da`. The `w3c-dom v0.0.0` requirement is a local development
placeholder, not a published release. Replace it with a released/pinned version
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
