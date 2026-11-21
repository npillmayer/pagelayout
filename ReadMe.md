# pagelayout

A Go module for template-based page composition using HTML/CSS page definitions
and fixed receiving frames. The overall machinery is still being designed.

## Implemented: TPL-01

Template loading lives in the [`template`](template/) subpackage, imported as
`github.com/npillmayer/pagelayout/template`. Its code, tests and fixtures are
encapsulated there; the root package is reserved for broader page composition.
Later styling and compilation stages will also belong to `template`.

The first slice provides private HTML collection parsing, definition discovery,
structural validation and a source model for later styling. It retains:

- The parsed html/head/body context and direct head style elements.
- Page definitions in document order, with exact, case-sensitive ID lookup.
- Each definition's page DIV and ordered, template-local frame identities.
- Source-node identities and paths in the parsed HTML tree.
- Structured, deterministic diagnostics, with original reader errors preserved.

Invalid input returns no partial collection. Successful source models own their
parse trees and must remain read-only during later phases. There is deliberately
no public loading API yet: CSS evaluation, geometry compilation and
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
positions or source lines. CSS text and attributes are retained without parsing:
this stage does not check `position: fixed`, sizes, flow names or CSS syntax.

## Dependencies and checks

Go **1.25.6** and `golang.org/x/net` **v0.49.0** match the checked-out `w3c-dom`
and `css-box-layout` baselines used for TPL-01. This stage uses their underlying
HTML parser directly; the styling and geometry modules will be integrated in
subsequent milestones. Dependency checksums are recorded in `go.sum`.

```sh
go test ./...
go vet ./...
go test ./template -run '^$' -fuzz '^FuzzTemplateCollection$' -fuzztime=10s -parallel=2
```

See [the loading package and PR plan](doc/TEMPLATE-LOADING.md),
[the broader design](doc/DESIGN.md), and
[the preparation phase](doc/PREPARATION.md).
