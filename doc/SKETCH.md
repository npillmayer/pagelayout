# Page-template sketch

Two alternatives below describe the same page and frame geometry: a custom XML
template and an HTML/CSS template. CSS assigns HTML content to named flows and
connects frames to those flows, following the approach described in
[REGIONS.md](REGIONS.md). The custom XML and standalone HTML examples are historical sketches. The HTML/CSS
collection format in the final section is adopted and implemented by `template`.
The final section extends the HTML/CSS approach into a collection of reusable
page templates in one HTML file.

## XML template

This example defines an A4 portrait page with a heading frame and two body
frames. Dimensions carry explicit units. Coordinates start at the page's
top-left corner, with x increasing rightwards and y downwards. Each frame
rectangle describes its available content area, without implicit padding or
border. Frame IDs are unique within the template.

```xml
<page-template id="two-column-article" version="draft-2">
  <description>A page with a heading frame and two body frames.</description>

  <page width="210mm" height="297mm" background="#ffffff"/>

  <frames>
    <frame id="heading"
           x="20mm" y="20mm" width="170mm" height="18mm"/>

    <frame id="body-left"
           x="20mm" y="46mm" width="80mm" height="231mm"/>

    <frame id="body-right"
           x="110mm" y="46mm" width="80mm" height="231mm"/>
  </frames>
</page-template>
```

The XML contains no flow names, content bindings, or continuation instructions.

## HTML/CSS template alternative

Here the page and its frames are DIVs. CSS supplies the same dimensions,
positions, and white background as the XML example. This is an alternative to
the XML template; the two templates would not be used together for one page.

```html
<div id="two-column-article" class="page">
  <div id="heading" class="frame"></div>
  <div id="body-left" class="frame"></div>
  <div id="body-right" class="frame"></div>
</div>
```

```css
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

#heading {
  left: 20mm;
  top: 20mm;
  width: 170mm;
  height: 18mm;
}

#body-left {
  left: 20mm;
  top: 46mm;
  width: 80mm;
  height: 231mm;
}

#body-right {
  left: 110mm;
  top: 46mm;
  width: 80mm;
  height: 231mm;
}
```

The framework uses the page dimensions as the viewport for fixed frame layout.
Frames must be direct children of the page DIV and use `position: fixed`;
nested template frames are excluded. The page wrapper remains static. Zero
padding and borders make frame dimensions their available content dimensions,
as in the XML sketch. The content example below supplies material to the frames.

## HTML content and CSS region flows

The following content and flow rules can accompany either template. With the XML
alternative, CSS addresses XML frames directly by ID, as our adaptation of the
Regions model. With the HTML/CSS alternative, the same selectors address the
template's DIVs. The content needs no additional receiving elements.

```html
<h1 class="page-heading">A typesetting workshop</h1>

<article class="story">
  <p>The workshop brings together the design of letters and the composition
     of pages.</p>
  <p>As the article grows, its content continues from one frame into the next.</p>
</article>
```

```css
/* Collect HTML content into named flows. */
.page-heading { flow-into: heading-flow; }
.story        { flow-into: article-flow; }

/* Connect the selected template's frames to those flows. */
#heading { flow-from: heading-flow; }
#body-left,
#body-right { flow-from: article-flow; }
```

The heading has its own flow. Both body frames receive `article-flow`, with an
intended reading order of `body-left` followed by `body-right`. In the HTML/CSS
alternative, the Regions model supplies that order through the receiving DIVs'
document order. Ordering for the XML alternative remains to be designed. The
comma-separated CSS selector assigns a flow; its selector order does not define
the chain. Content retains its source styling as it flows. These short paragraphs
illustrate the binding; longer content would be needed to exercise overflow.

The `flow-into` and `flow-from` properties are draft Regions syntax, used here as
the implemented routing/binding syntax in our restricted profile. See the
[CSS Regions draft](https://www.w3.org/TR/css-regions-1/) for their original model.

When the last frame cannot fit the remaining content, layout should report the
remainder. Cross-page continuation, repeated headings, and policies for
unbreakable content remain open for later iterations.

## Collection in one HTML file

Each `<template>` stores one page definition. Its `id` identifies the template;
`data-type="page"` marks its purpose for our framework. The inner `.page` DIV
owns the page geometry, and its child DIVs are the receiving frames. This example
contains the same two-column layout as above and a chapter-opening variant.

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

`<template>` stores inert content rather than acting as a page box. Its contents
belong to a separate document fragment. Geometry therefore goes on the inner
DIV, and selectors such as `.page.two-column` address the instantiated page.
Selectors such as `template .frame` do not reach into the stored fragment.
[HTML template semantics](https://html.spec.whatwg.org/multipage/scripting.html#the-template-element)

The loader compiles every definition in isolation, applying the collection's
stylesheet to its active page and frame DIVs. Fixed frames are resolved against
the page's own viewport; the `.page` DIV's ancestry alone does not establish that
viewport. The resulting catalog supports selection by template ID without
restyling or recalculating geometry. The collection alone displays no pages in a
browser. `pgtmpl serve` supplies a one-template-at-a-time SVG preview from the
compiled catalog, with navigation and channel-colored frame outlines.

Content belongs to a separate document (Cont-Doc), styled in its own ancestry.
The content's stylesheet supplies its routing rules:

```css
.page-heading { flow-into: heading-flow; }
.story        { flow-into: article-flow; }
```

These rules accompany the heading and article HTML from the earlier example;
they do not belong in the template collection. Frame `flow-from` values identify
the receiving channels. `content.Load` routes the source branches into named Box registers, and
`content.Prepare` plus the optional typography bridge supports reusable
preparation and whole-tree layout without merging cascades. Connecting those
registers to frames, finite-height fitting and continuation remain future work.

The collection above is kept in sync with the executable fixture
[`sketch.html`](../template/testdata/templates/sketch.html). A separate
[`article.html`](../template/testdata/content/article.html) illustrates Cont-Doc;
it is not an input to `template.Load`. Run the catalog examples from the module
root with `go test ./template -run '^Example' -v`.

Frame roles use classes rather than IDs so that multiple page instances can
coexist without duplicating element IDs. Within each instantiated two-column
page, the body DIVs' document order establishes left-to-right continuation.
Selecting templates for successive pages and carrying flows between those pages
remain separate design questions. A heading flow does not itself imply that
its content repeats on every page.
