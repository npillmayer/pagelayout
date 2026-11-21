# Collection-discovery fixtures

- `two-pages.html` contains two page definitions, ordered head styles and ignored
  non-page storage. Frame HTML IDs deliberately recur across definitions because
  each definition will have its own active styling document.
- `repaired.html` deliberately omits document/end tags and closes a template
  without first closing its DIVs. The tests assert the repaired HTML tree and the
  parser's representation of template contents.

These fixtures exercise the private TPL-01 source stage. CSS is retained without
evaluation, and the fixtures are not claims of compiled page geometry. Structural
failure cases are kept beside their expectations in `template/source_test.go`.
