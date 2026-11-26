// Package pagelayout composes styled content into template-based pages.
// It validates source ownership, fits prepared material into compiled frames,
// selects complete pagespan sequences and delivers their frozen output.
//
// # Workflow
//
// Load templates with the template package and a separate source document with
// the content package. Content keeps its original styles: placing it in a frame
// supplies receiving geometry, not a new CSS cascade. Prepare the source with
// content.Prepare and an explicit backend, such as preparation/typography.
//
// [Plan] validates owning links and forced breaks without fitting or advancing
// material. [NewPagination] binds the plan, catalog, matching preparation/session
// and fitting backend. Call [Pagination.Paginate] from [Pagination.InitialState]
// to select successive complete spans. Check [DocumentResult.Complete] as well
// as the error: infeasibility can be a normal, incomplete result.
//
// [Pagination.PageAttempt] fits one explicit template. [Pagination.PaginationSearch]
// selects one complete current span; [Pagination.AdvanceSpan] enters the next span
// from its exact drained state. States and register banks are persistent values,
// so rejected alternatives cannot consume material from another branch.
//
// # Ownership and delivery
//
// Residual content belongs to main. Explicit data-flow-anchor links activate
// secondary branches when their associated material is placed. Registers retain
// authoring order; all transitively related targets drain before a span closes.
// Forced break-before variants impose the same page boundary without parity rules.
//
// [Sequence.Shipout] separately delivers only successfully selected frozen pages.
// A consumer error preserves the selection and reports completed calls; there is
// no rollback of external effects or automatic retry. Interrupted search may
// expose a complete unselected incumbent, which Shipout rejects.
//
// Serialize pagination and use of its borrowed preparation session and services.
// Closing preparation prevents execution but leaves frozen pages inspectable and
// renderable while their borrowed font resources remain valid. Page/frame geometry
// uses CSS pixels; rendering and export are supplied by the caller.
//
// # Search policy
//
// Search retains 32 unfinished paths by default after each complete depth round.
// It ranks accumulated loss, then lexicographic catalog ordinals, and keeps the
// best completion separately. Width truncation is heuristic and reported explicitly.
// Optimality without truncation is relative to eligibility and the single locally
// selected page per template. Scoring coefficients and numerical work caps are
// configurable working hypotheses. No state merging or automatic widening occurs.
package pagelayout
