# prod
**Import:** `forge.lthn.ai/core/go-infra/cmd/prod`
**Files:** 7

## Types

This package exports no structs, interfaces, or type aliases.

## Functions

### `func AddProdCommands(root *cli.Command)`
Registers the exported `Cmd` tree on the shared CLI root so the `prod` command and its `status`, `setup`, `dns`, `lb`, and `ssh` subcommands become available.
