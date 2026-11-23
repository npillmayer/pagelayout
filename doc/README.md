# Documentation

Start with the guide for the package you want to use:

| Guide | Contents |
| --- | --- |
| [pagelayout](../README.md) | Package selection, template previews and the template/content workflow. |
| [template](../template/README.md) | Collection authoring, loading, selection, frame geometry, CSS overrides and diagnostics. |
| [content](../content/README.md) | Styled HTML, routing, registers, source inspection and preparation lifecycle. |
| [typography preparation](../preparation/typography/README.md) | Explicit fonts and shaping services, intrinsic measurement, two-width layout and retained results. |
| [pgtmpl](../pgtmpl/README.md) | Command-line checks, browser previews, collection lookup and stylesheet options. |

## Examples and expected output

- [Template collections](../examples/README.md): minimal, letterhead and magazine
  layouts, CSS overrides and invalid inputs.
- [Content documents](../content/testdata/README.md): channel ownership, source
  styles, decoded text ranges and error diagnostics.
- [Preparation inputs](../content/testdata/preparation/README.md): processed text,
  inline-block boundaries and hyphenation behavior.
- [Two-width preview](../preparation/typography/testdata/README.md): nested
  inline-block content rendered with glyph outlines.

## Detailed input references

- [Template HTML and CSS profile](TEMPLATE-LOADING.md#c-validate-source-declarations-and-effective-styles).
- [Content elements, properties and defaults](CONTENT-LOADING.md#6-adopted-initial-profile).
