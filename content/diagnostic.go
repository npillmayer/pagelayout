package content

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Working diagnostics keep original source coordinates, never frame IDs.
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

// LoadError owns ordered diagnostics. Use errors.As to inspect it; errors.Is/As
// also reach original causes through its diagnostic children.
type LoadError struct{ items []diagnostic }

// newLoadError takes an independent snapshot, ordered first by original HTML
// preorder, then by additional sheet order. CSS coordinates order checks within
// a source; stable ties retain check order. A later invalid phase need not run.
func newLoadError(items []diagnostic) *LoadError {
	ds := slices.Clone(items)
	slices.SortStableFunc(ds, func(a, b diagnostic) int {
		return cmp.Or(cmp.Compare(a.source.stylesheet, b.source.stylesheet),
			cmp.Compare(a.source.nodeID, b.source.nodeID),
			cmp.Compare(a.css.line, b.css.line), cmp.Compare(a.css.column, b.css.column))
	})
	return &LoadError{items: ds}
}

// Error formats the ordered loading diagnostics.
func (e *LoadError) Error() string {
	messages := make([]string, len(e.items))
	for i, d := range e.items {
		messages[i] = d.Error()
	}
	return strings.Join(messages, "\n")
}

func (e *LoadError) diagnostics() []diagnostic { return slices.Clone(e.items) }

// Diagnostics returns copied records in HTML source order, then additional
// stylesheet order. Failure stops the pipeline; later stages need not run.
func (e *LoadError) Diagnostics() []Diagnostic {
	result := make([]Diagnostic, len(e.items))
	for i, d := range e.items {
		result[i] = d.public()
	}
	return result
}

// Unwrap returns independent diagnostic children preserving upstream causes.
func (e *LoadError) Unwrap() []error {
	children := make([]error, len(e.items))
	for i, d := range e.items {
		children[i] = d.public()
	}
	return children
}

// CSSLocation refers to CSS text, not HTML offsets. Rule and Declaration are
// one-based ordinals; zero coordinates mean unavailable/not applicable.
type CSSLocation struct {
	Rule, Declaration, Line, Column int
	Property, Value                 string
	Inline                          bool
}

// Diagnostic is a copied load failure. Stage and Code identify the check;
// Message is descriptive prose. Related locates a conflicting route root or
// ancestor when present. Source has no invented node identity for external CSS.
type Diagnostic struct {
	Stage, Code     string
	Source, Related SourceRef
	CSS             CSSLocation
	Message         string
	cause           error
}

func (d diagnostic) public() Diagnostic {
	return Diagnostic{Stage: d.stage, Code: d.code, Source: publicSource(d.source),
		Related: publicSource(d.related), Message: d.message, cause: d.cause,
		CSS: CSSLocation{Rule: d.css.rule, Declaration: d.css.declaration,
			Line: d.css.line, Column: d.css.column, Property: d.css.property,
			Value: d.css.value, Inline: d.css.inline}}
}

// Error formats a diagnostic with its original source and cause.
func (d Diagnostic) Error() string {
	return (diagnostic{stage: d.Stage, code: d.Code,
		source: sourceRef{name: d.Source.Name, nodeID: sourceID(d.Source.NodeID), path: d.Source.Path},
		css: cssLocation{rule: d.CSS.Rule, declaration: d.CSS.Declaration, line: d.CSS.Line,
			column: d.CSS.Column, property: d.CSS.Property, value: d.CSS.Value, inline: d.CSS.Inline},
		message: d.Message, cause: d.cause}).Error()
}

// Unwrap returns the original reader, parser, styling or construction cause.
func (d Diagnostic) Unwrap() error { return d.cause }
