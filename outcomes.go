package pagelayout

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/npillmayer/css-box-layout/registers"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

// ErrorKind classifies invalid requests, unsupported behavior and operational failures.
type ErrorKind string

// Error categories are also exposed through the corresponding Err sentinels.
const (
	InvalidRequest   ErrorKind = "invalid-request"
	Unsupported      ErrorKind = "unsupported"
	ExecutionFailure ErrorKind = "execution"
	Cancelled        ErrorKind = "cancelled"
	LimitReached     ErrorKind = "limit"
)

// Pagination category sentinels support errors.Is through PaginationError.
var (
	ErrInvalidRequest = errors.New("invalid pagination request")
	ErrUnsupported    = errors.New("unsupported pagination behavior")
	ErrExecution      = errors.New("pagination execution failure")
	ErrCancelled      = errors.New("pagination cancelled")
	ErrLimit          = errors.New("pagination work limit")
)

// DiagnosticLocation identifies the template, receiver and qualified source position
// of a failure. Fields without a known location have their zero values.
type DiagnosticLocation struct {
	Template string
	Frame    template.FrameID
	Channel  string
	Box      content.Box
	Position content.LogicalPosition
}

// PaginationError preserves a failure category, available location and original cause.
// Use errors.Is for category sentinels and errors.As for structured details.
type PaginationError struct {
	Kind     ErrorKind
	Location DiagnosticLocation
	Cause    error
}

// Error formats the failure category, available location and original cause.
func (e *PaginationError) Error() string {
	return fmt.Sprintf("pagination %s (%s/%d %s): %v", e.Kind, e.Location.Template, e.Location.Frame, e.Location.Channel, e.Cause)
}

// Unwrap exposes the category sentinel and original cause, when present.
func (e *PaginationError) Unwrap() []error {
	tag := ErrExecution
	switch e.Kind {
	case InvalidRequest:
		tag = ErrInvalidRequest
	case Unsupported:
		tag = ErrUnsupported
	case Cancelled:
		tag = ErrCancelled
	case LimitReached:
		tag = ErrLimit
	}
	if e.Cause == nil {
		return []error{tag}
	}
	return []error{tag, e.Cause}
}
func paginationError(kind ErrorKind, where DiagnosticLocation, cause error) error {
	return &PaginationError{kind, where, cause}
}

// PageOutcome distinguishes a constructed page, an exhausted non-fit and an already
// drained input span. Zero is not a successful outcome; check the returned error first.
type PageOutcome uint8

const (
	// Constructed has a frozen page, assessment and exact successor.
	Constructed PageOutcome = iota + 1
	// NoFit means this template has no admissible page after complete enumeration.
	NoFit
	// SpanComplete means the input span is already drained; no page is produced.
	SpanComplete
)

// Candidate pairs an immutable page with its exact successor and loss.
type Candidate struct {
	// Set only from factual placement, never metadata/cursor advancement alone.
	placedMaterial bool
	origin         State
	successor      State
	template       string
	score          Score
	progress       []ChannelProgress
	page           *Page
}

// Page returns frozen rendering data without retaining resumption state.
func (c *Candidate) Page() *Page {
	if c == nil {
		return nil
	}
	if c.page == nil {
		return nil
	}
	p := *c.page
	return &p
}

// Origin returns the exact immutable input version for this attempt.
func (c *Candidate) Origin() State {
	if c == nil {
		return State{}
	}
	return c.origin
}

// Successor returns the exact immutable state after all channel advances on this page.
func (c *Candidate) Successor() State {
	if c == nil {
		return State{}
	}
	return c.successor
}

// Template returns the catalog ID used to construct this page.
func (c *Candidate) Template() string {
	if c == nil {
		return ""
	}
	return c.template
}

// Progress returns a copied main-first channel progress vector for local comparison.
func (c *Candidate) Progress() []ChannelProgress {
	if c == nil {
		return nil
	}
	return slices.Clone(c.progress)
}

// Score returns copied assessment facts and checked component costs.
func (c *Candidate) Score() Score {
	if c == nil {
		return Score{}
	}
	out := c.score
	out.Participating = slices.Clone(out.Participating)
	out.Exempt = slices.Clone(out.Exempt)
	out.Facts = cloneAssessment(out.Facts)
	return out
}

// PageAttemptResult is the atomic result of one template attempt. Only Constructed
// has a Candidate; errors return no usable page or successor.
type PageAttemptResult struct {
	outcome      PageOutcome
	candidate    *Candidate
	obstructions []QueueObstruction
}

// ObstructionReason describes why an entry-state queue head cannot currently be placed.
type ObstructionReason string

const (
	// AwaitingOwner means the queue head has no placed owning anchor.
	AwaitingOwner ObstructionReason = "awaiting-owner"
	// FutureSpanHead means the FIFO head belongs to a later span.
	FutureSpanHead ObstructionReason = "future-span-head"
	// MissingReceiver means this template has no frame for the channel.
	MissingReceiver ObstructionReason = "missing-receiver"
)

// QueueObstruction describes an entry-state reason that a channel cannot drain.
// PendingBehind retains authored order. These facts are not a search-infeasibility proof.
type QueueObstruction struct {
	Channel       string
	Head          BranchID
	Reason        ObstructionReason
	PendingBehind []BranchID
}

// Obstructions returns copied queue-head facts for NoFit. They describe this entry
// state and template, not proof that all templates or search paths fail.
func (r PageAttemptResult) Obstructions() []QueueObstruction {
	out := slices.Clone(r.obstructions)
	for i := range out {
		out[i].PendingBehind = slices.Clone(out[i].PendingBehind)
	}
	return out
}

// Outcome returns the normal attempt outcome; zero accompanies an invalid or failed result.
func (r PageAttemptResult) Outcome() PageOutcome { return r.outcome }

// Candidate returns the constructed page/state pair, or nil for every other outcome.
func (r PageAttemptResult) Candidate() *Candidate { return r.candidate }

// finishAttempt is the atomic boundary for page enumeration. An error or
// stopped enumeration cannot expose a provisional best or its successor.
func finishAttempt(input State, exhausted bool, best *Candidate, err error) (PageAttemptResult, error) {
	if err != nil {
		return PageAttemptResult{}, executionError(err)
	}
	if !input.Valid() {
		return PageAttemptResult{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("invalid input state"))
	}
	if input.SpanComplete() {
		return PageAttemptResult{outcome: SpanComplete}, nil
	}
	if !exhausted {
		return PageAttemptResult{}, paginationError(LimitReached, DiagnosticLocation{}, fmt.Errorf("enumeration stopped before exhaustion"))
	}
	if best == nil {
		return PageAttemptResult{outcome: NoFit}, nil
	}
	if !best.placedMaterial || best.template == "" || best.origin.data != input.data || !best.successor.Valid() || best.successor.data.context != input.data.context || best.successor.data.page != input.data.page+1 || best.score.Total.blocked || best.score.Total.units < input.data.context.policy.Page || !materialAdvanced(input, best.successor) {
		return PageAttemptResult{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("incomplete or foreign candidate"))
	}
	var sum Loss
	for _, part := range []Loss{best.score.Page, best.score.Underfill, best.score.Split, best.score.Carry, best.score.MissingMain} {
		var err error
		sum, err = sum.Add(part)
		if err != nil {
			return PageAttemptResult{}, executionError(err)
		}
	}
	if sum.Compare(best.score.Total) != 0 || best.score.Page.units != input.data.context.policy.Page {
		return PageAttemptResult{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("inconsistent candidate loss"))
	}
	frozen := freezeCandidate(best)
	frozen.progress = stateProgress(best.successor)
	return PageAttemptResult{outcome: Constructed, candidate: frozen}, nil
}

// SearchOutcome distinguishes selection, policy infeasibility, heuristic exhaustion
// and interruption. Zero is not a completed search; inspect the accompanying error.
type SearchOutcome uint8

const (
	// SearchComplete selects a complete span sequence, possibly after heuristic cuts.
	SearchComplete SearchOutcome = iota + 1
	// SearchInfeasible exhausts the configured search space without a completion or heuristic cuts.
	SearchInfeasible
	// SearchHeuristicExhausted finds no retained completion after heuristic cuts.
	SearchHeuristicExhausted
	// SearchCancelled reports explicit context cancellation; any incumbent is unselected.
	SearchCancelled
	// SearchLimited reports a work cap; any incumbent is unselected.
	SearchLimited
	// SearchFailed reports an invalid request or execution failure without selection.
	SearchFailed
)

// Sequence is a completed span, not a partial search path.
type Sequence struct {
	pages    []*Candidate
	final    State
	loss     Loss
	selected bool
}

// Pages returns independent candidate handles in page order, including for an
// unselected incumbent. Inspection does not authorize delivery.
func (s *Sequence) Pages() []*Candidate {
	if s == nil {
		return nil
	}
	pages := make([]*Candidate, len(s.pages))
	for i, p := range s.pages {
		pages[i] = freezeCandidate(p)
	}
	return pages
}

// FinalState returns the exact drained boundary state, before advancing to another span.
func (s *Sequence) FinalState() State {
	if s == nil {
		return State{}
	}
	return s.final
}

// Loss returns the checked sum of the sequence's page losses.
func (s *Sequence) Loss() Loss {
	if s == nil {
		return Loss{}
	}
	return s.loss
}

// SearchReport separates successful selection from a complete but unselected incumbent.
// Only SearchComplete has Selected output; interrupted output is inspectable through
// Incumbent and cannot be delivered by Sequence.Shipout.
type SearchReport struct {
	outcome             SearchOutcome
	selected, incumbent *Sequence
	heuristic           bool
	stats               SearchStats
}

// SearchStats counts work in this invocation, including an interrupted round's
// attempts. Rounds counts only fully evaluated and pruned depth rounds.
// Complete candidates never occupy beam slots. SafePruned and WidthPruned count
// unfinished paths, not unique logical states; no state merging is performed.
type SearchStats struct {
	Rounds, Attempts, NoFit, Completions, SafePruned, WidthPruned uint64
}

// Stats returns work counts for this search invocation.
func (r SearchReport) Stats() SearchStats { return r.stats }

// Outcome returns the search completion or failure classification.
func (r SearchReport) Outcome() SearchOutcome { return r.outcome }

// Selected returns an independent sequence handle on SearchComplete, otherwise nil.
func (r SearchReport) Selected() *Sequence { return copySequence(r.selected) }

// Incumbent is always explicitly unselected; it must not be committed/shipped.
func (r SearchReport) Incumbent() *Sequence { return copySequence(r.incumbent) }

// HeuristicTruncated reports whether the beam width discarded any unfinished path.
// Safe lower-bound pruning alone does not set it.
func (r SearchReport) HeuristicTruncated() bool { return r.heuristic }

func copySequence(s *Sequence) *Sequence {
	if s == nil {
		return nil
	}
	copy := *s
	return &copy
}
func finishSearch(outcome SearchOutcome, complete *Sequence, heuristic bool, err error) (SearchReport, error) {
	if err != nil {
		err = executionError(err)
	}
	invalid := func() (SearchReport, error) {
		return SearchReport{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("inconsistent search report"))
	}
	if complete != nil && (!complete.final.SpanComplete() || complete.loss.blocked) {
		return invalid()
	}
	if complete != nil {
		var total Loss
		for i, page := range complete.pages {
			if page == nil || !page.origin.Valid() || !page.successor.Valid() || page.origin.data.context != complete.final.data.context || page.origin.data.span != complete.final.data.span || page.successor.data.context != complete.final.data.context || i > 0 && page.origin.data != complete.pages[i-1].successor.data {
				return invalid()
			}
			if _, e := finishAttempt(page.origin, true, page, nil); e != nil {
				return invalid()
			}
			var e error
			total, e = total.Add(page.score.Total)
			if e != nil {
				return invalid()
			}
		}
		if total.Compare(complete.loss) != 0 || len(complete.pages) > 0 && complete.pages[len(complete.pages)-1].successor.data != complete.final.data {
			return invalid()
		}
		copy := *complete
		copy.pages = make([]*Candidate, len(complete.pages))
		for i, c := range complete.pages {
			copy.pages[i] = freezeCandidate(c)
		}
		complete = &copy
	}
	report := SearchReport{outcome: outcome, heuristic: heuristic}
	switch outcome {
	case SearchComplete:
		if err != nil || complete == nil {
			return invalid()
		}
		complete.selected = true
		report.selected = complete
	case SearchInfeasible:
		if heuristic || err != nil || complete != nil {
			return invalid()
		}
	case SearchHeuristicExhausted:
		if !heuristic || err != nil || complete != nil {
			return invalid()
		}
	case SearchCancelled, SearchLimited, SearchFailed:
		if outcome == SearchCancelled && !errors.Is(err, ErrCancelled) || outcome == SearchLimited && !errors.Is(err, ErrLimit) || outcome == SearchFailed && (errors.Is(err, ErrCancelled) || errors.Is(err, ErrLimit)) {
			return invalid()
		}
		if err == nil {
			return invalid()
		}
		if complete != nil {
			complete.selected = false
		}
		report.incumbent = complete
	default:
		return invalid()
	}
	return report, err
}

// workBudget counts completed/started work units including partial arrangements.
// No timeouts or retries are synthesized; callers check after synchronous calls.
type workBudget struct{ used, limit uint64 }

func (b *workBudget) before(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return paginationError(Cancelled, DiagnosticLocation{}, err)
	}
	if b.used >= b.limit {
		return paginationError(LimitReached, DiagnosticLocation{}, fmt.Errorf("work limit %d", b.limit))
	}
	b.used++
	return nil
}
func (b *workBudget) after(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return paginationError(Cancelled, DiagnosticLocation{}, err)
	}
	return nil
}

// Freeze all inspection slices at publication, not merely when inspected.
func freezeCandidate(c *Candidate) *Candidate {
	if c == nil {
		return nil
	}
	out := *c
	out.score = c.Score()
	out.progress = c.Progress()
	return &out
}
func materialAdvanced(from, to State) bool {
	for _, channel := range from.data.context.channels {
		a, _ := from.data.bank.Lookup(registers.RegisterID(channel))
		b, _ := to.data.bank.Lookup(registers.RegisterID(channel))
		if b.Len() < a.Len() {
			return true
		}
		cursor, ok := to.data.cursors[channel]
		if !ok {
			continue
		}
		old, had := from.data.cursors[channel]
		if !had {
			return true
		}
		if v, ok := cursor.position.Compare(old.position); ok && v > 0 {
			return true
		}
	}
	return false
}

func executionError(err error) error {
	var typed *PaginationError
	if errors.As(err, &typed) {
		return err
	}
	if errors.Is(err, ErrLimit) {
		return paginationError(LimitReached, DiagnosticLocation{}, err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return paginationError(Cancelled, DiagnosticLocation{}, err)
	}
	return paginationError(ExecutionFailure, DiagnosticLocation{}, err)
}
func stateProgress(s State) []ChannelProgress {
	var result []ChannelProgress
	for _, channel := range s.data.context.channels {
		reg, _ := s.data.bank.Lookup(registers.RegisterID(channel))
		completed := len(s.data.context.plan.document.Branches(channel)) - reg.Len()
		result = append(result, ChannelProgress{channel, uint64(completed), s.data.cursors[channel].position})
	}
	return result
}
