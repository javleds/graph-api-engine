package main

import (
	"flag"
	"fmt"
	"github.com/javleds/api-engine/internal/api/http/mapper"
	definition "github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
	"github.com/javleds/api-engine/internal/platform/config"
	"io"
	"os"
)

func validate(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	cfgPath := flags.String("config", "", "optional operator YAML config")
	path := flags.String("file", "", "input JSON file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *path == "" {
		return fmt.Errorf("--file is required")
	}
	limits := definition.DefaultValidationLimits()
	maxBytes, maxDepth := int64(262144), 64
	if *cfgPath != "" {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return err
		}
		limits = cfg.DefinitionLimits()
		maxBytes = int64(cfg.HTTP.MaxRequestBytes)
		maxDepth = cfg.Fetch.MaxJSONDepth
	}
	file, err := os.Open(*path)
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
