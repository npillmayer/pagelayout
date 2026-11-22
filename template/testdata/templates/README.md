# Template-loading fixtures

- `sketch.html` is the complete two-template collection from
  [`doc/SKETCH.md`](../../../doc/SKETCH.md). It exercises the full loader,
  including its `border: 0` and `background: #ffffff` shorthands, millimetre units,
  both flow channels and all five receiving frames. Executable Go examples embed
  it; an integration test checks that the documented HTML stays synchronized.
- `invalid-overlap.html` contains a valid definition followed by overlapping
  receivers in a second definition. The diagnostics example and reload tests
  demonstrate atomic rejection of the whole collection.
- `two-pages.html` is a source-stage fixture with two page definitions,
  ordered head styles and ignored
  non-page storage. Frame HTML IDs deliberately recur across definitions because
  each definition will have its own active styling document.
- `repaired.html` deliberately omits document/end tags and closes a template
  without first closing its DIVs. The tests assert the repaired HTML tree and the
  parser's representation of template contents.

`two-pages.html` and `repaired.html` intentionally lack complete geometry and are
not examples of valid compiled collections. Additional structural failure cases
are kept beside their expectations in `template/source_test.go`.

[`../content/article.html`](../content/article.html) is the separate content
document for the sketch. Its `flow-into` rules belong to Cont-Doc; only receiving
`flow-from` rules belong in the collection. Content preparation and rendering are
not implemented, and the content file is not an input to `template.Load`.

From the module root, run `go test ./template -run '^(Example|TestIntegration)' -v`
to execute the public examples and integration checks.
