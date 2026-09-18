package main

import (
	"fmt"
	"os"
	"runtime"
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
	if len(args) == 0 {
		return fmt.Errorf("usage: api-engine serve --config config.json | validate --file definition.json | migrate --config config.json | version")
	}
	switch args[0] {
	case "version":
		fmt.Println("api-engine", version, "commit", commit, "toolchain", runtime.Version())
		return nil
	case "serve":
		return serve(args[1:], false)
	case "migrate":
		return serve(args[1:], true)
	case "validate":
		return validate(args[1:])
	default:
		return fmt.Errorf("unknown command")
	}
}
