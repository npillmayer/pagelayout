// Package template loads HTML/CSS page-template collections into immutable catalogs.
//
// # Collections
//
// Store definitions as direct body children of the form
// <template data-type="page" id="name">. Each holds one page DIV whose direct,
// empty DIV children are receiving frames. The page has an absolute positive
// size; frames use position:fixed and flow-from. Source order determines template
// order and each channel's receiver chain. The storage template is absent from
// the instantiated page's selector ancestry; html/body attributes are preserved.
//
// [Load] reads caller-owned HTML, validates the repaired tree and all retained CSS
// declarations, then compiles every definition atomically. Losing and nonmatching
// declarations are checked too. Each definition has an isolated styling context.
// Frames must have positive content rectangles, lie within their page and have
// nonoverlapping border boxes. Touching edges are allowed. Unsupported input or
// invalid geometry returns a [LoadError] and no partial catalog.
//
// # Input and stylesheets
//
// [LoadFS] opens and closes an exact filesystem path. [LoadFile] uses injectable
// [IO] to search the user configuration templates directory before the working
// directory. Absolute paths are opened directly; relative paths escaping through
// .. use only the working directory. Only missing files permit fallback.
// [LoadFSWithOptions] and [LoadFileWithOptions] accept additional author CSS.
//
// The Stylesheets field of [LoadOptions] appends caller-supplied sheets after
// head CSS, retaining normal specificity, inline-style and importance precedence. No resource fetching
// or script execution occurs. CSS locations use the supplied stylesheet label;
// HTML diagnostics preserve original node identity and wrapped causes.
//
// # Catalog lifetime and coordinates
//
// [Catalog.Lookup] returns an independent template handle without parsing,
// recascading or geometry work. Catalogs retain compiled values, not DOM/CSSOM
// state. Accessors return copied descriptions and slices; inspection is safe
// after input closure and across concurrent readers.
//
// Page dimensions and frame border/content rectangles use page-local CSS pixels.
// A frame's content origin already includes its position and edges. [Paint]
// contains resolved solid backgrounds and borders. Flowing content is loaded and
// styled separately; a frame's flow-from selects a channel without restyling it.
// This package does not use Go's html/template or text/template engines.
package template
