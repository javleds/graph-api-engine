package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTypeRules(t *testing.T) {
	cases := map[string]bool{
		"package fixture; type One struct { Name string }": true,
		"package fixture; type One string; type Two int":   false,
		"package fixture; func hidden(){ type Local int }": false,
		"package fixture; var model struct {Name string}":  false,
		"package fixture; var done chan struct{}":          true,
	}
	for source, want := range cases {
		path := filepath.Join(t.TempDir(), "fixture.go")
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		if got := len(checkFile(path)) == 0; got != want {
			t.Errorf("%q: allowed=%v want %v", source, got, want)
		}
	}
}

func TestDependencies(t *testing.T) {
	if !forbiddenImport("internal/execution/domain/execution.go", "database/sql") {
		t.Fatal("domain SQL allowed")
	}
	if !forbiddenImport("internal/api/http/server.go", "github.com/javleds/api-engine/internal/worker") {
		t.Fatal("API worker allowed")
	}
	if forbiddenImport("internal/execution/domain/execution.go", "time") {
		t.Fatal("domain time forbidden")
	}
}
