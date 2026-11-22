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
// Geometry resolution is not implemented. The public loading API will be
// introduced when complete styled, sized catalogs can be returned.
//
// The intended compiled catalog is read-only and uses page-local CSS-pixel
// geometry. Content preparation, page trials and rendering are separate phases.
// This package does not use Go's html/template or text/template engines.
package template
