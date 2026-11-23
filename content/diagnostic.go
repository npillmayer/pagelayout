package content

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Source and diagnostic working types stay private until the complete material
// API is published. Coordinates are original source coordinates, never frame IDs.
type sourceRef struct {
	name       string
	nodeID     sourceID
	path       string
	stylesheet int // One-based additional sheet order; zero for HTML sources.
}

type cssLocation struct {
	rule, declaration, line, column int
	property, value                 string
	inline                          bool
}

type diagnostic struct {
	stage, code string
	source      sourceRef
	related     sourceRef // Conflicting route root/ancestor, when applicable.
	css         cssLocation
	message     string
	cause       error
}

func (d diagnostic) Error() string {
	name := d.source.name
	if name == "" {
		name = "<input>"
	}
	if d.source.path != "" {
		name += ": " + d.source.path
	}
	if d.css.line != 0 {
		name += fmt.Sprintf(" (CSS %d:%d", d.css.line, d.css.column)
		if d.css.inline {
			name += ", inline style"
		}
		name += ")"
	}
	if d.css.property != "" {
		name += fmt.Sprintf(" (property %q)", d.css.property)
	}
	message := fmt.Sprintf("%s [%s/%s]: %s", name, d.stage, d.code, d.message)
	if d.cause != nil {
		message += ": " + d.cause.Error()
	}
	return message
}

func (d diagnostic) Unwrap() error { return d.cause }

type loadError struct{ items []diagnostic }

// newLoadError takes an independent snapshot, ordered first by original HTML
// preorder, then by additional sheet order. CSS coordinates order checks within
// a source; stable ties retain check order. A later invalid phase need not run.
func newLoadError(items []diagnostic) *loadError {
	ds := slices.Clone(items)
	slices.SortStableFunc(ds, func(a, b diagnostic) int {
		return cmp.Or(cmp.Compare(a.source.stylesheet, b.source.stylesheet),
			cmp.Compare(a.source.nodeID, b.source.nodeID),
			cmp.Compare(a.css.line, b.css.line), cmp.Compare(a.css.column, b.css.column))
	})
	return &loadError{items: ds}
}

func (e *loadError) Error() string {
	messages := make([]string, len(e.items))
	for i, d := range e.items {
		messages[i] = d.Error()
	}
	return strings.Join(messages, "\n")
}

func (e *loadError) diagnostics() []diagnostic { return slices.Clone(e.items) }

func (e *loadError) Unwrap() []error {
	children := make([]error, len(e.items))
	for i, d := range e.items {
		children[i] = d
	}
	return children
}
