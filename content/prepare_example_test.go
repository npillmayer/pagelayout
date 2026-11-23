package content_test

import (
	"fmt"
	"strings"

	"github.com/npillmayer/pagelayout/content"
)

// This example backend records text only; it is not a typography implementation.
type recordingPreparer struct{}
type recordedGroup struct{ text string }

func (recordingPreparer) Preflight([]content.GroupInput) error { return nil }
func (recordingPreparer) PrepareGroup(g content.GroupInput) (content.GroupPreparation, error) {
	return content.GroupPreparation{Group: g.Group, Owner: g.Owner, Handle: &recordedGroup{text: g.Text}}, nil
}
func (g *recordedGroup) Release() error { g.text = ""; return nil }

func ExamplePrepare() {
	doc, err := content.Load(strings.NewReader(`<body style="hyphens:none"><p>Hello <span style="display:inline-block">inside</span>.</p>`), content.LoadOptions{})
	if err != nil {
		panic(err)
	}
	prepared, session, err := content.Prepare(doc, recordingPreparer{})
	if err != nil {
		panic(err)
	}
	defer session.Close()
	groups := prepared.Groups()
	fmt.Println("groups:", len(groups))
	for _, group := range groups {
		input, _ := prepared.Input(group.Group)
		fmt.Printf("text=%q atomics=%d\n", input.Text, len(input.Occurrences))
	}
	group := groups[0]
	var retained string
	err = session.WithGroup(prepared, group.Group, group.Owner, func(handle content.GroupHandle) error {
		// A real bridge executes typed backend operations here and retains only
		// their frozen results, never the borrowed preparation handle itself.
		retained = handle.(*recordedGroup).text
		return nil
	})
	if err != nil {
		panic(err)
	}
	if err := session.Close(); err != nil {
		panic(err)
	}
	input, _ := prepared.Input(group.Group)
	fmt.Println("retained after close:", input.Text == retained)
	// Output:
	// groups: 2
	// text="Hello ￼." atomics=1
	// text="inside" atomics=0
	// retained after close: true
}
