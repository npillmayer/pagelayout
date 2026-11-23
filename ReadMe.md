# pagelayout

A Go module for template-based page composition using HTML/CSS page definitions
and fixed receiving frames. The overall machinery is still being designed.

## Implemented: TPL-01 through TPL-05

Template loading lives in the [`template`](template/) subpackage, imported as
`github.com/npillmayer/pagelayout/template`. Its code, tests and fixtures are
encapsulated there; the root package is reserved for broader page composition.
Discovery, styling, validation and geometry compilation share this boundary.

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

TPL-04 exposes `template.Load`, resolves fixed geometry through BOX-01, checks
positive receiving areas, page bounds and border-box overlaps, and publishes an
immutable catalog. Frames and flow chains retain source order. Descriptions
contain resolved geometry, edges, paint and source references as values.

```go
catalog, err := template.Load(reader, template.LoadOptions{SourceName: "pages.html"})
if err != nil {
    return err
}
page, ok := catalog.Lookup("two-column-article")
if ok {
    size := page.Page() // Width/Height in CSS pixels
    frames := page.FramesForFlow("article-flow") // Copies in source order
    fmt.Printf("%g x %g px; %d article frames\n", size.Width, size.Height, len(frames))
}
```

Invalid input returns no partial catalog. `errors.As` exposes `*template.LoadError`
and its copied `Diagnostics()` values; original reader/parser/geometry errors
remain available through `errors.Is`/`errors.As`. Catalogs retain no DOM, CSSOM,
constraints or intermediate state. Lookup returns an independent handle, and
all returned descriptions/slices can be modified without changing the catalog.
Read-only catalog access is safe concurrently. `Load` does not close its reader.
TPL-05 completes file loading, initial discovery, executable examples and
integration tests. Content layout and rendering remain separate work.

## Executable examples

Run the Go examples from the module root:

```sh
go test ./template -run '^Example' -count=1 -v
```

[`example_test.go`](template/example_test.go) demonstrates:

- `Load`: compile a reader and inspect a receiving rectangle in CSS pixels.
- `LoadFS`: load the embedded two-template sketch, select both definitions and
  inspect their page sizes and source-ordered article frames.
- `LoadFile`: inject `IO` to locate a collection in a repository.
- `LoadError`: inspect structured diagnostics when one bad definition rejects
  an entire collection.

Each example has checked output and runs without host configuration or network
access. The full collection is in
[`sketch.html`](template/testdata/templates/sketch.html), synchronized by a test
with [SKETCH.md](doc/SKETCH.md). Its A4 pages compile to approximately
793.70 × 1122.52 CSS pixels. The two-column template has one heading frame and
two article frames; the chapter-opening template has one of each.

The companion [content document](template/testdata/content/article.html) owns
the `flow-into` rules. It illustrates future content input and is not passed to
the template loader. Template `flow-from` bindings and content styles stay in
separate documents and cascades; these examples do not lay out article content.

[`integration_test.go`](template/integration_test.go) verifies both definitions
through all three loaders, including geometry, shorthand paint, flow chains and
source references. Reload tests change styles and frame order, reject a later
invalid collection, disable source access and inspect both earlier catalogs
concurrently. Selection reads compiled values; it does not parse, style or resolve
geometry again.

## Finding and loading collection files

```go
// Search the user's template repository, then the current directory.
catalog, err := template.LoadFile("pages.html", nil) // nil selects OS I/O

// Load from a specific filesystem, including embed.FS, without discovery.
catalog, err = template.LoadFS(fsys, "pages.html")
```

`LoadFile` follows the purpose-made I/O pattern used in sibling `fontfind`:

```go
type IO interface {
    UserConfigDir() (string, error)
    DirFS(string) fs.FS
}
```

Supply an implementation to replace host access for tests or application-owned
storage. The default delegates to `os.UserConfigDir` and `os.DirFS`; there is no
global I/O override or cached search state. Relative collection names are tried
in `UserConfigDir()/templates` first, then relative to the current directory.
An unavailable configuration directory is skipped. Absolute paths are used
directly, and paths escaping through `..` are resolved from the current directory
only. Names identify collection files, with no implicit extension or recursive
search; use `Catalog.Lookup` to select a template inside the loaded collection.

Only a missing file permits fallback. Permission, read, validation and close
errors are reported, preserving the selected path and original cause through
`LoadError`. Missing-file diagnostics list candidates in search order. Both file
loaders close opened files before returning and publish no catalog on failure.
`LoadFS` uses slash-separated `io/fs` paths; `LoadFile` uses OS path syntax.

Configuration files listing repositories, system-wide shared directories and
their default locations remain to be designed. These loaders neither create
directories nor read configuration files; `IO` provides the host-access boundary
for that later work.

## Additional stylesheets

Additional author CSS can be supplied as text without modifying the collection:

```go
opts := template.LoadOptions{Stylesheets: []template.Stylesheet{
    {SourceName: "custom.css", CSS: `.page {background:#ffffff}`},
}}
catalog, err := template.LoadFSWithOptions(fsys, "pages.html", opts)
// Also available: Load(reader, opts), LoadFileWithOptions(name, hostIO, opts).
```

Sheets are appended in slice order after embedded head CSS. Normal specificity,
inline-style and `!important` precedence apply. Both original and additional
declarations must satisfy the CSS profile, including losing/nonmatching rules.
Overrides participate before geometry compilation; invalid effective geometry
rejects the whole collection. Additional CSS diagnostics name their stylesheet
and CSS location; geometry diagnostics retain the affected HTML source identity.
The file-loader variants preserve the selected collection path regardless of
`opts.SourceName`, and never search for or fetch the supplied stylesheet texts.
Existing `LoadFS` and `LoadFile` calls retain their behavior with no extra sheets.

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
Compilation rejects nonpositive receiving sizes, out-of-page border boxes and
border-box overlaps. Touching edges are allowed within a documented precision
tolerance; accepted geometry is not snapped or clipped. See
[the comparison policy](doc/TEMPLATE-LOADING.md#d-resolve-fixed-geometry-and-build-the-compiled-tree).

## Template-checking CLI

The [`examples`](examples/) directory contains three valid collections of
increasing complexity and three deliberately invalid `*_err.html` collections,
with expected diagnostics and CLI commands.

[`pgtmpl`](pgtmpl/) checks collections using the same Commando framework as
`tytui/tycli`. Run `go run ./pgtmpl load <html>` to check a local collection with
`template.LoadFSWithOptions`. Success summaries and error diagnostics go to stdout; loading
failures exit with status 1. `go run ./pgtmpl serve <html>` checks the collection
before starting a local browser preview at `http://127.0.0.1:8080/`. The viewer
shows one template at a time using compiled geometry and paint, with navigation
and optional frame outlines colored by flow channel. Invalid collections start
no server. `go run ./pgtmpl find <name>` uses `template.LoadFile` to search the
user configuration templates directory and then the current directory, validate
the collection and print its selected path. Results and errors go to stdout.
`load` and `serve` accept `--css FILE` for one additional author stylesheet, with
the CSS path relative to the current working directory. Try
`go run ./pgtmpl serve examples/03-magazine.html --css examples/magazine-override.css`.
Run `go run ./pgtmpl --help` for usage.

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
The collection from `doc/SKETCH.md` passes the complete loader with `border: 0`
and `background: #ffffff`. Integration tests also exercise nonzero padding and
solid colored borders/backgrounds on reloaded frames. Richer shorthand forms
remain diagnosed as unsupported.

BOX-01 provides `layout.ResolveFixedGeometry` in `css-box-layout`. A boundary
test resolves TPL-03's typed inputs directly, without tree construction or
content callbacks. TPL-04 uses that service for catalog compilation.

Development requires sibling checkouts at `../w3c-dom`, `../css-box-layout`,
`../tree` and `../schuko`,
selected by `go.mod` replacements. The baselines checked for TPL-05 are `w3c-dom` at
`ddac155`, `css-box-layout` at `edbd6e6`, `tree` at `167ff46c52db`, and
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

TPL-05 also checks the upstream boundaries against these same revisions:

```sh
(cd ../w3c-dom && go test ./... -count=1)
(cd ../css-box-layout && go test ./layout/... -count=1)
```

The loading milestone returns compiled templates. Pagination, content
preparation/rendering and configuration-driven shared repositories
remain outside it.

The first content-loading milestone is complete, as documented in
[Content loading and routing](doc/CONTENT-LOADING.md). It reuses
the tytui integration's source/style/projection contracts, partitions named flows
before normalization, and preserves residual content. CONT-00's reader/inspection
API, concrete box handles in generic registers, shared style-service ownership
and initial profile are adopted. [Acceptance fixtures and expected results](content/testdata/README.md)
are included. CONT-01 implements private UTF-8 source parsing, CSS inspection
and original-document styling, with inert/resource boundaries and structured
diagnostics. CONT-02 adds private authored/effective profile validation and a
detached, typed unsized projection with source typography, paint, edges, text
ranges and suppression classifications. Run `go test ./content` for these checks.
CONT-03 validates route ownership and partitions independent channel branches
and residual material, preserving source order, styles and emptied ancestors.
CONT-04 exposes `content.Load`, immutable Documents, copied source/box/group
inspection and an initial `RegisterBank[content.Box]`. Independent branches are
normalized through CSS core; residual material stays separate. Group inspection
includes anonymous owners and nested atomic interiors without measuring them.
DOM-CONT-01
adds the shared
`w3c-dom/style/textstyle` source snapshots, and TY-CONT-01 migrates tytui's
compatibility facade. DOM-CONT-02 adds checked source-font border resolution,
including inherited em/rem widths, with an acceptance-fixture integration test.
CONT-02 consumes these sibling services and preserves inherited source lengths
and unresolved percentages. BOX-CONT-01 supplies structured `layout.BuildError`
locations/causes and independent filtered-tree contract tests. BOX-CONT-02 adds
generic persistent `Register[T]`/`RegisterBank[T]` storage without payload-method
requirements. CONT-05 adds five [executable examples](content/example_test.go)
and a [content loading guide](content/README.md) with an auditable material report
and preparation handoff. Run `go test ./content -run '^Example' -v` from the module
root. No preparation or sizing is needed to inspect loaded material.
PREP-00 now pins preparation ownership, services, failure cleanup and the initial
hyphenation policy, with [acceptance fixtures](content/testdata/preparation/README.md).
BOX-PREP-01's shared group enumeration is implemented and used by content's
index. PREP-01's detached extraction/source mapping is next; engine preparation,
floats and page trials remain later work.

See [the loading package and PR plan](doc/TEMPLATE-LOADING.md),
[the broader design](doc/DESIGN.md), and
[the preparation phase](doc/PREPARATION.md).
