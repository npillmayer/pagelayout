// pgtmpl is a CLI for checking and previewing page-template collections.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/thatisuday/commando"
)

func main() {
	registry := commando.NewCommandRegistry().
		SetExecutableName("pgtmpl").
		SetVersion("0.1.0").
		SetDescription("Check, preview and locate HTML/CSS page-template collections.")

	root := registry.Register(nil)
	root.SetAction(func(_ map[string]commando.ArgValue, _ map[string]commando.FlagValue) {
		registry.PrintHelp(root)
	})

	registry.Register("load").
		SetShortDescription("Load an HTML page-template collection and report errors.").
		SetDescription("Load and check a local HTML page-template collection. Reports go to stdout; loading failures exit with status 1.").
		AddArgument("html", "Local HTML collection file", "").
		SetAction(func(args map[string]commando.ArgValue, _ map[string]commando.FlagValue) {
			os.Exit(runLoad(args["html"].Value, os.Stdout))
		})

	registry.Register("serve").
		SetShortDescription("Preview page templates in a browser.").
		SetDescription("Check a local collection, then serve its compiled geometry. Open the printed URL in a browser; Ctrl+C stops the server. Errors go to stdout.").
		AddArgument("html", "Local HTML collection file", "").
		AddFlag("addr", "HTTP listen address", commando.String, "127.0.0.1:8080").
		SetAction(func(args map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
			addr, _ := flags["addr"].GetString()
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			status := runServe(ctx, args["html"].Value, addr, os.Stdout)
			stop()
			os.Exit(status)
		})

	registry.Register("find").
		SetShortDescription("Locate a page-template collection by name.").
		SetDescription("Find and validate a collection using the template loader's search order: the user configuration templates directory, then the current directory. Print the selected path or errors to stdout; failures exit with status 1.").
		AddArgument("name", "Collection file name or path (including extension)", "").
		SetAction(func(args map[string]commando.ArgValue, _ map[string]commando.FlagValue) {
			os.Exit(runFind(args["name"].Value, nil, os.Stdout))
		})

	registry.Parse(nil)
}
