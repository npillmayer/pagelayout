package content

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func fixture(t testing.TB, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func sourceByID(t testing.TB, source *sourceDocument, id string) *sourceNode {
	t.Helper()
	for _, n := range source.nodes[1:] {
		if value, ok := attribute(n.node, "id"); ok && value == id {
			return n
		}
	}
	t.Fatalf("missing source ID %q", id)
	return nil
}

func assertLedger(t testing.TB, source *sourceDocument) {
	t.Helper()
	paths := make(map[string]bool)
	var visit func(*html.Node, sourceID)
	var count sourceID
	visit = func(n *html.Node, parent sourceID) {
		count++
		record := source.byNode[n]
		if record == nil || record.ref.nodeID != count || record.parent != parent || source.nodes[count] != record || paths[record.ref.path] {
			t.Fatalf("invalid source ledger at %v", n)
		}
		paths[record.ref.path] = true
		if n.Type == html.TextNode && record.text != n.Data {
			t.Fatal("decoded text changed")
		}
		index := 0
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if index >= len(record.children) || record.children[index] != count+1 {
				t.Fatal("invalid source child index")
			}
			visit(child, record.ref.nodeID)
			index++
		}
		if index != len(record.children) {
			t.Fatal("extra child identity")
		}
	}
	visit(source.document, 0)
	if int(count)+1 != len(source.nodes) || int(count) != len(source.byNode) || source.nodes[0] != nil {
		t.Fatal("source index is incomplete")
	}
}

func TestParseSourceIdentitiesAndDecodedBytes(t *testing.T) {
	s, err := parseSource(strings.NewReader(fixture(t, "routing.html")), "routing.html")
	if err != nil {
		t.Fatal(err)
	}
	assertLedger(t, s)
	for id, path := range map[string]string{
		"source":  "/html[1]/body[1]/section[1]",
		"note":    "/html[1]/body[1]/section[1]/aside[1]",
		"story-b": "/html[1]/body[1]/section[1]/article[2]",
	} {
		n := sourceByID(t, s, id)
		if n.ref.name != "routing.html" || n.ref.path != path || n.kind != sourceContent {
			t.Errorf("%s: %+v", id, n)
		}
	}
	for id, want := range map[string]string{"intro": "Residual café & introduction.", "paragraph-b": "Second\u00a0article with a soft\u00adhyphen."} {
		n := sourceByID(t, s, id)
		text := s.byNode[n.node.FirstChild]
		if text.text != want || text.ref.path != n.ref.path+"/text()[1]" {
			t.Errorf("decoded %s: %+v", id, text)
		}
	}
	if len(s.byNode[sourceByID(t, s, "intro").node.FirstChild].text) != 30 || len(s.byNode[sourceByID(t, s, "paragraph-b").node.FirstChild].text) != 36 {
		t.Fatal("wrong decoded byte lengths")
	}
}

func TestParseRecoveryAndEmptyContent(t *testing.T) {
	for _, input := range []string{"", fixture(t, "empty.html"), `<p id="one">first<p id="two">second<!--tail-->`} {
		s, err := parseSource(strings.NewReader(input), "recovered.html")
		if err != nil || s.html == nil || s.head == nil || s.body == nil {
			t.Fatalf("recovery failed: %v", err)
		}
		assertLedger(t, s)
		if strings.Contains(input, `id="two"`) {
			one, two := sourceByID(t, s, "one"), sourceByID(t, s, "two")
			if one.node.Parent != s.body || two.node.Parent != s.body || two.ref.path != "/html[1]/body[1]/p[2]" {
				t.Fatal("source indices did not use the recovered tree")
			}
		}
	}
}

type readerFailure struct{}

func (*readerFailure) Error() string { return "source unavailable" }

type failingReader struct{ cause error }

func (r failingReader) Read(p []byte) (int, error) {
	return copy(p, `<html><body>partial`), r.cause
}

type ownedReader struct {
	io.Reader
	closed bool
}

func (r *ownedReader) Close() error { r.closed = true; return nil }

func TestInputFailuresPreserveOwnershipAndCauses(t *testing.T) {
	cause := &readerFailure{}
	for _, tc := range []struct {
		name string
		r    io.Reader
		code string
	}{
		{"nil", nil, "input.nil"},
		{"reader", failingReader{cause}, "input.read"},
		{"encoding", strings.NewReader("<p>\xff</p>"), "input.encoding"},
		{"frameset", strings.NewReader(`<frameset><frame src="unused"></frameset>`), "input.document-structure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var r io.Reader = tc.r
			var owned *ownedReader
			if r != nil {
				owned = &ownedReader{Reader: r}
				r = owned
			}
			s, err := loadSource(r, LoadOptions{SourceName: "bad.html"})
			ds := expectFailure(t, s, err)
			if len(ds) != 1 || ds[0].stage != "input" || ds[0].code != tc.code || ds[0].source.name != "bad.html" || ds[0].source.nodeID != 0 {
				t.Fatalf("wrong input diagnostics: %+v", ds)
			}
			if owned != nil && owned.closed {
				t.Error("loader closed caller's reader")
			}
			if tc.name == "reader" {
				var original *readerFailure
				if !errors.Is(err, cause) || !errors.As(err, &original) || original != cause {
					t.Fatal("original read failure lost")
				}
			}
		})
	}
	r := &ownedReader{Reader: strings.NewReader("<p>valid UTF-8 \ufffd</p>")}
	if _, err := loadSource(r, LoadOptions{}); err != nil || r.closed {
		t.Fatalf("valid source or reader ownership: %v", err)
	}
}

func TestSourceCategoriesRetainExcludedText(t *testing.T) {
	s, err := loadSource(strings.NewReader(fixture(t, "suppression.html")), LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for id, kind := range map[string]sourceKind{
		"visible": sourceContent, "suppressed": sourceContent, "suppressed-child": sourceContent,
		"inert": sourceInert, "stored": sourceInert, "metadata": sourceMetadata,
	} {
		n := sourceByID(t, s.source, id)
		if n.kind != kind {
			t.Errorf("%s category %v, want %v", id, n.kind, kind)
		}
		if got := s.nodes[n.ref.nodeID]; (got != nil) != (kind == sourceContent) {
			t.Errorf("%s styled participation is wrong", id)
		}
	}
	for id, want := range map[string]string{"stored": "Stored source text.", "suppressed-child": "Suppressed source text.", "metadata": `{"label":"Non-content metadata"}`} {
		n := sourceByID(t, s.source, id)
		if s.source.byNode[n.node.FirstChild].text != want {
			t.Errorf("excluded source text lost for %s", id)
		}
	}
	assertLedger(t, s.source)
}

func FuzzParseSource(f *testing.F) {
	for _, input := range []string{"", "\xff", fixture(f, "routing.html"), fixture(f, "suppression.html"), `<frameset><frame></frameset>`, `<p>one<p>two`} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		s, err := parseSource(strings.NewReader(input), "fuzz.html")
		if err != nil {
			var failure *loadError
			if s != nil || !errors.As(err, &failure) || len(failure.items) == 0 {
				t.Fatal("partial source or unstructured error")
			}
			return
		}
		assertLedger(t, s)
	})
}
