# pgtmpl

A CLI companion for checking and previewing HTML/CSS page-template collections.
It uses `github.com/thatisuday/commando` v1.0.4, matching `tytui/tycli`.

From the pagelayout module root:

```sh
go run ./pgtmpl --help
go run ./pgtmpl load --help
go build -o /tmp/pgtmpl ./pgtmpl
```

The CLI registers these commands:

| Command | Behavior |
| --- | --- |
| `pgtmpl load <html>` | Load an HTML collection and report template errors. |
| `pgtmpl serve <html>` | Check a collection and serve its compiled page/frame previews. |
| `pgtmpl find <name>` | Find and validate a collection, then print its path. |

`load` uses `template.LoadFS` to load and check exactly the specified local file.
Relative and absolute paths are supported; no repository search is performed.
A successful load prints the file name and template count to stdout and exits
with status 0. Failures print all returned diagnostics to stdout and exit with
status 1. Reports retain the loader's source-node paths, template identities,
CSS locations, diagnostic codes and underlying causes. CSS line/column locations
refer to CSS text, not HTML file lines. The report heading includes the input
path; diagnostic source names are relative to the containing directory.

```sh
go run ./pgtmpl load template/testdata/templates/sketch.html
go run ./pgtmpl load template/testdata/templates/invalid-overlap.html
```

`serve` loads the whole collection with `template.LoadFS` before opening a
listener. Invalid collections print their diagnostics to stdout and exit with
status 1; no server or partial preview is published. On success, open the printed
URL in a browser. The default address is `127.0.0.1:8080`; use `--addr` to change
it (port `0` asks the OS to choose an available port). Ctrl+C stops the server.

```sh
go run ./pgtmpl serve template/testdata/templates/sketch.html
go run ./pgtmpl serve template/testdata/templates/sketch.html --addr 127.0.0.1:8081
```

The viewer shows one template at a time, with previous/next links and a template
selector. Its SVG uses the catalog's compiled page dimensions, frame border
rectangles and solid paint; the browser does not resolve the source CSS again.
The first CSS snippet adds colored outlines to top-level frames without changing
their geometry. A flow keeps its color across the collection, and a legend names
the channels and their matching frame numbers (for example, `#2, #3`).
The checked **Show frame borders** control hides these outlines
and frame numbers when unchecked; authored backgrounds and borders remain.
Hover over a frame to see its compiled receiving content rectangle in CSS pixels.

The preview scales to fit the available display area. It is a snapshot taken at
startup; restart the command after editing the collection. It does not render
flowing content, reload files, launch a browser automatically or serve source
files from disk. Navigation and the outline toggle work without JavaScript.

`find` uses `template.LoadFile` with the normal host filesystem. Relative names
are tried in `os.UserConfigDir()/templates`, then in the current directory.
Absolute paths are used directly; parent-relative paths escaping through `..`
use only the current directory. Include the file extension: no extension is
added, and this is not a recursive search or a template-ID lookup.

On success, stdout contains only the selected path and a newline, with exit
status 0. The path is preserved as reported by the loader; a current-directory
match can therefore be relative. The collection is fully validated before a
path is printed. Only a missing file permits fallback: an unreadable or invalid
collection stops the search. Errors, including all attempted paths when no file
is found, go to stdout with exit status 1.

```sh
go run ./pgtmpl find pages.html
go run ./pgtmpl find template/testdata/templates/sketch.html
```

Running without arguments displays help. Commando supplies `--help`, per-command
help and `--version`. As in `tycli`, Commando 1.0.4 prints CLI syntax errors to
stdout and exits with status 0. This includes a missing `load` argument; file
loading, search and server failures explicitly exit with status 1.
