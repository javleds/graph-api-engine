package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println("api-engine development", runtime.Version())
		return
	}
	fmt.Fprintln(os.Stderr, "usage: api-engine version (engine implementation in progress)")
	os.Exit(2)
}
