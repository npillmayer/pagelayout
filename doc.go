// Package pagelayout compiles source ownership and span plans for template-based
// page composition. Plan validates explicit owning links and main-flow breaks,
// preserves register order, and returns immutable source-qualified inspection.
//
// Load separate template and content documents with the template and content
// subpackages. Preparation, width-dependent layout and source planning are
// available; finite-height page fitting and pagination execution are separate
// work. Planning neither consumes banks nor activates targets or creates pages.
package pagelayout
