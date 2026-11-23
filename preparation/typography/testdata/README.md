# Two-width typography preview

Open [two-widths.svg](two-widths.svg) in a browser or SVG viewer. It displays the
[nested content example](../../../content/testdata/preparation/nested.html) at
120 and 240 CSS pixels, using one preparation and explicit `paragraph.FirstFit`.

Blue rectangles show completed CSS border boxes; black paths show Gentium Plus
glyph outlines. Nested inline-block interiors are separate paragraphs. At the
narrower width, “after.” moves to a second line.

The SVG embeds glyph paths, so viewing it needs no installed Gentium font.
Only its small labels use the viewer's sans-serif font. The preview shows
complete subtree layouts, not pagination or fitting into a finite-height frame.

For your own font and HTML, follow the
[preparation and layout example](../README.md#prepare-once-and-lay-out-at-two-widths).
Completed geometry and frozen paragraphs remain available after the preparation
session closes; keep font resources valid when retrieving outlines.

## Font provenance

The preview uses
[GentiumPlus-R.ttf](../../../../opentype/testdata/fonts/GentiumPlus-R.ttf), with
SHA-256 `fa09788adcc651d77eba7d96f8cc3a600d6484c58e3b4ff79c2b2464dbfea6f8`.
The [font license](../../../../opentype/testdata/fonts/ShaperTestLatin-LICENSE.txt)
contains the SIL Open Font License covering the source font. The SVG can be
viewed independently of that font file.

To convert the preview to a larger PNG with librsvg, run from the module directory:

```sh
rsvg-convert -z 2 -o /tmp/pagelayout-preview.png preparation/typography/testdata/two-widths.svg
```
