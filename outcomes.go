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

type ErrorKind string

const (
	InvalidRequest   ErrorKind = "invalid-request"
	Unsupported      ErrorKind = "unsupported"
	ExecutionFailure ErrorKind = "execution"
	Cancelled        ErrorKind = "cancelled"
	LimitReached     ErrorKind = "limit"
)

var (
	ErrInvalidRequest = errors.New("invalid pagination request")
	ErrUnsupported    = errors.New("unsupported pagination behavior")
	ErrExecution      = errors.New("pagination execution failure")
	ErrCancelled      = errors.New("pagination cancelled")
	ErrLimit          = errors.New("pagination work limit")
)

type DiagnosticLocation struct {
	Template string
	Frame    template.FrameID
	Channel  string
	Box      content.Box
	Position content.LogicalPosition
}
type PaginationError struct {
	Kind     ErrorKind
	Location DiagnosticLocation
	Cause    error
}

func (e *PaginationError) Error() string {
	return fmt.Sprintf("pagination %s (%s/%d %s): %v", e.Kind, e.Location.Template, e.Location.Frame, e.Location.Channel, e.Cause)
}
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

type PageOutcome uint8

const (
	Constructed PageOutcome = iota + 1
	NoFit
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

func (c *Candidate) Successor() State {
	if c == nil {
		return State{}
	}
	return c.successor
}
func (c *Candidate) Template() string {
	if c == nil {
		return ""
	}
	return c.template
}
func (c *Candidate) Progress() []ChannelProgress {
	if c == nil {
		return nil
	}
	return slices.Clone(c.progress)
}
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

type PageAttemptResult struct {
	outcome      PageOutcome
	candidate    *Candidate
	obstructions []QueueObstruction
}

type ObstructionReason string

const (
	AwaitingOwner   ObstructionReason = "awaiting-owner"
	FutureSpanHead  ObstructionReason = "future-span-head"
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

func (r PageAttemptResult) Obstructions() []QueueObstruction {
	out := slices.Clone(r.obstructions)
	for i := range out {
		out[i].PendingBehind = slices.Clone(out[i].PendingBehind)
	}
	return out
}

func (r PageAttemptResult) Outcome() PageOutcome  { return r.outcome }
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

type SearchOutcome uint8

const (
	SearchComplete SearchOutcome = iota + 1
	SearchInfeasible
	SearchHeuristicExhausted
	SearchCancelled
	SearchLimited
	SearchFailed
)

// Sequence is a completed span, not a partial search path.
type Sequence struct {
	pages []*Candidate
	final State
	loss  Loss
}

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
func (s *Sequence) FinalState() State {
	if s == nil {
		return State{}
	}
	return s.final
}
func (s *Sequence) Loss() Loss {
	if s == nil {
		return Loss{}
	}
	return s.loss
}

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

func (r SearchReport) Stats() SearchStats { return r.stats }

func (r SearchReport) Outcome() SearchOutcome { return r.outcome }
func (r SearchReport) Selected() *Sequence    { return copySequence(r.selected) }

// Incumbent is always explicitly unselected; it must not be committed/shipped.
func (r SearchReport) Incumbent() *Sequence     { return copySequence(r.incumbent) }
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
