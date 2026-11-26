// Package pagelayout compiles source ownership and span plans for template-based
// page composition. Plan validates explicit owning links and main-flow breaks,
// preserves register order, and returns immutable source-qualified inspection.
//
// Load separate template and content documents with the template and content
// subpackages. Preparation, width-dependent layout and source planning are
// available. NewPagination binds immutable state to a borrowed preparation session
// and fitting backend. PageAttempt constructs one page across flow channels,
// returning frozen output, checked loss and exact successor together. Selected
// owning material activates targets; FIFO queues and transitive draining are
// preserved. Planning itself neither consumes banks nor activates targets or
// creates pages. PaginationSearch selects one complete pagespan by deterministic
// beam search and reports truncation/interruption explicitly. Span transitions
// and document delivery remain separate work.
package pagelayout
