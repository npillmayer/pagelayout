# Template-loading fixtures

- `two-pages.html` contains two page definitions, ordered head styles and ignored
  non-page storage. Frame HTML IDs deliberately recur across definitions because
  each definition will have its own active styling document.
- `repaired.html` deliberately omits document/end tags and closes a template
  without first closing its DIVs. The tests assert the repaired HTML tree and the
  parser's representation of template contents.
- `sketch.html` copies the collection example from `doc/SKETCH.md`. Its original
  `border: 0` and `background: #ffffff` declarations exercise DOM-03 through
  isolated template styling, checked colors and effective border accessors.

These fixtures exercise the private source and styling stages; they are not
claims of compiled page geometry. Structural
failure cases are kept beside their expectations in `template/source_test.go`.
