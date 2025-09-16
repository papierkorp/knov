// Package search provides different search engine implementations
package search

import (
	"fmt"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
)

// Engine ..
type Engine interface {
	Initialize() error
	IndexAllFiles() error
	SearchFiles(query string, limit int) ([]files.File, error)
}

var currentEngine Engine

// InitSearch ..
func InitSearch() error {
	engineType := configmanager.GetSearchEngine()
	logging.LogInfo("initializing search engine: %s", engineType)

	var engine Engine
	switch engineType {
	case "sqlite":
		engine = &SQLiteEngine{}
	case "memory":
		engine = &InMemoryEngine{}
	case "grep":
		engine = &GrepEngine{}
	default:
		logging.LogWarning("unknown search engine '%s', using sqlite", engineType)
		engine = &SQLiteEngine{}
	}

	err := engine.Initialize()
	if err != nil {
		return fmt.Errorf("failed to initialize %s search engine: %w", engineType, err)
	}

	err = engine.IndexAllFiles()
	if err != nil {
		return fmt.Errorf("failed to index all files for %s engine: %w", engineType, err)
	}

	currentEngine = engine
	return nil
}

// IndexAllFiles ..
func IndexAllFiles() error {
	if currentEngine == nil {
		return fmt.Errorf("search engine not initialized")
	}
	return currentEngine.IndexAllFiles()
}

// SearchFiles ..
func SearchFiles(query string, limit int) ([]files.File, error) {
	if currentEngine == nil {
		return nil, fmt.Errorf("search engine not initialized")
	}
	return currentEngine.SearchFiles(query, limit)
}
