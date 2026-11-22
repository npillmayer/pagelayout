package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/npillmayer/pagelayout/template"
)

// runLoad checks exactly the named file, without searching template repositories.
// It writes the report to stdout and returns the command's exit status.
func runLoad(name, cssPath string, stdout io.Writer) int {
	catalog, err := loadCollection(name, cssPath)
	if err != nil {
		fmt.Fprintf(stdout, "pgtmpl: load %q failed:\n", name)
		// LoadError.Error includes every diagnostic in source order, with source
		// paths, template identities, CSS locations and underlying causes.
		fmt.Fprintln(stdout, err)
		return 1
	}
	fmt.Fprintf(stdout, "pgtmpl: loaded %q (%d templates)\n", name, len(catalog.IDs()))
	return 0
}

func loadCollection(name, cssPath string) (*template.Catalog, error) {
	var opts template.LoadOptions
	if cssPath != "" {
		text, err := os.ReadFile(cssPath)
		if err != nil {
			return nil, fmt.Errorf("read additional stylesheet %q: %w", cssPath, err)
		}
		opts.Stylesheets = []template.Stylesheet{{SourceName: cssPath, CSS: string(text)}}
	}
	// LoadFS accepts io/fs paths. Root the filesystem at the containing directory
	// so absolute paths and parent-relative paths also work on the command line.
	dir, base := filepath.Split(name)
	if dir == "" {
		dir = "."
	}
	return template.LoadFSWithOptions(os.DirFS(dir), base, opts)
}
