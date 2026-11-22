// Package template owns HTML/CSS page-template loading for pagelayout.
//
// Page definitions are stored in an HTML collection as direct body children
// named <template data-type="page" id="...">. Each contains one page DIV whose
// direct DIV children are empty receiving frames. Source order determines
// definition and frame order; classes are authoring conventions, not role markers.
//
// TPL-01 privately parses and validates collection structure. HTML parser
// recovery applies: it validates the resulting tree, not HTML syntax conformance.
// TPL-02 inspects CSS and styles an isolated html/body/page/frame document per
// definition using w3c-dom. Original source identities and pre-cascade declarations
// are retained. Unsupported rule forms and syntax failures reject the collection.
// TPL-03 checks the initial CSS profile and adapts effective page/frame values
// into detached paint, flow bindings and css-box-layout constraints in CSS pixels.
// Load compiles every definition into an immutable Catalog (TPL-04), resolving
// fixed frame geometry and checking positive receiving sizes, page bounds and
// border-box overlaps. A failure returns no partial catalog. Errors expose
// structured diagnostics through LoadError and retain original wrapped causes.
//
// LoadFS reads an explicit filesystem. LoadFile uses an injectable IO to search
// UserConfigDir()/templates before the current directory; absolute paths are
// opened directly. Both close their files and preserve the selected source name.
// System-wide repository configuration remains a future extension.
//
// Catalog descriptions use page-local CSS-pixel geometry and resolved solid
// paint. Accessors return copies; selection requires no parsing, styling or
// geometry work. Load retains no DOM/CSSOM or mutable intermediate inputs.
// Content preparation, page trials and rendering are separate phases.
// This package does not use Go's html/template or text/template engines.
package template
