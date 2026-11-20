# Candidates for page templates

Research date: 2026-11-17. This document records design inputs for the proposed
Go page-layout framework. It does not select a template language or promise
standards conformance. Examples are illustrative and have not been rendered.

## What we are comparing

Our intended content format is HTML/CSS, laid out inside regions defined by page
templates. Two candidates provide different starting points:

- **CSS Paged Media** supplies page styling within the CSS ecosystem.
- **XSL-FO** supplies an XML formatting vocabulary with explicit page masters.
  **Apache FOP** (Formatting Objects Processor) is a Java implementation that
  consumes FO trees and produces paginated output, primarily PDF. “XSL FOP” is
  therefore best understood here as XSL-FO's model and its implementation in FOP.
  [Apache FOP overview](https://xmlgraphics.apache.org/fop/)

We should distinguish adopting a model, accepting its syntax, and depending on
an existing formatter. These are separate decisions for a Go library.

## CSS Paged Media

### Page model

CSS Paged Media Level 3 is a W3C Working Draft dated 14 September 2023.
`@page` describes page dimensions, margins, and related properties. Normal
document content occupies the page area; 16 predefined margin boxes provide
positions for generated headers and footers. Named rules such as `@page chapter`
are selected through the content's `page` property. Pseudo-classes include
`:first`, `:left`, `:right`, and `:blank`. Declarations inside `@page` do not
style the document's HTML elements. These constructs describe a page plus
peripheral boxes, rather than arbitrary named frames.
[CSS Paged Media Level 3](https://www.w3.org/TR/css-page-3/)

An original example with a named page, alternating inner margins, and numbering:

```css
@page chapter {
  size: A4;
  margin: 22mm 18mm;
  @bottom-center { content: counter(page); }
}
@page chapter:left  { margin-right: 25mm; }
@page chapter:right { margin-left: 25mm; }

section.chapter { page: chapter; break-before: page; }
h2 { break-after: avoid-page; }
p  { orphans: 3; widows: 3; }
```

The margins assume left-to-right book binding. The page rules and counter use
the [Paged Media model](https://www.w3.org/TR/css-page-3/); the break and line-count
controls belong to [CSS Fragmentation](https://www.w3.org/TR/css-break-3/).

### Related specifications we would need

- **CSS Fragmentation Level 3** defines how content splits across pages, columns,
  or regions. `break-before`, `break-after`, and `break-inside` govern break
  opportunities; `orphans` and `widows` constrain line counts around a break.
  Avoidance constraints may be relaxed when necessary to make progress. Defining
  a page template alone does not implement this behavior.
  [CSS Fragmentation](https://www.w3.org/TR/css-break-3/)
- **Generated Content for Paged Media** adds named strings for running titles,
  running elements for formatted headers, and footnote mechanisms. Its published
  document is a Working Draft dated 25 January 2024. Features such as
  `string-set`/`string()` and `position: running(...)`/`element()` should be
  considered separately from basic page geometry.
  [Generated Content for Paged Media](https://www.w3.org/TR/css-gcpm-3/)
- **CSS Regions** is a separate proposal for named flows distributed through
  region chains, using `flow-into` and `flow-from`. Its published document is a
  Working Draft dated 9 October 2014. It is relevant to our frame concept, but
  adopting Paged Media does not automatically adopt Regions or provide an
  implementation of it.
  [CSS Regions Level 1](https://www.w3.org/TR/css-regions-1/)

**Assessment for this project:** CSS offers the closest authoring fit to our
HTML/CSS content. A bounded subset could be a useful first input format. We still
need to decide how arbitrary regions are declared and how content reaches them.
Before claiming compatibility, we should define an explicit supported profile
and test it; specification status alone says nothing about our implementation.

## XSL-FO and Apache FOP

### Template and flow model

XSL 1.1 is a W3C Recommendation dated 5 December 2006. Its formatting vocabulary
separates page definitions from content:

| Construct | Responsibility |
| --- | --- |
| `fo:layout-master-set` | Collect named page and sequence masters. |
| `fo:simple-page-master` | Define page geometry and regions. |
| `fo:page-sequence-master` | Specify a sequence of master references or alternatives. |
| `fo:conditional-page-master-reference` | Select by position, parity, or blankness. |
| `fo:page-sequence` | Associate content with a master or sequence master. |
| `fo:flow` | Supply content distributed across pages. |
| `fo:static-content` | Supply repeated content, commonly headers or footers. |

Region names are logical: `region-body`, `region-before`, `region-after`,
`region-start`, and `region-end`. Their physical placement depends on writing
mode. XSL-FO 1.1 permits multiple body regions and flow maps assigning flows to
regions, including continuation through several regions. These are standard
capabilities, not a statement about FOP support.
[XSL 1.1, section 6.4: pagination and layout objects](https://www.w3.org/TR/xsl11/)

### Small FO example

This original example follows the structure demonstrated in Apache's
[XSL-FO input guide](https://xmlgraphics.apache.org/fop/fo.html):

```xml
<fo:root xmlns:fo="http://www.w3.org/1999/XSL/Format">
  <fo:layout-master-set>
    <fo:simple-page-master master-name="article"
        page-width="210mm" page-height="297mm" margin="18mm">
      <fo:region-body margin-top="14mm" margin-bottom="12mm"/>
      <fo:region-before region-name="heading" extent="10mm"/>
      <fo:region-after region-name="folio" extent="8mm"/>
    </fo:simple-page-master>
  </fo:layout-master-set>

  <fo:page-sequence master-reference="article">
    <fo:static-content flow-name="heading">
      <fo:block>Page layout study</fo:block>
    </fo:static-content>
    <fo:static-content flow-name="folio">
      <fo:block text-align="center"><fo:page-number/></fo:block>
    </fo:static-content>
    <fo:flow flow-name="xsl-region-body">
      <fo:block>Content that continues onto subsequent pages.</fo:block>
    </fo:flow>
  </fo:page-sequence>
</fo:root>
```

The body margins reserve space for the header and footer; declaring their
extents alone is insufficient to keep body content clear of them. The guide also
demonstrates alternating masters, first-page variants, and blank pages.
[Apache FOP: XSL-FO input](https://xmlgraphics.apache.org/fop/fo.html)

### FOP implementation boundaries

Apache's compliance table reports support for the core page-sequence machinery,
the five region types, flowing content, and static content. Relevant limits are:

- `fo:flow-map` and its assignment/source/target objects are unsupported.
- `fo:simple-page-master` is partially supported: changing page width within a
  sequence requires a forced break.
- `page-position="only"` is unsupported, despite broader support for the property.

Treat this as documentation checked on the research date, not a verified test
result for a pinned FOP binary. The distinction between XSL-FO 1.1 and FOP is
especially important for a framework centered on connected regions.
[Apache FOP compliance table](https://xmlgraphics.apache.org/fop/compliance.html)

FOP's normal input is FO, supplied directly or generated from XML through XSLT.
HTML/CSS would require a conversion layer; an FO template is not a wrapper into
which arbitrary HTML can simply be inserted with its CSS semantics preserved.
[Apache FOP quick start](https://xmlgraphics.apache.org/fop/quickstartguide.html)

**Assessment for this project:** explicit masters, region identities, content
bindings, and sequence selection provide useful architectural ideas. A Go
framework could borrow those ideas while retaining HTML/CSS content. Accepting
FO syntax would require a documented subset and mapping; executing FOP would be
a separate Java integration and a different content-layout path.

## Comparison for our design

The following is a project assessment based on the models above, not an
implementation commitment.

| Design concern | CSS direction | XSL-FO/FOP direction |
| --- | --- | --- |
| Existing HTML/CSS content | Natural authoring fit. | Conversion needed to execute FOP. |
| Template abstraction | Derive a template from page rules. | Explicit page masters are a close conceptual fit. |
| Arbitrary linked frames | Requires a separate region design or profile. | Investigate FO flow maps as a model; FOP cannot be our oracle for them. |
| Native Go framework | Extend our CSS integration at the page boundary. | Borrow concepts or parse a subset; using the formatter adds Java. |
| Main integration work | Page-rule processing plus fragmentation. | Master interpretation plus content adaptation. |

## Proposed next investigation

Keep both candidates open. For the next prototype, consider a small internal
model independent of input syntax:

1. A template defines page dimensions and named region geometry.
2. Content bindings distinguish a continuing HTML stream from repeated content.
3. A selection policy chooses the template for each page.
4. A layout operation reports placed content, remaining content, and overflow.

This is a proposal derived from our requirements, not a claim that either
standard maps losslessly to such a model. Reuse the sibling projects for styling,
box layout, paragraph work, and rendering where their contracts fit; first
investigate how they can expose resumable layout under finite region height.

Evaluate both candidates against the same examples: one body region with a page
number; a first-page variant; alternating margins; then one HTML stream flowing
through two regions and onto another page. Add an oversized unbreakable item to
make overflow and progress rules explicit.

The main questions are whether we want a standard input format or only a model
inspired by one, how independently styled HTML fragments bind to regions, and
which layer owns fragmentation and continuation. These decisions should precede
committing the public Go API to either vocabulary.
