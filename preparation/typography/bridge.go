// Package typography adapts detached content groups to reusable typography
// preparations. Call content.Prepare with a Backend constructed by New. Font
// selection and discovery belong to the caller; no default backend is installed.
// Sessions own executable preparations, while source/paint metadata stays copied
// and immutable. Intrinsic measurement and independent whole-tree layout reuse
// these handles. Page selection, fragmentation and bank commits remain separate.
package typography

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

// Config borrows an already constructed StyleMapper (glyphing.NewStyleMapper),
// a factory returning fresh exclusive streams, and an optional explicit-marker
// hyphenator. Source hyphens:manual requires Hyphenator even without markers;
// hyphens:none never uses it. No service is closed by this bridge. Keep services,
// selected faces and resources stable for preparations and retained results;
// serialize sessions sharing them. Factory failures own their own cleanup.
type Config struct {
	Resolver      *glyphing.StyleMapper
	NewShaper     func() (contracts.StreamingShaper, error)
	Hyphenator    khipu.Hyphenator
	BufferOptions contracts.ShapeBufferOptions
}

// Backend implements content.Preparer without retaining document/session state.
// Its zero value is invalid. Configuration is copied; services remain borrowed.
type Backend struct{ config Config }

var _ content.Preparer = (*Backend)(nil)

// New checks explicit services without glyph queries, shaping or font discovery.
// Group-specific policy checks happen in whole-document Preflight.
func New(config Config) (*Backend, error) {
	b := &Backend{config: config}
	if err := b.checkServices(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Backend) checkServices() error {
	fail := func(message string) error {
		return &content.PreparationError{Stage: content.StagePreflight, Err: fmt.Errorf("%w: %s", contracts.ErrIllegalArguments, message)}
	}
	if b == nil || b.config.Resolver == nil || b.config.NewShaper == nil {
		return fail("explicit style mapper and shaper factory required")
	}
	defaults := b.config.Resolver.Defaults()
	if nilService(defaults.Font) || defaults.EmSize <= 0 || defaults.EmSize >= dimen.Fil {
		return fail("style mapper must contain an explicit loaded font selection")
	}
	o := b.config.BufferOptions
	if o.Flush > contracts.ShapeFlushCluster || o.HighWatermark < 0 || o.LowWatermark < 0 || o.MaxBuffer < 0 {
		return fail("invalid shaping buffer options")
	}
	return nil
}

// Preflight validates every group's text policy and detached coordinates before
// any backend preparation. It performs no font resolution, coverage, scaling,
// stream creation or shaping. It retains neither inputs nor prepared groups.
func (b *Backend) Preflight(groups []content.GroupInput) error {
	if err := b.checkServices(); err != nil {
		return err
	}
	for _, g := range groups {
		if _, err := b.mapInput(g); err != nil {
			return err
		}
	}
	return nil
}

// PrepareGroup itemizes and base-shapes one group exactly once without dimensions.
// Use content.Prepare to ensure every group's preflight precedes all service work.
// The returned handle belongs only to its session; its concrete type stays private.
func (b *Backend) PrepareGroup(g content.GroupInput) (content.GroupPreparation, error) {
	if err := b.checkServices(); err != nil {
		return content.GroupPreparation{}, err
	}
	m, err := b.mapInput(g)
	if err != nil {
		return content.GroupPreparation{}, err
	}
	fail := func(stage content.PreparationStage, err error) (content.GroupPreparation, error) {
		return content.GroupPreparation{}, groupError(g, stage, err)
	}
	r := b.config.Resolver
	// Resolve the owner independently, including empty/object-only paragraphs.
	defaults, err := r.ResolveStyle(m.owner, r.Defaults())
	if err != nil {
		return fail("itemization", err)
	}
	text, bindings, err := r.ItemizeInline(g.Text, m.styles, m.anchors)
	if err != nil {
		return fail("itemization", err)
	}
	var hyphenator khipu.Hyphenator
	if m.policy == khipu.ExplicitHyphenation {
		hyphenator = b.config.Hyphenator
	}
	p, err := paragraph.Prepare(paragraph.Input{Text: text, Direction: m.owner.Direction, Markup: m.markup, Atomics: m.anchors},
		khipu.PrepareOptions{Hyphenation: m.policy}, khipu.Config{Hyphenator: hyphenator, Streaming: &khipu.StreamingConfig{
			Resolver: r, Defaults: defaults, NewShaper: b.config.NewShaper, BufferOptions: b.config.BufferOptions,
		}})
	if err != nil {
		return fail(content.StagePreparation, err)
	}
	h := &preparedGroup{input: copyInput(g), paragraph: p, bindings: slices.Clone(bindings), lineLayout: m.lineLayout}
	return content.GroupPreparation{Group: g.Group, Owner: g.Owner, Handle: h}, nil
}

// Only the session owns this handle. Execution borrows it under the session's
// access guard; no accessor exports paragraph.Prepared or its mutable discovery.
type preparedGroup struct {
	input      content.GroupInput
	paragraph  *paragraph.Prepared
	bindings   []glyphing.FontTextRun
	lineLayout paragraph.LineLayout
}

func (g *preparedGroup) Release() error {
	if g != nil {
		*g = preparedGroup{}
	}
	return nil
}

func copyInput(g content.GroupInput) content.GroupInput {
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
func nilService(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		return r.IsNil()
	}
	return false
}
func groupError(g content.GroupInput, stage content.PreparationStage, err error) *content.PreparationError {
	return &content.PreparationError{Stage: stage, Group: g.Group, Owner: g.Owner, Source: g.Source, Err: err}
}

// Use the first contributing text fragment for a known processed byte. Shared
// collapsed spaces preserve first-source ownership; markers use atomic provenance.
func inputError(g content.GroupInput, offset uint64, err error) *content.PreparationError {
	e := groupError(g, content.StagePreflight, err)
	for _, s := range g.TextSources {
		if s.Start <= offset && offset < s.End {
			e.Source = s.Source
			e.Range = s.Range
			e.HasRange = true
			return e
		}
	}
	for _, a := range g.Occurrences {
		if a.Start <= offset && offset < a.End {
			e.Source = a.Source
			return e
		}
	}
	return e
}
