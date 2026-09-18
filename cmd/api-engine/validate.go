package main

import (
	"flag"
	"fmt"
	"github.com/javleds/api-engine/internal/api/http/mapper"
	definition "github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
	"os"
)

func validate(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	path := flags.String("file", "", "input JSON file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *path == "" {
		return fmt.Errorf("--file is required")
	}
	raw, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	input, err := schema.Parse(raw, 64)
	if err != nil {
		return err
	}
	if _, err := mapper.Compile(input, definition.DefaultValidationLimits()); err != nil {
		return err
	}
	fmt.Println("Definition is valid.")
	return nil
}
