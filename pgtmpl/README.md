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
| `pgtmpl serve <html>` | Placeholder for a browser preview server. |
| `pgtmpl find <name>` | Placeholder for locating a collection by name. |

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

`serve` and `find` remain placeholders: with their required argument, they print
`not implemented yet` to stderr and exit with status 1.

Running without arguments displays help. Commando supplies `--help`, per-command
help and `--version`. As in `tycli`, Commando 1.0.4 prints CLI syntax errors to
stdout and exits with status 0. This includes a missing `load` argument; file
loading failures and placeholder actions explicitly exit with status 1.
