package config

import (
	"fmt"
	"reflect"
)

// All numeric service limits are positive; optional group concurrency is checked separately.
func positiveValues(value any) error { return validatePositive(reflect.ValueOf(value), "") }
func validatePositive(value reflect.Value, path string) error {
	if value.Kind() == reflect.Struct {
		for i := 0; i < value.NumField(); i++ {
			name := value.Type().Field(i).Tag.Get("json")
			if err := validatePositive(value.Field(i), path+"/"+name); err != nil {
				return err
			}
		}
	}
	switch value.Kind() {
	case reflect.Int, reflect.Int64:
		if value.Int() <= 0 {
			return fmt.Errorf("configuration %s must be positive", path)
		}
	}
	return nil
}
