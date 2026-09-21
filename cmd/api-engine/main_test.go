package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandRegistersStableOperations(t *testing.T) {
	root := newRootCommand()
	for _, name := range []string{"serve", "migrate", "validate", "version"} {
		command, _, err := root.Find([]string{name})
		if err != nil || command.Name() != name {
			t.Fatalf("command %q: %v", name, err)
		}
	}
	validate, _, err := root.Find([]string{"validate"})
	if err != nil || validate.Flags().Lookup("file") == nil || validate.Flags().Lookup("config") == nil {
		t.Fatalf("validate flags: %v", err)
	}
}

func TestVersionCommandWritesBuildInformation(t *testing.T) {
	root := newRootCommand()
	output := &bytes.Buffer{}
	root.SetOut(output)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "api-engine "+version+" commit "+commit) {
		t.Fatalf("version output: %q", output.String())
	}
}
