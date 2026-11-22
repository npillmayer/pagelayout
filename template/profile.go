package template

import (
	"fmt"
	"slices"
	"strings"

	"github.com/npillmayer/w3c-dom/style"
	"github.com/npillmayer/w3c-dom/style/cssom"
	"github.com/npillmayer/w3c-dom/style/display"
)

var physicalSides = []string{"top", "right", "bottom", "left"}

// profileProperties is the deliberately small authoring vocabulary. Reset-only
// background/border-image properties produced by supported shorthands are not
// additional authored effects. Unknown properties (including custom properties)
// must not silently disappear from the eventual compiled description.
var profileProperties = strings.Fields(`
display position float visibility box-sizing writing-mode direction
width height top right bottom left margin padding
margin-top margin-right margin-bottom margin-left
padding-top padding-right padding-bottom padding-left
border border-top border-right border-bottom border-left
border-width border-style border-color
border-top-width border-right-width border-bottom-width border-left-width
border-top-style border-right-style border-bottom-style border-left-style
border-top-color border-right-color border-bottom-color border-left-color
color background background-color flow-from flow-into`)

// These are the only values introduced by the reset-only parts of DOM-03's
// supported shorthands. Effective values are checked too, after inheritance.
var paintResets = strings.Fields(`background-image background-position background-size
background-repeat background-attachment background-origin background-clip
border-image-source border-image-slice border-image-width border-image-outset border-image-repeat`)

func validateProfileSources(c *styledCollection) error {
	var ds []diagnostic
	check := func(ref sourceRef, rule int, inline bool, declarations []cssom.SourceDeclaration) {
		for _, d := range declarations {
			code, err := checkProfileDeclaration(d.Property, d.Value)
			if err != nil {
				ds = append(ds, diagnostic{stage: stageProfile, code: code, source: ref,
					css:     cssLocation{rule: rule, declaration: d.Index, line: d.Line, column: d.Column, property: d.Property, inline: inline},
					message: "declaration is outside the template CSS profile", cause: err})
			}
		}
	}
	for _, sheet := range c.styles.sheets {
		for _, rule := range sheet.inspection.Rules {
			check(sheet.source, rule.Index, false, rule.Declarations)
		}
	}
	// Use the source tree's identities, not map order, for reproducible diagnostics.
	for _, ref := range c.source.sources {
		if declarations, ok := c.styles.inline[ref.nodeID]; ok {
			check(ref, 0, true, declarations)
		}
	}
	if len(ds) > 0 {
		return orderedStyleError(ds)
	}
	return nil
}

func checkProfileDeclaration(key string, value style.Property) (diagnosticCode, error) {
	if !slices.Contains(profileProperties, key) {
		return codeProfileProperty, fmt.Errorf("unsupported property %q", key)
	}
	// Reuse upstream expansion and checked parsers. This checks every original
	// declaration, not a second cascade or a parser for CSS source text.
	values := []style.KeyValue{{Key: key, Value: value}}
	if expanded, err := style.SplitCompoundProperty(key, value); err == nil {
		values = expanded
	}
	for _, kv := range values {
		if err := checkProfileValue(kv.Key, kv.Value); err != nil {
			return codeProfileValue, err
		}
	}
	return "", nil
}

func checkProfileValue(key string, value style.Property) error {
	if wide := style.DeclarationKeyword(string(value)); wide != "" {
		if wide == "initial" || wide == "inherit" || wide == "unset" {
			return nil
		}
		return fmt.Errorf("%s: cascade rollback is unsupported", key)
	}
	bad := func() error { return fmt.Errorf("unsupported %s: %s", key, value) }
	if slices.Contains(paintResets, key) {
		if value == style.InitialProperty(key) {
			return nil
		}
		return bad()
	}
	if key == "display" {
		v, err := display.Parse(string(value))
		if err != nil {
			return err
		}
		if !blockFlow(v) {
			return bad()
		}
		return nil
	}
	if key == "flow-from" || key == "flow-into" {
		_, err := style.ParseFlowName(string(value))
		return err
	}
	if key == "color" || key == "background-color" || strings.HasSuffix(key, "-color") {
		_, err := style.ParseColorValue(string(value))
		return err
	}
	switch key {
	case "position", "float", "visibility", "box-sizing", "writing-mode", "direction":
		v, err := style.ParseBoxKeyword(key, string(value))
		if err != nil {
			return err
		}
		if !profileKeyword(key, v) {
			return bad()
		}
		return nil
	}
	if strings.HasSuffix(key, "-style") {
		v, err := style.ParseBoxKeyword(key, string(value))
		if err != nil {
			return err
		}
		if v != "solid" && v != "none" && v != "hidden" {
			return bad()
		}
		return nil
	}
	v, err := style.ParseBoxLength(key, string(value))
	if err != nil {
		return err
	}
	if v.Kind == style.KeywordLength && v.Keyword == "auto" && (key == "width" || key == "height" || slices.Contains(physicalSides, key)) {
		return nil
	}
	if v.Kind != style.AbsoluteLength {
		return bad()
	}
	if strings.HasPrefix(key, "margin-") && v.Absolute != 0 {
		return bad()
	}
	return nil
}

func blockFlow(v display.Value) bool {
	return v.Kind() == display.Box && v.Outer() == display.Block && v.Inner() == display.Flow && !v.IsListItem()
}

func profileKeyword(key, value string) bool {
	switch key {
	case "position":
		return value == "static" || value == "fixed"
	case "float":
		return value == "none"
	case "visibility":
		return value == "visible"
	case "box-sizing":
		return value == "content-box"
	case "writing-mode":
		return value == "horizontal-tb"
	case "direction":
		return value == "ltr"
	}
	return false
}
