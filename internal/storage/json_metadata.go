// Package storage - JSON metadata storage implementation
package storage

import (
	"encoding/json"
	"os"

	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/types"
)

// JSONMetadataStorage implements MetadataStorage using JSON files
type JSONMetadataStorage struct {
	*baseJSONStorage
}

// NewJSONMetadataStorage creates a new JSON metadata storage
func NewJSONMetadataStorage(basePath string) (*JSONMetadataStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	return &JSONMetadataStorage{
		baseJSONStorage: &baseJSONStorage{
			basePath: basePath,
		},
	}, nil
}

// Query searches for metadata matching criteria
func (js *JSONMetadataStorage) Query(criteria []types.Criteria, logic string) ([][]byte, error) {
	allKeys, err := js.List("")
	if err != nil {
		return nil, err
	}

	var results [][]byte

	for _, key := range allKeys {
		data, err := js.Get(key)
		if err != nil || data == nil {
			continue
		}

		var item map[string]any
		if err := json.Unmarshal(data, &item); err != nil {
			continue
		}

		if js.matchesCriteria(item, criteria, logic) {
			results = append(results, data)
		}
	}

	logging.LogDebug("query returned %d results", len(results))
	return results, nil
}

func (js *JSONMetadataStorage) matchesCriteria(item map[string]any, criteria []types.Criteria, logic string) bool {
	if len(criteria) == 0 {
		return true
	}

	matches := make([]bool, len(criteria))

	for i, c := range criteria {
		// get field descriptor
		field, ok := types.GetFieldDescriptor(c.Metadata)
		if !ok {
			logging.LogWarning("unknown field in json query: %s", c.Metadata)
			matches[i] = false
			continue
		}

		// get field value from item
		fieldValue, exists := item[field.DBColumn]
		if !exists {
			matches[i] = false
			continue
		}

		// use operator functions from filter package
		operator, err := filter.GetOperator(types.OperatorType(c.Operator))
		if err != nil {
			logging.LogWarning("unknown operator in json query: %s", c.Operator)
			matches[i] = false
			continue
		}

		match, err := operator(fieldValue, c.Value, field.Type)
		if err != nil {
			logging.LogWarning("error applying operator in json query: %v", err)
			matches[i] = false
			continue
		}

		matches[i] = match
	}

	if logic == "or" {
		for _, match := range matches {
			if match {
				return true
			}
		}
		return false
	}

	for _, match := range matches {
		if !match {
			return false
		}
	}
	return true
}

// GetAll retrieves all metadata entries
func (js *JSONMetadataStorage) GetAll() (map[string][]byte, error) {
	allKeys, err := js.List("")
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for _, key := range allKeys {
		data, err := js.Get(key)
		if err != nil || data == nil {
			continue
		}
		result[key] = data
	}

	return result, nil
}
