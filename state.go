package pagelayout

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/npillmayer/css-box-layout/registers"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

// WorkLimits are provisional configurable work-unit caps, never deadlines.
type WorkLimits struct{ LocalSteps, SpanAttempts, DepthRounds uint64 }

// DefaultWorkLimits returns working caps of 100,000 local steps, 1,000,000 page
// attempts per span and 10,000 depth rounds per span. These are work units, not timeouts.
func DefaultWorkLimits() WorkLimits { return WorkLimits{100_000, 1_000_000, 10_000} }
func (l WorkLimits) valid() bool    { return l.LocalSteps > 0 && l.SpanAttempts > 0 && l.DepthRounds > 0 }

// PaginationConfig binds immutable inputs to a borrowed serialized session.
// Nil Policy/Limits select defaults; BeamWidth zero selects the settled 32.
// Fitter is required for page construction; nil permits inspection and search
// through complete or certified empty spans. Eligible is nil for all templates.
type PaginationConfig struct {
	Plan      *SpanPlan
	Catalog   *template.Catalog
	Prepared  *content.PreparedSnapshot
	Session   *content.PreparationSession
	Policy    *LossPolicy
	Limits    *WorkLimits
	BeamWidth int
	Fitter    content.GroupFitter
	Eligible  TemplateEligibility
}
type stateContext struct {
	plan        *SpanPlan
	source      content.SourceID
	preparation content.PreparationID
	policy      LossPolicy
	limits      WorkLimits
	beam        int
	channels    []string
	branches    map[content.Box]PlanBranch
}

// Pagination owns execution access, not a mutable current position. Callers
// serialize use and keep borrowed services live; it never closes the session.
type Pagination struct {
	context  *stateContext
	catalog  template.Catalog
	prepared *content.PreparedSnapshot
	session  *content.PreparationSession
	backend  content.GroupFitter
	fitters  map[content.Box]*content.Fitter
	busy     bool
	eligible TemplateEligibility
}

// NewPagination binds a matching source plan, catalog and preparation/session to
// fixed policy, limits and eligibility. It copies configuration values, borrows
// the session/backend, and performs no font or fitting work. Invalid inputs return nil.
func NewPagination(config PaginationConfig) (*Pagination, error) {
	invalid := func(cause error) (*Pagination, error) {
		return nil, paginationError(InvalidRequest, DiagnosticLocation{}, cause)
	}
	if config.Plan == nil || config.Prepared == nil || !config.Prepared.ID().Valid() || !config.Plan.Matches(config.Prepared.Document()) {
		return invalid(content.ErrInvalidPreparation)
	}
	if err := config.Session.Check(config.Prepared); err != nil {
		return invalid(err)
	}
	if config.Catalog == nil || len(config.Catalog.IDs()) == 0 {
		return invalid(fmt.Errorf("compiled catalog required"))
	}
	policy := DefaultLossPolicy()
	if config.Policy != nil {
		policy = *config.Policy
	}
	if err := policy.Validate(); err != nil {
		return invalid(err)
	}
	limits := DefaultWorkLimits()
	if config.Limits != nil {
		limits = *config.Limits
	}
	if !limits.valid() {
		return invalid(fmt.Errorf("positive work limits required"))
	}
	beam := config.BeamWidth
	if beam == 0 {
		beam = 32
	}
	if beam < 1 {
		return invalid(fmt.Errorf("positive beam width required"))
	}
	plan := *config.Plan
	prepared := *config.Prepared
	c := &stateContext{plan: &plan, source: config.Plan.document.SourceID(), preparation: prepared.ID(), policy: policy, limits: limits, beam: beam, channels: config.Plan.document.Channels(), branches: map[content.Box]PlanBranch{}}
	for _, b := range config.Plan.branches {
		if !prepared.Covers(b.Material.Root) {
			return invalid(content.ErrInvalidPreparation)
		}
		c.branches[b.Material.Root] = b
	}
	return &Pagination{context: c, catalog: *config.Catalog, prepared: &prepared, session: config.Session, backend: config.Fitter, fitters: map[content.Box]*content.Fitter{}, eligible: config.Eligible}, nil
}

// Policy returns the run's scoring coefficients by value.
func (r *Pagination) Policy() LossPolicy {
	if r == nil || r.context == nil {
		return LossPolicy{}
	}
	return r.context.policy
}

// Limits returns the run's fixed execution caps by value.
func (r *Pagination) Limits() WorkLimits {
	if r == nil || r.context == nil {
		return WorkLimits{}
	}
	return r.context.limits
}

// BeamWidth returns the maximum unfinished paths retained after a search round.
func (r *Pagination) BeamWidth() int {
	if r == nil || r.context == nil {
		return 0
	}
	return r.context.beam
}

// headContinuation identifies the unchanged head and its logical position.
// Exact fitter checkpoints are held separately, never inferred from position.
type headContinuation struct {
	head     content.Box
	position content.LogicalPosition
}
type stateData struct {
	context   *stateContext
	bank      registers.RegisterBank[content.Box]
	cursors   map[string]headContinuation
	activated map[BranchID]bool
	span      int
	page      uint64
	fits      map[string]fitCursor
}

type fitCursor struct {
	continuation content.FitContinuation
	end          content.FitPosition
}

// State is an immutable context-qualified value. Its zero value is invalid.
// It retains no session, backend, mutable fitter scratch or source node pointer.
type State struct{ data *stateData }

// HeadInfo describes a channel's unchanged queue head and optional logical cursor.
// Position is inspection data; the state retains resumption authority separately.
type HeadInfo struct {
	Channel         string
	Branch          BranchID
	Box             content.Box
	HasContinuation bool
	Position        content.LogicalPosition
}

// StateInfo is a copied inspection snapshot. Heads follow channel order; Activated
// and Pending contain source-order branch IDs. Span and NextPage are one-based.
type StateInfo struct {
	Source             content.SourceID
	Preparation        content.PreparationID
	Span               int
	NextPage           uint64
	Heads              []HeadInfo
	Activated, Pending []BranchID
}

// Valid reports whether the state has a context identity. Use Pagination.Validate
// to check ownership, bank invariants and availability of the execution session.
func (s State) Valid() bool { return s.data != nil && s.data.context != nil }

// Bank returns the persistent bank version. Deriving another bank does not change
// the state or its exact continuations.
func (s State) Bank() registers.RegisterBank[content.Box] {
	if !s.Valid() {
		return registers.RegisterBank[content.Box]{}
	}
	return s.data.bank
}

// Info returns copied heads, activations and pending-target lists for inspection.
func (s State) Info() StateInfo {
	if !s.Valid() {
		return StateInfo{}
	}
	d, c := s.data, s.data.context
	out := StateInfo{Source: c.source, Preparation: c.preparation, Span: d.span, NextPage: d.page}
	remaining := map[content.Box]bool{}
	for _, channel := range c.channels {
		reg, _ := d.bank.Lookup(registers.RegisterID(channel))
		for item, rest, ok := reg.Take(); ok; item, rest, ok = rest.Take() {
			remaining[item] = true
		}
		if head, ok := d.bank.First(registers.RegisterID(channel)); ok {
			cursor, has := d.cursors[channel]
			out.Heads = append(out.Heads, HeadInfo{channel, c.branches[head].ID, head, has, cursor.position})
		}
	}
	for _, b := range c.plan.branches {
		if d.activated[b.ID] {
			out.Activated = append(out.Activated, b.ID)
			if remaining[b.Material.Root] {
				out.Pending = append(out.Pending, b.ID)
			}
		}
	}
	return out
}

// InitialState returns the source bank at span 1 and page 1 with no activated targets.
func (r *Pagination) InitialState() State {
	if r == nil || r.context == nil {
		return State{}
	}
	return State{&stateData{context: r.context, bank: r.context.plan.document.Bank(), span: 1, page: 1}}
}

// Validate checks identity, bank suffixes and head/position pairing before any
// backend call. It also rejects a mismatched, busy or closed borrowed session.
func (r *Pagination) Validate(s State) error {
	fail := func(cause error) error { return paginationError(InvalidRequest, DiagnosticLocation{}, cause) }
	if r == nil || r.context == nil || !s.Valid() || s.data.context != r.context {
		return fail(fmt.Errorf("foreign or invalid execution context"))
	}
	if r.prepared.ID() != r.context.preparation || r.prepared.Document().SourceID() != r.context.source {
		return fail(content.ErrInvalidPreparation)
	}
	if err := r.session.Check(r.prepared); err != nil {
		return fail(err)
	}
	d, c := s.data, r.context
	if d.span < 1 || d.span > len(c.plan.spans) || d.page == 0 {
		return fail(fmt.Errorf("invalid span or page number"))
	}
	nonempty := 0
	remaining := map[BranchID]bool{}
	for _, channel := range c.channels {
		reg, _ := d.bank.Lookup(registers.RegisterID(channel))
		if reg.Len() > 0 {
			nonempty++
		}
		original := c.plan.document.Branches(channel)
		if reg.Len() > len(original) {
			return fail(fmt.Errorf("bank is not a source suffix"))
		}
		offset := len(original) - reg.Len()
		for i := offset; i < len(original); i++ {
			item, rest, ok := reg.Take()
			if !ok || item != original[i].Root {
				return fail(fmt.Errorf("foreign/reordered bank item"))
			}
			remaining[c.branches[item].ID] = true
			reg = rest
		}
	}
	if nonempty != d.bank.Len() {
		return fail(fmt.Errorf("unknown register"))
	}
	if _, present := d.bank.First("main"); !present && d.span < len(c.plan.spans) {
		return fail(fmt.Errorf("main removed before final span"))
	}
	if d.span > 1 {
		if _, present := d.bank.First("main"); present {
			cursor, ok := d.cursors["main"]
			if !ok {
				return fail(fmt.Errorf("missing main span cursor"))
			}
			if v, ok := cursor.position.Compare(c.plan.spans[d.span-1].Start); !ok || v < 0 {
				return fail(fmt.Errorf("main precedes span"))
			}
		}
	}
	for channel, cursor := range d.cursors {
		cursorError := func(message string) error {
			return paginationError(InvalidRequest, DiagnosticLocation{Channel: channel, Box: cursor.head, Position: cursor.position}, fmt.Errorf("%s", message))
		}

		head, ok := d.bank.First(registers.RegisterID(channel))
		if !ok || head != cursor.head || !cursor.position.Valid() {
			return cursorError("continuation does not belong to head")
		}
		if channel != "main" && !d.activated[c.branches[head].ID] {
			return cursorError("inactive head has continuation")
		}
		info, _ := head.Info()
		start, end := c.plan.document.TreeBounds(info.Tree)
		lo, ok1 := cursor.position.Compare(start)
		hi, ok2 := cursor.position.Compare(end)
		if !ok1 || !ok2 || lo <= 0 || hi >= 0 {
			return cursorError("continuation is not within head")
		}
		if channel == "main" {
			stop := c.plan.spans[d.span-1].End
			if v, ok := cursor.position.Compare(stop); !ok || v > 0 {
				return cursorError("main passed span boundary")
			}
		}
	}
	for channel, cursor := range d.fits {
		head, ok := d.cursors[channel]
		f := r.fitters[head.head]
		comparison, comparable := cursor.end.Logical().Compare(head.position)
		if !ok || !f.Owns(cursor.continuation) || !comparable || comparison != 0 {
			return fail(fmt.Errorf("foreign or detached fitting checkpoint"))
		}
	}
	for id, active := range d.activated {
		if !active || id == 0 || uint64(id) > uint64(len(c.plan.branches)) {
			return fail(fmt.Errorf("invalid activation"))
		}
		b := c.plan.branches[int(id)-1]
		if b.Channel == "main" || b.Span > d.span {
			return fail(fmt.Errorf("ineligible activation"))
		}
		for _, a := range c.plan.anchors {
			if a.Target == id && c.plan.branches[int(a.OwnerBranch)-1].Channel != "main" && !d.activated[a.OwnerBranch] {
				return fail(fmt.Errorf("inactive transitive owner"))
			}
		}
	}
	for _, b := range c.plan.branches {
		if b.Channel != "main" {
			if !remaining[b.ID] && !d.activated[b.ID] {
				return fail(fmt.Errorf("unactivated target consumed"))
			}
			if b.Span < d.span && remaining[b.ID] {
				return fail(fmt.Errorf("earlier span is not drained"))
			}
		}
	}
	return nil
}

type headAdvance struct {
	channel  string
	head     content.Box
	complete bool
	position content.LogicalPosition
}

// advance derives bank/position changes privately. The page builder pairs each
// partial head with its certified fitting checkpoint before publication.
func (r *Pagination) advance(s State, steps []headAdvance, activate []BranchID, page bool) (State, error) {
	if err := r.Validate(s); err != nil {
		return State{}, err
	}
	d := *s.data
	d.cursors = maps.Clone(d.cursors)
	d.fits = maps.Clone(d.fits)
	if d.cursors == nil {
		d.cursors = map[string]headContinuation{}
	}
	d.activated = maps.Clone(d.activated)
	if d.activated == nil {
		d.activated = map[BranchID]bool{}
	}
	for _, id := range activate {
		d.activated[id] = true
	}
	for _, step := range steps {
		delete(d.fits, step.channel)
		head, ok := d.bank.First(registers.RegisterID(step.channel))
		if !ok || head != step.head {
			return State{}, paginationError(InvalidRequest, DiagnosticLocation{Channel: step.channel, Box: step.head, Position: step.position}, fmt.Errorf("wrong head"))
		}
		if step.complete {
			if step.channel == "main" && d.span != len(r.context.plan.spans) {
				return State{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("cannot consume main beyond span"))
			}
			d.bank = func() registers.RegisterBank[content.Box] {
				_, next, _ := d.bank.Take(registers.RegisterID(step.channel))
				return next
			}()
			delete(d.cursors, step.channel)
		} else {
			if old, ok := d.cursors[step.channel]; ok {
				if v, comparable := step.position.Compare(old.position); !comparable || v <= 0 {
					return State{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("nonadvancing cursor"))
				}
			}
			d.cursors[step.channel] = headContinuation{head, step.position}
		}
	}
	if page {
		if len(steps) == 0 {
			return State{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("metadata alone cannot justify a page"))
		}
		if d.page == math.MaxUint64 {
			return State{}, paginationError(ExecutionFailure, DiagnosticLocation{}, ErrLossOverflow)
		}
		d.page++
	}
	next := State{&d}
	if err := r.Validate(next); err != nil {
		return State{}, err
	}
	return next, nil
}

// ChannelProgress is source-relative: completed whole items first, then the
// exact position in the unchanged head. Zero position means head beginning.
type ChannelProgress struct {
	Channel   string
	Completed uint64
	Position  content.LogicalPosition
}

// LocalRank compares arrangements from one template and starting state. Progress
// is ordered main first, then source-channel order; Enumeration breaks remaining ties.
type LocalRank struct {
	Loss        Loss
	Progress    []ChannelProgress
	Enumeration uint64
}

// CompareLocal returns negative when a is preferred within one template.
// Ranks must come from the same starting state and template; validate candidate
// identities before ranking. Vectors use identical main-first/source-channel order.
func CompareLocal(a, b LocalRank) (int, error) {
	if len(a.Progress) != len(b.Progress) {
		return 0, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("different progress vectors"))
	}
	seen := map[string]bool{}
	for i := range a.Progress {
		name := a.Progress[i].Channel
		if name == "" || seen[name] || i == 0 && name != "main" {
			return 0, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("progress must use main-first distinct channels"))
		}
		seen[name] = true
		if a.Progress[i].Channel != b.Progress[i].Channel {
			return 0, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("different channel order"))
		}
	}
	if c := a.Loss.Compare(b.Loss); c != 0 {
		return c, nil
	}
	for i, x := range a.Progress {
		y := b.Progress[i]
		if x.Completed != y.Completed {
			if x.Completed > y.Completed {
				return -1, nil
			}
			return 1, nil
		}
		if x.Position.Valid() != y.Position.Valid() {
			if x.Position.Valid() {
				return -1, nil
			}
			return 1, nil
		}
		if x.Position.Valid() {
			c, ok := x.Position.Compare(y.Position)
			if !ok {
				return 0, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("foreign progress positions"))
			}
			if c != 0 {
				return -c, nil
			}
		}
	}
	if a.Enumeration < b.Enumeration {
		return -1, nil
	}
	if a.Enumeration > b.Enumeration {
		return 1, nil
	}
	return 0, nil
}

// SequenceRank contains accumulated loss and zero-based catalog ordinals in page order.
type SequenceRank struct {
	Loss      Loss
	Templates []uint64
}

// CompareSequences prefers lower loss, then lexicographically earlier template
// ordinals. It returns negative when a is preferred and never compares progress.
func CompareSequences(a, b SequenceRank) int {
	if c := a.Loss.Compare(b.Loss); c != 0 {
		return c
	}
	return slices.Compare(a.Templates, b.Templates)
}

// SpanComplete reports factual exhaustion at the stop boundary and complete
// transitive draining. Immediately due metadata must be processed by the
// page constructor before calling it; this query does not synthesize pages.
func (s State) SpanComplete() bool {
	if !s.Valid() {
		return false
	}
	d, c := s.data, s.data.context
	if d.span < 1 || d.span > len(c.plan.spans) {
		return false
	}
	if s.mainRemaining() {
		return false
	}
	pending := map[BranchID]bool{}
	for _, h := range s.Info().Pending {
		pending[h] = true
	}
	for _, id := range c.plan.spans[d.span-1].Targets {
		if !d.activated[id] || pending[id] {
			return false
		}
	}
	return true
}

func (s State) mainRemaining() bool {
	if !s.Valid() {
		return false
	}
	d := s.data
	if _, present := d.bank.First("main"); !present {
		return false
	}
	cursor, ok := d.cursors["main"]
	if !ok {
		return true
	}
	v, comparable := cursor.position.Compare(d.context.plan.spans[d.span-1].End)
	return !comparable || v != 0
}
func (r *Pagination) advanceSpan(s State) (State, error) {
	if err := r.Validate(s); err != nil {
		return State{}, err
	}
	if !s.SpanComplete() || s.data.span >= len(r.context.plan.spans) {
		return State{}, paginationError(InvalidRequest, DiagnosticLocation{}, fmt.Errorf("span is not drained or is terminal"))
	}
	d := *s.data
	d.span++
	next := State{&d}
	if err := r.Validate(next); err != nil {
		return State{}, err
	}
	return next, nil
}
