package template

import "io"

// Stylesheet is an additional author stylesheet. CSS is its text; SourceName
// labels diagnostics (for example, a local filename), not a URL to fetch.
// An empty SourceName receives a generated label. No external resources load.
type Stylesheet struct {
	SourceName string
	CSS        string
}

// LoadOptions supplies collection loading context. The zero value is valid.
type LoadOptions struct {
	// SourceName labels diagnostics and source references. It does not enable
	// external resource loading or define a stylesheet base URL.
	SourceName string
	// Stylesheets are applied in order after the collection's head stylesheets.
	// Normal author cascade rules, including specificity, inline styles and
	// !important, apply. Every declaration is checked against the CSS profile,
	// including nonmatching/losing declarations. Geometry is compiled afterward.
	Stylesheets []Stylesheet
}

// Load reads an HTML page-template collection, styles each definition in
// isolation, validates the initial CSS profile and compiles all geometry.
// It returns a complete immutable catalog, or nil with a *LoadError. A failure
// never publishes a partial catalog or changes previously returned catalogs.
// The reader remains owned by the caller and is not closed.
//
// Frames need positive receiving areas, must fit within the page, and must not
// overlap in their border boxes. Touching edges are allowed within the documented
// numerical tolerance. Parsing follows HTML recovery; this is not an HTML
// conformance checker. No scripts, external resources or content layout run.
func Load(r io.Reader, opts LoadOptions) (*Catalog, error) {
	source, err := parseTemplateCollection(r, opts.SourceName)
	if err != nil {
		return nil, err
	}
	styled, err := styleTemplateCollection(source, opts.Stylesheets...)
	if err != nil {
		return nil, err
	}
	validated, err := validateTemplateCollection(styled)
	if err != nil {
		return nil, err
	}
	return compileTemplates(validated)
}
