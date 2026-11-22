package main

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

type findTestIO struct {
	config string
	roots  map[string]fs.FS
}

func (h findTestIO) UserConfigDir() (string, error) { return h.config, nil }
func (h findTestIO) DirFS(dir string) fs.FS {
	if files, ok := h.roots[dir]; ok {
		return files
	}
	return fstest.MapFS{}
}

func TestFind(t *testing.T) {
	const valid = `<body><template id="page" data-type="page"><div style="width:300px;height:200px"></div></template></body>`
	files := func(text string) fs.FS {
		return fstest.MapFS{"pages.html": &fstest.MapFile{Data: []byte(text)}}
	}
	config := filepath.Join(t.TempDir(), "config")
	repo := filepath.Join(config, "templates")
	repoFile := filepath.Join(repo, "pages.html")
	absDir := t.TempDir()
	absFile := filepath.Join(absDir, "pages.html")
	parentFile := filepath.Join("..", "pages.html")
	for _, tc := range []struct {
		name, input string
		roots       map[string]fs.FS
		status      int
		path        string
		diagnostics []string
	}{
		{"repository preferred", "pages.html", map[string]fs.FS{repo: files(valid), ".": files(valid)}, 0, repoFile, nil},
		{"local fallback", "pages.html", map[string]fs.FS{".": files(valid)}, 0, "pages.html", nil},
		{"absolute path", absFile, map[string]fs.FS{repo: files(valid), absDir: files(valid)}, 0, absFile, nil},
		{"parent path", parentFile, map[string]fs.FS{repo: files(valid), "..": files(valid)}, 0, parentFile, nil},
		{"missing", "pages.html", nil, 1, "", []string{repoFile + " [io/input.open]", "\npages.html [io/input.open]"}},
		{"broken repository stops search", "pages.html", map[string]fs.FS{repo: files("<body>"), ".": files(valid)}, 1, "", []string{repoFile, "[discovery/collection.empty]"}},
		{"broken local file", "pages.html", map[string]fs.FS{".": files("<body>")}, 1, "", []string{"pages.html", "[discovery/collection.empty]"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			status := runFind(tc.input, findTestIO{config: config, roots: tc.roots}, &output)
			if status != tc.status {
				t.Fatalf("exit %d, want %d: %s", status, tc.status, output.String())
			}
			if status == 0 {
				if output.String() != tc.path+"\n" {
					t.Fatalf("got %q, want only the selected path %q", output.String(), tc.path)
				}
				return
			}
			if !strings.HasPrefix(output.String(), "pgtmpl: find ") {
				t.Fatalf("missing error heading: %s", output.String())
			}
			remaining := output.String()
			for _, diagnostic := range tc.diagnostics {
				index := strings.Index(remaining, diagnostic)
				if index < 0 {
					t.Fatalf("missing or out-of-order diagnostic %q: %s", diagnostic, output.String())
				}
				remaining = remaining[index+len(diagnostic):]
			}
		})
	}
}
