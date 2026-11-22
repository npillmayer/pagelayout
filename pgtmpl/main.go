// pgtmpl is a CLI for checking and previewing page-template collections.
package main

import (
	"fmt"
	"os"

	"github.com/thatisuday/commando"
)

func main() {
	registry := commando.NewCommandRegistry().
		SetExecutableName("pgtmpl").
		SetVersion("0.1.0").
		SetDescription("Check, preview and locate HTML/CSS page-template collections. Preview and lookup are not implemented yet.")

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

	for _, command := range []struct {
		name, description, argument, argumentDescription string
	}{
		{"serve", "Start a web server to preview page templates in a browser.", "html", "Local HTML collection file"},
		{"find", "Locate a page-template collection by name.", "name", "Collection name"},
	} {
		registry.Register(command.name).
			SetShortDescription(command.description).
			SetDescription(command.description+" Not implemented yet.").
			AddArgument(command.argument, command.argumentDescription, "").
			SetAction(func(_ map[string]commando.ArgValue, _ map[string]commando.FlagValue) {
				fmt.Fprintf(os.Stderr, "pgtmpl: %s: not implemented yet\n", command.name)
				os.Exit(1)
			})
	}

	registry.Parse(nil)
}
