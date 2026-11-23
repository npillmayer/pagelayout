// Package content is the content-loading pipeline for template-based page layout.
//
// CONT-01 implements private source parsing, CSS inspection and styling in the
// original document context. CONT-02 checks the authored/effective content
// profile and projects detached source records and typed, unsized render nodes.
// CONT-03 validates route ownership and partitions channel branches and residual
// material before normalization. The public material loader and inspection API
// will follow normalization; no partial source model is exported here.
// No resources are fetched, scripts executed, boxes built or fonts consulted.
package content
