package template

import (
	"fmt"
	"slices"
	"strings"
)

// Internal diagnostics retain private stage/code types. Public diagnostics are
// detached snapshots; codes and paths do not depend on human-readable messages.
type loadStage string

const (
	stageIO        loadStage = "io"
	stageParse     loadStage = "parse"
	stageDiscovery loadStage = "discovery"
	stageStructure loadStage = "structure"
	stageCSS       loadStage = "css"
	stageStyling   loadStage = "styling"
	stageProfile   loadStage = "profile"
	stageGeometry  loadStage = "geometry"
)

type diagnosticCode string

const (
	codeInputPath           diagnosticCode = "input.path"
	codeInputOpen           diagnosticCode = "input.open"
	codeInputClose          diagnosticCode = "input.close"
	codeNilInput            diagnosticCode = "input.nil"
	codeParseFailure        diagnosticCode = "input.parse"
	codeDocumentStructure   diagnosticCode = "collection.document-structure"
	codeEmptyCollection     diagnosticCode = "collection.empty"
	codeHeadContent         diagnosticCode = "collection.head-content"
	codeBodyContent         diagnosticCode = "collection.body-content"
	codeTemplateLocation    diagnosticCode = "template.location"
	codeMissingTemplateID   diagnosticCode = "template.id-missing"
	codeInvalidTemplateID   diagnosticCode = "template.id-invalid"
	codeDuplicateTemplateID diagnosticCode = "template.id-duplicate"
	codeTemplateContent     diagnosticCode = "template.content"
	codeMissingPage         diagnosticCode = "template.page-missing"
	codeMultiplePages       diagnosticCode = "template.page-multiple"
	codePageElement         diagnosticCode = "template.page-element"
	codePageContent         diagnosticCode = "page.content"
	codeFrameElement        diagnosticCode = "frame.element"
	codeFrameContent        diagnosticCode = "frame.content"
	codeInvalidElementID    diagnosticCode = "element.id-invalid"
	codeDuplicateElementID  diagnosticCode = "element.id-duplicate"
	codeStyleAttribute      diagnosticCode = "css.style-attribute"
	codeStyleRegistration   diagnosticCode = "css.registration"
	codeStyleFailure        diagnosticCode = "css.styling"
	codeProfileProperty     diagnosticCode = "profile.property"
	codeProfileValue        diagnosticCode = "profile.value"
	codeProfileBox          diagnosticCode = "profile.box"
	codeProfileSizing       diagnosticCode = "profile.sizing"
	codeProfileFlow         diagnosticCode = "profile.flow"
	codeGeometryInvalid     diagnosticCode = "geometry.invalid"
	codeGeometryResolve     diagnosticCode = "geometry.resolve"
	codeGeometryContent     diagnosticCode = "geometry.content-dependent"
	codeGeometryPosition    diagnosticCode = "geometry.position"
	codeGeometryReceiving   diagnosticCode = "geometry.receiving-size"
	codeGeometryBounds      diagnosticCode = "geometry.bounds"
	codeGeometryOverlap     diagnosticCode = "geometry.overlap"
)

// CSS locations refer to the style element's text or style attribute value.
// HTML locations remain source paths; rule/declaration ordinals are one-based.
type cssLocation struct {
	rule, declaration, line, column int
	property                        string
	inline                          bool
}

type diagnostic struct {
	stage   loadStage
	code    diagnosticCode
	source  sourceRef
	css     cssLocation
	message string
	cause   error
}

func (d diagnostic) Error() string {
	name := d.source.name
	if name == "" {
		name = "<input>"
	}
	where := name
	if d.source.path != "" {
		where += ": " + d.source.path
	}
	if d.source.templateID != "" {
		where += fmt.Sprintf(" (template %q)", d.source.templateID)
	}
	if d.css.line != 0 {
		where += fmt.Sprintf(" (CSS %d:%d", d.css.line, d.css.column)
		if d.css.inline {
			where += ", inline style"
		}
		if d.css.rule != 0 {
			where += fmt.Sprintf(", rule %d", d.css.rule)
		}
		if d.css.declaration != 0 {
			where += fmt.Sprintf(", declaration %d", d.css.declaration)
		}
		where += ")"
	} else if d.css.property != "" {
		where += fmt.Sprintf(" (CSS property %q)", d.css.property)
	}
	message := fmt.Sprintf("%s [%s/%s]: %s", where, d.stage, d.code, d.message)
	if d.cause != nil {
		message += ": " + d.cause.Error()
	}
	return message
}

func (d diagnostic) Unwrap() error { return d.cause }

// CSSLocation refers to a style element's text or a style attribute. Rule and
// Declaration are one-based ordinals; zero means unavailable/not applicable.
// Line/Column are CSS locations, not enclosing HTML locations. Effective-style
// and geometry diagnostics identify a node/property without inventing positions.
type CSSLocation struct {
	Rule, Declaration, Line, Column int
	Property                        string
	Inline                          bool
}

// Diagnostic is a value snapshot of one load failure. Stage and Code are stable
// machine-readable identifiers; Message is descriptive text. Unwrap preserves
// the original cause when available. Mutating a snapshot does not change its error.
type Diagnostic struct {
	Stage, Code string
	Source      SourceRef
	CSS         CSSLocation
	Message     string
	cause       error
}

func (d diagnostic) public() Diagnostic {
	return Diagnostic{Stage: string(d.stage), Code: string(d.code), Source: publicSource(d.source),
		CSS:     CSSLocation{Rule: d.css.rule, Declaration: d.css.declaration, Line: d.css.line, Column: d.css.column, Property: d.css.property, Inline: d.css.inline},
		Message: d.message, cause: d.cause}
}

// Error formats this diagnostic with source context and its original cause.
func (d Diagnostic) Error() string {
	return (diagnostic{stage: loadStage(d.Stage), code: diagnosticCode(d.Code),
		source: sourceRef{name: d.Source.Name, templateID: d.Source.TemplateID, path: d.Source.Path, nodeID: sourceNodeID(d.Source.NodeID)},
		css:    cssLocation{rule: d.CSS.Rule, declaration: d.CSS.Declaration, line: d.CSS.Line, column: d.CSS.Column, property: d.CSS.Property, inline: d.CSS.Inline}, message: d.Message, cause: d.cause}).Error()
}

// Unwrap returns the original filesystem, parser, reader, styling or geometry
// error, if any.
func (d Diagnostic) Unwrap() error { return d.cause }

// LoadError owns an ordered diagnostic snapshot. Use errors.As to inspect it;
// errors.Is/As also reach original causes through its diagnostic children.
// Diagnostics and Unwrap return copies, and expose no DOM or mutable style state.
type LoadError struct {
	items []diagnostic
}

func newLoadError(items []diagnostic) *LoadError {
	return &LoadError{items: slices.Clone(items)}
}

// Error formats diagnostics in source order, or search order for file lookup.
func (e *LoadError) Error() string {
	messages := make([]string, len(e.items))
	for i, d := range e.items {
		messages[i] = d.Error()
	}
	return strings.Join(messages, "\n")
}

func (e *LoadError) diagnostics() []diagnostic { return slices.Clone(e.items) }

// Diagnostics returns independent value snapshots in deterministic source order,
// or search order for file lookup. A close failure follows any load diagnostics.
func (e *LoadError) Diagnostics() []Diagnostic {
	ds := make([]Diagnostic, len(e.items))
	for i, d := range e.items {
		ds[i] = d.public()
	}
	return ds
}

// Unwrap returns diagnostic children without exposing the error's backing slice.
func (e *LoadError) Unwrap() []error {
	causes := make([]error, len(e.items))
	for i, d := range e.items {
		causes[i] = d.public()
	}
	return causes
}
