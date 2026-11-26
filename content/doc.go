// Package content loads styled HTML, prepares immutable flow material and fits
// fragments through explicit backend contracts.
//
// # Loading and inspection
//
// [Load] styles the complete source in its original ancestry, validates the
// supported profile, partitions nonnested block branches by flow-into, and
// normalizes each tree. Residual material belongs to main; a valid explicit
// flow-into:main is redundant. Nested routes remain errors. Publication is atomic.
// Readers remain caller-owned; no resources or fonts are fetched and no scripts
// are executed. [LoadError] preserves ordered source/CSS diagnostics and causes.
//
// [Document.Bank] holds whole branch-root [Box] values in source order. Derived
// banks do not mutate the document. Box identity includes snapshot and tree;
// inspection returns copied source, style and structural descriptions. Boxes keep
// source values unsized until receiving constraints are supplied.
//
// [Document.SourceEvents] retains IDs, owning annotations and forced breaks.
// [Document.EventPositions] maps them through normalization and processed text
// without fonts. Logical positions support inspection/comparison, not resumption.
// The root pagelayout package owns ownership validation, activation and span planning.
//
// # Preparation and execution
//
// [Prepare] extracts [GroupInput], preflights the entire document, then prepares
// every group, including empty groups and inline-block interiors. It publishes
// immutable [PreparedSnapshot] metadata and a separate [PreparationSession]
// owning executable handles. Source boxes and banks are unchanged. Failure
// releases acquired handles in reverse order and publishes neither result.
//
// Use the matching source/snapshot/session for [MeasureIntrinsic], [Layout] and
// [Fit]. These operations reserve the session, borrow backend handles only for
// synchronous callbacks, and publish complete results atomically. The optional
// preparation/typography package supplies font-backed implementations and typed
// frozen paragraph inspection. Custom backends implement the neutral interfaces.
//
// # Fragments and lifetime
//
// [NewFitter] creates a reusable structural fitter. [Fit] takes finite CSS-pixel
// constraints and returns alternatives pairing frozen output with exact opaque
// continuations. Reuse the same fitter for changed-width continuation. Failed or
// interrupted enumeration returns no partial candidate set. [ComposeFragments]
// places branch fragments in one receiver with normal margin collapse.
//
// Serialize all execution and [PreparationSession.Close], including shared
// borrowed services. Close releases owned handles, never borrowed font resources.
// Copied metadata and frozen output survive closure while their rendering resources
// remain valid. Page selection, channel advancement and output delivery belong to
// the root pagelayout package.
package content
