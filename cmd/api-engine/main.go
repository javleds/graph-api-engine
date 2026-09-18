package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: api-engine version | validate --file definition.json")
	}
	switch args[0] {
	case "version":
		fmt.Println("api-engine development", runtime.Version())
		return nil
	case "validate":
		return validate(args[1:])
	default:
		return fmt.Errorf("unknown command")
	}
}
