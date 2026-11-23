package typography

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/pagelayout/content"
	fontmeta "github.com/npillmayer/typography/font"
	"github.com/npillmayer/typography/fontresource"
	"github.com/npillmayer/typography/glyphing"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

type testFace struct{ metrics, scales atomic.Int64 }

func (f *testFace) Metrics() (contracts.FontMetrics, error) {
	f.metrics.Add(1)
	return helpers.SyntheticFont{}.Metrics()
}
func (f *testFace) Scale(size dimen.DU) (contracts.FontScale, error) {
	f.scales.Add(1)
	return helpers.SyntheticFont{}.Scale(size)
}

type shapeCall struct {
	text string
	size dimen.DU
}
type testServices struct {
	face                              *testFace
	coverage                          int
	covered                           []rune
	created                           int
	streams                           []*testStream
	mu                                sync.Mutex
	shapes                            []shapeCall
	failFactory, failCheck, failShape int
	failRune                          rune
	cause                             error
	hyphens                           int
}

func testBackend(t *testing.T) (*Backend, *testServices) {
	t.Helper()
	s := &testServices{face: &testFace{}, cause: errors.New("injected service failure")}
	resource, err := fontresource.Read(strings.NewReader("explicit synthetic fixture"), fontresource.Origin{Provider: "test", Name: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	fonts := glyphing.SelectedFonts{Default: resource.ID(), Size: 10 * dimen.BP, Faces: []glyphing.FaceEntry{{Face: s.face, Resource: resource, Info: fontmeta.Info{ResourceID: resource.ID(), SHA256: resource.SHA256(), Origin: resource.Origin()}}}}
	r, err := glyphing.NewStyleMapper(fonts, glyphing.CoverageFuncs{Check: func(f contracts.FontFace) error {
		if f != s.face {
			return contracts.ErrUnsupportedFontFace
		}
		return nil
	}, Lookup: func(_ contracts.FontFace, r rune) (uint32, error) {
		s.coverage++
		s.covered = append(s.covered, r)
		if r == s.failRune {
			return 0, s.cause
		}
		return uint32(r) + 1, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	newShaper := func() (contracts.StreamingShaper, error) {
		s.created++
		if s.created == s.failFactory {
			return nil, s.cause
		}
		inner, err := helpers.NewSyntheticStreamingShaper(8)
		if err != nil {
			return nil, err
		}
		stream := &testStream{StreamingShaper: inner, services: s, index: s.created}
		s.streams = append(s.streams, stream)
		return stream, nil
	}
	b, err := New(Config{Resolver: r, NewShaper: newShaper, Hyphenator: markerPolicy{s}})
	if err != nil {
		t.Fatal(err)
	}
	return b, s
}

type markerPolicy struct{ s *testServices }

func (h markerPolicy) Discretionaries(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	h.s.hyphens++
	if word != "\u00ad" {
		return nil, errors.New("expected explicit-marker request")
	}
	return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
}

type testStream struct {
	contracts.StreamingShaper
	services *testServices
	index    int
	finished atomic.Bool
	failed   atomic.Bool
}

func (s *testStream) CheckFace(f contracts.FontFace) error {
	if s.index == s.services.failCheck {
		return s.services.cause
	}
	if f != s.services.face {
		return contracts.ErrUnsupportedFontFace
	}
	return nil
}
func (s *testStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	if s.index == s.services.failShape {
		s.failed.Store(true)
		_ = s.StreamingShaper.Finish()
		return s.services.cause
	}
	reader := &recordingReader{RuneReader: r}
	size := p.Font.EmSize()
	// This explicit fake adapter accepts the counting face and delegates scalar
	// output to the synthetic stream using the same physical size.
	font, err := contracts.NewFontInstance(helpers.NewSyntheticFont(), size)
	if err != nil {
		return err
	}
	p.Font = font
	err = s.StreamingShaper.Shape(ctx, p, reader, o)
	s.services.mu.Lock()
	s.services.shapes = append(s.services.shapes, shapeCall{reader.text.String(), size})
	s.services.mu.Unlock()
	return err
}
func (s *testStream) Finish() error {
	s.finished.Store(true)
	err := s.StreamingShaper.Finish()
	if s.failed.Load() {
		return s.services.cause
	}
	return err
}
func (s *testStream) Err() error {
	if s.failed.Load() {
		return s.services.cause
	}
	return s.StreamingShaper.Err()
}

type recordingReader struct {
	contracts.RuneReader
	text strings.Builder
}

func (r *recordingReader) ReadRune() (rune, int, error) {
	c, n, err := r.RuneReader.ReadRune()
	if err == nil {
		r.text.WriteRune(c)
	}
	return c, n, err
}

type recordingBackend struct {
	*Backend
	calls   int
	handles []*preparedGroup
}

func (b *recordingBackend) PrepareGroup(g content.GroupInput) (content.GroupPreparation, error) {
	b.calls++
	r, err := b.Backend.PrepareGroup(g)
	if r.Handle != nil {
		b.handles = append(b.handles, r.Handle.(*preparedGroup))
	}
	return r, err
}
func load(t *testing.T, source string) *content.Document {
	t.Helper()
	d, err := content.Load(strings.NewReader(source), content.LoadOptions{SourceName: "test.html"})
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func fixture(t *testing.T, name string) *content.Document {
	t.Helper()
	text, err := os.ReadFile("../../content/testdata/preparation/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return load(t, string(text))
}
func checkFinished(t *testing.T, s *testServices) {
	t.Helper()
	for _, stream := range s.streams {
		if !stream.finished.Load() {
			t.Fatal("unfinished stream", stream.index)
		}
	}
}

func TestBridgeAcceptanceFixtures(t *testing.T) {
	data, err := os.ReadFile("../../content/testdata/preparation/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		File         string
		PrepareCalls int    `json:"prepare_calls"`
		PrepareError string `json:"prepare_error"`
		Groups       []struct{ Text string }
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.File, func(t *testing.T) {
			b, services := testBackend(t)
			rec := &recordingBackend{Backend: b}
			d := fixture(t, tc.File)
			p, session, err := content.Prepare(d, rec)
			if tc.PrepareError != "" {
				var e *content.PreparationError
				if p != nil || session != nil || !errors.Is(err, khipu.ErrUnsupportedPolicy) || !errors.As(err, &e) || e.Stage != content.StagePreflight || !e.HasRange {
					t.Fatal("policy preflight", err)
				}
				if services.created != 0 || services.coverage != 0 || services.face.scales.Load() != 0 || rec.calls != 0 {
					t.Fatal("policy failure called services")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if rec.calls != tc.PrepareCalls || services.created != tc.PrepareCalls || services.face.metrics.Load() != 0 || services.hyphens != 0 {
				t.Fatal("unexpected preparation/metric/discretionary calls", rec.calls, services.created, services.face.metrics.Load(), services.hyphens)
			}
			checkFinished(t, services)
			for i, g := range p.Groups() {
				input, ok := p.Input(g.Group)
				if !ok || input.Text != tc.Groups[i].Text || !reflect.DeepEqual(rec.handles[i].input, input) || rec.handles[i].paragraph == nil {
					t.Fatal("lost prepared source/atomic metadata")
				}
				for _, a := range input.Occurrences {
					for _, binding := range rec.handles[i].bindings {
						if binding.Start < a.End && a.Start < binding.End {
							t.Fatal("atomic marker received font binding")
						}
					}
				}
			}
			if err := session.Close(); err != nil {
				t.Fatal(err)
			}
			for _, h := range rec.handles {
				if h.paragraph != nil || h.input.Group.Valid() || h.bindings != nil {
					t.Fatal("release retained preparation")
				}
			}
			if len(p.Groups()) != tc.PrepareCalls {
				t.Fatal("closure invalidated manifest")
			}
		})
	}
}

func TestBridgeLateFailuresAndRetry(t *testing.T) {
	for _, mode := range []string{"itemization", "factory", "face", "shape"} {
		t.Run(mode, func(t *testing.T) {
			b, s := testBackend(t)
			d := fixture(t, "branches.html")
			rec := &recordingBackend{Backend: b}
			old, oldSession, err := content.Prepare(d, b)
			if err != nil {
				t.Fatal(err)
			}
			defer oldSession.Close()
			base := s.created
			switch mode {
			case "itemization":
				s.failRune = 'S'
			case "factory":
				s.failFactory = base + 4
			case "face":
				s.failCheck = base + 4
			case "shape":
				s.failShape = base + 4
			}
			p, session, err := content.Prepare(d, rec)
			var e *content.PreparationError
			if p != nil || session != nil || !errors.Is(err, s.cause) || !errors.As(err, &e) || e.Group != old.Groups()[3].Group {
				t.Fatal("failure publication/source", err)
			}
			if mode == "itemization" {
				if e.Stage != "itemization" {
					t.Fatal(e.Stage)
				}
			} else {
				var upstream *paragraph.Error
				if e.Stage != content.StagePreparation || !errors.As(err, &upstream) {
					t.Fatal("lost paragraph stage", err)
				}
			}
			checkFinished(t, s)
			if len(rec.handles) != 3 {
				t.Fatal("expected three prior successes")
			}
			for _, h := range rec.handles {
				if h.paragraph != nil {
					t.Fatal("failed pair retained owned paragraph")
				}
			}
			s.failRune = 0
			s.failFactory = 0
			s.failCheck = 0
			s.failShape = 0
			retry, session, err := content.Prepare(d, b)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			if retry.ID() == old.ID() {
				t.Fatal("reused preparation identity")
			}
			g := old.Groups()[0]
			if err := oldSession.WithGroup(old, g.Group, g.Owner, func(handle content.GroupHandle) error {
				if handle.(*preparedGroup).paragraph == nil {
					t.Fatal("old handle damaged")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBridgePoliciesAndExplicitServices(t *testing.T) {
	b, s := testBackend(t)
	for _, cfg := range []Config{{}, {Resolver: b.config.Resolver}, {NewShaper: b.config.NewShaper}, {Resolver: &glyphing.StyleMapper{}, NewShaper: b.config.NewShaper}} {
		if backend, err := New(cfg); backend != nil || err == nil {
			t.Fatal("implicit backend allowed")
		}
	}
	b.config.Hyphenator = nil
	for _, html := range []string{`<body><p>no marker but manual</p>`, `<body><p></p>`, `<body><p><span style="display:inline-block;hyphens:none"></span></p>`} {
		p, session, err := content.Prepare(load(t, html), b)
		if p != nil || session != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("missing manual hyphenator", err)
		}
	}
	if s.created != 0 || s.coverage != 0 || s.face.scales.Load() != 0 {
		t.Fatal("missing service invoked font work")
	}
	// Omitted policy and an atomic's policy do not select outer text policy.
	good := `<body style="hyphens:none"><p><span style="hyphens:auto"> </span>A<span style="display:inline-block;hyphens:manual"><span style="hyphens:none">B</span></span></p>`
	p, session, err := content.Prepare(load(t, good), b)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if len(p.Groups()) != 2 {
		t.Fatal("atomic interior missing")
	}
	base := s.created
	bad := `<body style="hyphens:none"><p>first</p><div style="flow-into:main">last<span style="hyphens:manual">manual</span></div>`
	p, failed, err := content.Prepare(load(t, bad), b)
	if p != nil || failed != nil || !errors.Is(err, khipu.ErrUnsupportedPolicy) || s.created != base {
		t.Fatal("mixed policy must fail before any group", err)
	}
}

func TestBridgeManualDiscoveryRemainsLazy(t *testing.T) {
	for _, name := range []string{"manual.html", "manual-many.html"} {
		t.Run(name, func(t *testing.T) {
			b, s := testBackend(t)
			p, session, err := content.Prepare(fixture(t, name), b)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			if s.hyphens != 0 || s.created != 1 {
				t.Fatal("preparation measured or discovered alternatives")
			}
			g := p.Groups()[0]
			err = session.WithGroup(p, g.Group, g.Owner, func(h content.GroupHandle) error {
				// Direct facade check, alongside PREP-03's public bridge tests.
				_, err := h.(*preparedGroup).paragraph.MeasureIntrinsic(khipu.MeasureRequest{})
				return err
			})
			if name == "manual-many.html" {
				if !errors.Is(err, khipu.ErrUnsupportedPolicy) {
					t.Fatal("upstream restriction hidden", err)
				}
			} else {
				if err != nil || s.hyphens != 1 {
					t.Fatal("manual protocol", err, s.hyphens)
				}
			}
			checkFinished(t, s)
		})
	}
	// NoHyphenation preserves authored bytes but never invokes the supplied policy.
	b, s := testBackend(t)
	p, session, err := content.Prepare(load(t, `<body style="hyphens:none">co&shy;operate`), b)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	g := p.Groups()[0]
	err = session.WithGroup(p, g.Group, g.Owner, func(h content.GroupHandle) error {
		_, err := h.(*preparedGroup).paragraph.MeasureIntrinsic(khipu.MeasureRequest{})
		return err
	})
	input, _ := p.Input(g.Group)
	if err != nil || s.hyphens != 0 || input.Text != "co\u00adoperate" {
		t.Fatal("disabled policy lost source", err)
	}
}
