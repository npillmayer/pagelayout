# CSS Regions: a design reference

Research date: 2026-11-17. Companion to [TEMPLATES.md](TEMPLATES.md).

## Proposal and status

**CSS Regions Module Level 1** proposed distributing content through a chain of
separately positioned boxes, enabling magazine-like layouts. The latest published
W3C Working Draft is dated **9 October 2014**; it never became a Recommendation.
[Published draft](https://www.w3.org/TR/css-regions-1/)

An **Editor's Draft dated 18 January 2026** remains available, explicitly marked
as work in progress. Its existence does not establish interoperable browser
support or renewed implementation commitments.
[Editor's Draft](https://drafts.csswg.org/css-regions/)

## How it worked

| Concept | Meaning in the published draft |
| --- | --- |
| Named flow | Content collected in document order using `flow-into`. |
| Region chain | Recipient boxes associated through `flow-from`, ordered by document order. |
| Fragmentation | Content continues at a break into the next region. |
| Overset | Content remains after the last region; CSSOM exposes fit/empty/overset information. |

Illustrative historical syntax, not a runnable browser compatibility example:

```html
<style>
  article { flow-into: story; }
  .frame { flow-from: story; width: 70mm; height: 100mm; }
</style>
<article><p>A story long enough to continue through several frames.</p></article>
<div class="frame"></div>
<div class="frame"></div>
```

Content leaves normal visual flow without changing its DOM ancestry or CSS
inheritance. Region layout and automatic creation of additional recipient boxes
are separate concerns. The proposal therefore did not itself supply page
templates or generate pages when frames ran out.
[Published draft: concepts, properties, and CSSOM](https://www.w3.org/TR/css-regions-1/)

## Why it stalled

- **2013–2014:** Adobe contributors sought to ship Regions in Blink after a
  prefixed implementation shipped in Safari. Blink maintainers opposed shipping,
  citing pervasive layout/painting complexity and interference with mobile
  performance work. Contributors disputed how much complexity was specific to
  Regions versus fragmentation also needed for columns and printing. Accessibility
  ordering problems were also reported. These were engineering and prioritization
  disputes, not proof that linked text frames are inherently impractical.
  [Contemporary Blink discussion](https://groups.google.com/a/chromium.org/g/blink-dev/c/kTktlHPJn4Q/m/YrnfLxeMO7IJ)
- **2017:** WebKit's removal issue cited high maintenance costs and its position
  as the remaining engine supporting the feature unprefixed. The removal issue
  is resolved as fixed.
  [WebKit removal record](https://bugs.webkit.org/show_bug.cgi?id=174978)

Thus, “never came to fruition” means it failed to become an established,
interoperable web feature; experimental and shipped implementations did exist.

## Lessons for this Go package

The following are design proposals, not decisions to implement the CSS draft:

- Separate **content streams**, **region geometry**, and **flow order**. Our page
  templates can create the regions; bindings can connect HTML content to them.
- Return explicit continuation and overflow information. Do not treat clipping
  as successful consumption of content.
- Preserve source identities and logical reading order independently of visual
  placement, including in any future accessible output.
- Keep fragmentation behind a clear interface to the sibling layout packages.
  Start with fixed rectangular frames; investigate changing widths, nested
  fragmentation, and cyclic bindings before expanding the contract.

CSS Regions is useful conceptual prior art for linked frames. Reusing that model
does not require adopting its CSS syntax or claiming browser compatibility.
