# Shipout

TeX uses the term "shipout" for the subroutine of completing a document-page and
writing it out on file (in print format). The shipout procedure collects vertical
material until the page box is (more than) full, then freezes the material and
puts it to the `output routine`.


## Local vs global Optimization

TeX does global optimization for paragraph breaks, but not for pages. In modern
times, documents fit into memory easily and we may think about solutions for
document-wide (or chapter-wide) optimal page-breaks. "Shipout" for global optimization
would then mean the output of a series of pages at once.

We will leave the decision about local or global pagination open for now, but we
will collect arguments for and against it in this document.

### Types of Media

For belletristic books, single page decision for breaks is usually good enough, as 
there is not much variety from page to page in terms of typesetting. For our intended
template-based page-layout this is not necessarily the primary target. Layout templates
would lend themselves well for magazines or scientific papers.


## Pagination Strategy

We will call locations in the document where a forced page-break sits "synchronisation
points". Examples may be chapters or the document end.

For a global optimization there will be a couple of trials running. These are
conceptually independent from one another. Each one owns a local copy of the 
flow-material registers. A trial consumes the flow-material and tries out different
page templates, filling the frames with the material. Synchronisation points
will force a decision for the best trial-result. The trial-result is then
transformed to a sequence of pages, which may be shipped out.

Measureing the quality of trial-results may require a penalty model, similar to
the evaluation of horizontal material (see TeX or module `khipu`). Penalties may
measure (for example):

  - Fitting / filling of text to frames
  - Distance of float material referenced on the page and their final location
  - Sizing factor for images
  - Widows / clubs

For local optimization (one page per trial run) the decision point is always the current
page. (For very long books without forced page-breaks at chapters, an artificial
intermediate decision point would need to be introduced, if too much memory would
be consumed. Disregarded for now.)


## Template Selection

It is yet unclear how trial select templates for consideration. Strategies could
include:

  - Number of float boxes on the template
  - Other flow target referenced on the current page (aside, table, …)
  - Chapter / section headings
  - ToC or other special material pages


## Concurrency

Trial runs for sequences of pages offer a natural opportunity for concurrency.
Synchronisation points will collect goroutines and their trial-results. Then
the next bunch of trial-goroutines would start.
