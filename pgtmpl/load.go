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
func runLoad(name string, stdout io.Writer) int {
	// LoadFS accepts io/fs paths. Root the filesystem at the containing directory
	// so absolute paths and parent-relative paths also work on the command line.
	dir, base := filepath.Split(name)
	if dir == "" {
		dir = "."
	}
	catalog, err := template.LoadFS(os.DirFS(dir), base)
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
