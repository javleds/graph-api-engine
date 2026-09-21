package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var version = "development"
var commit = "unknown"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := newRootCommand()
	command.SetArgs(args)
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	return command.Execute()
}

func newRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:           "api-engine",
		Short:         "API Execution Engine",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("usage: api-engine serve --config config.yml | validate --file definition.json | migrate --config config.yml | version")
		},
	}
	command.AddCommand(newServeCommand("serve", false))
	command.AddCommand(newServeCommand("migrate", true))
	command.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		Args:  cobra.NoArgs,
		Run: func(command *cobra.Command, _ []string) {
			fmt.Fprintln(command.OutOrStdout(), "api-engine", version, "commit", commit, "toolchain", runtime.Version())
		},
	})
	command.AddCommand(newValidateCommand())
	return command
}
