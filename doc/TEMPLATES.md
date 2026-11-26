# Authoring page-template collections

A collection describes available page designs, their receiving frames and channel
bindings. It is separate from the content document. Use the [template package](../template/README.md)
to load it or [pgtmpl](../pgtmpl/README.md) to inspect it before pagination.

## Page definitions

Store each definition in a direct body child `<template data-type="page" id="…">`.
Inside it, one page DIV sets the page size; its direct empty DIV children define
fixed frames. Target page/frame classes in CSS, not the inert storage wrapper.
Only that definition participates when its active document is styled.

The catalog preserves definition order. Pagination uses this order to settle
otherwise equal template and sequence choices. Different IDs can describe a
narrow opening, a wider continuation or a page receiving only secondary material.
An explicit eligibility callback can constrain which definitions search tries.

## Receiving frames

Give each frame a `flow-from` name, absolute position and positive receiving size.
The default content channel is `main`. Multiple frames with the same name form
a region chain in source order; their widths may differ. Do not reorder the DOM
merely to make its order follow screen coordinates unless that is the intended
flow sequence.

The page is the fixed-position viewport. A frame's border box must lie within it
and cannot overlap another frame. Padding and borders reduce its content area.
Use compiled `ContentBox` coordinates for fitting; their origins already include
the frame's position and edges.

Template frames do not restyle content. They supply dimensions and their own
paint; text, paragraph paint and box edges retain source-document styles. Put
`flow-into` rules in the content document and `flow-from` rules in the collection.

## Overrides and validation

Additional author stylesheets can change paint, positions or sizes through the
normal cascade. Choose sufficient selector specificity or explicit importance
when needed. All definitions are revalidated after the combined cascade; an
override that creates an overlap or empty receiving area rejects the collection.

[Examples](../examples/README.md) range from a single frame to letterhead and
magazine designs and include an external override. The full
[HTML/CSS profile](TEMPLATE-LOADING.md#supported-html-and-css-profile) defines
admitted properties and units; loading deliberately rejects unsupported source
CSS even when the declaration would lose or not match.

## Choosing pages

Collection loading supplies reusable geometry, not content-dependent page choices.
Pagination tries eligible templates from immutable register states, measures local
page quality and selects sequences within synchronization spans. See
[SHIPOUT.md](SHIPOUT.md) for tie rules, secondary-channel activation and search
limits. A preview of empty frames alone cannot establish that a content document
will fit or drain successfully.
