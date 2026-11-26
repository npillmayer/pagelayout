package content

import "io"

// Stylesheet supplies additional author CSS as text. SourceName labels its
// diagnostics; it is not a path or URL to load. An empty name gets a generated
// label. No resource I/O is implied by any of these values.
type Stylesheet struct {
	SourceName string
	CSS        string
}

// LoadOptions supplies source context for content loading. The zero value is valid.
type LoadOptions struct {
	SourceName string
	// Stylesheets follow embedded head sheets in supplied order. Normal author
	// specificity, inline precedence and importance still apply.
	Stylesheets []Stylesheet
}

// Load parses, styles, routes and normalizes a complete content document. It
// publishes no partial Document on failure. The caller owns r and supplies any
// additional CSS as text; loading neither fetches resources nor opens fonts.
func Load(r io.Reader, opts LoadOptions) (*Document, error) {
	p, err := loadPartition(r, opts)
	if err != nil {
		return nil, err
	}
	return normalizePartition(p)
}

// loadSource parses and styles the original document. Its result is source, not a
// Document or validated layout material. Every invocation owns fresh DOM/CSSOM
// state. A failure returns no partial source. The caller retains reader ownership.
func loadSource(r io.Reader, opts LoadOptions) (*styledSource, error) {
	source, err := parseSource(r, opts.SourceName)
	if err != nil {
		return nil, err
	}
	return styleSource(source, opts.Stylesheets)
}
