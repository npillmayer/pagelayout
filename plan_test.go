package pagelayout

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/npillmayer/pagelayout/content"
)

func sourceLabel(d *content.Document, s content.SourceRef) string {
	n, _ := d.NodeSource(s.NodeID)
	for _, a := range n.Attributes {
		if a.Key == "id" {
			return a.Val
		}
	}
	return n.Name
}
func fixturePlan(t *testing.T, file string, opts content.LoadOptions) (*content.Document, *SpanPlan, error) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata/pagination", file))
	if err != nil {
		t.Fatal(err)
	}
	opts.SourceName = file
	d, err := content.Load(strings.NewReader(string(b)), opts)
	if err != nil {
		return nil, nil, err
	}
	p, err := Plan(d)
	return d, p, err
}
func TestSourcePlanFixtures(t *testing.T) {
	var table struct {
		Cases []struct {
			ID, File, Gate string
			Stylesheets    []string
			CSS            string `json:"additional_css"`
			Expect         struct {
				Outcome     string
				Channels    []string
				Queues      map[string][]string
				Diagnostics []string            `json:"required_diagnostics"`
				SourceIDs   []string            `json:"source_ids"`
				Boundaries  []string            `json:"boundaries_before"`
				Boundary    string              `json:"boundary_before"`
				SpanTargets map[string][]string `json:"span_targets"`
				Owners      map[string]string
				Anchors     []struct {
					ID, Target, Attach, Block string
					Span                      int
				}
				Anchor    string
				Affinity  string
				Following string `json:"following_material"`
			}
		}
	}
	raw, err := os.ReadFile("testdata/pagination/source-expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	for _, tc := range table.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			opts := content.LoadOptions{}
			for _, f := range tc.Stylesheets {
				css, err := os.ReadFile(filepath.Join("testdata/pagination", f))
				if err != nil {
					t.Fatal(err)
				}
				opts.Stylesheets = append(opts.Stylesheets, content.Stylesheet{SourceName: f, CSS: string(css)})
			}
			if tc.CSS != "" {
				opts.Stylesheets = append(opts.Stylesheets, content.Stylesheet{SourceName: "additional.css", CSS: tc.CSS})
			}
			d, p, err := fixturePlan(t, tc.File, opts)
			if tc.Expect.Outcome == "error" {
				if err == nil || p != nil {
					t.Fatal("invalid plan published")
				}
				if tc.Gate == "SHIP-02" {
					var pe *PlanError
					if !errors.As(err, &pe) || !errors.Is(err, ErrInvalidPlan) {
						t.Fatal(err)
					}
					labels := map[string]bool{}
					for _, di := range pe.Diagnostics() {
						labels[sourceLabel(d, di.Source)] = true
						for _, related := range di.Related {
							labels[sourceLabel(d, related)] = true
						}
					}
					for _, label := range tc.Expect.SourceIDs {
						if !labels[label] {
							t.Fatal("lost diagnostic source", label, pe.Diagnostics())
						}
					}
					for _, code := range tc.Expect.Diagnostics {
						found := false
						for _, di := range pe.Diagnostics() {
							if di.Code == code {
								found = true
							}
						}
						if !found {
							t.Fatal("missing", code, err)
						}
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.Expect.Channels != nil && !reflect.DeepEqual(tc.Expect.Channels, d.Channels()) {
				t.Fatal("channels", d.Channels())
			}
			branches := map[BranchID]string{}
			for _, b := range p.Branches() {
				branches[b.ID] = sourceLabel(d, b.Material.Source)
			}
			for channel, want := range tc.Expect.Queues {
				var got []string
				for _, b := range d.Branches(channel) {
					got = append(got, sourceLabel(d, b.Source))
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatal("queue", channel, got, want)
				}
			}
			byAnchor := map[string]Anchor{}
			for _, a := range p.Anchors() {
				byAnchor[sourceLabel(d, a.Source)] = a
			}
			for _, want := range tc.Expect.Anchors {
				a, ok := byAnchor[want.ID]
				if !ok || string(a.Affinity) != want.Attach {
					t.Fatal("affinity", want.ID, a.Affinity, want.Attach)
				}
				if want.Target != "" && branches[a.Target] != want.Target {
					t.Fatal("target", a)
				}
				if want.Block != "" && sourceLabel(d, a.Block) != want.Block {
					t.Fatal("block", a)
				}
				if want.Span != 0 && a.Span != want.Span {
					t.Fatal("span", a)
				}
			}
			if tc.Expect.Anchor != "" {
				a := byAnchor[tc.Expect.Anchor]
				if string(a.Affinity) != tc.Expect.Affinity || sourceLabel(d, a.Position.Info().Source) != tc.Expect.Following {
					t.Fatal("split-inline mapping", a)
				}
			}
			for target, want := range tc.Expect.Owners {
				found := false
				for _, b := range p.Branches() {
					if branches[b.ID] == target {
						found = true
						if sourceLabel(d, b.Owner) != want {
							t.Fatal("owner", b)
						}
					}
				}
				if !found {
					t.Fatal("missing branch", target)
				}
			}
			for span, want := range tc.Expect.SpanTargets {
				i, _ := strconv.Atoi(span)
				var got []string
				for _, bid := range p.Spans()[i-1].Targets {
					got = append(got, branches[bid])
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatal("span targets", span, got, want)
				}
			}
			var boundaries []string
			for _, b := range p.Boundaries() {
				if !b.Initial && !b.Terminal {
					boundaries = append(boundaries, sourceLabel(d, b.Position.Info().Source))
				}
			}
			if tc.Expect.Boundaries != nil && !reflect.DeepEqual(boundaries, tc.Expect.Boundaries) {
				t.Fatal("boundaries", boundaries, tc.Expect.Boundaries)
			}
			if tc.Expect.Boundary != "" && !slices.Contains(boundaries, tc.Expect.Boundary) {
				t.Fatal("missing boundary", boundaries)
			}
			if tc.ID == "coincident-breaks" {
				count := 0
				for _, label := range boundaries {
					if label == "propagated" {
						count++
					}
				}
				if count != 1 {
					t.Fatal("coincident breaks", count)
				}
			}
			bs := p.Boundaries()
			if len(bs) == 0 || !bs[len(bs)-1].Terminal {
				t.Fatal("missing terminal drain")
			}
		})
	}
}

func TestPlanIdentityCopiesAndConcurrentInspection(t *testing.T) {
	d, p, err := fixturePlan(t, "content/transitive.html", content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := fixturePlan(t, "content/transitive.html", content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Matches(d) || p.Matches(other) || p.Matches(nil) {
		t.Fatal("identity")
	}
	wantA, wantB, wantS := p.Anchors(), p.Boundaries(), p.Spans()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				a := p.Anchors()
				a[0].Source.Name = "changed"
				a[0].Coverage[0].Start = 999
				b := p.Boundaries()
				b[0].Sources[0].Name = "changed"
				spans := p.Spans()
				spans[0].Targets[0] = 999
				branches := p.Branches()
				branches[0].Channel = "changed"
				copy := p.Document()
				*copy = content.Document{}
				if !reflect.DeepEqual(wantA, p.Anchors()) || !reflect.DeepEqual(wantB, p.Boundaries()) || !reflect.DeepEqual(wantS, p.Spans()) {
					t.Error("aliased plan")
				}
			}
		}()
	}
	wg.Wait()
	if _, err := Plan(nil); !errors.Is(err, ErrInvalidPlan) {
		t.Fatal(err)
	}
	if _, err := Plan(&content.Document{}); !errors.Is(err, ErrInvalidPlan) {
		t.Fatal(err)
	}
}

func TestPlanOwnerValidationAndDiagnostics(t *testing.T) {
	for _, href := range []string{"", "other.html#n", "https://example.com/#n", "#missing", "#%xx"} {
		source := "<body><a data-flow-anchor href=\"" + href + "\">owner</a><aside id=\"n\" style=\"flow-into:notes\">note</aside>"
		d, err := content.Load(strings.NewReader(source), content.LoadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		p, err := Plan(d)
		var pe *PlanError
		if p != nil || !errors.As(err, &pe) {
			t.Fatal("invalid href accepted", href, err)
		}
		ds := pe.Diagnostics()
		if len(ds) == 0 {
			t.Fatal("no diagnostics")
		}
		original := err.Error()
		ds[0].Code = "changed"
		if len(ds[0].Related) > 0 {
			ds[0].Related[0].Name = "changed"
		}
		if err.Error() != original {
			t.Fatal("aliased diagnostics")
		}
	}
	source := "<body><p><a data-flow-anchor href=\"#%6e\">owner</a><a id=\"x\" href=\"#y\">xref</a><a id=\"y\" href=\"#x\">xref</a></p><aside id=\"n\" style=\"flow-into:notes\">note</aside>"
	d, err := content.Load(strings.NewReader(source), content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Plan(d)
	if err != nil || len(p.Anchors()) != 1 {
		t.Fatal("decoded fragment / ordinary cycles", err)
	}
}

func TestPlanVisibleCoverageIsOneActivation(t *testing.T) {
	d, p, err := fixturePlan(t, "content/anchors.html", content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range p.Anchors() {
		if a.Affinity != content.AffinityVisible {
			continue
		}
		if len(a.Coverage) == 0 || !a.Position.Valid() {
			t.Fatal("missing visible coverage")
		}
		var bytes uint64
		for _, c := range a.Coverage {
			if c.Start >= c.End {
				t.Fatal("empty visible range")
			}
			bytes += c.End - c.Start
		}
		if sourceLabel(d, a.Source) == "visible-anchor" && bytes < 20 {
			t.Fatal("multiline coverage truncated", a)
		}
	}
	d, p, err = fixturePlan(t, "content/transitive.html", content.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Branches("main")) != 1 || len(p.Spans()) != 2 {
		t.Fatal("within-main span")
	}
	main, _ := d.Bank().First("main")
	if main != d.Residual().Root {
		t.Fatal("planning consumed main")
	}
	c, ok := p.Spans()[0].End.Compare(p.Spans()[1].Start)
	if !ok || c != 0 {
		t.Fatal("inexact span boundary")
	}
}
