// Package filter - Operator implementations
package filter

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"knov/internal/types"
)

type filterValue any

// OperatorFunc is a function that applies an operator to field and filter values
type OperatorFunc func(fieldValue, filterValue any, fieldType types.FieldType) (bool, error)

// operatorRegistry maps operator types to their implementations
var operatorRegistry = map[types.OperatorType]OperatorFunc{
	types.OpEquals:    applyEquals,
	types.OpContains:  applyContains,
	types.OpIn:        applyIn,
	types.OpGreater:   applyGreater,
	types.OpLess:      applyLess,
	types.OpRegex:     applyRegex,
	types.OpGreaterEq: applyGreaterEq,
	types.OpLessEq:    applyLessEq,
}

// GetOperator returns the operator function for the given operator type
func GetOperator(op types.OperatorType) (OperatorFunc, error) {
	fn, ok := operatorRegistry[op]
	if !ok {
		return nil, fmt.Errorf("unknown operator: %s", op)
	}
	return fn, nil
}

// applyEquals checks if field value equals filter value
func applyEquals(fieldValue, filterValue any, fieldType types.FieldType) (bool, error) {
	switch fieldType {
	case types.StringField:
		fv, ok1 := fieldValue.(string)
		filterv, ok2 := filterValue.(string)
		if !ok1 || !ok2 {
			return false, fmt.Errorf("type mismatch for equals on string field")
		}
		return fv == filterv, nil

	case types.DateField:
		fv, ok1 := fieldValue.(time.Time)
		var filterv time.Time
		var ok2 bool

		// handle both time.Time and string for filter value
		switch v := filterValue.(type) {
		case time.Time:
			filterv = v
			ok2 = true
		case string:
			parsed, err := time.Parse("2006-01-02", v)
			if err != nil {
				parsed, err = time.Parse(time.RFC3339, v)
				if err != nil {
					return false, fmt.Errorf("invalid date format: %v", v)
				}
			}
			filterv = parsed
			ok2 = true
		}

		if !ok1 || !ok2 {
			return false, fmt.Errorf("type mismatch for equals on date field")
		}
		return fv.Equal(filterv), nil

	case types.IntField:
		fv, ok1 := fieldValue.(int64)
		filterv, ok2 := filterValue.(int64)
		if !ok1 || !ok2 {
			return false, fmt.Errorf("type mismatch for equals on int field")
		}
		return fv == filterv, nil

	case types.BoolField:
		fv, ok1 := fieldValue.(bool)
		filterv, ok2 := filterValue.(bool)
		if !ok1 || !ok2 {
			return false, fmt.Errorf("type mismatch for equals on bool field")
		}
		return fv == filterv, nil

	default:
		return false, fmt.Errorf("equals not supported for field type: %v", fieldType)
	}
}

// applyContains checks if field contains filter value
func applyContains(fieldValue, filterValue any, fieldType types.FieldType) (bool, error) {
	switch fieldType {
	case types.StringField:
		fv, ok1 := fieldValue.(string)
		filterv, ok2 := filterValue.(string)
		if !ok1 || !ok2 {
			return false, fmt.Errorf("type mismatch for contains on string field")
		}
		return strings.Contains(strings.ToLower(fv), strings.ToLower(filterv)), nil

	case types.ArrayField:
		// field value is array, check if it contains filter value
		filterv, ok := filterValue.(string)
		if !ok {
			return false, fmt.Errorf("filter value must be string for contains on array")
		}

		// handle different array types
		switch arr := fieldValue.(type) {
		case []string:
			for _, item := range arr {
				if strings.Contains(strings.ToLower(item), strings.ToLower(filterv)) {
					return true, nil
				}
			}
			return false, nil
		case []any:
			for _, item := range arr {
				if str, ok := item.(string); ok {
					if strings.Contains(strings.ToLower(str), strings.ToLower(filterv)) {
						return true, nil
					}
				}
			}
			return false, nil
		default:
			return false, fmt.Errorf("unsupported array type for contains")
		}

	default:
		return false, fmt.Errorf("contains not supported for field type: %v", fieldType)
	}
}

// applyIn checks if field value is in the filter value list
func applyIn(fieldValue, filterValue any, fieldType types.FieldType) (bool, error) {
	// filter value should be comma-separated string or []string
	var filterValues []string

	switch v := filterValue.(type) {
	case string:
		// split by comma
		filterValues = strings.Split(v, ",")
		for i := range filterValues {
			filterValues[i] = strings.TrimSpace(filterValues[i])
		}
	case []string:
		filterValues = v
	case []any:
		filterValues = make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				filterValues = append(filterValues, str)
			}
		}
	default:
		return false, fmt.Errorf("filter value for 'in' must be string or array")
	}

	switch fieldType {
	case types.StringField:
		fv, ok := fieldValue.(string)
		if !ok {
			return false, fmt.Errorf("type mismatch for in on string field")
		}
		return slices.Contains(filterValues, fv), nil

	case types.ArrayField:
		// field is array, check if any element matches any filter value
		switch arr := fieldValue.(type) {
		case []string:
			for _, item := range arr {
				if slices.Contains(filterValues, item) {
					return true, nil
				}
			}
			return false, nil
		case []any:
			for _, item := range arr {
				if str, ok := item.(string); ok {
					if slices.Contains(filterValues, str) {
						return true, nil
					}
				}
			}
			return false, nil
		default:
			return false, fmt.Errorf("unsupported array type for in")
		}

	default:
		return false, fmt.Errorf("in not supported for field type: %v", fieldType)
	}
}

// applyGreater checks if field value is greater than filter value
func applyGreater(fieldValue, filterValue any, fieldType types.FieldType) (bool, error) {
	switch fieldType {
	case types.DateField:
		fv, ok1 := fieldValue.(time.Time)
		var filterv time.Time
		var ok2 bool

		switch v := filterValue.(type) {
		case time.Time:
			filterv = v
			ok2 = true
		case string:
			parsed, err := time.Parse("2006-01-02", v)
			if err != nil {
				parsed, err = time.Parse(time.RFC3339, v)
				if err != nil {
					return false, fmt.Errorf("invalid date format: %v", v)
				}
			}
			filterv = parsed
			ok2 = true
		}

		if !ok1 || !ok2 {
			return false, fmt.Errorf("type mismatch for greater on date field")
		}
		return fv.After(filterv), nil

	case types.IntField:
		fv, ok1 := fieldValue.(int64)
		filterv, ok2 := filterValue.(int64)
		if !ok1 || !ok2 {
			return false, fmt.Errorf("type mismatch for greater on int field")
		}
		return fv > filterv, nil

	default:
		return false, fmt.Errorf("greater not supported for field type: %v", fieldType)
	}
}

// applyLess checks if field value is less than filter value
func applyLess(fieldValue, filterValue any, fieldType types.FieldType) (bool, error) {
	switch fieldType {
	case types.DateField:
		fv, ok1 := fieldValue.(time.Time)
		var filterv time.Time
		var ok2 bool

		switch v := filterValue.(type) {
		case time.Time:
			filterv = v
			ok2 = true
		case string:
			parsed, err := time.Parse("2006-01-02", v)
			if err != nil {
				parsed, err = time.Parse(time.RFC3339, v)
				if err != nil {
					return false, fmt.Errorf("invalid date format: %v", v)
				}
			}
			filterv = parsed
			ok2 = true
		}

		if !ok1 || !ok2 {
			return false, fmt.Errorf("type mismatch for less on date field")
		}
		return fv.Before(filterv), nil

	case types.IntField:
		fv, ok1 := fieldValue.(int64)
		filterv, ok2 := filterValue.(int64)
		if !ok1 || !ok2 {
			return false, fmt.Errorf("type mismatch for less on int field")
		}
		return fv < filterv, nil

	default:
		return false, fmt.Errorf("less not supported for field type: %v", fieldType)
	}
}

// applyGreaterEq checks if field value is greater than or equal to filter value
func applyGreaterEq(fieldValue, filterValue any, fieldType types.FieldType) (bool, error) {
	eq, err := applyEquals(fieldValue, filterValue, fieldType)
	if err != nil {
		return false, err
	}
	if eq {
		return true, nil
	}
	return applyGreater(fieldValue, filterValue, fieldType)
}

// applyLessEq checks if field value is less than or equal to filter value
func applyLessEq(fieldValue, filterValue any, fieldType types.FieldType) (bool, error) {
	eq, err := applyEquals(fieldValue, filterValue, fieldType)
	if err != nil {
		return false, err
	}
	if eq {
		return true, nil
	}
	return applyLess(fieldValue, filterValue, fieldType)
}

// applyRegex checks if field value matches filter regex pattern
func applyRegex(fieldValue, filterValue any, fieldType types.FieldType) (bool, error) {
	if fieldType != types.StringField {
		return false, fmt.Errorf("regex only supported for string fields")
	}

	fv, ok1 := fieldValue.(string)
	pattern, ok2 := filterValue.(string)
	if !ok1 || !ok2 {
		return false, fmt.Errorf("type mismatch for regex")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern: %v", err)
	}

	return re.MatchString(fv), nil
}
