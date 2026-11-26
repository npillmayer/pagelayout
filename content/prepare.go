package content

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	boxtext "github.com/npillmayer/css-box-layout/text"
)

// Preparer is an explicitly supplied backend. Preflight receives every group in
// residual-first/global-branch/preorder order and must validate all service and
// policy requirements without font, shaping or layout work. PrepareGroup is then
// called once per group in that same order, including empty groups. Neither call
// receives receiver geometry. Each call owns independent input slices and may
// retain them without affecting the source, manifest or another call.
//
// A successful PrepareGroup transfers one exclusively owned handle to the
// session. On failure the backend releases its in-progress work and returns no
// live handle. No panic recovery is provided. Serialize calls sharing services;
// no default backend or resource discovery is supplied by content.
type Preparer interface {
	Preflight([]GroupInput) error
	PrepareGroup(GroupInput) (GroupPreparation, error)
}

// GroupHandle is an owned backend execution capability. Release drops owned
// preparations, never borrowed resolvers, fonts or resources required by frozen
// output. The optional backend adds its own typed execution methods. Only the
// session calls Release; callers use WithGroup for temporary, serialized access.
type GroupHandle interface {
	Release() error
}

// GroupPreparation echoes the input's qualified identity with its new handle.
// Handles must be independently owned, not shared between results or sessions.
type GroupPreparation struct {
	Group, Owner Box
	Handle       GroupHandle
}

// PreparationID is an opaque, comparable identity for one successful pair.
// Its zero value is invalid. It is not a persistent or serializable identifier.
type PreparationID struct{ token *preparationIdentity }
type preparationIdentity struct{ marker byte } // Nonzero size ensures distinct pointers.

// Valid reports whether id was assigned to a completed preparation.
func (id PreparationID) Valid() bool { return id.token != nil }

// PreparedSnapshot is immutable whole-document coverage and copied group input.
// It retains source material, but never a backend, session or execution handle.
// Inspection is safe concurrently and remains valid after session closure.
type PreparedSnapshot struct{ data *preparedData }
type preparedData struct {
	id     PreparationID
	source *snapshot
	inputs []GroupInput
	index  map[Box]int
}

// ID identifies this preparation, including when its source has another pair.
func (p *PreparedSnapshot) ID() PreparationID {
	if p == nil || p.data == nil {
		return PreparationID{}
	}
	return p.data.id
}

// Document returns an independent handle to the unchanged source snapshot.
func (p *PreparedSnapshot) Document() *Document {
	if p == nil || p.data == nil {
		return nil
	}
	return &Document{material: p.data.source}
}

// Covers reports whether b belongs to this completely prepared source. This
// includes branch roots, groups and atomic boxes; foreign/zero handles fail.
func (p *PreparedSnapshot) Covers(b Box) bool {
	return p != nil && p.data != nil && b.owner == p.data.source && b.Valid()
}

// Groups returns copied coverage metadata in preparation order.
func (p *PreparedSnapshot) Groups() []GroupInfo {
	if p == nil || p.data == nil {
		return nil
	}
	groups := make([]GroupInfo, len(p.data.inputs))
	for i, g := range p.data.inputs {
		groups[i] = g.GroupInfo
		groups[i].Atomics = slices.Clone(g.Atomics)
	}
	return groups
}

// Input returns an independent copy for an actual covered group. A covered
// branch root is not itself a group; use Groups to enumerate its paragraphs.
func (p *PreparedSnapshot) Input(group Box) (GroupInput, bool) {
	if p == nil || p.data == nil {
		return GroupInput{}, false
	}
	i, ok := p.data.index[group]
	if !ok {
		return GroupInput{}, false
	}
	return cloneGroupInput(p.data.inputs[i]), true
}

// PreparationSession owns mutable backend handles, separately from the immutable
// snapshot and source bank. Callers must serialize all session/shared-service
// operations, including Close. Copies of this wrapper share one lifecycle.
// No lock, concurrent-execution promise or implicit reprepare is provided.
type PreparationSession struct{ state *preparationState }
type preparationState struct {
	prepared *preparedData
	owned    []ownedGroup
	index    map[Box]int
	closed   bool
	busy     bool
	closeErr error
}
type ownedGroup struct {
	info   GroupInfo
	handle GroupHandle
}

// Preparation failures support errors.Is in addition to source diagnostics.
var (
	ErrInvalidPreparation  = errors.New("invalid preparation or source identity")
	ErrPreparationProtocol = errors.New("preparer protocol violation")
	ErrClosedSession       = errors.New("preparation session is closed")
	ErrSessionBusy         = errors.New("preparation session is in use")
)

// PreparationStage locates a failure in the neutral lifecycle or a backend.
type PreparationStage string

const (
	StageExtraction  PreparationStage = "extraction"
	StagePreflight   PreparationStage = "preflight"
	StagePreparation PreparationStage = "preparation"
	StageAccess      PreparationStage = "access"
	StageExecution   PreparationStage = "execution"
	StageMeasurement PreparationStage = "measurement"
	StageLayout      PreparationStage = "layout"
	StageFitting     PreparationStage = "fitting"
	StageRelease     PreparationStage = "release"
)

// PreparationError retains qualified group/owner identities and the original
// cause. Source/Range identify a source span when available, otherwise the owner
// context. Identity/lifecycle failures may have no source. Backends may return
// this type to supply a more precise stage/source (such as itemization/layout).
type PreparationError struct {
	Stage        PreparationStage
	Group, Owner Box
	Source       SourceRef
	Range        boxtext.TextRange
	HasRange     bool
	Err          error
}

func (e *PreparationError) Error() string {
	where := e.Source.Name
	if e.Source.Path != "" {
		where += ": " + e.Source.Path
	}
	if where != "" {
		where += ": "
	}
	return fmt.Sprintf("%scontent %s: %v", where, e.Stage, e.Err)
}
func (e *PreparationError) Unwrap() error { return e.Err }

// Prepare extracts and validates all input, runs backend preflight once, then
// prepares every group. A failure publishes neither result and releases owned
// handles in reverse creation order, continuing after release failures. Causes
// from the initiating failure and cleanup remain reachable with errors.Is/As.
// The source and bank are never changed. Even an empty document needs an
// explicit backend and preflight; no font or layout service is called by content.
func Prepare(d *Document, backend Preparer) (*PreparedSnapshot, *PreparationSession, error) {
	if d == nil || d.material == nil || nilInterface(backend) {
		return nil, nil, &PreparationError{Stage: StagePreflight, Err: fmt.Errorf("%w: document and explicit preparer required", ErrInvalidPreparation)}
	}
	source := d.material
	var inputs []GroupInput
	for tree := 1; tree < len(source.trees); tree++ {
		for i := range source.trees[tree].groups {
			g, err := source.extractGroup(TreeID(tree), i)
			if err != nil {
				var e *extractionError
				if errors.As(err, &e) {
					return nil, nil, &PreparationError{Stage: StageExtraction, Group: e.Group, Owner: e.Owner,
						Source: e.Source, Range: e.Range, HasRange: e.HasRange, Err: err}
				}
				return nil, nil, &PreparationError{Stage: StageExtraction, Err: err}
			}
			if err := validateGroupInput(*g); err != nil {
				return nil, nil, preparationFailure(StagePreflight, g.GroupInfo, err)
			}
			inputs = append(inputs, *g)
		}
	}
	mapped := source.positionEvents(inputs)
	attachGroupEvents(inputs, mapped)
	for _, g := range inputs {
		if err := validateGroupEvents(g); err != nil {
			return nil, nil, preparationFailure(StageExtraction, g.GroupInfo, err)
		}
	}
	preflight := make([]GroupInput, len(inputs))
	for i, g := range inputs {
		preflight[i] = cloneGroupInput(g)
	}
	if err := backend.Preflight(preflight); err != nil {
		var detail *PreparationError
		if errors.As(err, &detail) {
			for _, g := range inputs {
				if detail.Group == g.Group {
					return nil, nil, preparationFailure(StagePreflight, g.GroupInfo, err)
				}
			}
		}
		return nil, nil, &PreparationError{Stage: StagePreflight, Err: err}
	}
	var owned []ownedGroup
	fail := func(err error) (*PreparedSnapshot, *PreparationSession, error) {
		return nil, nil, errors.Join(err, releaseGroups(owned))
	}
	index := make(map[Box]int, len(inputs))
	for i, g := range inputs {
		result, err := backend.PrepareGroup(cloneGroupInput(g))
		// Defensively release even a handle returned in violation of the error
		// contract. Its cleanup must never suppress the initiating cause.
		if !nilInterface(result.Handle) {
			owned = append(owned, ownedGroup{g.GroupInfo, result.Handle})
		}
		if err != nil {
			return fail(preparationFailure(StagePreparation, g.GroupInfo, err))
		}
		if nilInterface(result.Handle) {
			return fail(preparationFailure(StagePreparation, g.GroupInfo, fmt.Errorf("%w: missing group handle", ErrPreparationProtocol)))
		}
		if result.Group != g.Group || result.Owner != g.Owner {
			return fail(preparationFailure(StagePreparation, g.GroupInfo, fmt.Errorf("%w: returned group/owner differs from input", ErrPreparationProtocol)))
		}
		if _, duplicate := index[g.Group]; duplicate {
			return fail(preparationFailure(StagePreparation, g.GroupInfo, fmt.Errorf("%w: duplicate group", ErrPreparationProtocol)))
		}
		index[g.Group] = i
	}
	if len(index) != len(inputs) || len(owned) != len(inputs) {
		return fail(&PreparationError{Stage: StagePreparation, Err: ErrPreparationProtocol})
	}
	data := &preparedData{id: PreparationID{&preparationIdentity{}}, source: source, inputs: inputs, index: index}
	p := &PreparedSnapshot{data: data}
	session := &PreparationSession{state: &preparationState{prepared: data, owned: owned, index: index}}
	return p, session, nil
}

// WithGroup checks the exact snapshot/session pair and group/owner before
// invoking use. The callback may type-assert its backend's handle to execute an
// operation. It must not retain/release that handle or reenter the session.
// Backend-specific requests and typed/frozen results belong to the callback,
// never to source boxes or this neutral snapshot. Failure leaves the session
// reusable; only the backend's complete memoized discoveries may change.
func (s *PreparationSession) WithGroup(p *PreparedSnapshot, group, owner Box, use func(GroupHandle) error) error {
	return s.withAccess(p, func(st *preparationState) error {
		return st.useGroup(group, owner, use)
	})
}

// Reserve the entire operation, including recursive core callbacks, so even
// groups requiring no backend work check pair identity and session lifecycle.
func (s *PreparationSession) withAccess(p *PreparedSnapshot, use func(*preparationState) error) error {
	if s == nil || s.state == nil || s.state.closed {
		return &PreparationError{Stage: StageAccess, Err: ErrClosedSession}
	}
	st := s.state
	if st.busy {
		return &PreparationError{Stage: StageAccess, Err: ErrSessionBusy}
	}
	if p == nil || p.data == nil || p.data != st.prepared {
		return &PreparationError{Stage: StageAccess, Err: ErrInvalidPreparation}
	}
	st.busy = true
	defer func() { st.busy = false }()
	return use(st)
}

func (st *preparationState) useGroup(group, owner Box, use func(GroupHandle) error) error {
	i, ok := st.index[group]
	if !ok || st.owned[i].info.Owner != owner || use == nil {
		return &PreparationError{Stage: StageAccess, Err: ErrInvalidPreparation}
	}
	if err := use(st.owned[i].handle); err != nil {
		return preparationFailure(StageExecution, st.owned[i].info, err)
	}
	return nil
}

// Close releases handles in reverse order, drops session references and never
// closes borrowed backend services. It is idempotent, including after cleanup
// errors: subsequent calls return the same error without repeating release.
// Snapshots/source/frozen results remain valid. A nil or zero session is closed.
func (s *PreparationSession) Close() error {
	if s == nil || s.state == nil {
		return nil
	}
	st := s.state
	if st.closed {
		return st.closeErr
	}
	if st.busy {
		return &PreparationError{Stage: StageAccess, Err: ErrSessionBusy}
	}
	st.closed = true
	owned := st.owned
	st.owned, st.index, st.prepared = nil, nil, nil
	st.closeErr = releaseGroups(owned)
	return st.closeErr
}

func releaseGroups(owned []ownedGroup) error {
	var failures []error
	for i := len(owned) - 1; i >= 0; i-- {
		if err := owned[i].handle.Release(); err != nil {
			failures = append(failures, preparationFailure(StageRelease, owned[i].info, err))
		}
		owned[i].handle = nil
	}
	return errors.Join(failures...)
}

func cloneGroupInput(g GroupInput) GroupInput {
	g.Events = slices.Clone(g.Events)
	g.Atomics = slices.Clone(g.Atomics)
	g.Styles = slices.Clone(g.Styles)
	g.Paint = slices.Clone(g.Paint)
	g.Hyphens = slices.Clone(g.Hyphens)
	g.Isolates = slices.Clone(g.Isolates)
	g.TextSources = slices.Clone(g.TextSources)
	g.Occurrences = slices.Clone(g.Occurrences)
	return g
}

func nilInterface(v any) bool {
	if v == nil {
		return true
	}
	switch r := reflect.ValueOf(v); r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return r.IsNil()
	}
	return false
}

func preparationFailure(stage PreparationStage, info GroupInfo, err error) *PreparationError {
	e := &PreparationError{Stage: stage, Group: info.Group, Owner: info.Owner, Source: info.Source, Err: err}
	var detail *PreparationError
	if errors.As(err, &detail) && detail.Group == info.Group && detail.Owner == info.Owner {
		if detail.Stage != "" && stage != StageRelease {
			e.Stage = detail.Stage
		}
		// Accept only a location in this group's source tree. Backend errors
		// without such a location fall back to the owner context.
		s := info.Group.owner
		id := detail.Source.NodeID
		if id > 0 && uint64(id) < uint64(len(s.sources)) && s.sources[id].Tree == info.Group.tree && s.sources[id].Source == detail.Source {
			e.Source = detail.Source
			if text, ok := s.texts[boxtext.TextSourceID(id)]; detail.HasRange && ok && validByteRange(text.Text, detail.Range.Start, detail.Range.End) {
				e.Range, e.HasRange = detail.Range, true
			}
		}
	}
	return e
}

// Check validates this borrowed session's identity and availability without
// invoking backend services. Like WithGroup, callers serialize it with execution.
func (s *PreparationSession) Check(p *PreparedSnapshot) error {
	return s.withAccess(p, func(*preparationState) error { return nil })
}
