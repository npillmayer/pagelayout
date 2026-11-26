// Package pagelayout compiles source ownership and span plans for template-based
// page composition. Plan validates explicit owning links and main-flow breaks,
// preserves register order, and returns immutable source-qualified inspection.
//
// Load separate template and content documents with the template and content
// subpackages. Preparation, width-dependent layout and source planning are
// available. NewPagination binds immutable state to a borrowed preparation session
// and fitting backend. PageAttempt constructs one main-only page, returning its
// frozen output, checked loss and exact successor together. Planning itself
// neither consumes banks nor activates targets or creates pages. Multi-channel
// construction, template-sequence search and span delivery remain separate work.
package pagelayout
