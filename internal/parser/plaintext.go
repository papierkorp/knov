package parser

import (
	"os"
	"strings"

	"knov/internal/logging"
)

type PlaintextHandler struct{}

func NewPlaintextHandler() *PlaintextHandler {
	return &PlaintextHandler{}
}

func (h *PlaintextHandler) CanHandle(filename string) bool {
	return true
}

func (h *PlaintextHandler) GetContent(filepath string) ([]byte, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", filepath, err)
		return nil, err
	}
	return content, nil
}

func (h *PlaintextHandler) Parse(content []byte) ([]byte, error) {
	s := string(content)
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return []byte(s), nil
}

func (h *PlaintextHandler) Render(content []byte) ([]byte, error) {
	html := "<pre>" + string(content) + "</pre>"
	return []byte(html), nil
}

func (h *PlaintextHandler) ExtractLinks(content []byte) []string {
	return []string{}
}

func (h *PlaintextHandler) Name() string {
	return "plaintext"
}
