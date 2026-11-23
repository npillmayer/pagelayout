package content

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/npillmayer/w3c-dom/dimen"
	"golang.org/x/net/html"
)

// PREP-00 pins inputs and future acceptance results, not an extraction or
// preparation implementation. Check today's loader and the byte-coordinate
// consistency of the authored expectations; PREP-01/02 will test actual results.
type preparationFixture struct {
	File         string
	Channels     []string
	Branches     []string
	PrepareCalls int    `json:"prepare_calls"`
	PrepareError string `json:"prepare_error"`
	MeasureError string `json:"measure_error"`
	ErrorSource  string `json:"error_source"`
	Raw          map[string][]string
	Groups       []struct {
		Owner, Tree, Text string
		FontPx            int `json:"font_px"`
		Policy            string
		ParentAtomic      string `json:"parent_atomic"`
		Atomics           []struct {
			Owner  string
			Offset int
		}
		Spans []struct {
			Owner             string
			Node              int
			Source, Processed [2]int
			Kind              string
		}
	}
}

func TestPreparationAcceptanceInputs(t *testing.T) {
	data, err := os.ReadFile("testdata/preparation/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []preparationFixture
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.File, func(t *testing.T) {
			input, err := os.ReadFile("testdata/preparation/" + tc.File)
			if err != nil {
				t.Fatal(err)
			}
			d := loaded(t, string(input), LoadOptions{SourceName: tc.File})
			if !slices.Equal(d.Channels(), tc.Channels) {
				t.Fatal("channel inventory", d.Channels())
			}
			var branches []BranchInfo
			for _, channel := range d.Channels() {
				branches = append(branches, d.Branches(channel)...)
			}
			slices.SortFunc(branches, func(a, b BranchInfo) int { return a.Order - b.Order })
			if len(branches) != len(tc.Branches) {
				t.Fatal("branch count")
			}
			groups := d.Groups(d.Residual().Tree)
			trees := map[string]TreeID{"residual": d.Residual().Tree}
			for i, branch := range branches {
				source := materialSource(t, d, tc.Branches[i])
				if branch.Source != source.Source {
					t.Fatal("global branch order")
				}
				trees[tc.Branches[i]] = branch.Tree
				groups = append(groups, d.Groups(branch.Tree)...)
			}
			if len(groups) != len(tc.Groups) {
				t.Fatalf("groups = %d, want %d", len(groups), len(tc.Groups))
			}
			rawCoverage := make(map[string][][]int)
			for owner, want := range tc.Raw {
				for _, raw := range want {
					rawCoverage[owner] = append(rawCoverage[owner], make([]int, len(raw)))
				}
				s := materialSource(t, d, owner)
				var got []string
				for _, child := range s.Children {
					n, _ := d.NodeSource(child)
					if n.Kind == html.TextNode {
						// Text source IDs are private; use the retained text ledger
						// to verify raw input without inventing paragraph extraction.
						for _, text := range d.material.texts {
							if text.Source.NodeID == child {
								got = append(got, text.Text)
							}
						}
					}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("#%s decoded text = %q, want %q", owner, got, want)
				}
			}
			for i, want := range tc.Groups {
				g := groups[i]
				s := materialSource(t, d, want.Owner)
				info, _ := g.Group.Info()
				if g.Source != s.Source || info.Tree != trees[want.Tree] || g.TextStyle.FontSize != dimen.DU(want.FontPx)*dimen.PX || g.TextStyle.Hyphens != want.Policy {
					t.Fatalf("group %d owner/tree/context changed: %+v", i, g)
				}
				if want.ParentAtomic == "" {
					if g.ParentAtomic.Valid() {
						t.Fatal("unexpected parent atomic")
					}
				} else {
					parent, ok := g.ParentAtomic.Info()
					if !ok || parent.Source != materialSource(t, d, want.ParentAtomic).Source {
						t.Fatal("parent atomic")
					}
				}
				if len(g.Atomics) != len(want.Atomics) {
					t.Fatal("atomic count")
				}
				covered := make([]bool, len(want.Text))
				for j, atom := range want.Atomics {
					info, _ := g.Atomics[j].Info()
					if info.Source != materialSource(t, d, atom.Owner).Source {
						t.Fatal("atomic identity/order")
					}
					marker := fixtureRange(t, want.Text, [2]int{atom.Offset, atom.Offset + 3})
					if marker != "\ufffc" {
						t.Fatal("anchor not on marker")
					}
					for k := atom.Offset; k < atom.Offset+3; k++ {
						covered[k] = true
					}
				}
				for _, span := range want.Spans {
					if span.Node <= 0 || span.Node > len(tc.Raw[span.Owner]) {
						t.Fatal("unknown raw source")
					}
					raw := fixtureRange(t, tc.Raw[span.Owner][span.Node-1], span.Source)
					for k := span.Source[0]; k < span.Source[1]; k++ {
						rawCoverage[span.Owner][span.Node-1][k]++
					}
					processed := fixtureRange(t, want.Text, span.Processed)
					switch span.Kind {
					case "identity":
						if raw != processed {
							t.Fatal("identity source mapping", raw, processed)
						}
					case "collapsed", "omitted":
						if strings.Trim(raw, " \t\r\n\f") != "" || (span.Kind == "omitted" && processed != "") || (span.Kind == "collapsed" && processed != " ") {
							t.Fatal("whitespace source mapping")
						}
					default:
						t.Fatal("unknown span kind")
					}
					for k := span.Processed[0]; k < span.Processed[1]; k++ {
						covered[k] = true
					}
				}
				if slices.Contains(covered, false) {
					t.Fatal("expected output bytes lack source or atomic correspondence")
				}
			}
			for owner, nodes := range rawCoverage {
				for _, counts := range nodes {
					for _, count := range counts {
						if count != 1 {
							t.Fatalf("#%s raw bytes omitted or mapped twice in expectations", owner)
						}
					}
				}
			}
			if tc.PrepareError == "" {
				if tc.PrepareCalls != len(groups) {
					t.Fatal("expected one preparation per group")
				}
			} else {
				if tc.PrepareCalls != 0 {
					t.Fatal("policy preflight must precede all engine calls")
				}
				materialSource(t, d, tc.ErrorSource)
			}
		})
	}
}

func fixtureRange(t *testing.T, text string, r [2]int) string {
	t.Helper()
	if r[0] < 0 || r[1] < r[0] || r[1] > len(text) ||
		!utf8.ValidString(text[:r[0]]) || !utf8.ValidString(text[:r[1]]) {
		t.Fatalf("invalid UTF-8 fixture range %v in %q", r, text)
	}
	return text[r[0]:r[1]]
}
