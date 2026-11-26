package pagelayout

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// PageConsumer receives one selected frozen page, never execution state or a
// speculative candidate. Calls are synchronous in page order. Retaining the
// handle is safe; borrowed font/rendering resources must remain live separately.
type PageConsumer func(*Page) error

// ErrDelivery identifies consumer failure or cancellation during delivery.
var ErrDelivery = errors.New("page delivery failed")

// DeliveryError preserves the consumer or cancellation cause. Page identifies
// the failing/interrupted call or next undelivered page (zero if none remains).
// Only DeliveryReport.FailedPage identifies a consumer call that returned error.
type DeliveryError struct {
	Page  uint64
	Cause error
}

// Error formats the delivery page number and original cause.
func (e *DeliveryError) Error() string {
	return fmt.Sprintf("page delivery at %d: %v", e.Page, e.Cause)
}

// Unwrap exposes ErrDelivery and the original consumer or cancellation cause.
func (e *DeliveryError) Unwrap() []error { return []error{ErrDelivery, e.Cause} }

// DeliveryReport records calls made by one Shipout invocation. Its zero value
// represents no delivery. It is an observation, not an automatic retry cursor.
type DeliveryReport struct {
	completed    []uint64
	failed, next uint64
	valid        bool
}

// CompletedPages returns page numbers whose consumer calls returned nil, in order.
func (r DeliveryReport) CompletedPages() []uint64 { return slices.Clone(r.completed) }

// FailedPage is the page whose consumer returned an error, or zero.
func (r DeliveryReport) FailedPage() uint64 { return r.failed }

// NextPage is the first page not successfully delivered, or zero if none remains.
func (r DeliveryReport) NextPage() uint64 { return r.next }

// Complete reports that every page call succeeded, even if cancellation was
// observed after the final call. An empty selected sequence is complete too.
func (r DeliveryReport) Complete() bool { return r.valid && r.next == 0 }

// Shipout delivers only a successfully selected span. Incumbents and zero
// sequences are rejected before any consumer call. Delivery needs no live
// pagination/preparation session and never modifies the selected sequence.
// A failed consumer may have partially written its page; no rollback or retry
// is attempted. CompletedPages records successful calls even if cancellation is
// observed immediately afterward, including after the last page.
func (s *Sequence) Shipout(ctx context.Context, consumer PageConsumer) (DeliveryReport, error) {
	if ctx == nil || consumer == nil || s == nil || !s.selected {
		return DeliveryReport{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("context, selected sequence and consumer required"))
	}
	// Snapshot the handle before callbacks, which may overwrite caller handles.
	pages := s.pages
	for _, c := range pages {
		if c == nil || c.page == nil {
			return DeliveryReport{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("selected page output required"))
		}
	}
	out := DeliveryReport{valid: true}
	if len(pages) > 0 {
		out.next = pages[0].page.Number()
	}
	for i, c := range pages {
		if err := ctx.Err(); err != nil {
			return out, &DeliveryError{Page: out.next, Cause: err}
		}
		if err := consumer(c.Page()); err != nil {
			out.failed = out.next
			return out, &DeliveryError{Page: out.failed, Cause: err}
		}
		out.completed = append(out.completed, out.next)
		out.next = 0
		if i+1 < len(pages) {
			out.next = pages[i+1].page.Number()
		}
		if err := ctx.Err(); err != nil {
			return out, &DeliveryError{Page: c.page.Number(), Cause: err}
		}
	}
	if err := ctx.Err(); err != nil {
		return out, &DeliveryError{Cause: err}
	}
	return out, nil
}
