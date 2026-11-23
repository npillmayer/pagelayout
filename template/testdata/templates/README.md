# Template collection examples

Use these HTML files with the [`template` loaders](../../README.md). From the
module directory, preview the complete collection with:

```sh
go run ./pgtmpl serve template/testdata/templates/sketch.html
```

| File | How to use it |
| --- | --- |
| [sketch.html](sketch.html) | A complete collection with two templates, millimetre dimensions, two flow channels and five receiving frames. Inspect page sizes, solid paint and source-ordered frame chains. |
| [invalid-overlap.html](invalid-overlap.html) | Load to inspect overlap diagnostics. A valid first definition followed by invalid overlapping receivers rejects the entire collection. |
| [two-pages.html](two-pages.html) | Read as an example of two definitions, head styles and ignored non-page storage. It omits required geometry and is not a loadable catalog. |
| [repaired.html](repaired.html) | Read as an example of omitted document/end tags and unclosed DIVs. It also omits required geometry and is not a loadable catalog. |

The separate [article document](../content/article.html) uses `flow-into` and
loads with `content.Load`. It is not an input to `template.Load` or `pgtmpl`.
Only receiving `flow-from` rules belong in the collection.

For additional complete collections and stylesheet overrides, see the
[module examples](../../../examples/README.md).
