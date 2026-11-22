package main

import (
	"fmt"
	"io"

	"github.com/npillmayer/pagelayout/template"
)

// runFind delegates search and validation to LoadFile. A nil hostIO uses the OS.
func runFind(name string, hostIO template.IO, stdout io.Writer) int {
	catalog, err := template.LoadFile(name, hostIO)
	if err != nil {
		fmt.Fprintf(stdout, "pgtmpl: find %q failed:\n%v\n", name, err)
		return 1
	}
	// Successful loads are nonempty; every page retains the same selected source
	// path. Read it from the catalog rather than guessing which candidate won.
	page, _ := catalog.Lookup(catalog.IDs()[0])
	fmt.Fprintln(stdout, page.Page().Source.Name)
	return 0
}
