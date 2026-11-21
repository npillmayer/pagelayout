// Package template owns HTML/CSS page-template loading for pagelayout.
//
// Page definitions are stored in an HTML collection as direct body children
// named <template data-type="page" id="...">. Each contains one page DIV whose
// direct DIV children are empty receiving frames. Source order determines
// definition and frame order; classes are authoring conventions, not role markers.
//
// The implemented TPL-01 stage privately parses and validates collection
// structure, retaining source identities for subsequent styling. HTML parser
// recovery applies: this stage validates the resulting tree, not HTML syntax
// conformance. It neither parses CSS nor computes geometry. The public loading
// API will be introduced when complete styled, sized catalogs can be returned.
//
// The intended compiled catalog is read-only and uses page-local CSS-pixel
// geometry. Content preparation, page trials and rendering are separate phases.
// This package does not use Go's html/template or text/template engines.
package template
