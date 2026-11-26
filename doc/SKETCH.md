# A complete page-template collection

This collection contains a two-column article page and a chapter-opening page.
Both use a heading channel and an article channel. Sizes are authored in
millimetres; loading returns compiled CSS-pixel rectangles and source-ordered
frame chains.

Preview it from the module directory:

```sh
go run ./pgtmpl serve template/testdata/templates/sketch.html
```

## Collection in one HTML file

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Page-template collection</title>
  <style>
    .page {
      position: static;
      width: 210mm;
      height: 297mm;
      background: #ffffff;
      box-sizing: content-box;
      margin: 0;
      padding: 0;
      border: 0;
    }

    .page > .frame {
      position: fixed;
      box-sizing: content-box;
      margin: 0;
      padding: 0;
      border: 0;
    }

    /* The two-column page. */
    .page.two-column > .heading {
      left: 20mm; top: 20mm; width: 170mm; height: 18mm;
    }
    .page.two-column > .body-left {
      left: 20mm; top: 46mm; width: 80mm; height: 231mm;
    }
    .page.two-column > .body-right {
      left: 110mm; top: 46mm; width: 80mm; height: 231mm;
    }

    /* A larger heading followed by a single wide body frame. */
    .page.chapter-opening > .heading {
      left: 20mm; top: 20mm; width: 170mm; height: 38mm;
    }
    .page.chapter-opening > .body {
      left: 20mm; top: 70mm; width: 170mm; height: 207mm;
    }

    /* Both templates accept the same named flows. */
    .page > .heading { flow-from: heading-flow; }
    .page > .body    { flow-from: article-flow; }
  </style>
</head>
<body>
  <template id="two-column-article" data-type="page">
    <div class="page two-column">
      <div class="frame heading"></div>
      <div class="frame body body-left"></div>
      <div class="frame body body-right"></div>
    </div>
  </template>

  <template id="chapter-opening" data-type="page">
    <div class="page chapter-opening">
      <div class="frame heading"></div>
      <div class="frame body"></div>
    </div>
  </template>
</body>
</html>
```

## Loading and content

The collection above is synchronized with
[sketch.html](../template/testdata/templates/sketch.html). Use `template.Load`,
`LoadFS` or `LoadFile` to obtain its immutable catalog; see the
[client examples](../template/README.md).

The separate [article document](../template/testdata/content/article.html)
demonstrates `flow-into` routing and loads with `content.Load`. Its content styles
never enter the template cascade. These inputs demonstrate loading and inspection;
they do not supply the owning links and default-main receiving design required
for a complete pagination example.

For an end-to-end document, use the [pagination proof inputs](../testdata/pagination/README.md#real-font-document-proofs),
which combine a separate template collection, main/secondary channels, owning
links and explicit font services. Additional template designs and an external
stylesheet override are in [examples](../examples/README.md).
