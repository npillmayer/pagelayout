// Command pgtmpl checks, finds and previews HTML/CSS page-template collections.
//
// Usage:
//
//	pgtmpl load <html> [--css FILE]
//	pgtmpl serve <html> [--css FILE] [--addr ADDRESS]
//	pgtmpl find <name>
//
// Load and serve read an explicit local path; find uses template.LoadFile search
// and validation. Results and diagnostics go to stdout. Loading failures exit
// with status 1. Additional CSS follows normal author cascade rules.
//
// Serve validates and renders the entire catalog before listening, by default
// on 127.0.0.1:8080. Its browser viewer shows one template at a time from compiled
// geometry and paint, with navigation and optional channel-colored outlines.
// The preview is a startup snapshot; it does not paginate content or live reload.
package main
