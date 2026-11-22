package main

import (
	"bytes"
	"context"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAdditionalCSS(t *testing.T) {
	// Both paths are relative to the CLI working directory, not to each other.
	catalog, err := loadCollection("../examples/03-magazine.html", "../examples/magazine-override.css")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := catalog.Lookup("magazine-opening")
	if page.Page().Paint.Background != (color.NRGBA{255, 255, 255, 255}) || page.Frames()[0].Border.Bottom != 1 {
		t.Fatal("override example did not change compiled paint/border")
	}
	notes := page.FramesForFlow("notes")[0]
	if notes.ContentBox.W != 184 || notes.ContentBox.X != 558 || notes.BorderBox.W != 220 {
		t.Fatalf("notes geometry did not reflect override: %+v", notes)
	}
	var report bytes.Buffer
	if status := runLoad("../examples/03-magazine.html", "../examples/magazine-override.css", &report); status != 0 || !strings.Contains(report.String(), "(2 templates)") {
		t.Fatalf("exit %d: %s", status, report.String())
	}
}

func TestAdditionalCSSFailures(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ name, css, message string }{
		{"missing.css", "", "read additional stylesheet"},
		{"invalid.css", `.page {width:50%}`, "profile.value"},
		{"geometry.css", `.opening > .article {left:790px}`, "geometry.bounds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cssPath := filepath.Join(dir, tc.name)
			if tc.css != "" {
				if err := os.WriteFile(cssPath, []byte(tc.css), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, command := range []string{"load", "serve"} {
				var report bytes.Buffer
				var status int
				if command == "load" {
					status = runLoad("../examples/03-magazine.html", cssPath, &report)
				} else {
					status = runServe(context.Background(), "../examples/03-magazine.html", "invalid-listen-address", cssPath, &report)
				}
				if status != 1 || !strings.Contains(report.String(), tc.message) || strings.Contains(report.String(), "pgtmpl: serve") {
					t.Fatalf("%s: exit %d: %s", command, status, report.String())
				}
				if tc.name != "geometry.css" && !strings.Contains(report.String(), cssPath) {
					t.Fatalf("missing stylesheet path: %s", report.String())
				}
			}
		})
	}
}
