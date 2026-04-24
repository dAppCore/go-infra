package prod

import (
	"dappco.re/go/cli/pkg/cli"
)

func init() {
	cli.RegisterCommands(AddProdCommands)
}

// AddProdCommands registers the 'prod' command and all subcommands.
// Usage: prod.AddProdCommands(root)
func AddProdCommands(root *cli.Command) {
	root.AddCommand(Cmd)
}
