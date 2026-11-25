package pagelayout_test

import (
	"fmt"
	"strings"

	"github.com/npillmayer/pagelayout"
	"github.com/npillmayer/pagelayout/content"
)

func ExamplePlan() {
	source := "<body><p>First chapter <a href=\"#note\" data-flow-anchor>1</a>.</p><section style=\"break-before:page\"><p>Second chapter.</p></section><aside id=\"note\" style=\"flow-into:notes\">Note text.</aside></body>"
	document, err := content.Load(strings.NewReader(source), content.LoadOptions{SourceName: "article.html"})
	if err != nil {
		panic(err)
	}
	plan, err := pagelayout.Plan(document)
	if err != nil {
		panic(err)
	}
	fmt.Println("anchors:", len(plan.Anchors()))
	for _, span := range plan.Spans() {
		fmt.Printf("span %d: %d secondary targets\n", span.ID, len(span.Targets))
	}
	fmt.Println("source matches:", plan.Matches(document))
	// Output:
	// anchors: 1
	// span 1: 1 secondary targets
	// span 2: 0 secondary targets
	// source matches: true
}
