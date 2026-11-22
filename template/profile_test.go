package template

import (
	"errors"
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/w3c-dom/style"
)

func profileHTML(head, page, frame string) string {
	return fmt.Sprintf(`<html><head><style>
.page { width:300px; height:200px }
.frame { position:fixed; left:0; top:0; width:80px; height:30px; flow-from:Main }
%s</style></head><body><template id="a" data-type="page"><div class="page" style="%s"><div class="frame" style="%s"></div></div></template></body></html>`, head, page, frame)
}

func validateHTML(t *testing.T, input string) (*validatedCollection, error) {
	t.Helper()
	s := styleCollectionOK(t, parseCollectionOK(t, input))
	return validateTemplateCollection(s)
}

func validatedHTML(t *testing.T, input string) *validatedCollection {
	t.Helper()
	v, err := validateHTML(t, input)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestTypedTemplateDefaultsAndPaint(t *testing.T) {
	v := validatedHTML(t, profileHTML("", "", ""))
	a := v.byID["a"]
	if len(v.templates) != 1 || v.templates[0] != a || a.page.width != 300 || a.page.height != 200 || a.page.paint.background != (color.NRGBA{}) {
		t.Fatalf("wrong page: %+v", a)
	}
	f := a.frames[0]
	s := f.constraints
	if f.id != 1 || f.flow != "Main" || f.source.templateID != "a" || f.source.nodeID == 0 || !strings.Contains(f.source.path, "template[1]") {
		t.Fatalf("lost identity: %+v", f)
	}
	if s.Position != layout.PositionFixed || s.Width != (layout.Length{Kind: layout.LenPx, Value: 80}) || s.Fixed.Height.Value != 30 {
		t.Fatalf("wrong sizes: %+v", s)
	}
	if s.Fixed.Insets.Left != (layout.Length{Kind: layout.LenPx}) || s.Fixed.Insets.Top != (layout.Length{Kind: layout.LenPx}) || s.Fixed.Insets.Right.Kind != layout.LenAuto || s.Fixed.Insets.Bottom.Kind != layout.LenAuto {
		t.Fatalf("auto and zero confused: %+v", s.Fixed.Insets)
	}
	if s.Padding != (layout.EdgeLengths{}) || s.Border != (layout.EdgeLengths{}) || s.Margin != (layout.EdgeLengths{}) || s.FontSizePx != 0 {
		t.Fatalf("invented edges/font context: %+v", s)
	}
	for _, b := range f.paint.borders {
		if b.style != "none" || b.color != (color.NRGBA{A: 255}) {
			t.Fatalf("wrong initial border: %+v", b)
		}
	}

	v = validatedHTML(t, profileHTML(`html {color:rgb(255, 0, 0)}`, "background:#ffffff", `padding:3px 4px 5px 6px;border:2px solid;background:currentcolor;border-left-color:transparent`))
	f = v.templates[0].frames[0]
	if f.constraints.Padding != (layout.EdgeLengths{Top: px(3), Right: px(4), Bottom: px(5), Left: px(6)}) || f.constraints.Border.Left.Value != 2 {
		t.Fatalf("edges: %+v", f.constraints)
	}
	if f.paint.background != (color.NRGBA{R: 255, A: 255}) || f.paint.borders[0].color != f.paint.background || f.paint.borders[3].color != (color.NRGBA{}) {
		t.Fatalf("paint: %+v", f.paint)
	}
	if v.templates[0].page.paint.background != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatal("lost shorthand background")
	}
}

func px(value float32) layout.Length { return layout.Length{Kind: layout.LenPx, Value: value} }

func TestTypedAbsoluteUnitsAndStretch(t *testing.T) {
	for _, unit := range []string{"96px", "72pt", "1in", "2.54cm", "25.4mm", "6pc", "101.6Q"} {
		t.Run(unit, func(t *testing.T) {
			v := validatedHTML(t, profileHTML("", "width:"+unit+";height:"+unit, "width:"+unit+";left:auto;right:-.5px"))
			if v.templates[0].page.width != 96 || v.templates[0].page.height != 96 || v.templates[0].frames[0].constraints.Width.Value != 96 {
				t.Fatal("absolute units were not converted from DU to CSS pixels")
			}
			f := v.templates[0].frames[0].constraints.Fixed
			if f.Insets.Right.Value != -.5 || f.Insets.Left.Kind != layout.LenAuto {
				t.Fatalf("lost anchoring: %+v", f)
			}
		})
	}
	v := validatedHTML(t, profileHTML("", "", `width:auto;height:auto;left:10px;right:20px;top:30px;bottom:40px`))
	s := v.templates[0].frames[0].constraints
	if s.Width.Kind != layout.LenAuto || s.Fixed.Height.Kind != layout.LenAuto || s.Fixed.Insets.Right.Value != 20 || s.Fixed.Insets.Bottom.Value != 40 {
		t.Fatalf("stretch resolved prematurely: %+v", s)
	}
	// Overconstraints and out-of-page boxes are left to the geometry service.
	validatedHTML(t, profileHTML("", "", `left:500px;right:20px;width:80px`))
}

func TestProfileRejectsUnsupportedSource(t *testing.T) {
	for _, tc := range []struct {
		decl, key string
		code      diagnosticCode
	}{
		{"transform:none", "transform", codeProfileProperty}, {"overflow:hidden", "overflow", codeProfileProperty},
		{"clip-path:none", "clip-path", codeProfileProperty}, {"opacity:.5", "opacity", codeProfileProperty},
		{"background-image:none", "background-image", codeProfileProperty}, {"border-image-source:url(a)", "border-image-source", codeProfileProperty},
		{"content:'x'", "content", codeProfileProperty}, {"min-width:0", "min-width", codeProfileProperty},
		{"max-height:100px", "max-height", codeProfileProperty}, {"border-radius:0", "border-radius", codeProfileProperty},
		{"font-size:12px", "font-size", codeProfileProperty}, {"unknown-extension:12px", "unknown-extension", codeProfileProperty},
		{"width:10%", "width", codeProfileValue}, {"height:1em", "height", codeProfileValue},
		{"width:min-content", "width", codeProfileValue}, {"width:calc(1px + 2px)", "width", codeProfileValue},
		{"width:var(--size)", "width", codeProfileValue}, {"color:var(--ink)", "color", codeProfileValue},
		{"margin:1px", "margin", codeProfileValue}, {"margin:auto", "margin", codeProfileValue},
		{"padding:1%", "padding", codeProfileValue}, {"border-width:1em", "border-width", codeProfileValue},
		{"border:1px dashed red", "border", codeProfileValue}, {"position:absolute", "position", codeProfileValue},
		{"display:none", "display", codeProfileValue}, {"display:contents", "display", codeProfileValue},
		{"display:grid", "display", codeProfileValue}, {"display:flow-root", "display", codeProfileValue},
		{"display:inline-block", "display", codeProfileValue}, {"display:list-item", "display", codeProfileValue},
		{"visibility:hidden", "visibility", codeProfileValue}, {"float:left", "float", codeProfileValue},
		{"box-sizing:border-box", "box-sizing", codeProfileValue}, {"writing-mode:vertical-rl", "writing-mode", codeProfileValue},
		{"direction:rtl", "direction", codeProfileValue}, {"border:revert", "border", codeProfileValue},
		{"display:revert-layer", "display", codeProfileValue},
	} {
		for _, route := range []string{"unmatched", "losing", "inline"} {
			t.Run(tc.decl+"/"+route, func(t *testing.T) {
				head, inline := "", ""
				switch route {
				case "unmatched":
					head = ".unused {" + tc.decl + "}"
				case "losing":
					head = ".frame {" + tc.decl + ";" + tc.key + ":initial}"
				case "inline":
					inline = tc.decl
				}
				v, err := validateHTML(t, profileHTML(head, "", inline))
				if v != nil || err == nil {
					t.Fatal("published unsupported profile")
				}
				ds := errorDiagnostics(t, err)
				count := 1
				if route == "losing" && tc.code == codeProfileProperty {
					count = 2
				}
				if len(ds) != count || ds[0].code != tc.code || ds[0].stage != stageProfile || ds[0].css.property != tc.key || ds[0].css.line == 0 || ds[0].css.declaration == 0 || ds[0].source.nodeID == 0 || ds[0].cause == nil {
					t.Fatalf("wrong diagnostic: %+v", ds)
				}
				if ds[0].css.inline != (route == "inline") || route == "inline" && ds[0].source.templateID != "a" {
					t.Fatalf("wrong source: %+v", ds[0])
				}
			})
		}
	}
}

func TestEffectiveTemplateRequirements(t *testing.T) {
	for _, tc := range []struct {
		name, page, frame, key string
		code                   diagnosticCode
	}{
		{"page auto width", "width:auto", "", "width", codeProfileSizing},
		{"page zero height", "height:0", "", "height", codeProfileSizing},
		{"page rounded zero", "width:.00000001px", "", "width", codeProfileSizing},
		{"page fixed", "position:fixed", "", "position", codeProfileBox},
		{"page insets", "left:0", "", "left", codeProfileBox},
		{"page padding", "padding:1px", "", "padding-top", codeProfileBox},
		{"page border", "border:1px solid", "", "border-top-width", codeProfileBox},
		{"page receiver", "flow-from:Main", "", "flow-from", codeProfileFlow},
		{"static frame", "", "position:static", "position", codeProfileBox},
		{"inherited static frame", "", "position:inherit", "position", codeProfileBox},
		{"initial inline display", "", "display:initial", "display", codeProfileBox},
		{"missing receiver", "", "flow-from:unset", "flow-from", codeProfileFlow},
		{"none receiver", "", "flow-from:none", "flow-from", codeProfileFlow},
		{"source frame", "", "flow-into:Main", "flow-into", codeProfileFlow},
		{"zero width", "", "width:0", "width", codeProfileSizing},
		{"both auto horizontal", "", "left:auto;right:auto", "width", codeProfileSizing},
		{"both auto vertical", "", "top:auto;bottom:auto", "height", codeProfileSizing},
		{"content width", "", "width:auto", "width", codeProfileSizing},
		{"content height", "", "height:auto", "height", codeProfileSizing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := validateHTML(t, profileHTML("", tc.page, tc.frame))
			if v != nil || err == nil {
				t.Fatal("published invalid template")
			}
			found := false
			for _, d := range errorDiagnostics(t, err) {
				if d.stage != stageProfile || d.source.templateID != "a" || d.source.nodeID == 0 || d.css.line != 0 {
					t.Fatalf("invented/missing source: %+v", d)
				}
				found = found || d.code == tc.code && d.css.property == tc.key
			}
			if !found {
				t.Fatalf("missing %s %s: %v", tc.code, tc.key, err)
			}
		})
	}
	// Ancestors are validated too, including non-inherited display suppression.
	v, err := validateHTML(t, profileHTML("body {display:initial}", "", ""))
	if v != nil || err == nil || !strings.Contains(err.Error(), "body[1]") {
		t.Fatal("invalid active ancestor escaped validation")
	}
}

func TestTypedInheritanceAndFlowNames(t *testing.T) {
	v := validatedHTML(t, profileHTML(`html {color:blue}`, `border:9px hidden;background:currentcolor`, `border:inherit;border-style:solid;background:inherit;flow-from:\4d ain-Flow`))
	f := v.templates[0].frames[0]
	if f.flow != "Main-Flow" || f.constraints.Border.Left.Value != 0 || f.paint.borders[3].style != "solid" || f.paint.background != (color.NRGBA{B: 255, A: 255}) {
		t.Fatalf("lost effective inheritance: %+v", f)
	}
	v = validatedHTML(t, profileHTML("", "", `padding:initial;border:unset;background:initial;display:block flow`))
	if v.templates[0].frames[0].paint.background != (color.NRGBA{}) {
		t.Fatal("initial/unset failed")
	}
}

func TestProfileSourceDiagnosticsDeterministic(t *testing.T) {
	input := strings.Replace(profileHTML(`.unused {width:10%;opacity:.5}`, `opacity:1`, `transform:none;overflow:hidden`), "<html>", `<html style="min-width:0">`, 1)
	s := styleCollectionOK(t, parseCollectionOK(t, input))
	var want string
	for i := 0; i < 20; i++ {
		v, err := validateTemplateCollection(s)
		if v != nil || err == nil {
			t.Fatal("expected atomic failure")
		}
		ds := errorDiagnostics(t, err)
		if len(ds) != 6 {
			t.Fatalf("diagnostics: %v", err)
		}
		for j := 1; j < len(ds); j++ {
			if ds[j].source.nodeID < ds[j-1].source.nodeID {
				t.Fatal("nondeterministic source order")
			}
		}
		if i == 0 {
			want = err.Error()
		} else if want != err.Error() {
			t.Fatal("diagnostic order changed")
		}
	}
}

func TestProfilePreservesCheckedParserCause(t *testing.T) {
	_, err := validateHTML(t, profileHTML("", "", `width:var(--x)`))
	if !errors.Is(err, style.ErrInvalidValue) {
		t.Fatalf("lost upstream cause: %v", err)
	}
}

func TestValidatedSketchOwnershipAndOrder(t *testing.T) {
	c := readCollectionFixture(t, "sketch.html")
	s := styleCollectionOK(t, c)
	v, err := validateTemplateCollection(s)
	if err != nil {
		t.Fatal(err)
	}
	again, err := validateTemplateCollection(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v, again) {
		t.Fatal("adaptation changed results")
	}
	if len(v.templates) != 2 || len(v.templates[0].frames) != 3 || len(v.templates[1].frames) != 2 || v.templates[0].id != "two-column-article" {
		t.Fatal("lost definition/frame order")
	}
	for i, f := range v.templates[0].frames {
		want := "article-flow"
		if i == 0 {
			want = "heading-flow"
		}
		if f.id != frameID(i+1) || f.flow != want {
			t.Fatal("lost receiver identity/order")
		}
	}
	// No DOM or mutable style state is shared, including FixedStyle pointers.
	v.templates[0].frames[0].constraints.Fixed.Insets.Top.Value = 999
	v.templates[0].frames[0].paint.background = color.NRGBA{R: 1}
	if reflect.DeepEqual(v, again) || again.templates[0].frames[0].constraints.Fixed.Insets.Top.Value == 999 {
		t.Fatal("adaptations share storage")
	}
	s.templates[0].nodes[s.templates[0].active.page].SetStyles(nil)
	if again.templates[0].page.width <= 0 || again.templates[0].frames[0].constraints.Fixed.Insets.Top.Value == 999 {
		t.Fatal("result retained mutable styling state")
	}
	// A later bad definition must prevent publication of a partial collection.
	input := `<body><template id="blank" data-type="page"><div style="width:10px;height:20px"></div></template><template id="bad" data-type="page"><div></div></template></body>`
	bad, err := validateHTML(t, input)
	if bad != nil || err == nil || !strings.Contains(err.Error(), `template "bad"`) {
		t.Fatal("published partial collection")
	}
	blank := validatedHTML(t, strings.Split(input, `<template id="bad"`)[0]+"</body>")
	if len(blank.templates[0].frames) != 0 {
		t.Fatal("blank page got a fabricated receiver")
	}
}

func TestValidationPipelineRejectsEarlierFailures(t *testing.T) {
	for _, tc := range []struct {
		input string
		stage loadStage
	}{
		{profileHTML("", "", `flow-from:two names`), stageCSS},
		{profileHTML("", "", `flow-from:default`), stageCSS},
		{profileHTML("", "", `--size:12px`), stageCSS},
		{profileHTML("", "", `background:url(a)`), stageCSS},
		{profileHTML(".unused[ {width:12px}", "", ""), stageCSS},
		{profileHTML("@media print {.frame {width:12px}}", "", ""), stageCSS},
		{`<body><template id="a" data-type="page"><div><div><div></div></div></div></template>`, stageStructure},
	} {
		c, err := parseTemplateCollection(strings.NewReader(tc.input), "invalid.html")
		if err == nil {
			var s *styledCollection
			s, err = styleTemplateCollection(c)
			if err == nil {
				_, err = validateTemplateCollection(s)
			}
		}
		if err == nil {
			t.Fatal("accepted invalid pipeline input")
		}
		for _, d := range errorDiagnostics(t, err) {
			if d.stage != tc.stage {
				t.Fatalf("wrong stage: %v", err)
			}
		}
	}
}

func TestTypedCascadeAndIndependentFailures(t *testing.T) {
	input := profileHTML(`.frame {border:3px solid red!important;background:blue!important}
.frame {border:0;background:red}`, "", `background:green!important;border-left-color:blue!important`)
	v := validatedHTML(t, input)
	f := v.templates[0].frames[0]
	if f.constraints.Border.Left.Value != 3 || f.paint.borders[0].color != (color.NRGBA{R: 255, A: 255}) || f.paint.borders[3].color != (color.NRGBA{B: 255, A: 255}) || f.paint.background != (color.NRGBA{G: 128, A: 255}) {
		t.Fatalf("cascade lost during adaptation: %+v", f)
	}
	input = `<body><template id="a" data-type="page"><div></div></template><template id="b" data-type="page"><div></div></template></body>`
	bad, err := validateHTML(t, input)
	if bad != nil || err == nil {
		t.Fatal("published partially valid definitions")
	}
	ds := errorDiagnostics(t, err)
	if len(ds) != 4 || ds[0].source.templateID != "a" || ds[2].source.templateID != "b" {
		t.Fatalf("lost independent failures/order: %v", err)
	}
}

func TestTypedFramesKeepSourceOrderAndFlowCase(t *testing.T) {
	input := `<body><template id="a" data-type="page"><div style="width:300px;height:200px">
<div style="position:fixed;right:0;top:0;width:10px;height:20px;flow-from:Main"></div>
<div style="position:fixed;left:0;top:0;width:10px;height:20px;flow-from:main"></div>
<div style="position:fixed;left:20px;top:0;width:10px;height:20px;flow-from:Main"></div>
</div></template></body>`
	v := validatedHTML(t, input)
	frames := v.templates[0].frames
	if frames[0].constraints.Fixed.Insets.Right.Kind != layout.LenPx || frames[0].constraints.Fixed.Insets.Left.Kind != layout.LenAuto {
		t.Fatal("frames were reordered visually")
	}
	for i, f := range frames {
		if f.id != frameID(i+1) || f.flow != []string{"Main", "main", "Main"}[i] {
			t.Fatal("flow names were normalized or regrouped")
		}
	}
}
