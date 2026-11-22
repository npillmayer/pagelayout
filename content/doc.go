// Package content is the content-loading pipeline for template-based page layout.
//
// CONT-01 implements private source parsing, CSS inspection and styling in the
// original document context. The public material loader and inspection API will
// follow normalization and routing; no partial source model is exported here.
// No resources are fetched, scripts executed, boxes built or fonts consulted.
package content
