// Package storage - JSON config storage implementation
package storage

import "os"

// JSONConfigStorage implements ConfigStorage using JSON files
type JSONConfigStorage struct {
	*baseJSONStorage
}

// NewJSONConfigStorage creates a new JSON config storage
func NewJSONConfigStorage(basePath string) (*JSONConfigStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	return &JSONConfigStorage{
		baseJSONStorage: &baseJSONStorage{
			basePath: basePath,
		},
	}, nil
}
