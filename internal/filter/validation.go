// Package filter - Criteria validation
package filter

import (
	"fmt"
	"time"

	"knov/internal/types"
)

// ValidateCriteria validates a single criterion
func ValidateCriteria(c Criteria) error {
	// get field descriptor
	field, ok := types.GetFieldDescriptor(c.Metadata)
	if !ok {
		return fmt.Errorf("unknown metadata field: %s (valid fields: %v)", c.Metadata, types.AllFieldNames())
	}

	// check if operator is supported for this field
	opType := types.OperatorType(c.Operator)
	if !field.SupportsOperator(opType) {
		return fmt.Errorf("operator '%s' not supported for field '%s' (supported: %v)",
			c.Operator, field.Name, field.Operators)
	}

	// validate value type matches field type
	if err := validateValueType(c.Value, field.Type, c.Operator); err != nil {
		return fmt.Errorf("invalid value for field '%s': %v", field.Name, err)
	}

	// validate action
	if c.Action != "include" && c.Action != "exclude" {
		return fmt.Errorf("invalid action: %s (must be 'include' or 'exclude')", c.Action)
	}

	return nil
}

// ValidateConfig validates an entire filter configuration
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// validate logic
	if config.Logic != "and" && config.Logic != "or" {
		return fmt.Errorf("invalid logic: %s (must be 'and' or 'or')", config.Logic)
	}

	// validate each criterion
	for i, criterion := range config.Criteria {
		if err := ValidateCriteria(criterion); err != nil {
			return fmt.Errorf("criterion %d: %v", i, err)
		}
	}

	return nil
}

// validateValueType checks if the value type is appropriate for the field type
func validateValueType(value any, fieldType types.FieldType, operator string) error {
	if value == nil {
		return fmt.Errorf("value cannot be nil")
	}

	switch fieldType {
	case types.StringField:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}

	case types.ArrayField:
		// for array fields, value depends on operator
		if operator == "in" {
			// can be string (comma-separated) or []string
			switch value.(type) {
			case string, []string, []any:
				return nil
			default:
				return fmt.Errorf("expected string or array for 'in' operator, got %T", value)
			}
		} else {
			// contains operator expects string
			if _, ok := value.(string); !ok {
				return fmt.Errorf("expected string for 'contains' on array field, got %T", value)
			}
		}

	case types.DateField:
		// can be time.Time or string
		switch v := value.(type) {
		case time.Time:
			return nil
		case string:
			// try to parse as date
			_, err := time.Parse("2006-01-02", v)
			if err != nil {
				_, err = time.Parse(time.RFC3339, v)
				if err != nil {
					return fmt.Errorf("invalid date format: %v", v)
				}
			}
			return nil
		default:
			return fmt.Errorf("expected date (time.Time or string), got %T", value)
		}

	case types.IntField:
		switch value.(type) {
		case int, int32, int64:
			return nil
		default:
			return fmt.Errorf("expected integer, got %T", value)
		}

	case types.BoolField:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	}

	return nil
}

// GetFieldType returns the field type for a given metadata field name
func GetFieldType(fieldName string) (types.FieldType, error) {
	field, ok := types.GetFieldDescriptor(fieldName)
	if !ok {
		return 0, fmt.Errorf("unknown field: %s", fieldName)
	}
	return field.Type, nil
}
