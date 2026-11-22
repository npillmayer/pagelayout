package template_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/npillmayer/pagelayout/template"
)

// Each fake models one rooted filesystem; no lookup may escape into the OS.
type collectionIO struct {
	configDir   string
	configErr   error
	roots       map[string]fs.FS
	dirs        []string
	configCalls int
}

func (h *collectionIO) UserConfigDir() (string, error) {
	h.configCalls++
	return h.configDir, h.configErr
}

func (h *collectionIO) DirFS(dir string) fs.FS {
	h.dirs = append(h.dirs, dir)
	if root, ok := h.roots[dir]; ok {
		return root
	}
	return fstest.MapFS{}
}

type collectionFS struct {
	fs.FS
	openErr, readErr, closeErr error
	opens                      []string
	closes                     int
}

func (f *collectionFS) Open(name string) (fs.File, error) {
	f.opens = append(f.opens, name)
	if f.openErr != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: f.openErr}
	}
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &collectionFile{File: file, owner: f}, nil
}

type collectionFile struct {
	fs.File
	owner *collectionFS
}

func (f *collectionFile) Read(b []byte) (int, error) {
	if f.owner.readErr != nil {
		return 0, f.owner.readErr
	}
	return f.File.Read(b)
}

func (f *collectionFile) Close() error {
	f.owner.closes++
	err := f.File.Close()
	if f.owner.closeErr != nil {
		return f.owner.closeErr
	}
	return err
}

func collectionFiles(name, pageCSS string) fstest.MapFS {
	return fstest.MapFS{name: &fstest.MapFile{Data: []byte(collection(pageCSS, ""))}}
}

func fileDiagnostics(t *testing.T, catalog *template.Catalog, err error) []template.Diagnostic {
	t.Helper()
	var loadErr *template.LoadError
	if catalog != nil || !errors.As(err, &loadErr) {
		t.Fatalf("expected nil catalog and LoadError, got %v, %v", catalog, err)
	}
	return loadErr.Diagnostics()
}

func TestLoadFS(t *testing.T) {
	files := &collectionFS{FS: collectionFiles("nested/pages.html", "")}
	catalog, err := template.LoadFS(files, "nested/pages.html")
	if err != nil {
		t.Fatal(err)
	}
	page := lookup(t, catalog, "a")
	if page.Page().Source.Name != "nested/pages.html" || page.Frames()[0].Source.Name != "nested/pages.html" {
		t.Fatal("file source was not preserved")
	}
	if files.closes != 1 || !reflect.DeepEqual(files.opens, []string{"nested/pages.html"}) {
		t.Fatalf("file lifecycle: %+v", files)
	}
	files.FS = nil
	if page.FramesForFlow("Main")[0].ContentBox.W != 10 {
		t.Fatal("catalog depended on the open file")
	}
}

func TestLoadFSInvalidAndMissingPaths(t *testing.T) {
	for _, name := range []string{"", "../pages.html", "/pages.html", "nested//pages.html", "./pages.html"} {
		t.Run(name, func(t *testing.T) {
			files := &collectionFS{FS: fstest.MapFS{}}
			catalog, err := template.LoadFS(files, name)
			ds := fileDiagnostics(t, catalog, err)
			if !errors.Is(err, fs.ErrInvalid) || ds[0].Code != "input.path" || len(files.opens) != 0 {
				t.Fatalf("invalid path reached filesystem: %v, %+v", err, files)
			}
		})
	}
	catalog, err := template.LoadFS(nil, "pages.html")
	fileDiagnostics(t, catalog, err)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatal(err)
	}
	catalog, err = template.LoadFS(fstest.MapFS{}, "pages.html")
	ds := fileDiagnostics(t, catalog, err)
	var pathErr *fs.PathError
	if !errors.Is(err, fs.ErrNotExist) || !errors.As(err, &pathErr) || ds[0].Code != "input.open" || ds[0].Source.Name != "pages.html" {
		t.Fatal(err)
	}
}

func TestLoadFSClosesOnFailures(t *testing.T) {
	readErr, closeErr := errors.New("read failed"), errors.New("close failed")
	for _, tc := range []struct {
		name              string
		data              string
		readErr, closeErr error
		codes             []string
	}{
		{"compile", "<body>", nil, nil, []string{"collection.empty"}},
		{"read", collection(""), readErr, nil, []string{"input.parse"}},
		{"close", collection(""), nil, closeErr, []string{"input.close"}},
		{"read and close", collection(""), readErr, closeErr, []string{"input.parse", "input.close"}},
		{"compile and close", "<body>", nil, closeErr, []string{"collection.empty", "input.close"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := &collectionFS{FS: fstest.MapFS{"pages.html": &fstest.MapFile{Data: []byte(tc.data)}}, readErr: tc.readErr, closeErr: tc.closeErr}
			catalog, err := template.LoadFS(files, "pages.html")
			ds := fileDiagnostics(t, catalog, err)
			var codes []string
			for _, d := range ds {
				codes = append(codes, d.Code)
				if d.Source.Name != "pages.html" {
					t.Fatal(d)
				}
			}
			if files.closes != 1 || !reflect.DeepEqual(codes, tc.codes) ||
				(tc.readErr != nil && !errors.Is(err, tc.readErr)) || (tc.closeErr != nil && !errors.Is(err, tc.closeErr)) {
				t.Fatalf("failure lifecycle: closes=%d, error=%v", files.closes, err)
			}
		})
	}
}

func TestLoadFileSearchOrder(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config")
	repo := filepath.Join(config, "templates")
	absDir := t.TempDir()
	for _, tc := range []struct {
		name, path, configDir string
		configErr             error
		roots                 map[string]fs.FS
		wantDirs              []string
		wantSource            string
		wantWidth             float32
		wantConfigCalls       int
	}{
		{"repository wins", "pages.html", config, nil,
			map[string]fs.FS{repo: collectionFiles("pages.html", "width:400px"), ".": collectionFiles("pages.html", "")},
			[]string{repo}, filepath.Join(repo, "pages.html"), 400, 1},
		{"missing repository", "pages.html", config, nil,
			map[string]fs.FS{".": collectionFiles("pages.html", "")},
			[]string{repo, "."}, "pages.html", 300, 1},
		{"missing collection", "pages.html", config, nil,
			map[string]fs.FS{repo: collectionFiles("other.html", ""), ".": collectionFiles("pages.html", "")},
			[]string{repo, "."}, "pages.html", 300, 1},
		{"config unavailable", "pages.html", "", errors.New("no config directory"),
			map[string]fs.FS{".": collectionFiles("pages.html", "")},
			[]string{"."}, "pages.html", 300, 1},
		{"config empty", "pages.html", "", nil,
			map[string]fs.FS{".": collectionFiles("pages.html", "")},
			[]string{"."}, "pages.html", 300, 1},
		{"nested repository path", filepath.Join("forms", "pages.html"), config, nil,
			map[string]fs.FS{repo: collectionFiles("forms/pages.html", "width:400px")},
			[]string{repo}, filepath.Join(repo, "forms", "pages.html"), 400, 1},
		{"nested local path", filepath.Join("forms", "pages.html"), config, nil,
			map[string]fs.FS{"forms": collectionFiles("pages.html", "")},
			[]string{repo, "forms"}, filepath.Join("forms", "pages.html"), 300, 1},
		{"dot prefix", "./pages.html", config, nil,
			map[string]fs.FS{repo: collectionFiles("pages.html", "")},
			[]string{repo}, filepath.Join(repo, "pages.html"), 300, 1},
		{"absolute bypasses repository", filepath.Join(absDir, "pages.html"), config, nil,
			map[string]fs.FS{absDir: collectionFiles("pages.html", ""), repo: collectionFiles("pages.html", "width:400px")},
			[]string{absDir}, filepath.Join(absDir, "pages.html"), 300, 0},
		{"parent path is local only", filepath.Join("..", "pages.html"), config, nil,
			map[string]fs.FS{"..": collectionFiles("pages.html", "")},
			[]string{".."}, filepath.Join("..", "pages.html"), 300, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &collectionIO{configDir: tc.configDir, configErr: tc.configErr, roots: tc.roots}
			catalog, err := template.LoadFile(tc.path, host)
			if err != nil {
				t.Fatal(err)
			}
			page := lookup(t, catalog, "a").Page()
			if page.Width != tc.wantWidth || page.Source.Name != tc.wantSource || !reflect.DeepEqual(host.dirs, tc.wantDirs) || host.configCalls != tc.wantConfigCalls {
				t.Fatalf("page=%+v, host=%+v", page, host)
			}
		})
	}
}

func TestLoadFileDoesNotHideRepositoryFailures(t *testing.T) {
	config := t.TempDir()
	repo := filepath.Join(config, "templates")
	for _, tc := range []struct {
		name  string
		files *collectionFS
		code  string
	}{
		{"permission", &collectionFS{openErr: fs.ErrPermission}, "input.open"},
		{"invalid collection", &collectionFS{FS: fstest.MapFS{"pages.html": &fstest.MapFile{Data: []byte("<body>")}}}, "collection.empty"},
		{"read not found", &collectionFS{FS: collectionFiles("pages.html", ""), readErr: fs.ErrNotExist}, "input.parse"},
		{"close not found", &collectionFS{FS: collectionFiles("pages.html", ""), closeErr: fs.ErrNotExist}, "input.close"},
		{"directory", &collectionFS{FS: fstest.MapFS{"pages.html": &fstest.MapFile{Mode: fs.ModeDir}}}, "input.parse"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &collectionIO{configDir: config, roots: map[string]fs.FS{repo: tc.files, ".": collectionFiles("pages.html", "")}}
			catalog, err := template.LoadFile("pages.html", host)
			ds := fileDiagnostics(t, catalog, err)
			if ds[0].Code != tc.code || ds[0].Source.Name != filepath.Join(repo, "pages.html") || !reflect.DeepEqual(host.dirs, []string{repo}) {
				t.Fatalf("wrong failure or fallback: %v, dirs=%v", err, host.dirs)
			}
		})
	}
}

func TestLoadFileMissingAndInvalid(t *testing.T) {
	config := t.TempDir()
	host := &collectionIO{configDir: config}
	catalog, err := template.LoadFile("pages.html", host)
	ds := fileDiagnostics(t, catalog, err)
	if !errors.Is(err, fs.ErrNotExist) || len(ds) != 2 || ds[0].Source.Name != filepath.Join(config, "templates", "pages.html") || ds[1].Source.Name != "pages.html" {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.Stage != "io" || d.Code != "input.open" {
			t.Fatal(d)
		}
	}
	for _, name := range []string{"", ".", ".."} {
		host = &collectionIO{configDir: config}
		catalog, err = template.LoadFile(name, host)
		fileDiagnostics(t, catalog, err)
		if !errors.Is(err, fs.ErrInvalid) || host.configCalls != 0 || len(host.dirs) != 0 {
			t.Fatalf("invalid name %q reached host: %v", name, host)
		}
	}
	host = &collectionIO{configDir: config, roots: map[string]fs.FS{filepath.Join(config, "templates"): nil}}
	catalog, err = template.LoadFile("pages.html", host)
	fileDiagnostics(t, catalog, err)
	if !errors.Is(err, fs.ErrInvalid) || len(host.dirs) != 1 {
		t.Fatal("nil filesystem was not diagnosed")
	}
}

func TestLoadFileFinalOpenFailure(t *testing.T) {
	config := t.TempDir()
	host := &collectionIO{configDir: config, roots: map[string]fs.FS{".": &collectionFS{openErr: fs.ErrPermission}}}
	catalog, err := template.LoadFile("pages.html", host)
	ds := fileDiagnostics(t, catalog, err)
	if !errors.Is(err, fs.ErrPermission) || errors.Is(err, fs.ErrNotExist) || len(ds) != 1 || ds[0].Source.Name != "pages.html" {
		t.Fatalf("repository miss obscured the final permission error: %v", err)
	}

	// An absolute pathname is authoritative, including when it is missing.
	abs := filepath.Join(t.TempDir(), "pages.html")
	host = &collectionIO{configDir: config, roots: map[string]fs.FS{
		filepath.Join(config, "templates"): collectionFiles("pages.html", ""),
		".":                                collectionFiles("pages.html", ""),
	}}
	catalog, err = template.LoadFile(abs, host)
	ds = fileDiagnostics(t, catalog, err)
	if !errors.Is(err, fs.ErrNotExist) || len(ds) != 1 || ds[0].Source.Name != abs || host.configCalls != 0 || !reflect.DeepEqual(host.dirs, []string{filepath.Dir(abs)}) {
		t.Fatalf("absolute path triggered a search: %v, host=%+v", err, host)
	}
}

func TestLoadFileSystemIO(t *testing.T) {
	base := t.TempDir()
	// Keep all OS-default discovery inside the test directory on supported hosts.
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "AppData", "home"} {
		t.Setenv(key, base)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("host has no configuration directory: %v", err)
	}
	repo := filepath.Join(config, "templates")
	work := filepath.Join(base, "work")
	for _, dir := range []string{repo, work} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for path, css := range map[string]string{
		filepath.Join(repo, "pages.html"): "width:400px",
		filepath.Join(work, "pages.html"): "",
		filepath.Join(work, "local.html"): "width:500px",
	} {
		if err := os.WriteFile(path, []byte(collection(css, "")), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(work)
	for _, tc := range []struct {
		name, source string
		width        float32
	}{
		{"pages.html", filepath.Join(repo, "pages.html"), 400},
		{"local.html", "local.html", 500},
		{filepath.Join(work, "pages.html"), filepath.Join(work, "pages.html"), 300},
	} {
		catalog, err := template.LoadFile(tc.name, nil)
		if err != nil {
			t.Fatal(err)
		}
		page := lookup(t, catalog, "a").Page()
		if page.Width != tc.width || page.Source.Name != tc.source {
			t.Fatalf("system lookup: %+v", page)
		}
	}
}
