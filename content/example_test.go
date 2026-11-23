package content_test

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/w3c-dom/dimen"
)

// Go runs examples in the package directory. These inputs are repository
// fixtures; loading never performs template-repository discovery or fetching.
func readExample(name string, opts content.LoadOptions) (*content.Document, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() // The caller, not content.Load, owns reader lifetime.
	opts.SourceName = name
	return content.Load(f, opts)
}

// The original sketch's content is loaded separately from its page templates.
func ExampleLoad() {
	doc, err := readExample("../template/testdata/content/article.html", content.LoadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, channel := range doc.Channels() {
		branches := doc.Branches(channel)
		fmt.Printf("%s: branches=%d groups=%d\n", channel, len(branches), len(doc.Groups(branches[0].Tree)))
	}
	residual := doc.Residual()
	root, _ := residual.Root.Info()
	fmt.Printf("residual: %s, %d group\n", root.Source.Path, len(doc.Groups(residual.Tree)))
	fmt.Printf("named registers: %d\n", doc.Bank().Len())
	// Output:
	// heading-flow: branches=1 groups=1
	// article-flow: branches=1 groups=2
	// residual: /html[1]/body[1], 1 group
	// named registers: 2
}

// A diagnostic report needs only copied public records and comparable handles.
// It uses source paths and local group ordinals instead of freezing BoxID values.
// The report is for humans, not a serialization or paragraph-extraction format.
func describeMaterial(doc *content.Document) string {
	var out strings.Builder
	fmt.Fprintf(&out, "channels: %s\n", strings.Join(doc.Channels(), ", "))
	groups := func(tree content.TreeID) {
		for i, g := range doc.Groups(tree) {
			var atomics []string
			for _, atom := range g.Atomics {
				atomics = append(atomics, boxLabel(doc, atom))
			}
			fmt.Fprintf(&out, "    group %d: owner=%s context=%s atomics=[%s] inside=%s\n",
				i+1, boxLabel(doc, g.Owner), g.Source.Path,
				strings.Join(atomics, ", "), boxLabel(doc, g.ParentAtomic))
		}
	}
	for _, channel := range doc.Channels() {
		branches := doc.Branches(channel)
		fmt.Fprintf(&out, "channel %q: branches=%d\n", channel, len(branches))
		for _, b := range branches {
			fmt.Fprintf(&out, "  branch %d: %s %s:%s\n", b.Order, boxLabel(doc, b.Root), b.Source.Name, b.Source.Path)
			groups(b.Tree)
		}
	}
	r := doc.Residual()
	if info, ok := r.Root.Info(); ok {
		fmt.Fprintf(&out, "residual: %s:%s\n", info.Source.Name, info.Source.Path)
	} else {
		fmt.Fprintln(&out, "residual: no material root")
	}
	groups(r.Tree)
	return out.String()
}

func boxLabel(doc *content.Document, box content.Box) string {
	info, ok := box.Info()
	if !ok {
		return "-"
	}
	if info.Source.NodeID == 0 {
		return "anonymous"
	}
	source, _ := doc.NodeSource(info.Source.NodeID)
	for _, attr := range source.Attributes {
		if attr.Namespace == "" && attr.Key == "id" {
			return "#" + attr.Val
		}
	}
	return info.Source.Path
}

func ExampleDocument() {
	doc, err := readExample("testdata/routing.html", content.LoadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print(describeMaterial(doc))
	// Output:
	// channels: article, notes
	// channel "article": branches=2
	//   branch 1: #story-a testdata/routing.html:/html[1]/body[1]/section[1]/article[1]
	//     group 1: owner=#paragraph-a context=/html[1]/body[1]/section[1]/article[1]/p[1] atomics=[#tag] inside=-
	//     group 2: owner=#tag context=/html[1]/body[1]/section[1]/article[1]/p[1]/span[1] atomics=[] inside=#tag
	//   branch 3: #story-b testdata/routing.html:/html[1]/body[1]/section[1]/article[2]
	//     group 1: owner=#paragraph-b context=/html[1]/body[1]/section[1]/article[2]/p[1] atomics=[] inside=-
	// channel "notes": branches=1
	//   branch 2: #note testdata/routing.html:/html[1]/body[1]/section[1]/aside[1]
	//     group 1: owner=#note context=/html[1]/body[1]/section[1]/aside[1] atomics=[] inside=-
	// residual: testdata/routing.html:/html[1]/body[1]
	//     group 1: owner=#intro context=/html[1]/body[1]/p[1] atomics=[] inside=-
	//     group 2: owner=#ending context=/html[1]/body[1]/section[1]/p[1] atomics=[] inside=-
}

func ExampleDocument_Bank() {
	doc, err := readExample("testdata/routing.html", content.LoadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	initial := doc.Bank()
	a, candidate, _ := initial.Take("article")
	b, drained, _ := candidate.Take("article")
	fmt.Printf("taken: %s, %s\n", boxLabel(doc, a), boxLabel(doc, b))
	missing, _, ok := drained.Take("article")
	fmt.Printf("article exhausted: %t; missing handle valid: %t\n", !ok, missing.Valid())
	note, _ := drained.First("notes")
	fmt.Printf("notes retained: %s\n", boxLabel(doc, note))
	// This restores a whole branch; it does not represent a partly fitted box.
	restored := candidate.Prepend("article", a)
	initialHead, _ := initial.First("article")
	candidateHead, _ := candidate.First("article")
	restoredHead, _ := restored.First("article")
	fmt.Printf("initial: %s\n", boxLabel(doc, initialHead))
	fmt.Printf("candidate: %s\n", boxLabel(doc, candidateHead))
	fmt.Printf("restored: %s\n", boxLabel(doc, restoredHead))
	fmt.Printf("source inventory: %s\n", strings.Join(doc.Channels(), ", "))
	// Output:
	// taken: #story-a, #story-b
	// article exhausted: true; missing handle valid: false
	// notes retained: #note
	// initial: #story-a
	// candidate: #story-b
	// restored: #story-a
	// source inventory: article, notes
}

func ExampleLoad_stylesheets() {
	base, err := readExample("testdata/routing.html", content.LoadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	css, err := os.ReadFile("testdata/routing-override.css")
	if err != nil {
		fmt.Println(err)
		return
	}
	override, err := readExample("testdata/routing.html", content.LoadOptions{
		Stylesheets: []content.Stylesheet{{SourceName: "routing-override.css", CSS: string(css)}},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("base channels: %s\n", strings.Join(base.Channels(), ", "))
	fmt.Printf("override channels: %s\n", strings.Join(override.Channels(), ", "))
	for _, doc := range []*content.Document{base, override} {
		b := doc.Branches(doc.Channels()[0])[1]
		info, _ := b.Root.Info()
		fmt.Printf("story-b: font=%.0fpx margin-left=%.0fpx width=%.0f%% color=%s\n",
			float64(info.TextStyle.FontSize)/float64(dimen.PX), info.Constraints.Margin.Left.Value,
			100*info.Constraints.Width.Value, info.TextStyle.Color)
	}
	// The second load has a new snapshot; base and its initial bank stay intact.
	// Width is still a percentage constraint, not measured geometry.

	// Output:
	// base channels: article, notes
	// override channels: features, notes
	// story-b: font=20px margin-left=20px width=75% color=#203040
	// story-b: font=24px margin-left=24px width=75% color=#008855
}

func ExampleLoadError() {
	doc, err := readExample("testdata/nested-flow_err.html", content.LoadOptions{})
	var failure *content.LoadError
	if !errors.As(err, &failure) {
		fmt.Printf("expected a load error, got %v\n", err)
		return
	}
	fmt.Printf("document published: %t\n", doc != nil)
	for _, d := range failure.Diagnostics() {
		fmt.Printf("%s: %s/%s\n", d.Source.Name, d.Stage, d.Code)
		fmt.Printf("  source: %s\n  related: %s\n", d.Source.Path, d.Related.Path)
	}
	// Output:
	// document published: false
	// testdata/nested-flow_err.html: routing/route.nested
	//   source: /html[1]/body[1]/article[2]/aside[1]
	//   related: /html[1]/body[1]/article[2]
}
