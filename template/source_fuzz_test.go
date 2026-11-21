package template

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// Exercise parser recovery as well as validation: arbitrary HTML must either
// publish a complete source model or produce a structured error with no model.
func FuzzTemplateCollection(f *testing.F) {
	for _, name := range []string{"two-pages.html", "repaired.html"} {
		input, err := os.ReadFile("testdata/templates/" + name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(input))
	}
	f.Add("")
	f.Add(`<html><frameset><frame></frameset></html>`)
	f.Add(`<head><template id="a" data-type="page"><div></div></template></head>`)
	f.Add(`<body><template data-type="page" id="a"><div><div>&nbsp;</div></div></template>`)
	f.Add(`<body><template><template data-type="page" id="nested"><div></div></template></template>`)
	f.Fuzz(func(t *testing.T, input string) {
		c, err := parseTemplateCollection(strings.NewReader(input), "fuzz.html")
		if err != nil {
			var failure *loadError
			if c != nil || !errors.As(err, &failure) || len(failure.items) == 0 {
				t.Fatalf("partial model or unstructured failure: %v, %v", c, err)
			}
			return
		}
		if c == nil || len(c.templates) == 0 || len(c.byID) != len(c.templates) {
			t.Fatal("incomplete collection published")
		}
		for _, definition := range c.templates {
			if !validID(definition.id) || c.byID[definition.id] != definition || definition.element.Parent != c.body || definition.page.Parent != definition.element {
				t.Fatal("invalid definition published")
			}
			for i, frame := range definition.frames {
				if frame.id != frameID(i+1) || frame.element.Parent != definition.page {
					t.Fatal("invalid frame ownership published")
				}
				for node := frame.element.FirstChild; node != nil; node = node.NextSibling {
					if !isFormatting(node) {
						t.Fatal("nonempty receiving frame published")
					}
				}
			}
		}
		assertSourceIndex(t, c)
	})
}
