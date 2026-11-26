# typography preparation

Prepare loaded content with selected fonts, measure intrinsic widths, and lay
out whole subtrees at different widths. Use an import alias to distinguish this
package from the font and paragraph packages:

```go
import prep "github.com/npillmayer/pagelayout/preparation/typography"
```

## Prepare once and lay out at two widths

The following program accepts a font-file path. It uses the OpenType adapter to
load that font and supply fresh shaping streams. Save it as `preview.go` in an
application using this module, then run `go run preview.go /path/to/font.ttf`.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	prep "github.com/npillmayer/pagelayout/preparation/typography"
	"github.com/npillmayer/typography/glyphing"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: preview /path/to/font.ttf")
	}
	if err := run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}

func run(fontPath string) error {
	services := glyphot.Services{}
	face, err := services.Load(context.Background(), fontPath, nil)
	if err != nil {
		return err
	}
	mapper, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{
		Default: face.Resource.ID(),
		Size:    15 * dimen.BP,
		Faces:   []glyphing.FaceEntry{face},
	}, services)
	if err != nil {
		return err
	}
	backend, err := prep.New(prep.Config{
		Resolver: mapper,
		NewShaper: func() (contracts.StreamingShaper, error) {
			return glyphot.NewStream(32)
		},
	})
	if err != nil {
		return err
	}
	doc, err := content.Load(strings.NewReader(`<body style="font-size:20px;hyphens:none">
      <section style="flow-into:main">Before
        <span style="display:inline-block;font-size:12px">Wide
          <span style="display:inline-block">Inner.</span>
        </span> after.</section>
    </body>`), content.LoadOptions{SourceName: "article.html"})
	if err != nil {
		return err
	}
	prepared, session, err := content.Prepare(doc, backend)
	if err != nil {
		return err
	}
	defer session.Close()

	root, ok := doc.Bank().First("main")
	if !ok {
		return fmt.Errorf("missing main branch")
	}
	bounds, err := prep.MeasureIntrinsic(prepared, session, root, layout.ResolveContext{})
	if err != nil {
		return err
	}
	fmt.Printf("intrinsic border widths: %g..%g CSS px\n", bounds.MinContent, bounds.MaxContent)

	narrow, err := prep.Layout(prepared, session, root,
		prep.LayoutRequest{Width: 120, Breaker: paragraph.FirstFit})
	if err != nil {
		return err
	}
	wide, err := prep.Layout(prepared, session, root,
		prep.LayoutRequest{Width: 240, Breaker: paragraph.FirstFit})
	if err != nil {
		return err
	}
	fmt.Println("content widths:", narrow.Geometry()[root].Content.W, wide.Geometry()[root].Content.W)

	if err := session.Close(); err != nil {
		return err
	}
	group, ok := narrow.Group(narrow.Groups()[0].Group)
	fmt.Println("paragraph retained after Close:", ok && len(group.Paragraph.VList.Lines()) > 0)
	return nil
}
```

The two content widths are 120 and 240 CSS pixels. Intrinsic bounds and line
breaks depend on the supplied font. Both results remain usable after closing
the session. The [two-width SVG](testdata/two-widths.svg) illustrates the same
nested inline-block content with Gentium Plus.

## Select services and text policy

`New(Config)` requires a `Resolver` and a `NewShaper` factory. A
`glyphing.StyleMapper` supplies the resolver; its selected faces determine
which fonts are available. Each factory call must return a fresh, exclusively
owned stream. A failing factory must clean up its own resources and return no
stream. No fonts are discovered or downloaded by this package.

Use `Config.Hyphenator` to supply a manual-marker service when the source uses
`hyphens:manual`. This is the default source policy, so the example explicitly
sets `hyphens:none` to run without a hyphenator.

| Source policy | Preparation behavior |
| --- | --- |
| Uniform `none` | No hyphenator required; authored soft-hyphen bytes remain in the source mapping. |
| Uniform `manual` | Requires an explicit U+00AD-marker hyphenator, even when the group has no marker. |
| Mixed policies in one group, or `auto` | Rejected with `khipu.ErrUnsupportedPolicy`. |

Policy and service validation covers the whole document before preparation
resolves fonts or shapes text. Empty and object-only groups use their owner's policy; processed
text spans supply policy for other groups. Normal whitespace and normal
line-height are required. Bidi isolates are supported except for coincident
nested openings or closings at the same processed-text offset.

The document, its registers and its `content.Box` handles remain unchanged by
preparation. All groups are prepared, including residual content, empty groups
and inline-block interiors. Changing a layout width reuses this preparation.
Changing source styles or selected fonts requires a new preparation.

## Measure intrinsic widths

`MeasureIntrinsic(prepared, session, root, context)` returns `MinContent` and
`MaxContent` border-box widths in CSS pixels. They include the root's padding
and borders but exclude its own margins. The root must be a covered block
container from the prepared document.

Use `layout.ResolveContext{ContainingBlock: layout.Rect{W: 600}}` when root
percentage lengths need a containing width. This context resolves constraints;
it is not a line-breaking width. Dependencies on an unknown auto width can
produce errors. Inline-block widths must be auto or fixed pixel values.

Measurement produces no final height or geometry. Manual hyphenation supports
bounded explicit alternatives; multiple eligible splits in one textbox can
return an unsupported-policy error. Such a failure leaves the preparation
usable. Manual alternatives can be shaped lazily during measurement or layout,
so reuse does not guarantee zero later service calls.

## Lay out and inspect results

Call `Layout` with `LayoutRequest{Width: width, Breaker: paragraph.FirstFit}`.
Width is a finite, nonnegative containing-block width in CSS pixels; source CSS
still controls each box's used width. `FirstFit` must be selected explicitly.
Other breakers are unsupported. Zero width and overfull lines may produce valid
results; a successful result does not prove that it fits a frame.

`Layout` returns a whole subtree without a height limit. It does not select
pages, create within-box continuations or advance a register bank. Keep multiple
results to compare widths; discard a candidate by dropping its result.
A failed call returns nil and leaves earlier results unchanged.

| Result accessor | Use |
| --- | --- |
| `Root()` | Identify the laid-out source box. |
| `Geometry()` | Obtain copied CSS geometry keyed by qualified `content.Box` handles. |
| `Groups()` | List laid-out groups in normalized preorder. |
| `Group(box)` | Obtain copied source/paint input and a frozen paragraph result for a group. |

Geometry coordinates use CSS pixels. A geometry record's `RelativeTo` identifies
its coordinate origin by BoxID within the result root's tree. Paragraph VList
coordinates and atomic placements use big points relative to the owner's content
origin; 1 CSS pixel equals 0.75 big points. Account for these units and origins
when drawing. Do not stack already positioned lines or atomic interiors again.

## Fit frames and resume

Create a reusable `content.Fitter` after preparation, then call `Fit` with an
explicit `paragraph.FirstFit` breaker. Both `hyphens:none` and supported
`hyphens:manual` use the same API. Width and height are CSS pixels.

```go
fitter, err := content.NewFitter(prepared, session, root)
if err != nil {
    return err
}
choices, err := prep.Fit(ctx, prepared, session, fitter, prep.FitRequest{
    FitRequest: content.FitRequest{Width: 240, Height: 300},
    Breaker: paragraph.FirstFit,
})
if err != nil {
    return err
}
if len(choices) == 0 {
    return nil // ordinary non-fit; retry other receiving dimensions
}
selected := choices[0]
fragment := selected.Fragment()
next, err := prep.Fit(ctx, prepared, session, fitter, prep.FitRequest{
    FitRequest: content.FitRequest{
        Width: 180, Height: 300, Continuation: selected.Continuation(),
    },
    Breaker: paragraph.FirstFit,
})
_, _, _ = fragment, next, err
```

See [the executable two-receiver example](fit_example_test.go). The candidate
set contains all supported whole-line/block prefixes in deterministic core order.
`Progress`, `Complete`, `StopReached`, and `End` describe each alternative;
`Continuation` is its exact resumable remainder. Keep the same fitter and
preparation/session pair. `content.FitRequest.Stop` accepts a qualified structural
boundary, and `MaxSteps` supplies a local work cap. Interrupted or failed fits
return nil; an empty successful set means no fit. No bank advances automatically.

Retain `selected.Fragment()` for rendering without checkpoints or execution
handles. Its `Groups()` returns occurrence IDs; `Group(id)` gives a
`FragmentGroup` with copied source/paint input, frozen `Paragraph`, logical prefix
`Info`, original-source `Coverage`, and source `Events`. Two pieces of one source
group have different IDs. Use `Input.Group` to relate them to the original group.
Atomic interiors have separate group occurrences and matching retained geometry.

Group `X`/`Y` are receiver-relative CSS pixels; frozen paragraph coordinates are
local big points. Box `Geometry()` uses parent-content `RelativeTo` coordinates.
Use `UsedValues()` and `Containers()` for the actual retained borders, padding
and paint. Feed each frozen paragraph to typography's `render.BuildFragmentProof`
with its input text/paint; drawing atomic interiors and applying receiver transforms
remain application responsibilities.

Coverage retains whole shaping clusters and manual replacement source partitions,
including when one cluster overlaps several original nodes. `FragmentSource.Range`
addresses decoded source text, while `Start`/`End` address processed group bytes.
Generated replacement characters do not imply per-character source mappings.
Event associations follow logical coverage and empty-anchor affinity, never bidi
visual order. A group's continuation prevents replay of its reported events.
The page coordinator handles cross-group deduplication, structural block events,
flow activation and forced breaks; no event material is inserted into text.

Fitting rounds receiving widths inward to the typography unit and tests height
without upward quantization. Source geometry and conversion limits are checked.
Consumed atomic interiors remain unchanged across narrower/wider suffixes.
Discard alternatives by dropping their handles; frozen bundles remain usable
after `session.Close()` while their borrowed font resources stay valid.

## Manage lifetime and errors

Keep each `PreparedSnapshot` with its matching `PreparationSession`, and use
boxes from the covered source document. The session must remain open for
measurement, layout and fitting. Serialize all operations sharing a session or
borrowed font/shaper/hyphenator services, including preparation and `Close`; separate
sessions do not make shared services safe for concurrent use.

Close the session when no more measurements or layouts are needed. `Close` is
idempotent and releases owned execution state without closing borrowed services.
Frozen output and copied metadata remain usable afterward. Keep selected font
resources valid for as long as retained output needs their metrics or outlines.
Immutable metadata can be inspected concurrently.

Preparation failure returns no partial snapshot/session. Later measurement or
layout failures preserve the preparation for another request. Use `errors.As`
with `*content.PreparationError` to inspect stage, qualified group/owner and
source/range information when available. `errors.Is` and `errors.As` also reach
underlying font, policy, shaping and cleanup errors.
