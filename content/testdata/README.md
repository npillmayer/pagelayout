# Content examples and expected results

These files are content documents for [`content.Load`](../README.md), not
page-template collections for `pgtmpl`. Load each HTML file as a complete document
with its embedded CSS. Supply the override only for the named variant below.
Use the filename as `LoadOptions.SourceName` to label diagnostics.

| Input | Use it to inspect |
| --- | --- |
| [routing.html](routing.html) | Main plus two secondary channels, three routed branches and an inline-block interior. |
| [routing-override.css](routing-override.css) | Changes to routing, source typography, colors and font-relative edges. Apply it to `routing.html` through `LoadOptions.Stylesheets`. |
| [residual-inline.html](residual-inline.html) | Adjacent residual text after extracting branches, and an emptied but decorated ancestor. |
| [suppression.html](suppression.html) | Source classifications for display-suppressed, inert and metadata nodes. |
| [empty.html](empty.html) | An empty body with a valid residual root and empty group. |
| [nested-flow_err.html](nested-flow_err.html), [float_err.html](float_err.html) | Routing and CSS-profile errors. No document is returned. |

Loading needs no template catalog, receiving width or font service. Source
styles and raw text are available immediately; use `content.Prepare` separately
for processed paragraph input. The [preparation examples](preparation/README.md)
show that step's text and policy behavior.

## Reading the results

`#name` denotes an HTML ID used to identify an original source element below.
It is not a normalized BoxID. Compare the source identities carried by box
handles to relate results to the original elements. The tree labels R, A, N and
B below are explanatory labels, not numeric API values.

All box identities are qualified by snapshot and tree. Independent loads have
distinct handles even if their local numeric IDs repeat. Inspection returns
copies; register entries are valid concrete `content.Box` handles. Each entry
represents a branch root and its complete subtree.

Group counts refer to actual `BoxAnonymousInline` groups, visited in normalized
preorder, including empty groups and groups inside inline-blocks. Owner means
the nearest normalized block container. Whitespace-only indentation between
block children may be omitted by normalization, but its decoded source bytes
remain available and its ownership/classification is accounted for.

Raw text ranges use UTF-8 byte offsets in individual decoded DOM text nodes.
Entity spelling and HTML source offsets are not those coordinates. Loading does
not collapse paragraph text, insert atomic text markers, shape text or produce
used geometry.

## routing.html

Input: [routing.html](routing.html), without additional CSS. Success, no
diagnostics. `Channels()` is `[main, article, notes]`: main first, then secondary
names in first-occurrence order. The initial bank has three nonempty registers
and four total items, including the residual root in main.

### Ownership and ordering

| Tree | Destination | Root and owned source elements |
| --- | --- | --- |
| R | `main`, sole item; also returned by `Residual()` | `body`, `#intro`, `#source`, `#ending`; excludes all three routed branches. |
| A | `article`, first item | `#story-a`, `#paragraph-a`, `#tag` and their text. |
| N | `notes`, first item | `#note` and its text. |
| B | `article`, second item | `#story-b`, `#paragraph-b` and their text. |

Main has branch order 1. The secondary roots follow as `#story-a` (2), `#note`
(3), `#story-b` (4). Grouping
items into registers does not change that source-order metadata. The three
branches have distinct trees, and R is distinct from each of them. `#source`
remains only in R as a layout box; its original ancestry is available as style
context for A, N and B. There is no synthetic parent merging A and B.

Every participating source element and decoded text range belongs to exactly
one destination. Ancestor references used for provenance do not count as copied
layout material. Head/style/title and other metadata are classified separately.

### Normalized groups

| Tree | Ordered group owners | Group contents and boundaries |
| --- | --- | --- |
| R | `#intro`, `#ending` | One text group for each paragraph. No groups from the extracted branches. |
| A | `#paragraph-a`, `#tag` | Outer paragraph contains a text prefix, the `#tag` atomic box and a text suffix; `#tag` has its own interior text group. |
| N | `#note` | One group owned by the routed aside. |
| B | `#paragraph-b` | One group owned by the second article's paragraph. |

There are six groups in total. The atomic box remains in A, is not a new
register item and has `width:auto` without a used width or height. Its interior
text is separate from the outer group's text sequence. Groups and owners are
identified by valid qualified Box handles; anonymous groups have no
invented source NodeID. The paragraph context comes from the owner.

### Effective source styles and constraints

| Source | Font size (CSS px) | Foreground | Other values |
| --- | --- | --- | --- |
| `html`, `body`, `#intro` | 16 | `#000000` on html; `#203040` on body/intro | Source root font is 16px. |
| `#source`, `#ending` | 20 | `#203040` | Residual structure keeps the section's font context. |
| `#story-a`, `#paragraph-a`, `#tag` | 20 | `#224466` | Color originates in story-a's inline declaration; paragraph margins are zero. |
| `#note` | 20 | `#884400` | Original `.story + .note` matching survives extraction. |
| `#story-b`, `#paragraph-b` | 20 | `#203040` | Paragraph margins are zero; branch-root edges below are retained. |

All these nodes inherit language `en`, direction LTR, normal whitespace and
normal line-height. Defaults retain serif, weight 400 and normal slant. Hyphenation
policy is `manual`; loading does not disable it or attempt hyphenation.

For `#story-b`, width is `layout.LenPercent` with fraction `0.75`, not a
pixel width. Margin-left `1em` resolves to 20px, padding-left `0.5rem` to 8px,
and border-left `0.1em solid currentcolor` to approximately 2px solid `#203040`.
Font-relative lengths can have small rounding differences from DU conversion.
Other edges retain their defaults. These are computed constraints, not an
established border-box size or
placement; no receiving-frame data participates.

### Decoded text references

Each row identifies a distinct text node and its full half-open byte range.
Escapes in this table describe Unicode values; the stored strings contain those
characters, not literal backslash sequences.

| Source text node | Decoded string | Byte range |
| --- | --- | --- |
| `#intro`, sole child | `Residual café & introduction.` | `[0, 30)` |
| `#paragraph-a`, first child | `First article with an ` (trailing space) | `[0, 22)` |
| `#tag`, sole child | `atomic interior` | `[0, 15)` |
| `#paragraph-a`, last child | `.` | `[0, 1)` |
| `#note`, sole child | `A source-styled note.` | `[0, 21)` |
| `#paragraph-b`, sole child | `Second\u00a0article with a soft\u00adhyphen.` | `[0, 36)` |
| `#ending`, sole child | `Residual section ending.` | `[0, 24)` |

The `&amp;`, NBSP and soft-hyphen entities decode once. Preserve the multibyte
`é`, U+00A0 and U+00AD with valid byte boundaries and independent source IDs.

### Derive register banks

Starting from `initial := document.Bank()`:

1. Taking from `article` returns A and a new bank whose article head is B;
   `initial` still has A followed by B. Both banks retain R in main and N in notes.
2. Taking article again from the derived bank returns B and removes the article
   register from that derived bank. A further take returns invalid zero Box and
   `false`. Main and notes remain, so that bank's `Len()` is two.
3. Prepending the unchanged whole A item to the bank from step 1 restores A, B
   there. It changes neither step 1's bank nor `initial`.
4. An independently derived bank can consume N while leaving article unchanged.
   Neither derivation commits a new current bank automatically.
5. `document.Channels()` stays `[main, article, notes]` even when a derived bank has
   drained a register. Inventory is source metadata, not the candidate bank's keys.
6. Retained Box handles remain valid and inspectable after the public Document
   reference is released. They retain the source snapshot needed by later
   preparation. Copied inspection records cannot mutate that snapshot.

These operations restore entire boxes only. Taking a box does not fit it into
a frame or create a partially consumed remainder.

## routing.html with routing-override.css

Load the same HTML with one `Stylesheet` containing the text of
[routing-override.css](routing-override.css), named `routing-override.css`.
Success. It is appended after the head stylesheet as ordinary author CSS.

| Observation | Expected result |
| --- | --- |
| Channel order | `[main, features, notes]`; no `article` register. |
| Branch membership | A then B in features, N in notes; residual ownership and all six groups unchanged. |
| Section and descendants' font size | 24px; html/body/intro remain 16px. |
| Note color | `#006699`: later equal-specificity adjacent-sibling rule wins; still-later `.note` red rule has lower specificity and loses. |
| Story-a and descendants' color | `#224466`: normal `.story` rule cannot override the inline declaration. |
| Story-b and descendants' color | `#008855`: important author declaration overrides normal inline color. |
| Story-b left edges | Margin 24px, padding 8px, border approximately 2.4px solid `#008855`; width remains fraction `0.75`. |
| Source text and atomic structure | Same decoded strings/ranges and auto-width atomic boundary as the base case. |

This load has a fresh snapshot and distinct handles. A retained base Document
and its bank keep the original channel names, styles and lengths. Identical HTML
bytes do not imply identical material identity when CSS differs. No override is
fed into a template document implicitly.

## residual-inline.html

Input: [residual-inline.html](residual-inline.html). Success. Channels are
`[main, notes]`. Main holds the residual root; notes holds `#aside` then `#moved`,
in separate trees.

| Destination | Result |
| --- | --- |
| First notes branch | `#aside` and `Routed aside.`; one group owned by `#aside`, source font 20px. |
| Second notes branch | `#moved` and `Moved from decorated ancestor.`; one group owned by `#moved`, source font 16px. |
| Main/residual | `body`, `#split` and `#emptied`; both routed roots are absent. |

After filtering, `#split` has one horizontal group containing two text nodes in
order: `Before ` and ` after.`, each with its own source reference `[0, 7)`.
Both boundary spaces remain in raw storage; paragraph whitespace processing
belongs to preparation. The formerly separated text shares one group, without
`Routed aside.` in the residual text. The group owner remains `#split`, with
20px source font and `#203040` foreground.

`#emptied` remains a residual block with 16px padding and a 1px solid `#884400`
border on every side. Its normalized children contain one empty horizontal
group, owned by `#emptied`, with 16px font context. Its former child's tree
does not acquire a copy of those ancestor edges. Thus the residual has two
groups, and all trees together have four. The empty decorated block remains
available for layout; loading does not measure its height.

## suppression.html

Input: [suppression.html](suppression.html). Success. Channels are `[main]`;
the bank has one register containing the residual root. It contains body and
`#visible`, with
one group owned by `#visible` and text `Visible residual text.`. Defaults are
serif, 16px, weight 400, normal slant, black, language en, LTR and normal
whitespace/line-height, with manual hyphenation metadata.

| Source | Classification and consequence |
| --- | --- |
| `#suppressed` and `#suppressed-child` | Display-suppressed subtree. No hidden/nested channels and no nested-route error; valid authored CSS is still inspected. |
| `#inert` / `#stored` | Inert template boundary and contents. No inert channel or group; the stored inline declaration is not active CSS. |
| `#metadata` | Non-content script/metadata. No execution, boxes, text group or flow item. |
| Head/title/style/meta | Non-content source; direct head style supplies CSS, not flow text. |

Decoded source and diagnostic identities remain available for excluded
subtrees. Their text is classified as excluded, rather than residual material.
Formatting-only whitespace can be omitted from normalized groups.

## empty.html

Input: [empty.html](empty.html). Success with channel `[main]` and one bank item.
The sole item is the residual body root, with one empty inline group owned by body.
The group has no non-whitespace content or atomics. It has the same default source
typography as in suppression.html, including language en.
No placeholder paragraph text is invented and no line height is measured.

## Expected errors

| Input | Stage/code | Source information | Publication |
| --- | --- | --- | --- |
| [nested-flow_err.html](nested-flow_err.html) | `routing` / `route.nested` | Conflicting child `#inner` (`notes`) and ancestor `#outer` (`article`), original DOM paths and filename. | No Document, bank or valid handles escape, including the earlier valid `#earlier` branch. |
| [float_err.html](float_err.html) | `profile` / `profile.value` | Property `float`, value `right`, input filename and position in its embedded stylesheet; identify `#illustration` if an effective-node diagnostic is also emitted. | No Document or bank; no illustrations register is created. |

The float example uses a DIV placeholder, so the reported error concerns
`float` rather than an image or table. Numeric source IDs and error prose are
not stable identifiers; inspect `Stage`, `Code`, source paths and wrapped causes.
