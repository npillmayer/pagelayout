package pagelayout

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/pagelayout/template"
)

type contractPreparer struct{ calls int }
type contractHandle struct{}

func (*contractHandle) Release() error                         { return nil }
func (*contractPreparer) Preflight([]content.GroupInput) error { return nil }
func (p *contractPreparer) PrepareGroup(g content.GroupInput) (content.GroupPreparation, error) {
	p.calls++
	return content.GroupPreparation{Group: g.Group, Owner: g.Owner, Handle: &contractHandle{}}, nil
}
func testPagination(t *testing.T) (*Pagination, PaginationConfig, *contractPreparer) {
	t.Helper()
	d, p, err := fixturePlan(t, "content/transitive.html", content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	backend := &contractPreparer{}
	prepared, session, err := content.Prepare(d, backend)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	catalog, err := template.LoadFS(os.DirFS("testdata/pagination"), "templates/receivers.html")
	if err != nil {
		t.Fatal(err)
	}
	cfg := PaginationConfig{Plan: p, Catalog: catalog, Prepared: prepared, Session: session}
	run, err := NewPagination(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return run, cfg, backend
}
func branchNamed(t *testing.T, p *SpanPlan, name string) PlanBranch {
	t.Helper()
	for _, b := range p.branches {
		if sourceLabel(p.Document(), b.Material.Source) == name {
			return b
		}
	}
	t.Fatal("no branch", name)
	return PlanBranch{}
}
func firstSpanState(t *testing.T, r *Pagination) (State, State) {
	t.Helper()
	initial := r.InitialState()
	p := r.context.plan
	main := p.branches[0]
	note := branchNamed(t, p, "note")
	figure := branchNamed(t, p, "figure")
	next, err := r.advance(initial, []headAdvance{
		{"main", main.Material.Root, false, p.spans[0].End},
		{"notes", note.Material.Root, true, content.LogicalPosition{}},
		{"figures", figure.Material.Root, true, content.LogicalPosition{}},
	}, []BranchID{note.ID, figure.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	return initial, next
}
func TestImmutableStateAndSpanDrain(t *testing.T) {
	r, _, _ := testPagination(t)
	initial, next := firstSpanState(t, r)
	if initial.SpanComplete() || !next.SpanComplete() || initial.Info().NextPage != 1 || next.Info().NextPage != 2 {
		t.Fatal("state lifecycle")
	}
	if len(initial.Info().Activated) != 0 || len(next.Info().Activated) != 2 || len(next.Info().Pending) != 0 {
		t.Fatal("activation/drain")
	}
	if initial.Bank().Len() != 3 || next.Bank().Len() != 2 {
		t.Fatal("persistent banks")
	}
	info := next.Info()
	info.Heads[0].Channel = "changed"
	info.Activated[0] = 999
	if next.Info().Heads[0].Channel != "main" || next.Info().Activated[0] == 999 {
		t.Fatal("inspection alias")
	}
	_, derived, _ := initial.Bank().Take("notes")
	if derived.Len() != initial.Bank().Len() {
		t.Fatal("unexpected fixture")
	}
	first, _ := initial.Bank().First("notes")
	if first != branchNamed(t, r.context.plan, "note").Material.Root {
		t.Fatal("bank mutated")
	}
	advanced, err := r.advanceSpan(next)
	if err != nil || advanced.Info().Span != 2 || advanced.Info().NextPage != 2 {
		t.Fatal(advanced.Info(), err)
	}
	later := branchNamed(t, r.context.plan, "later")
	final, err := r.advance(advanced, []headAdvance{{"main", r.context.plan.branches[0].Material.Root, true, content.LogicalPosition{}}, {"notes", later.Material.Root, true, content.LogicalPosition{}}}, []BranchID{later.ID}, true)
	if err != nil || !final.SpanComplete() || final.Bank().Len() != 0 || final.Info().NextPage != 3 {
		t.Fatal(final.Info(), err)
	}
	if _, err = r.advanceSpan(final); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("reopened terminal span")
	}
}
func TestStateValidationAndAtomicFailure(t *testing.T) {
	r, cfg, backend := testPagination(t)
	s := r.InitialState()
	calls := backend.calls
	foreign, err := NewPagination(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = foreign.Validate(s); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("foreign context accepted")
	}
	second, session, err := content.Prepare(cfg.Plan.Document(), &contractPreparer{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	wrong := cfg
	wrong.Prepared = second
	if _, err = NewPagination(wrong); !errors.Is(err, content.ErrInvalidPreparation) || !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("preparation mismatch", err)
	}
	original := s.Info()
	note, later := branchNamed(t, cfg.Plan, "note"), branchNamed(t, cfg.Plan, "later")
	_, err = r.advance(s, []headAdvance{{"notes", note.Material.Root, true, content.LogicalPosition{}}, {"notes", note.Material.Root, true, content.LogicalPosition{}}}, []BranchID{note.ID}, true)
	if !errors.Is(err, ErrInvalidRequest) || !reflect.DeepEqual(original, s.Info()) {
		t.Fatal("late error mutated input", err)
	}
	if _, err = r.advance(s, nil, nil, true); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("metadata page")
	}
	if _, err = r.advance(s, nil, []BranchID{later.ID}, false); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("future activation")
	}
	_, next := firstSpanState(t, r)
	d := *next.data
	d.cursors = map[string]headContinuation{"notes": {note.Material.Root, cfg.Plan.spans[0].End}}
	if err = r.Validate(State{&d}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("wrong-head cursor", err)
	}
	d = *next.data
	d.cursors = map[string]headContinuation{"missing": {later.Material.Root, cfg.Plan.spans[0].End}}
	if err = r.Validate(State{&d}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("empty-register cursor", err)
	}
	if backend.calls != calls {
		t.Fatal("validation invoked backend")
	}
	cfg.Session.Close()
	if err = r.Validate(s); !errors.Is(err, ErrInvalidRequest) || !errors.Is(err, content.ErrClosedSession) {
		t.Fatal("closed session", err)
	}
}
func TestInitializationCopiesPolicyAndLimits(t *testing.T) {
	_, cfg, _ := testPagination(t)
	policy := DefaultLossPolicy()
	limits := DefaultWorkLimits()
	cfg.Policy = &policy
	cfg.Limits = &limits
	r, err := NewPagination(cfg)
	if err != nil {
		t.Fatal(err)
	}
	policy.Page = 0
	limits.LocalSteps = 0
	if r.Policy().Page == 0 || r.Limits().LocalSteps == 0 || r.BeamWidth() != 32 {
		t.Fatal("configuration aliased")
	}
	if _, err = NewPagination(cfg); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatal(err)
	}
	cfg.Policy = nil
	if _, err = NewPagination(cfg); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("zero limits")
	}
	cfg.Limits = nil
	cfg.BeamWidth = -1
	if _, err = NewPagination(cfg); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("negative beam")
	}
}

func TestCompletedHeadThenPartialNextHead(t *testing.T) {
	_, cfg, _ := testPagination(t)
	source := "<body style=\"hyphens:none\"><p><a href=\"#a\" data-flow-anchor>A</a><a href=\"#b\" data-flow-anchor>B</a><a href=\"#c\" data-flow-anchor>C</a></p><aside id=\"a\" style=\"flow-into:notes\">A</aside><aside id=\"b\" style=\"flow-into:notes\">Before <a href=\"#d\" data-flow-anchor>D</a> after</aside><aside id=\"c\" style=\"flow-into:notes\">C</aside><figure id=\"d\" style=\"flow-into:figures\">D</figure>"
	doc, err := content.Load(strings.NewReader(source), content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(doc)
	if err != nil {
		t.Fatal(err)
	}
	prepared, session, err := content.Prepare(doc, &contractPreparer{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	cfg.Plan = plan
	cfg.Prepared = prepared
	cfg.Session = session
	r, err := NewPagination(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, b := branchNamed(t, plan, "a"), branchNamed(t, plan, "b")
	var position content.LogicalPosition
	for _, anchor := range plan.anchors {
		if anchor.OwnerBranch == b.ID {
			position = anchor.Position
		}
	}
	initial := r.InitialState()
	next, err := r.advance(initial, []headAdvance{{"notes", a.Material.Root, true, content.LogicalPosition{}}, {"notes", b.Material.Root, false, position}}, []BranchID{a.ID, b.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	head, _ := next.Bank().First("notes")
	if head != b.Material.Root {
		t.Fatal("partial branch was removed")
	}
	queue, _ := next.Bank().Lookup("notes")
	if queue.Len() != 2 {
		t.Fatal("suffix changed")
	}
	if next.data.cursors["notes"].head != head || next.data.cursors["notes"].position != position {
		t.Fatal("cursor detached from head")
	}
	if _, ok := initial.data.cursors["notes"]; ok {
		t.Fatal("original cursor mutated")
	}
	// Config handles are copied; overwriting a caller's handle cannot erase state.
	*plan = SpanPlan{}
	*prepared = content.PreparedSnapshot{}
	if err := r.Validate(next); err != nil {
		t.Fatal("configuration handle lifetime", err)
	}
}

func TestForeignSourceAndCursor(t *testing.T) {
	r, cfg, _ := testPagination(t)
	other, _, _ := testPagination(t)
	wrong := cfg
	wrong.Plan = other.context.plan
	if _, err := NewPagination(wrong); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("foreign source", err)
	}
	initial := r.InitialState()
	data := *initial.data
	main, _ := initial.Bank().First("main")
	data.cursors = map[string]headContinuation{"main": {main, other.context.plan.spans[0].End}}
	if err := r.Validate(State{&data}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("foreign cursor", err)
	}
	if (State{}).Valid() || (State{}).SpanComplete() || len((State{}).Info().Heads) != 0 {
		t.Fatal("zero state")
	}
}
