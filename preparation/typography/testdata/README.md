# PREP-05 real-font proof

[two-widths.svg](two-widths.svg) renders the unchanged
[nested content fixture](../../../content/testdata/preparation/nested.html) at
120 and 240 CSS pixels. Both candidates use one preparation and explicit FirstFit.
The blue rectangles are completed CSS border boxes; black paths are retained
Gentium Plus glyph outlines. Nested inline-block interiors remain separate
paragraphs. At the narrower width, “after.” moves to a second line.

The test writer reads only completed geometry, placements and outlines, after
closing the preparation session. It is not a page composer or production
document renderer. The SVG contains glyph paths, so viewing it needs no installed
Gentium font; only its small labels use the viewer's sans-serif font.

## Font fixtures

Tests load these existing sibling files explicitly and check their SHA-256:

| File in `../opentype/testdata/fonts` (from the module root) | SHA-256 | Purpose |
| --- | --- | --- |
| `GentiumPlus-R.ttf` | `fa09788adcc651d77eba7d96f8cc3a600d6484c58e3b4ff79c2b2464dbfea6f8` | Real shaping, metrics, frozen output and outlines |
| `ShaperTestLatin-NoEacute.ttf` | `a31c0d92101751fcd0643803bfb004060db47bdb5d77018cdf659526ed92be9b` | Different font resource for the same ASCII content; derived from Gentium, with U+00E9 removed and font names changed |

The sibling's `NORMALIZATION.md` records provenance and generation of the
derivative; `ShaperTestLatin-LICENSE.txt` carries the embedded SIL Open Font
License 1.1 covering the source and derivative. No font binaries are copied here.
The module already uses local sibling replacements; missing or changed fixtures
fail the tests instead of skipping them or selecting a host font.

Both fonts pass the normal strict OpenType parser. No test-font exception or
permissive parser mode is used. Concrete adapters are imported only by test and
example files; production dependency tests still exclude them from the bridge.

## Reproduce

From the pagelayout module root:

```sh
go test ./preparation/typography -run '^ExampleLayout$' -v
go test ./preparation/typography -run '^TestRealFont' -count=1
PAGELAYOUT_PREP_PROOF=testdata/two-widths.svg go test ./preparation/typography -run '^TestRealFontTwoWidths$' -count=1
```

The explicit proof path is relative to the tested package directory; ordinary
tests do not write files. To inspect with librsvg, optionally render the SVG:

```sh
rsvg-convert -z 2 -o /tmp/pagelayout-prep05.png preparation/typography/testdata/two-widths.svg
```

`TestRealFontResultsReleaseWorkingState` checks none/manual policy and both
retained-closed/dropped session wrappers. Weak references observe preparation,
streams, backend, factory/discovery recipe and collector collection while source,
banks and frozen output stay alive. It then builds real outlines again. Existing
synthetic integration tests separately cover precise geometry, later callback
failure/retry, source mappings and serialized session access.
