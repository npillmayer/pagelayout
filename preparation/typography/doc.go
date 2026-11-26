// Package typography connects pagelayout content to reusable font-backed
// paragraph preparation, measurement, layout and finite-height fitting.
//
// # Services and policy
//
// Construct a [Backend] with [New] and pass it to content.Prepare. [Config]
// borrows an explicit style mapper, a factory for fresh shaping streams and,
// for hyphens:manual, an explicit-marker hyphenator. Font discovery and concrete
// font/shaping implementations belong to the caller. No default service is installed.
//
// Preparation admits uniform none or manual hyphenation per group and preserves
// source style, paint and language/direction metadata. Unsupported policies fail
// preflight before service work. Base paragraph encoding is reused; manual
// replacement alternatives can require lazy discovery and shaping during execution.
//
// # Layout and fitting
//
// [MeasureIntrinsic] returns CSS-pixel width bounds from the matching preparation.
// [Layout] returns a complete subtree at a width. [Fit] enumerates finite-height
// fragments using a reusable content.Fitter, exact checkpoints and explicit FirstFit.
// Consumed atomic interiors remain frozen; suffix interiors can change at a new width.
// [FittingAdapter] supplies the same fitting protocol to pagelayout.NewPagination.
//
// [InspectFragment] recovers typed paragraphs, source coverage and owning-event
// associations from frozen page fragments without execution. Geometry uses CSS
// pixels, while frozen paragraph coordinates use local big points. Apply the
// receiver and paragraph origins once when rendering; do not restack lines.
//
// # Ownership
//
// Serialize session operations and shared borrowed services, including Close.
// Execution failures preserve prior preparations and output. Frozen [Result] and
// [Fragment] values retain no session, trial or preparation factory, and remain
// usable after session closure while borrowed font/rendering resources live.
// Rendering and exporting pages are caller responsibilities.
package typography
