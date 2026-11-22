package content

import (
	"fmt"
	"slices"
	"strings"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/w3c-dom/style"
	"github.com/npillmayer/w3c-dom/style/cssom"
	"github.com/npillmayer/w3c-dom/style/display"
	"github.com/npillmayer/w3c-dom/style/textstyle"
)

// This is the content authoring vocabulary, independent of the template profile.
// Check every declaration, including losing/nonmatching and suppressed sources.
var profileProperties = strings.Fields(`
display position float visibility box-sizing writing-mode direction flow-into flow-from
width height min-width min-height max-width max-height top right bottom left
margin padding margin-top margin-right margin-bottom margin-left
padding-top padding-right padding-bottom padding-left
border border-top border-right border-bottom border-left border-width border-style border-color
border-top-width border-right-width border-bottom-width border-left-width
border-top-style border-right-style border-bottom-style border-left-style
border-top-color border-right-color border-bottom-color border-left-color
color background background-color font-family font-size font-weight font-style
white-space line-height unicode-bidi hyphens word-spacing letter-spacing word-break overflow-wrap`)

// Only shorthand expansion introduces these reset longhands. They do not add
// support for authored image resources, background positioning or border images.
var paintResets = strings.Fields(`background-image background-position background-size
background-repeat background-attachment background-origin background-clip
border-image-source border-image-slice border-image-width border-image-outset border-image-repeat`)

func validateProfileSources(s *styledSource) error {
	var ds []diagnostic
	check := func(ref sourceRef, rule int, inline bool, declarations []cssom.SourceDeclaration) {
		for _, d := range declarations {
			code, err := checkProfileDeclaration(d.Property, d.Value)
			if err != nil {
				ds = append(ds, diagnostic{stage: "profile", code: code, source: ref,
					css:     cssLocation{rule: rule, declaration: d.Index, line: d.Line, column: d.Column, property: d.Property, value: string(d.Value), inline: inline},
					message: "declaration is outside the content CSS profile", cause: err})
			}
		}
	}
	for _, sheet := range s.styles.sheets {
		for _, rule := range sheet.inspection.Rules {
			check(sheet.source, rule.Index, false, rule.Declarations)
		}
	}
	for _, n := range s.source.nodes[1:] {
		check(n.ref, 0, true, s.styles.inline[n.ref.nodeID])
	}
	if len(ds) > 0 {
		return newLoadError(ds)
	}
	return nil
}

func checkProfileDeclaration(key string, value style.Property) (string, error) {
	if !slices.Contains(profileProperties, key) {
		return "profile.property", fmt.Errorf("unsupported content property %q", key)
	}
	values := []style.KeyValue{{Key: key, Value: value}}
	if expanded, err := style.SplitCompoundProperty(key, value); err == nil {
		values = expanded
	}
	for _, v := range values {
		if err := checkProfileValue(v.Key, v.Value); err != nil {
			return "profile.value", err
		}
	}
	return "", nil
}

func checkProfileValue(key string, value style.Property) error {
	if wide := style.DeclarationKeyword(string(value)); wide != "" {
		if wide == "initial" || wide == "inherit" || wide == "unset" {
			return nil
		}
		return fmt.Errorf("%s: unsupported cascade rollback", key)
	}
	bad := func() error { return fmt.Errorf("unsupported %s: %s", key, value) }
	if slices.Contains(paintResets, key) {
		if value == style.InitialProperty(key) {
			return nil
		}
		return bad()
	}
	if key == "display" {
		d, err := display.Parse(string(value))
		if err != nil {
			return err
		}
		if d.Kind() == display.None {
			return nil
		}
		_, err = projectDisplay(d)
		return err
	}
	if key == "flow-into" || key == "flow-from" {
		name, err := style.ParseFlowName(string(value))
		if err == nil && key == "flow-from" && name != "" {
			return bad()
		}
		return err
	}
	if key == "color" || key == "background-color" || strings.HasSuffix(key, "-color") {
		_, err := style.ParseColorValue(string(value))
		return err
	}
	switch key {
	case "position", "float", "visibility", "box-sizing", "writing-mode", "direction":
		word, err := style.ParseBoxKeyword(key, string(value))
		if err != nil {
			return err
		}
		if !profileKeyword(key, word) {
			return bad()
		}
		return nil
	case "font-family", "font-size", "font-weight", "font-style":
		return textstyle.ValidateDeclaration(key, string(value))
	case "white-space", "line-height", "unicode-bidi", "hyphens", "word-spacing", "letter-spacing", "word-break", "overflow-wrap":
		word, err := textstyle.ParseKeyword(key, string(value))
		if err != nil {
			return err
		}
		if key == "hyphens" || word == "normal" || key == "unicode-bidi" && word == "isolate" {
			return nil
		}
		return bad()
	}
	if strings.HasSuffix(key, "-style") {
		word, err := style.ParseBoxKeyword(key, string(value))
		if err != nil {
			return err
		}
		if word == "none" || word == "hidden" || word == "solid" {
			return nil
		}
		return bad()
	}
	l, err := style.ParseBoxLength(key, string(value))
	if err != nil {
		return err
	}
	switch key {
	case "height", "min-width", "min-height", "top", "right", "bottom", "left":
		if l.Kind == style.KeywordLength && l.Keyword == "auto" {
			return nil
		}
		return bad()
	case "max-width", "max-height":
		if l.Kind == style.KeywordLength && l.Keyword == "none" {
			return nil
		}
		return bad()
	}
	if l.Kind == style.AbsoluteLength || l.Kind == style.PercentageLength {
		return nil
	}
	if l.Kind == style.RelativeLength && (l.Unit == "em" || l.Unit == "rem") {
		return nil
	}
	if l.Kind == style.KeywordLength && l.Keyword == "auto" && (key == "width" || strings.HasPrefix(key, "margin-")) {
		return nil
	}
	return bad()
}

func profileKeyword(key, value string) bool {
	switch key {
	case "position":
		return value == "static"
	case "float":
		return value == "none"
	case "visibility":
		return value == "visible"
	case "box-sizing":
		return value == "content-box"
	case "writing-mode":
		return value == "horizontal-tb"
	case "direction":
		return value == "ltr" || value == "rtl"
	}
	return false
}

func projectDisplay(v display.Value) (layout.Display, error) {
	bad := func() (layout.Display, error) {
		return layout.Display{}, fmt.Errorf("unsupported content display: %s", v.CSS())
	}
	if v.Kind() != display.Box || v.IsListItem() {
		return bad()
	}
	d := layout.Display{Kind: layout.DisplayBox}
	switch v.Outer() {
	case display.Block:
		d.Outer = layout.OuterBlock
	case display.Inline:
		d.Outer = layout.OuterInline
	default:
		return bad()
	}
	switch v.Inner() {
	case display.Flow:
		d.Inner = layout.InnerFlow
	case display.FlowRoot:
		d.Inner = layout.InnerFlowRoot
	default:
		return bad()
	}
	return d, nil
}
