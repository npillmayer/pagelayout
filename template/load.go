package template

import "io"

// LoadOptions supplies collection loading context. The zero value is valid.
type LoadOptions struct {
	// SourceName labels diagnostics and source references. It does not enable
	// external resource loading or define a stylesheet base URL.
	SourceName string
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
	styled, err := styleTemplateCollection(source)
	if err != nil {
		return nil, err
	}
	validated, err := validateTemplateCollection(styled)
	if err != nil {
		return nil, err
	}
	return compileTemplates(validated)
}
