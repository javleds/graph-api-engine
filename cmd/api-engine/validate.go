package main

import (
	"fmt"
	"github.com/javleds/api-engine/internal/api/http/mapper"
	definition "github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
	"github.com/javleds/api-engine/internal/platform/config"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newValidateCommand() *cobra.Command {
	var configPath string
	var path string
	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate an execution definition",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return runValidate(configPath, path)
		},
	}
	command.Flags().StringVar(&configPath, "config", "", "optional operator YAML config")
	command.Flags().StringVar(&path, "file", "", "input JSON file")
	return command
}

func runValidate(configPath, path string) error {
	if path == "" {
		return fmt.Errorf("--file is required")
	}
	limits := definition.DefaultValidationLimits()
	maxBytes, maxDepth := int64(262144), 64
	if configPath != "" {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		limits = cfg.DefinitionLimits()
		maxBytes = int64(cfg.HTTP.MaxRequestBytes)
		maxDepth = cfg.Fetch.MaxJSONDepth
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(raw)) > maxBytes {
		return fmt.Errorf("REQUEST_TOO_LARGE")
	}
	input, err := schema.Parse(raw, maxDepth)
	if err != nil {
		return err
	}
	if _, err := mapper.Compile(input, limits); err != nil {
		return err
	}
	fmt.Println("Definition is valid.")
	return nil
}
