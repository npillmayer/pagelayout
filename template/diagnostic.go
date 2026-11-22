package template

import (
	"fmt"
	"slices"
	"strings"
)

// These types remain private until the complete loading API is ready. Codes and
// paths identify failures independently of the human-readable message.
type loadStage string

const (
	stageParse     loadStage = "parse"
	stageDiscovery loadStage = "discovery"
	stageStructure loadStage = "structure"
	stageCSS       loadStage = "css"
	stageStyling   loadStage = "styling"
)

type diagnosticCode string

const (
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
	}
	message := fmt.Sprintf("%s [%s/%s]: %s", where, d.stage, d.code, d.message)
	if d.cause != nil {
		message += ": " + d.cause.Error()
	}
	return message
}

func (d diagnostic) Unwrap() error { return d.cause }

// loadError owns an ordered snapshot. Neither diagnostics nor Unwrap exposes its
// backing slice. Each diagnostic wraps its original cause, if one exists.
type loadError struct {
	items []diagnostic
}

func newLoadError(items []diagnostic) *loadError {
	return &loadError{items: slices.Clone(items)}
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
	causes := make([]error, len(e.items))
	for i, d := range e.items {
		causes[i] = d
	}
	return causes
}
