# Flow channels and receiving regions

Pagelayout uses `flow-into` and `flow-from` as named-flow properties within its
own authoring profile. The Go framework performs routing and pagination; it does
not delegate that work to a browser.

| Concept | Representation |
| --- | --- |
| Source branch | A non-nested block branch assigned with `flow-into: name`. |
| Flow channel | A persistent FIFO queue of qualified `content.Box` handles. |
| Default channel | `main`, containing the residual source tree when present. |
| Frame or region | A direct fixed child of a page DIV receiving `flow-from: name`. |
| Region chain | All frames bound to one channel, in template source order. |
| Continuation | Exact opaque fitting state for the unchanged queue head. |

## Source styling and receiving geometry

Route extraction preserves the source document's computed styles and provenance.
A branch placed into a frame is not reparented in the DOM or restyled against
frame selectors. Frame dimensions affect used widths, atomic interiors and line
breaks. Source typography, paint and policy remain unchanged.

A continuation may enter a frame of a different width. Its accepted prefix stays
frozen; the suffix is fitted under the new constraints. A text offset, remaining
height or sum of frame heights cannot substitute for the exact checkpoint.

## Order and placement relationships

Both source queues and receiving chains retain authoring order. A page attempt
cannot skip an unconsumed queue head to reach material whose anchor appears
first. Owning links annotated with `data-flow-anchor` activate secondary material
when placed; ordinary references do not. Deferral carries loss, and all related
material must drain before the current pagespan closes.

A template need not provide a receiver for every channel. Missing secondary
receivers leave targets pending and incur carry loss. A future-span queue head
can obstruct draining without making the source plan malformed. These rules are
specified in [SHIPOUT.md](SHIPOUT.md).

## Boundaries

Nested route roots, inline extraction, atomic-interior extraction and floats are
outside the content profile. Frames are top-level fixed boxes without nested
receivers. Repeated/running material and mandatory per-page channels require
separate contracts. See the [content reference](CONTENT-LOADING.md) and
[template reference](TEMPLATE-LOADING.md) for validation details.
