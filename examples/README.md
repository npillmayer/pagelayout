# Template collections

These files are template collections for `pgtmpl` and the `template` loaders.
Receiving frames are empty: content documents and their `flow-into` rules belong
elsewhere. All frames are direct page children with `position:fixed`; dimensions
and insets are resolved against their containing page.

## Valid examples, from simple to more involved

| File | Templates | Demonstrates |
| --- | --- | --- |
| [01-minimal.html](01-minimal.html) | `note` | One small page and one `main` frame; inline styles and pixel dimensions. |
| [02-letterhead.html](02-letterhead.html) | `letter` | A4 dimensions in millimetres, three flow channels, and auto sizes resolved from opposing insets. |
| [03-magazine.html](03-magazine.html) | `magazine-opening`, `magazine-continuation` | Two page variants, an ordered two-column article chain, a separate notes frame, solid paint, padding and borders. |

Run from the module root:

```sh
go run ./pgtmpl load examples/01-minimal.html
go run ./pgtmpl load examples/02-letterhead.html
go run ./pgtmpl load examples/03-magazine.html
go run ./pgtmpl serve examples/03-magazine.html
```

Open the URL printed by `serve`. Flip between the magazine templates to see how
the same article channel receives a wide frame on one page and two narrower
frames on the next. This illustrates template geometry; it does not paginate or
flow article content. The notes frame also demonstrates the difference between
its content rectangle and its larger border rectangle. Toggle **Show frame
borders** to distinguish the viewer's diagnostic outlines from authored paint.

## Deliberately invalid examples

Names end in `_err.html`. Each file includes a comment explaining the defect
and its correction. These should fail loading, not produce a usable catalog.

| File | Expected diagnostic | Defect |
| --- | --- | --- |
| [nested-frame_err.html](nested-frame_err.html) | `structure/frame.content` | A template frame contains another DIV. |
| [percentage-width_err.html](percentage-width_err.html) | `profile/profile.value` | A percentage width is outside the supported CSS profile. |
| [overlapping-frames_err.html](overlapping-frames_err.html) | `geometry/geometry.overlap` | The second definition has overlapping frames; the entire collection is rejected, including the valid first definition. |

```sh
go run ./pgtmpl load examples/nested-frame_err.html
go run ./pgtmpl load examples/percentage-width_err.html
go run ./pgtmpl load examples/overlapping-frames_err.html
```

The CLI prints diagnostics to stdout and exits with status 1. `go run` additionally
prints its own `exit status 1` message to stderr. `serve` with any invalid example
must report errors and start no server; `find` also validates before returning a
path. To try lookup, run `go run ./pgtmpl find examples/01-minimal.html`; the usual
user-configuration-directory search takes precedence over the local file.
