# pgtmpl

A CLI companion for checking and previewing HTML/CSS page-template collections.
It uses `github.com/thatisuday/commando` v1.0.4, matching `tytui/tycli`.

From the pagelayout module root:

```sh
go run ./pgtmpl --help
go run ./pgtmpl load --help
go build -o /tmp/pgtmpl ./pgtmpl
```

The initial scaffold registers these commands:

| Command | Intended behavior |
| --- | --- |
| `pgtmpl load <html>` | Load an HTML collection and report template errors. |
| `pgtmpl serve <html>` | Start a web server to preview the collection in a browser. |
| `pgtmpl find <name>` | Locate a collection by name. |

All three commands are placeholders: with their required argument, they print
`not implemented yet` to stderr and exit with status 1. No files are read, no
server is started, and no collection search is performed. Loading will use the
`template` package once its public API is available.

Running without arguments displays help. Commando supplies `--help`, per-command
help and `--version`. As in `tycli`, Commando 1.0.4 prints CLI syntax errors to
stdout and exits with status 0; placeholder actions explicitly exit with status 1.
