package filetype

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/logging"
)

type FilterHandler struct{}

func NewFilterHandler() *FilterHandler {
	return &FilterHandler{}
}

func (h *FilterHandler) CanHandle(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".filter"
}

func (h *FilterHandler) GetContent(filepath string) ([]byte, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		logging.LogError("failed to read filter file %s: %v", filepath, err)
		return nil, err
	}
	return content, nil
}

func (h *FilterHandler) Parse(content []byte) ([]byte, error) {
	// validate JSON
	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		logging.LogError("failed to parse filter json: %v", err)
		return nil, err
	}
	return content, nil
}

func (h *FilterHandler) Render(content []byte) ([]byte, error) {
	// Return raw JSON content - the file view handler will execute the filter
	return content, nil
}

func (h *FilterHandler) ExtractLinks(content []byte) []string {
	return []string{}
}

func (h *FilterHandler) Name() string {
	return "filter"
}
