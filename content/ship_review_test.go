package content

import (
	"github.com/npillmayer/css-box-layout/layout"
	"testing"
)

func TestReviewNestedAtomicPositionsHaveConsistentOrder(t *testing.T) {
	d := loaded(t, "<body><p><a id=\"left\" data-flow-anchor href=\"#n\">L</a><span id=\"atomic\" style=\"display:inline-block\"><a id=\"inside\" data-flow-anchor href=\"#m\">I</a></span><a id=\"tail\" data-flow-anchor href=\"#z\"></a></p>", LoadOptions{})
	events, err := d.EventPositions()
	if err != nil {
		t.Fatal(err)
	}
	// The outside trailing marker must attach after the indivisible atomic, not
	// to an interior paragraph. Its logical position must follow interior content.
	tail := events[2]
	if len(tail.Coverage) != 1 || !tail.Coverage[0].Atomic {
		t.Fatalf("outside affinity entered atomic interior: %+v", tail)
	}
	if c, ok := events[1].Position.Compare(tail.Position); !ok || c >= 0 {
		t.Fatal("atomic end precedes its interior")
	}
}

func TestReviewAffinitySeesEmptyReceivingBlocks(t *testing.T) {
	d := loaded(t, "<body><div id=\"outer\"><p>Before</p><a id=\"owner\" data-flow-anchor href=\"#n\"></a><section id=\"next\" style=\"padding:3px;break-before:page\"></section></div>", LoadOptions{})
	events, err := d.EventPositions()
	if err != nil {
		t.Fatal(err)
	}
	owner := events[0]
	if owner.Affinity != AffinityFollowing || owner.Position.Info().Source != materialSource(t, d, "next").Source {
		t.Fatalf("retained empty block missed: %+v", owner)
	}
}

func TestReviewPositionComparisonTransitivity(t *testing.T) {
	d := loaded(t, "<body><p><a id=outer href=#n data-flow-anchor><span style='display:inline-block'><span id=inner data-flow-anchor>I</span></span></a><a id=tail href=#t data-flow-anchor></a></p>", LoadOptions{})
	events, err := d.EventPositions()
	if err != nil {
		t.Fatal(err)
	}
	if len(events[0].Coverage) != 1 || !events[0].Coverage[0].Atomic {
		t.Fatal("outer link counts interior separately", events[0])
	}
	positions := []LogicalPosition{}
	for _, e := range events {
		positions = append(positions, e.Position)
		for _, c := range e.Coverage {
			positions = append(positions, c.Position)
		}
	}
	for _, a := range positions {
		for _, b := range positions {
			ab, ok := a.Compare(b)
			if !ok {
				t.Fatal("same tree incomparable")
			}
			ba, _ := b.Compare(a)
			if ab != -ba {
				t.Fatal("antisymmetry")
			}
			for _, c := range positions {
				bc, _ := b.Compare(c)
				ac, _ := a.Compare(c)
				if ab <= 0 && bc <= 0 && ac > 0 {
					t.Fatal("nontransitive positions")
				}
			}
		}
	}
	if c, _ := events[1].Position.Compare(events[2].Position); c >= 0 {
		t.Fatal("atomic interior must precede outside end")
	}
}

func TestReviewOutsideAnchorPreparesInOuterGroup(t *testing.T) {
	d := loaded(t, "<body style='hyphens:none'><p id=p><span style='display:inline-block'>inside</span><a id=tail data-flow-anchor href=#n></a></p>", LoadOptions{})
	backend := &mockPreparer{}
	prepared, session, err := Prepare(d, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	count := 0
	for _, g := range prepared.Groups() {
		input, _ := prepared.Input(g.Group)
		for _, e := range input.Events {
			if e.Source == materialSource(t, d, "tail").Source {
				count++
				if g.Source != materialSource(t, d, "p").Source || !e.Atomic || e.Start != 3 || e.End != 3 {
					t.Fatal("outside event belongs to atomic interior", input)
				}
			}
		}
	}
	if count != 1 {
		t.Fatal("event count", count)
	}
}

func TestReviewSharedProcessedBoundaryHasOneLogicalPosition(t *testing.T) {
	d := loaded(t, "<body><p><span>ab</span><span>cd</span></p>", LoadOptions{})
	inputs, err := d.material.positionInputs()
	if err != nil {
		t.Fatal(err)
	}
	g := inputs[0]
	before := map[Box]layout.StructuralBoundary{}
	after := map[Box]layout.StructuralBoundary{}
	for _, b := range d.material.trees[1].provenance.Boundaries() {
		box := Box{d.material, 1, b.Box}
		if b.Edge == layout.BoundaryBefore {
			before[box] = b
		} else {
			after[box] = b
		}
	}
	left, right := g.TextSources[0], g.TextSources[1]
	a := d.material.logical(1, before[left.Box], left.Source)
	b := d.material.logical(1, before[right.Box], right.Source)
	a.group = g.Group
	a.offset = left.End
	a.textPosition = true
	b.group = g.Group
	b.offset = right.Start
	b.textPosition = true
	events := []PositionedEvent{{Position: a}, {Position: b}}
	canonicalizeEventPositions(inputs, before, after, events)
	if c, ok := events[0].Position.Compare(events[1].Position); !ok || c != 0 {
		t.Fatal("one processed boundary has different logical positions", c)
	}
	if events[0].Position.Info().Box != left.Box || events[1].Position.Info().Box != right.Box {
		t.Fatal("canonical ordering erased source correspondence")
	}
}
