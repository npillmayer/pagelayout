package template

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadErrorPreservesContextAndCause(t *testing.T) {
	cause := errors.New("read interrupted")
	input := []diagnostic{{
		stage: stageParse, code: codeParseFailure,
		source:  sourceRef{name: "pages.html", templateID: "front", path: "/html[1]/body[1]/template[1]"},
		message: "cannot parse collection HTML", cause: cause,
	}}
	err := newLoadError(input)
	input[0].message = "changed by caller"
	copy := err.diagnostics()
	copy[0].source.name = "changed.html"
	children := err.Unwrap()
	children[0] = nil
	if !errors.Is(err, cause) {
		t.Fatal("original cause was lost")
	}
	var d diagnostic
	if !errors.As(err, &d) || d.code != codeParseFailure || d.source.templateID != "front" {
		t.Fatalf("structured diagnostic is not available through wrapping: %v", err)
	}
	message := err.Error()
	for _, want := range []string{"pages.html", "template \"front\"", "/html[1]/body[1]/template[1]", "parse/input.parse", "cannot parse collection HTML", "read interrupted"} {
		if !strings.Contains(message, want) {
			t.Errorf("diagnostic %q lacks %q", message, want)
		}
	}
	if strings.Contains(message, "changed") {
		t.Fatal("diagnostic slices alias caller storage")
	}
}

func TestDiagnosticWithoutSourceName(t *testing.T) {
	d := diagnostic{stage: stageParse, code: codeNilInput, message: "an HTML reader is required"}
	if got := d.Error(); got != "<input> [parse/input.nil]: an HTML reader is required" {
		t.Fatalf("diagnostic = %q", got)
	}
	if d.Unwrap() != nil {
		t.Fatal("unexpected cause")
	}
}
