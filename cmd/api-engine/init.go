package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	secretDirectoryMode os.FileMode = 0700
	secretFileMode      os.FileMode = 0644
)

type initPrincipalFile struct {
	Version    int             `json:"version"`
	Principals []initPrincipal `json:"principals"`
}

type initPrincipal struct {
	OwnerID     string   `json:"ownerId"`
	Enabled     bool     `json:"enabled"`
	TokenSHA256 string   `json:"tokenSha256"`
	Permissions []string `json:"permissions"`
}

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create local configuration and credentials",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			directory, err := os.Getwd()
			if err != nil {
				return err
			}
			return initializeProject(directory, command.OutOrStdout())
		},
	}
}

func initializeProject(directory string, output io.Writer) error {
	root, err := filepath.Abs(directory)
	if err != nil {
		return err
	}

	envTemplate, err := readTemplate(root, ".env.example")
	if err != nil {
		return err
	}
	configTemplate, err := readTemplate(root, filepath.Join("config", "config.example.yml"))
	if err != nil {
		return err
	}

	if err := createFileIfMissing(filepath.Join(root, ".env"), envTemplate, 0600); err != nil {
		return err
	}
	if err := createFileIfMissing(filepath.Join(root, "config", "config.yml"), configTemplate, 0644); err != nil {
		return err
	}

	secretsDirectory := filepath.Join(root, "secrets")
	if err := os.MkdirAll(secretsDirectory, secretDirectoryMode); err != nil {
		return fmt.Errorf("create secrets directory: %w", err)
	}
	if err := os.Chmod(secretsDirectory, secretDirectoryMode); err != nil {
		return fmt.Errorf("secure secrets directory: %w", err)
	}

	for _, name := range []string{"engine-identity-key", "engine-encryption-key"} {
		key, err := randomBytes(32)
		if err != nil {
			return fmt.Errorf("generate %s: %w", name, err)
		}
		if err := createFileIfMissing(filepath.Join(secretsDirectory, name), key, secretFileMode); err != nil {
			return err
		}
	}

	token, created, err := createPrincipalsFile(filepath.Join(secretsDirectory, "engine-principals.json"))
	if err != nil {
		return err
	}

	fmt.Fprintln(output, "Local setup is ready. Review .env and config/config.yml before starting the service.")
	if created {
		fmt.Fprintln(output, "Store this API bearer token now; it will not be shown again:")
		fmt.Fprintln(output, token)
	}
	return nil
}

func readTemplate(root, name string) ([]byte, error) {
	path := filepath.Join(root, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read setup template %s: %w", name, err)
	}
	return data, nil
}

func createPrincipalsFile(path string) (string, bool, error) {
	tokenBytes, err := randomBytes(32)
	if err != nil {
		return "", false, fmt.Errorf("generate API token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	data, err := json.MarshalIndent(initPrincipalFile{
		Version: 1,
		Principals: []initPrincipal{{
			OwnerID:     "local-developer",
			Enabled:     true,
			TokenSHA256: hex.EncodeToString(hash[:]),
			Permissions: []string{"executions:create", "executions:read", "metrics:read", "cache:clear"},
		}},
	}, "", "  ")
	if err != nil {
		return "", false, err
	}
	data = append(data, '\n')
	created, err := createFileIfMissingWithStatus(path, data, secretFileMode)
	if err != nil {
		return "", false, err
	}
	if !created {
		return "", false, nil
	}
	return token, true, nil
}

func randomBytes(length int) ([]byte, error) {
	data := make([]byte, length)
	if _, err := rand.Read(data); err != nil {
		return nil, err
	}
	return data, nil
}

func createFileIfMissing(path string, data []byte, mode os.FileMode) error {
	_, err := createFileIfMissingWithStatus(path, data, mode)
	return err
}

func createFileIfMissingWithStatus(path string, data []byte, mode os.FileMode) (bool, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if errors.Is(err, os.ErrExist) {
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return false, fmt.Errorf("inspect existing %s: %w", path, statErr)
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("existing %s must be a regular file", path)
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()
	if err := file.Chmod(mode); err != nil {
		return false, fmt.Errorf("set permissions on %s: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
