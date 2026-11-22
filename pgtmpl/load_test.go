package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPaths(t *testing.T) {
	collection, err := os.ReadFile("../template/testdata/templates/sketch.html")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "collections"), 0755); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, "collections", "page templates.html")
	if err := os.WriteFile(name, collection, 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(dir, "collections"))
	for _, path := range []string{
		"page templates.html",
		"./page templates.html",
		filepath.Join("..", "collections", "page templates.html"),
		name,
	} {
		t.Run(path, func(t *testing.T) {
			var report bytes.Buffer
			if status := runLoad(path, "", &report); status != 0 {
				t.Fatalf("exit %d: %s", status, report.String())
			}
			if !strings.Contains(report.String(), "(2 templates)") || !strings.Contains(report.String(), path) {
				t.Fatalf("missing success summary: %s", report.String())
			}
		})
	}
}

func TestLoadFailures(t *testing.T) {
	fixtures, err := filepath.Abs("../template/testdata/templates")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	multiple := filepath.Join(dir, "multiple.html")
	if err := os.WriteFile(multiple, []byte(`<body>
<template id="first" data-type="page"></template>
<template id="second" data-type="page"></template>
</body>`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path string
		want       []string
	}{
		{"missing file", filepath.Join(dir, "missing.html"), []string{"[io/input.open]", "cannot open collection"}},
		{"overlap", filepath.Join(fixtures, "invalid-overlap.html"), []string{`(template "overlap")`, "[geometry/geometry.overlap]"}},
		{"multiple diagnostics", multiple, []string{`(template "first") [structure/template.page-missing]`, `(template "second") [structure/template.page-missing]`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var report bytes.Buffer
			if status := runLoad(tc.path, "", &report); status != 1 {
				t.Fatalf("exit %d, want 1: %s", status, report.String())
			}
			text := report.String()
			if !strings.Contains(text, tc.path) || strings.Contains(text, "pgtmpl: loaded") {
				t.Fatalf("incorrect failure summary: %s", text)
			}
			// Check that all expected diagnostics survive in their original order.
			for _, want := range tc.want {
				index := strings.Index(text, want)
				if index < 0 {
					t.Fatalf("missing %q in report: %s", want, report.String())
				}
				text = text[index+len(want):]
			}
		})
	}
}
