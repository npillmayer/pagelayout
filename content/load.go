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

// loadSource is the private CONT-01 boundary. Its result is styled source, not a
// Document or validated layout material. Every invocation owns fresh DOM/CSSOM
// state. A failure returns no partial source. The caller retains reader ownership.
func loadSource(r io.Reader, opts LoadOptions) (*styledSource, error) {
	source, err := parseSource(r, opts.SourceName)
	if err != nil {
		return nil, err
	}
	return styleSource(source, opts.Stylesheets)
}
