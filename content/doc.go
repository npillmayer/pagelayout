// Package content is the content-loading pipeline for template-based page layout.
//
// Load styles HTML in its original source context, checks the content profile,
// partitions named flow branches and residual material, and normalizes each tree
// through css-box-layout. It publishes an immutable Document only after every
// branch succeeds. Inspection returns copies; Box handles retain private trees,
// source text and styles, and qualify identity by snapshot and tree.
//
// The initial register bank contains whole branch-root Boxes in source order.
// Deriving another bank leaves the Document unchanged. Residual() aliases the default main tree; valid main directives stay in source
// order there. SourceEvents exposes detached IDs, anchors and forced breaks. Groups include empty paragraphs
// and nested inline-block interiors, with source owner context preserved.
//
// Loading fetches no resources, executes no scripts and consults no fonts. Boxes
// remain unsized; preparation, used geometry and continuation belong to later
// phases. Readers remain caller-owned. LoadError exposes ordered, source-aware
// diagnostics and preserves original causes.
//
// Prepare extracts detached GroupInput records and invokes an explicit Preparer
// after whole-document preflight. Success publishes immutable PreparedSnapshot
// metadata plus a separate PreparationSession owning backend handles. Banks and
// source boxes stay unchanged. Session.WithGroup validates qualified identities;
// callers serialize execution and Close. Cleanup releases owned handles in
// reverse order and preserves causes through PreparationError. The optional
// preparation/typography package supplies the typography backend. MeasureIntrinsic
// reuses existing handles through a neutral measurement callback while keeping
// trees private. Layout reserves the same pair for CSS used sizing and complete
// subtree flow, with separate callback/result state per invocation. It publishes
// copied CSS geometry only after success; the optional bridge owns frozen
// paragraph output. NewFitter/Fit reserve the same session for finite-height
// structural alternatives and exact continuations; each alternative pairs a
// frozen render fragment with its remainder. Failed/interrupted calls publish
// no partial candidate set. Page selection and bank commits remain separate.
//
// Executable examples demonstrate the sketch input, a channel/branch/group
// report, persistent banks, additional stylesheets and structured failures.
// The content README documents the handoff to the later preparation bridge;
// source-child inspection must not be mistaken for normalized group extraction.
// EventPositions maps annotations using builder provenance and extraction ranges;
// Prepare carries copied GroupInput.Events without inserting text. The root
// pagelayout.Plan owns ownership validation and span construction.
package content
