# Documentation

## Package usage

| Guide | Contents |
| --- | --- |
| [pagelayout](../README.md) | Plans, page attempts, search, document pagination and selected-page delivery. |
| [template](../template/README.md) | Loading, selection, compiled geometry, CSS overrides and diagnostics. |
| [content](../content/README.md) | Source styles, routing, registers, inspection and neutral preparation/fitting. |
| [typography preparation](../preparation/typography/README.md) | Explicit services, preparation reuse, measurement, layout and exact fitting. |
| [pgtmpl](../pgtmpl/README.md) | Command-line validation, browser previews, lookup and stylesheet options. |

## Authoring and API references

- [Authoring template collections](TEMPLATES.md) and [complete collection](SKETCH.md).
- [Flow channels and regions](REGIONS.md).
- [Template HTML/CSS profile and geometry](TEMPLATE-LOADING.md).
- [Content profile, routing and source identities](CONTENT-LOADING.md).
- [Preparation, measurement and fitting](PREPARATION.md).
- [Pagination, loss, outcomes and delivery](SHIPOUT.md).

## Examples and expected output

- [Template collections](../examples/README.md): simple and multi-frame designs,
  CSS overrides and invalid inputs.
- [Content documents](../content/testdata/README.md): ownership, source styles,
  decoded text ranges and diagnostics.
- [Preparation inputs](../content/testdata/preparation/README.md): processed text,
  atomic boundaries and hyphenation policy.
- [Two-width preview](../preparation/typography/testdata/README.md): nested
  inline-block content rendered with glyph outlines.
- [Pagination samples](../testdata/pagination/README.md): multi-page output,
  changing frame widths, manual hyphenation and secondary channels.
