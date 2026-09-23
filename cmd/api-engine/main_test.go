package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommandRegistersStableOperations(t *testing.T) {
	root := newRootCommand()
	for _, name := range []string{"init", "serve", "migrate", "validate", "version"} {
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

func TestInitializeProjectCreatesUsableLocalSetup(t *testing.T) {
	directory := setupTemplateDirectory(t)
	output := &bytes.Buffer{}
	if err := initializeProject(directory, output); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{".env", "config/config.yml", "secrets/engine-identity-key", "secrets/engine-encryption-key", "secrets/engine-principals.json"} {
		if _, err := os.Stat(filepath.Join(directory, path)); err != nil {
			t.Errorf("%s was not created: %v", path, err)
		}
	}
	for _, name := range []string{"engine-identity-key", "engine-encryption-key"} {
		key, err := os.ReadFile(filepath.Join(directory, "secrets", name))
		if err != nil || len(key) != 32 {
			t.Errorf("%s length = %d, error = %v", name, len(key), err)
		}
	}
	info, err := os.Stat(filepath.Join(directory, "secrets"))
	if err != nil {
		t.Errorf("inspect secrets directory: %v", err)
	} else if info.Mode().Perm() != secretDirectoryMode {
		t.Errorf("secrets directory permissions = %o", info.Mode().Perm())
	}

	token := strings.TrimSpace(strings.Split(output.String(), "again:\n")[1])
	data, err := os.ReadFile(filepath.Join(directory, "secrets", "engine-principals.json"))
	if err != nil {
		t.Fatal(err)
	}
	var principals initPrincipalFile
	if err := json.Unmarshal(data, &principals); err != nil {
		t.Fatal(err)
	}
	if principals.Version != 1 || len(principals.Principals) != 1 {
		t.Fatalf("unexpected principals: %+v", principals)
	}
	hash := sha256.Sum256([]byte(token))
	principal := principals.Principals[0]
	if principal.OwnerID != "local-developer" || !principal.Enabled || principal.TokenSHA256 != hex.EncodeToString(hash[:]) {
		t.Fatalf("principal does not match generated token: %+v", principal)
	}
}

func TestInitializeProjectDoesNotReplaceExistingMaterial(t *testing.T) {
	directory := setupTemplateDirectory(t)
	secretsDirectory := filepath.Join(directory, "secrets")
	if err := os.Mkdir(secretsDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	principalPath := filepath.Join(secretsDirectory, "engine-principals.json")
	if err := os.WriteFile(principalPath, []byte("existing principals"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := initializeProject(directory, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	output := &bytes.Buffer{}
	if err := initializeProject(directory, output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(principalPath)
	if err != nil || string(data) != "existing principals" {
		t.Fatalf("existing principals changed: %q, %v", data, err)
	}
	if strings.Contains(output.String(), "bearer token") {
		t.Fatalf("existing principals caused token output: %q", output.String())
	}
}

func TestInitializeProjectRejectsDirectoryAtAFilePath(t *testing.T) {
	directory := setupTemplateDirectory(t)
	if err := os.Mkdir(filepath.Join(directory, ".env"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := initializeProject(directory, &bytes.Buffer{}); err == nil {
		t.Fatal("directory accepted where .env file is required")
	}
}

func setupTemplateDirectory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".env.example"), []byte("APP_PORT=8080\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config", "config.example.yml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return directory
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
