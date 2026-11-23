package content

import (
	"errors"
	"testing"
)

func TestPreparationInputContract(t *testing.T) {
	d := loaded(t, `<body>é <span dir="rtl">אב<span style="display:inline-block">inner</span>ג</span> z`, LoadOptions{})
	g := extractAllGroups(t, d)[0]
	other := loaded(t, `<body>foreign</body>`, LoadOptions{})
	if err := validateGroupInput(*g); err != nil {
		t.Fatal(err)
	}
	for name, corrupt := range map[string]func(*GroupInput){
		"foreign owner":        func(g *GroupInput) { g.Owner = other.Groups(1)[0].Owner },
		"style splits rune":    func(g *GroupInput) { g.Styles[0].Start = 1 },
		"missing paint":        func(g *GroupInput) { g.Paint = nil },
		"missing source":       func(g *GroupInput) { g.TextSources = g.TextSources[1:] },
		"source range":         func(g *GroupInput) { g.TextSources[0].Range.Start = 1 },
		"duplicate source":     func(g *GroupInput) { g.TextSources = append(g.TextSources, g.TextSources[0]) },
		"marker offset":        func(g *GroupInput) { g.Occurrences[0].Start++ },
		"atomic identity":      func(g *GroupInput) { g.Occurrences[0].Box = g.Group },
		"missing policy":       func(g *GroupInput) { g.Hyphens = nil },
		"policy covers marker": func(g *GroupInput) { g.Hyphens[0].End = g.Occurrences[0].End },
		"isolate splits rune":  func(g *GroupInput) { g.Isolates[0].Start = 1 },
		"crossing isolates": func(g *GroupInput) {
			s := g.Isolates[0]
			s.Start = 0
			s.End = g.Isolates[0].Start + 2
			g.Isolates = append(g.Isolates, s)
		},
	} {
		t.Run(name, func(t *testing.T) {
			in := cloneGroupInput(*g)
			corrupt(&in)
			if err := validateGroupInput(in); !errors.Is(err, ErrInvalidPreparation) {
				t.Fatal("invalid prepared input admitted", err)
			}
		})
	}
}

func TestPreparationBackendDiagnostics(t *testing.T) {
	d := preparationDocument(t, "manual.html")
	input := extractAllGroups(t, d)[0]
	span := input.TextSources[0]
	cause := errors.New("font failure")
	located := &PreparationError{Stage: "itemization", Group: input.Group, Owner: input.Owner, Source: span.Source, Range: span.Range, HasRange: true, Err: cause}
	m := &mockPreparer{prepare: func(GroupInput, int) (GroupPreparation, error) { return GroupPreparation{}, located }}
	p, s, err := Prepare(d, m)
	var got *PreparationError
	if p != nil || s != nil || !errors.Is(err, cause) || !errors.As(err, &got) || got == located || got.Stage != "itemization" || got.Source != span.Source || got.Range != span.Range || !got.HasRange {
		t.Fatal("lost backend location", err)
	}
	if located.Err != cause || located.Stage != "itemization" {
		t.Fatal("mutated backend error")
	}
	// Cleanup always reports release as the outer stage, with the backend's
	// richer error and original cause still reachable underneath it.
	e := preparationFailure(StageRelease, input.GroupInfo, located)
	if e.Stage != StageRelease || !errors.Is(e, cause) {
		t.Fatal("release stage")
	}
	located.Range.End = 999
	e = preparationFailure(StagePreparation, input.GroupInfo, located)
	if e.HasRange {
		t.Fatal("published an invalid backend source range")
	}
}
