# pgtmpl

Check, locate and preview HTML/CSS page-template collections from the command
line. Run these commands from the pagelayout module directory:

```sh
go run ./pgtmpl --help
go run ./pgtmpl load examples/01-minimal.html
go run ./pgtmpl serve examples/03-magazine.html
```

For a standalone command, install with `go install ./pgtmpl`, then use `pgtmpl`
from your Go executable directory. The examples below use `go run ./pgtmpl`; replace that prefix with `pgtmpl`
when using the installed command.

| Command | Result |
| --- | --- |
| `load <html>` | Check an explicit local collection and report its template count or errors. |
| `serve <html> [--addr ADDRESS]` | Check a collection and serve page/frame previews in a browser. |
| `find <name>` | Search for and validate a collection, then print its path. |

## Check a collection

```sh
go run ./pgtmpl load examples/03-magazine.html
go run ./pgtmpl load examples/overlapping-frames_err.html
```

`load` accepts relative or absolute HTML paths without searching collection
repositories. Success prints the file name and template count to stdout.
Failure prints diagnostics to stdout and exits with status 1. One invalid
page definition rejects the entire collection.

Diagnostics identify template IDs, source-node paths, CSS locations and error
codes when available. CSS line/column values refer to CSS text, not HTML file
lines. The heading shows the input path; diagnostic source names are relative
to its containing directory.

## Preview frames in a browser

```sh
go run ./pgtmpl serve examples/03-magazine.html
go run ./pgtmpl serve examples/03-magazine.html --addr 127.0.0.1:8081
```

Open the printed URL. The default address is `127.0.0.1:8080`; port `0` asks
the OS to select an available port. Ctrl+C stops the server. An invalid
collection reports errors and starts no server.

The viewer shows one template at a time. Use the selector or previous/next links
to navigate. The SVG displays resolved page dimensions, frame rectangles and
solid paint. Hover over a frame to inspect its receiving content rectangle in
CSS pixels.

**Show frame borders** adds colored outlines and frame numbers. A channel keeps
its color across the collection; the legend lists its frame numbers, such as
`#2, #3`. Uncheck the control to hide these display outlines. Authored borders
and backgrounds remain visible, so an authored border beside an outline can
appear thicker until the outlines are hidden.

The preview scales to the available display area and shows a startup snapshot.
Restart after editing input files. It displays templates without flowing content
and does not launch the browser automatically.

## Apply your own stylesheet

Both `load` and `serve` accept one additional local stylesheet with `--css`:

```sh
go run ./pgtmpl load examples/03-magazine.html --css examples/magazine-override.css
go run ./pgtmpl serve examples/03-magazine.html --css examples/magazine-override.css
```

Relative CSS paths resolve from the working directory, independently of the
HTML path. Absolute paths are accepted. CSS follows the collection's head
styles in the normal author cascade: specificity, inline styles and
`!important` retain their normal precedence.

The override can change paint and receiving geometry. It must satisfy the same
CSS profile as the collection, including declarations that lose in the cascade
or match no elements. Missing CSS files and invalid overrides report errors
before serving. Stylesheet URLs, linked sheets and `@import` resources are not
fetched. Omit `--css` to see the unchanged collection, and restart `serve` after
editing either file.

The [magazine override](../examples/magazine-override.css) demonstrates changes
to backgrounds, borders and padding. The viewer's colored outlines are separate
from these authored styles.

## Find a collection

```sh
go run ./pgtmpl find pages.html
go run ./pgtmpl find examples/01-minimal.html
```

Relative names are tried under `os.UserConfigDir()/templates`, then in the
working directory. Absolute paths are used directly. Relative paths escaping
through `..` use only the working directory. Include the file extension;
`find` is not recursive and does not search for a template ID within a collection.

Only a missing file permits fallback. An unreadable or invalid collection
stops the search. Success prints just the validated collection's path and a
newline to stdout; a working-directory match may be a relative path. Failure
prints diagnostics, including attempted paths for missing files, to stdout.

## Output and exit status

Loading, search and server errors go to stdout and exit with status 1.
Successful `load` and `find` commands exit with status 0. When using `go run`,
Go additionally prints its own `exit status 1` message to stderr on failure.

Run without arguments or with `--help` for help; each subcommand also accepts
`--help`. Command-syntax errors, such as a missing positional argument, print
to stdout and exit with status 0. Do not rely on exit status alone to detect
incorrect command syntax.

See the [collection examples](../examples/README.md) for more inputs and the
[template package guide](../template/README.md) for collection authoring rules.
