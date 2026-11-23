# Typography preparation bridge

This optional package implements `content.Preparer` using
`typography/glyphing.StyleMapper` and `typography/glyphing/paragraph.Prepare`.
It prepares every normalized group before receiving geometry is known. The
caller supplies already selected fonts and matching services:

```go
import (
    "github.com/npillmayer/pagelayout/content"
    prep "github.com/npillmayer/pagelayout/preparation/typography"
)

// mapper was constructed with glyphing.NewStyleMapper from loaded faces and
// explicit coverage. newShaper creates a fresh, exclusively owned stream.
backend, err := prep.New(prep.Config{
    Resolver: mapper,
    NewShaper: newShaper,
    Hyphenator: explicitMarkers, // Required for source hyphens:manual.
})
if err != nil {
    return err
}
prepared, session, err := content.Prepare(doc, backend)
if err != nil {
    return err // No partial snapshot/session; prior acquired handles released.
}
defer session.Close()
for _, group := range prepared.Groups() {
    input, _ := prepared.Input(group.Group)
    // Inspect input.Text, Paint, TextSources and Occurrences as detached values.
    _ = input
}
```

There is no implicit synthetic backend, font discovery, resource fetching or
OpenType dependency in the package imports. Applications may use typography's
OpenType adapter to build the services. The module graph includes typography's
adapter dependencies; the dependency test separately verifies that the bridge's
production package graph excludes them and that content/template/CSS core remain
independent of typography and khipu.

Preflight checks all groups before font resolution, glyph coverage, scaling,
stream creation or shaping. Source policy comes from contributing processed text
spans, with the first source owning collapsed whitespace. Omitted whitespace and
structural markers contribute no policy; empty/object-only groups use the owner.
Uniform `none` disables discovery while preserving authored soft-hyphen bytes.
Uniform `manual` requires an explicitly supplied U+00AD hyphenator even without
markers. Mixed policies and `auto` return `khipu.ErrUnsupportedPolicy`; missing
services return `contracts.ErrIllegalArguments`. No policy is silently replaced.

The current text profile requires normal whitespace/line-height. Bidi isolates
use zero-byte markup with the original processed offsets. The upstream callback
can encode one opening and one closing event per offset; coincident nested
openings/closings fail preflight. This is an integration limit to address through
a later uax/paragraph extension, not a permanent restriction on loaded content.

For each group, the bridge converts DOM dimensions explicitly to typography DU,
checking the finite range before narrowing. It resolves the owner's typography
separately from the first text run, itemizes with `ItemizeInline`, and passes the
returned styled text unchanged to `paragraph.Prepare`. Paint is retained only in
the copied content metadata. Atomic occurrences select U+FFFC markers without
font binding or interior flattening; an authored U+FFFC remains ordinary text.
Each interior group is prepared independently. No receiving width, object size,
vertical metrics, line breaking or geometry is computed during preparation.

The private session handle retains `paragraph.Prepared`, copied source/atomic
metadata, font bindings and normal line-height values for the later trial bridge.
It exports no mutable paragraph accessor. Session Close drops these references;
borrowed resolvers, fonts and factories are never closed. Selected resources must
remain valid for frozen results. Serialize all session/shared-service operations.
Streams belong to each shaping operation and are finished by the typography/
khipu lifecycle, including unused streams after acceptance failure. A failing
factory owns its cleanup and must return no stream. Original errors and paragraph
stages remain reachable through `content.PreparationError`; known input-span
errors retain decoded source ranges, while unlocated engine errors use the owner.

Base preparation runs once per group. Manual alternatives may be shaped lazily
later; this is not a promise of zero later service calls. Intrinsic measurement
retains the upstream restriction to bounded explicit alternatives and reports
unsupported multiple splits without changing source policy. PREP-03 now provides
the public intrinsic operation; PREP-04 adds independent whole-tree layout.

After preparation, measure a branch or residual block container using the same
pair (import `github.com/npillmayer/css-box-layout/layout` for the context):

```go
bounds, err := prep.MeasureIntrinsic(prepared, session, doc.Residual().Root,
    layout.ResolveContext{ContainingBlock: layout.Rect{W: 600}})
```

`bounds.MinContent` and `MaxContent` are CSS-pixel **border-box** bounds, including
the selected root's padding/borders and excluding its own margins. A definite root
width contributes that prescribed width; auto width recursively measures content.
The containing width supplies root percentage resolution, not a paragraph line
constraint. Nested dependencies on unknown auto widths retain the CSS core's
explicit errors. Inline-block widths remain auto or fixed pixels in this profile.

Core performs recursive aggregation and supplies atomic border bounds; the bridge
adds each atomic's signed horizontal margins once. It converts each primitive to
DU before summing in int64, checking finite ranges before narrowing and after
summary rounding. No final heights, vertical metrics, line breaking or geometry
are computed. Every invocation uses fresh scratch and returns value widths, with
no result cache keyed only by group. The entire operation reserves the session;
pair/coverage errors never cause re-preparation. Measurement errors return zero
bounds, preserve original causes/source context and leave the pair reusable.

The content-side adapter exposes qualified box handles and borrowed value views,
never a raw mutable LayoutNode. `content.MeasureIntrinsic` and `GroupMeasurer`
are the neutral orchestration contract for other explicitly supplied backends.

Lay out the same prepared subtree at different widths with explicit FirstFit:

```go
first, err := prep.Layout(prepared, session, doc.Residual().Root,
    prep.LayoutRequest{Width: 120, Breaker: paragraph.FirstFit})
// Handle err before using first. Import typography/glyphing/paragraph.
second, err := prep.Layout(prepared, session, doc.Residual().Root,
    prep.LayoutRequest{Width: 240, Breaker: paragraph.FirstFit})
```

Both successful results coexist; a later failure returns nil without changing
either. The bridge supplies CSS normal line height, explicit zero side skips and
bottom-margin-edge atomic alignment. Other breakers, page fitting, continuation
and bank commits are outside this first slice. Width is a containing-block width;
source CSS still determines content widths. Zero width and overfull lines are valid
where supported by the paragraph engine.

`Result.Geometry()` copies CSS-pixel geometry keyed by qualified `content.Box`.
`RelativeTo` is a BoxID within the result root's tree. `Groups()` returns normalized
preorder; `Group(box)` copies the source/paint input and paragraph envelope. Its
frozen paragraph and VList are immutable. Paragraph coordinates and atomic
placements remain big points relative to the owner's content origin. Do not
translate or stack them a second time. Result accessors work after session Close;
keep the borrowed font resources valid for retained output.

Each invocation owns callback tracking and a private paragraph collector.
`content.Layout` checks complete subtree coverage and reserves the pair across
used-value resolution, recursive measurement and final layout. Only complete
success publishes CSS geometry together with all paragraphs. A failed final
callback is not retried within that invocation; a fresh call may retry using the
same preparation. The bridge uses Prepared.Trial/Trial.Assemble, never prepares on
a cache miss, and retains no trial requests or geometry on the session handles.

Tests exercise the real mapper/itemizer/paragraph pipeline with explicitly
injected fake fonts and synthetic streams. They cover all eight preparation
fixtures, owner struts, paint bypass, source/atomic mappings, lazy manual policy,
late failures, stream finishing, reverse cleanup through content's session,
retry, retained output after Close, and production dependency boundaries. They
do not claim real-font visual proof or page composition; that remains PREP-05.
Measurement tests also cover the acceptance fixtures, nested auto widths, signed
and fractional contributions, request changes, percentage-cycle errors and lazy
manual discovery failure/retry without repeating base shaping.
Layout tests check independent two-width results, exact atomic geometry, source
and bank stability, detached accessors, identity/coverage/reentrancy, failed core
validation and late assembly failures followed by retry. Real-font visual and GC
lifetime proofs remain PREP-05.
