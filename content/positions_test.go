package content

import (
	boxtext "github.com/npillmayer/css-box-layout/text"
	"reflect"
	"strings"
	"testing"
)

func TestEventPreparationCoverageAndLifetime(t *testing.T) {
	d := loaded(t, `<body style="hyphens:none"><p id="p">  é <a id="visible" href="#note" data-flow-anchor> אב  cd </a><a id="tail" href="#other" data-flow-anchor> </a></p><div id="empty"><a id="empty-owner" data-flow-anchor href="#third"></a></div><aside id="note" style="flow-into:notes">N</aside><aside id="other" style="flow-into:notes">O</aside><aside id="third" style="flow-into:notes">T</aside></body>`, LoadOptions{})
	events, err := d.EventPositions()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Affinity != AffinityVisible || events[1].Affinity != AffinityPreceding || events[2].Affinity != AffinityStructural {
		t.Fatal(events)
	}
	backend := &mockPreparer{}
	prepared, session, err := Prepare(d, backend)
	if err != nil {
		t.Fatal(err)
	}
	if err = session.Close(); err != nil {
		t.Fatal(err)
	}
	var found []GroupEvent
	for _, group := range prepared.Groups() {
		input, _ := prepared.Input(group.Group)
		for _, event := range input.Events {
			found = append(found, event)
			if event.End > uint64(len(input.Text)) || event.Start > event.End {
				t.Fatal("bad processed position", event)
			}
			if event.Affinity == AffinityVisible {
				text, ok := d.TextSource(boxtext.TextSourceID(event.CoverageSource.NodeID))
				if !ok {
					t.Fatal("lost source")
				}
				if event.Range.End > uint64(len(text.Text)) {
					t.Fatal("bad source range")
				}
			}
		}
		input.Events = nil
		again, _ := prepared.Input(group.Group)
		if len(again.Events) > 0 {
			again.Events[0].Start = 999
			retained, _ := prepared.Input(group.Group)
			if retained.Events[0].Start == 999 {
				t.Fatal("prepared alias")
			}
		}
	}
	for _, event := range events {
		seen := false
		for _, g := range found {
			if g.Source == event.Event.Source {
				seen = true
			}
		}
		if !seen {
			t.Fatal("event missing from preparation", event)
		}
	}
	original := events[0].Coverage[0]
	events[0].Coverage[0].Start = 999
	again, err := d.EventPositions()
	if err != nil || again[0].Coverage[0] != original {
		t.Fatal("mapped-event alias", err)
	}
	// Removal of annotations cannot change extraction text, ranges or box IDs.
	without := loaded(t, `<body style="hyphens:none"><p id="p">  é <a id="visible" href="#note"> אב  cd </a><a id="tail" href="#other"> </a></p><div id="empty"><a id="empty-owner" href="#third"></a></div><aside id="note" style="flow-into:notes">N</aside><aside id="other" style="flow-into:notes">O</aside><aside id="third" style="flow-into:notes">T</aside></body>`, LoadOptions{})
	left, _ := d.material.positionInputs()
	right, _ := without.material.positionInputs()
	for i, g := range left {
		if g.Text != right[i].Text || g.Group.id != right[i].Group.id || len(g.TextSources) != len(right[i].TextSources) {
			t.Fatal("events changed material")
		}
	}
}

func TestPositionAffinityAcrossOmissionAndText(t *testing.T) {
	d := loaded(t, `<body><div id="outer"><p>before</p><a id="omitted" href="#n" data-flow-anchor> </a><div id="next">after</div></div><aside id="n" style="flow-into:notes">N</aside>`, LoadOptions{})
	positions, err := d.EventPositions()
	if err != nil {
		t.Fatal(err)
	}
	p := positions[0]
	if p.Affinity != AffinityFollowing || p.Position.Info().Source != materialSource(t, d, "next").Source || p.Block != materialSource(t, d, "outer").Source {
		t.Fatal(p)
	}
	foreign := loaded(t, `<body>foreign`, LoadOptions{})
	other, _ := foreign.TreeBounds(foreign.Residual().Tree)
	if _, ok := p.Position.Compare(other); ok {
		t.Fatal("foreign comparison")
	}
	if _, ok := p.Position.Compare(LogicalPosition{}); ok {
		t.Fatal("invalid comparison")
	}
	if c, ok := p.Position.Compare(p.Position); !ok || c != 0 {
		t.Fatal("self comparison")
	}
}

func TestBreakPositionsAndExclusion(t *testing.T) {
	for _, src := range []string{`<body><span style="break-before:page">x</span>`, `<body><div style="display:inline-block"><div style="break-before:page">x</div></div>`, `<body><aside style="flow-into:notes;break-before:page">x</aside>`} {
		d := loaded(t, src, LoadOptions{})
		events, err := d.EventPositions()
		if err != nil || len(events) != 1 || !events[0].UnsupportedBreak {
			t.Fatal(events, err)
		}
	}
	d := loaded(t, `<body><p>earlier</p><section id="container" style="padding-top:3px"><h2 id="child" style="break-before:page">Heading</h2></section><div style="display:none;break-before:page">hidden</div>`, LoadOptions{})
	events, err := d.EventPositions()
	if err != nil || len(events) != 1 || events[0].Position.Info().Source != materialSource(t, d, "container").Source {
		t.Fatal(events, err)
	}
	if events[0].Event.Source != materialSource(t, d, "child").Source {
		t.Fatal("original source lost")
	}
}

func TestPreparationEventInputCopies(t *testing.T) {
	d := loaded(t, `<body style="hyphens:none"><p><a data-flow-anchor href="#n">abc</a></p><aside id="n" style="flow-into:notes">note</aside>`, LoadOptions{})
	backend := &mockPreparer{preflight: func(inputs []GroupInput) error { inputs[0].Events[0].Start = 999; return nil }}
	prepared, session, err := Prepare(d, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	first, _ := prepared.Input(prepared.Groups()[0].Group)
	if first.Events[0].Start != 0 {
		t.Fatal("preflight changed retained mapping")
	}
	if !strings.Contains(first.Text, "abc") || !reflect.DeepEqual(first.Events, backend.handles[0].input.Events) {
		t.Fatal("backend mapping")
	}
}
