package config

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var configValidator = validator.New(validator.WithRequiredStructEnabled())

func validateFields(value Config) error {
	if err := configValidator.Struct(value); err != nil {
		return fmt.Errorf("invalid configuration fields: %w", err)
	}
	return nil
}
