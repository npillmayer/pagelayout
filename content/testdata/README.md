# CONT-00 content fixtures and expected results

Date: 2026-11-22. Status: adopted acceptance contract. These files accompany
[CONTENT-LOADING.md](../../doc/CONTENT-LOADING.md). CONT-01's private source
parser/inspector/styler and CONT-02's typed projection/profile are implemented.
Tests cover source styles, typography, paint, typed lengths, decoded text ranges,
unsized atomics and participation classifications. CONT-03 tests ordered channel
branches, exclusive source ownership and residual projections. CONT-04's
`document_test.go` and `normalize_test.go` verify normalized trees/groups,
public inspection, atomic publication and persistent bank/handle lifetime.
CONT-05's executable examples remain pending. These are content documents,
not template collections accepted by `pgtmpl`.

Run `go test ./content` from the module root for source through publication checks.
`loadSource` retains both `_err` files as styled source. `loadProjection` rejects
`float_err.html` with the expected authored profile diagnostic; nested-flow
ownership fails in CONT-03's `loadPartition` with both conflicting sources.
Public `Load` composes those stages with normalization and group indexing.
The private trees remain inaccessible through mutable pointers; inspection
returns copies and registers hold read-only branch-root Box handles.

DOM-CONT-02 adds a focused integration check in `content/border_fonts_test.go`:
the shared text-style snapshots and border getter resolve story-b's `.1em`
border to 98304 DU (2px), or 117965 DU (approximately 2.4px) with the override.
Currentcolor follows each source cascade. This check uses no frame or font
services. CONT-02's projection tests now also verify these effective edges and
paint alongside the remaining source constraints and snapshots.

Load each HTML file as a complete body with its embedded CSS. Supply the override
CSS only in the explicitly named variant. No template catalog, receiver size,
font service, network fetch or resource resolver is an input. Label source
diagnostics with the fixture filename and additional CSS with its own filename.

## Assertion conventions

`#name` denotes an HTML ID used to identify an original source element for these
assertions. It is not a normalized BoxID. Resolve IDs in the repaired source DOM,
then compare source identities carried by the resulting box handles. The tree
labels R, A, N and B below are explanatory labels, not numeric API values.

All box identities are qualified by snapshot and tree. Independent loads have
distinct handles even if their local numeric IDs repeat. Inspection returns
copies; register entries are valid concrete `content.Box` handles. Each entry
represents a branch root and its complete subtree.

Group counts refer to actual `BoxAnonymousInline` groups, visited in normalized
preorder, including empty groups and groups inside inline-blocks. Owner means
the nearest normalized block container. Whitespace-only indentation between
block children may be omitted by normalization, but its decoded source bytes
remain available and its ownership/classification is accounted for.

Raw text assertions use UTF-8 byte offsets in individual decoded DOM text nodes.
Entity spelling and HTML source offsets are not those coordinates. This milestone
does not produce collapsed paragraph text, insert textual atomic markers, shape
text, resolve used geometry or create within-box continuations.

## routing.html

Input: [routing.html](routing.html), without additional CSS. Success, no
diagnostics. `Channels()` is `[article, notes]`, in first-occurrence order.
The initial bank has two nonempty registers and three total items.

### Ownership and ordering

| Tree | Destination | Root and owned source elements |
| --- | --- | --- |
| R | Residual, outside the bank | `body`, `#intro`, `#source`, `#ending`; excludes all three routed branches. |
| A | `article`, first item | `#story-a`, `#paragraph-a`, `#tag` and their text. |
| N | `notes`, first item | `#note` and its text. |
| B | `article`, second item | `#story-b`, `#paragraph-b` and their text. |

The branch roots occur globally as `#story-a`, `#note`, `#story-b`. Grouping
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
text must not be flattened into the outer group's text sequence. Groups and
owners are identified by valid qualified Box handles; anonymous groups have no
invented source NodeID. The paragraph context comes from the owner.

### Effective source styles and constraints

| Source | Font size (CSS px) | Foreground | Other required values |
| --- | --- | --- | --- |
| `html`, `body`, `#intro` | 16 | `#000000` on html; `#203040` on body/intro | Source root font is 16px. |
| `#source`, `#ending` | 20 | `#203040` | Residual structure keeps the section's font context. |
| `#story-a`, `#paragraph-a`, `#tag` | 20 | `#224466` | Color originates in story-a's inline declaration; paragraph margins are zero. |
| `#note` | 20 | `#884400` | Original `.story + .note` matching survives extraction. |
| `#story-b`, `#paragraph-b` | 20 | `#203040` | Paragraph margins are zero; branch-root edges below are retained. |

All these nodes inherit language `en`, direction LTR, normal whitespace and
normal line-height. Defaults retain serif, weight 400 and normal slant. Hyphenation
policy is `manual`; loading does not disable it or attempt hyphenation.

For `#story-b`, preserve width as `layout.LenPercent` with fraction `0.75`, not
a guessed pixel width. Resolve margin-left `1em` to 20px, padding-left `0.5rem`
to 8px, and border-left `0.1em solid currentcolor` to approximately 2px solid
`#203040`, using checked DOM font-context resolution. Allow the documented DU
conversion precision when comparing resolved lengths. Other edges retain their
defaults. These are computed constraints, not an established border-box size or
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

### Persistent register assertions

Starting from `initial := document.Bank()`:

1. Taking from `article` returns A and a new bank whose article head is B;
   `initial` still has A followed by B, and both banks still have N in notes.
2. Taking article again from the derived bank returns B and removes the article
   register from that derived bank. A further take returns invalid zero Box and
   `false`. The notes register remains, so that bank's `Len()` is one.
3. Prepending the unchanged whole A item to the bank from step 1 restores A, B
   there. It changes neither step 1's bank nor `initial`.
4. An independently derived bank can consume N while leaving article unchanged.
   Neither derivation commits a new current bank automatically.
5. `document.Channels()` stays `[article, notes]` even when a derived bank has
   drained a register. Inventory is source metadata, not the candidate bank's keys.
6. Retained Box handles remain valid and inspectable after the public Document
   reference is released. They retain the source snapshot needed by later
   preparation. Copied inspection records cannot mutate that snapshot.

These assertions restore entire boxes only; no assertion claims that taking a
box fits it or creates a partially consumed remainder.

## routing.html with routing-override.css

Load the same HTML with one `Stylesheet` containing the text of
[routing-override.css](routing-override.css), named `routing-override.css`.
Success. It is appended after the head stylesheet as ordinary author CSS.

| Observation | Expected result |
| --- | --- |
| Channel order | `[features, notes]`; no `article` register. |
| Branch membership | A then B in features, N in notes; residual ownership and all six groups unchanged. |
| Section and descendants' font size | 24px; html/body/intro remain 16px. |
| Note color | `#006699`: later equal-specificity sibling rule wins; still-later `.note` red rule has lower specificity and loses. |
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
`[notes]`; its register holds `#aside` then `#moved`, in separate trees.

| Destination | Required result |
| --- | --- |
| First notes branch | `#aside` and `Routed aside.`; one group owned by `#aside`, source font 20px. |
| Second notes branch | `#moved` and `Moved from decorated ancestor.`; one group owned by `#moved`, source font 16px. |
| Residual | `body`, `#split` and `#emptied`; both routed roots are absent. |

After filtering, `#split` has one horizontal group containing two text nodes in
order: `Before ` and ` after.`, each with its own source reference `[0, 7)`.
Preserve both boundary spaces in raw storage. Paragraph whitespace processing
belongs to preparation. Do not retain two separate groups caused by the former
intervening aside, and do not include `Routed aside.` in residual text. The
group owner remains `#split`, with 20px source font and `#203040` foreground.

`#emptied` remains a residual block with 16px padding and a 1px solid `#884400`
border on every side. Its normalized children contain one empty horizontal
group, owned by `#emptied`, with 16px font context. Its former child's tree
does not acquire a copy of those ancestor edges. Thus the residual has two
groups, and all trees together have four. Empty does not mean unowned, invisible
or removable; this milestone does not measure any group's height.

## suppression.html

Input: [suppression.html](suppression.html). Success. Channels are empty and
the bank has zero registers. The residual contains body and `#visible`, with
one group owned by `#visible` and text `Visible residual text.`. Defaults are
serif, 16px, weight 400, normal slant, black, language en, LTR and normal
whitespace/line-height, with manual hyphenation metadata.

| Source | Classification and consequence |
| --- | --- |
| `#suppressed` and `#suppressed-child` | Display-suppressed subtree. No hidden/nested channels and no nested-route error; valid authored CSS is still inspected. |
| `#inert` / `#stored` | Inert template boundary and contents. No inert channel or group; the stored inline declaration is not active CSS. |
| `#metadata` | Non-content script/metadata. No execution, boxes, text group or flow item. |
| Head/title/style/meta | Non-content source; direct head style supplies CSS, not flow text. |

Retain decoded source and diagnostic identities for the excluded subtrees so
their absence from material is explained. Do not treat their text as missing
residual material. Formatting-only whitespace is handled by normalization.

## empty.html

Input: [empty.html](empty.html). Success with no channels and an empty bank.
The residual root is body. The builder supplies one anonymous inline group for
this empty/formatting-whitespace-only container; it must be indexed with body
as its owner. It has no non-whitespace content, atomics or register item. Retain
the same default source typography as in suppression.html, including language en.
No placeholder paragraph text is invented and no line height is measured.

## Expected errors

| Input | Stage/code | Required source information | Publication |
| --- | --- | --- | --- |
| [nested-flow_err.html](nested-flow_err.html) | `routing` / `route.nested` | Conflicting child `#inner` (`notes`) and ancestor `#outer` (`article`), original DOM paths and filename. | No Document, bank or valid handles escape, including the earlier valid `#earlier` branch. |
| [float_err.html](float_err.html) | `profile` / `profile.value` | Property `float`, value `right`, fixture filename and position in its embedded stylesheet; identify `#illustration` if an effective-node diagnostic is also emitted. | No Document or bank. Do not silently ignore float or create an illustrations register. |

The float fixture intentionally uses a DIV placeholder so an unsupported image
resource or table does not obscure the float diagnostic. It exercises the
deferred scope boundary, not future image/table-to-frame placement. Error prose
and numeric source IDs are not fixed. Preserve wrapped upstream causes wherever
an upstream operation fails, and order diagnostics deterministically.

## Additional focused assertions

These small fixture files anchor the contract; future tests should add focused
variants without requiring separate fixture files for every case:

- Same-name nested roots and explicit `flow-into:inherit` still fail; a child
  `flow-into:none` stays owned by its routed ancestor.
- Named roots on html/body, on inline/inline-block elements or inside atomic
  ancestry fail without partial publication.
- Unsupported authored CSS still fails when it loses or matches only suppressed
  nodes. Distinguish this from excluding inert template content.
- Visibility hidden/collapse, zero font size, constrained height and unsupported
  resource requirements receive source-specific errors; source/read failures and
  invalid UTF-8 never publish partial material.
- Independent builds retain qualified IDs and source references. A later build
  failure cannot publish earlier successful trees or modify a retained snapshot.
- Loading invokes no font lookup, shaping, intrinsic measurement, used-value
  resolution, frame fitting, rendering or continuation operations.

Use the existing [sketch content](../../template/testdata/content/article.html)
as an additional CONT-05 integration input. General fragmentation, residual
binding and float placement remain later decisions.
