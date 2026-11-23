package typography

import (
	"runtime"
	"testing"
	"weak"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/css-box-layout/registers"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/pagelayout/content"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

// Discovery retains its recipe, which retains this factory/hyphenator. This
// canary tests that ownership transitively without unsafe access to sibling
// internals or a production diagnostic accessor. Font/coverage services must
// not point back to it: retained output intentionally keeps those services.
type lifetimeRecipe struct {
	streams     []weak.Pointer[glyphot.Stream]
	discoveries int
}

func (r *lifetimeRecipe) stream() (contracts.StreamingShaper, error) {
	s, err := glyphot.NewStream(32)
	if err == nil {
		r.streams = append(r.streams, weak.Make(s))
	}
	return s, err
}
func (r *lifetimeRecipe) Discretionaries(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	r.discoveries++
	if word != "\u00ad" {
		return nil, errRealService
	}
	return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
}

type weakPreparation struct {
	backend    weak.Pointer[Backend]
	session    weak.Pointer[content.PreparationSession]
	recipe     weak.Pointer[lifetimeRecipe]
	groups     []weak.Pointer[preparedGroup]
	paragraphs []weak.Pointer[paragraph.Prepared]
	collector  weak.Pointer[layoutInvocation]
	streams    []weak.Pointer[glyphot.Stream]
}
type retainedPreparation struct {
	manifest *content.PreparedSnapshot
	bank     registers.RegisterBank[content.Box]
	results  []*Result
	closed   *content.PreparationSession
}

// Keep working references in a separate stack frame, as in typography's own
// lifetime regressions. Returning a bank/manifest deliberately keeps all source
// material alive while executable preparation must become collectible.
func retainedRealPreparation(t *testing.T, manual, keepClosed bool) (weakPreparation, retainedPreparation) {
	t.Helper()
	b, _, _ := realBackend(t, "GentiumPlus-R.ttf")
	recipe := &lifetimeRecipe{}
	b.config.NewShaper = recipe.stream
	b.config.Hyphenator = recipe
	source := `<body style="hyphens:none;font-size:20px"><section style="flow-into:main">Before <span style="display:inline-block">Nested text.</span> after.</section></body>`
	if manual {
		source = `<body style="hyphens:manual;font-size:20px"><section style="flow-into:main">co&shy;operate</section></body>`
	}
	d := load(t, source)
	p, s, err := content.Prepare(d, b)
	if err != nil {
		t.Fatal(err)
	}
	refs := weakPreparation{backend: weak.Make(b), session: weak.Make(s), recipe: weak.Make(recipe)}
	for _, g := range p.Groups() {
		if err := s.WithGroup(p, g.Group, g.Owner, func(handle content.GroupHandle) error {
			h := handle.(*preparedGroup)
			refs.groups = append(refs.groups, weak.Make(h))
			refs.paragraphs = append(refs.paragraphs, weak.Make(h.paragraph))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	root := d.Branches("main")[0].Root
	if _, err := MeasureIntrinsic(p, s, root, layout.ResolveContext{}); err != nil {
		t.Fatal(err)
	}
	if manual && recipe.discoveries == 0 {
		t.Fatal("lifetime proof did not populate discovery")
	}
	// Exercise the real collector and retain exactly the bundle that Layout
	// publishes, while capturing a weak reference to invocation-only scratch.
	i := &layoutInvocation{breaker: paragraph.FirstFit, groups: make(map[content.Box]GroupResult)}
	css, err := content.Layout(p, s, root, 120, i)
	if err != nil {
		t.Fatal(err)
	}
	refs.collector = weak.Make(i)
	first := &Result{css: css, groups: i.groups}
	second, err := Layout(p, s, root, LayoutRequest{240, paragraph.FirstFit})
	if err != nil {
		t.Fatal(err)
	}
	refs.streams = append(refs.streams, recipe.streams...)
	runtime.GC()
	for _, w := range refs.paragraphs {
		if w.Value() == nil {
			t.Fatal("live session lost preparation")
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	retained := retainedPreparation{manifest: p, bank: d.Bank(), results: []*Result{first, second}}
	if keepClosed {
		retained.closed = s
	}
	return refs, retained
}

func TestRealFontResultsReleaseWorkingState(t *testing.T) {
	for _, manual := range []bool{false, true} {
		for _, keepClosed := range []bool{false, true} {
			name := "none"
			if manual {
				name = "manual"
			}
			if keepClosed {
				name += "/closed-wrapper"
			} else {
				name += "/dropped-wrapper"
			}
			t.Run(name, func(t *testing.T) {
				refs, retained := retainedRealPreparation(t, manual, keepClosed)
				for range 5 {
					runtime.GC()
				}
				if refs.backend.Value() != nil || refs.recipe.Value() != nil || refs.collector.Value() != nil {
					t.Fatal("output/source retains backend, discovery recipe or invocation")
				}
				if !keepClosed && refs.session.Value() != nil {
					t.Fatal("output/source retains session")
				}
				for _, w := range refs.groups {
					if w.Value() != nil {
						t.Fatal("closed session retains group handle")
					}
				}
				for _, w := range refs.paragraphs {
					if w.Value() != nil {
						t.Fatal("output retains paragraph preparation")
					}
				}
				for _, w := range refs.streams {
					if w.Value() != nil {
						t.Fatal("output retains stream workspace")
					}
				}
				item, ok := retained.bank.First("main")
				if !ok || !retained.manifest.Covers(item) {
					t.Fatal("source lifetime lost")
				}
				for _, result := range retained.results {
					proofs := realProofs(t, result)
					if len(proofs) == 0 || len(proofs[0].Glyphs) == 0 {
						t.Fatal("collected preparation took retained font resources with it")
					}
				}
				runtime.KeepAlive(retained)
			})
		}
	}
}
