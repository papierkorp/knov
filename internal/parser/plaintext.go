package parser

import (
	"strings"
)

type PlaintextHandler struct{}

func NewPlaintextHandler() *PlaintextHandler {
	return &PlaintextHandler{}
}

func (h *PlaintextHandler) CanHandle(filename string) bool {
	return true
}

func (h *PlaintextHandler) Parse(content []byte) ([]byte, error) {
	s := string(content)
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return []byte(s), nil
}

func (h *PlaintextHandler) Render(content []byte, filePath string) ([]byte, error) {
	html := "<pre>" + string(content) + "</pre>"
	return []byte(html), nil
}

func (h *PlaintextHandler) ExtractLinks(content []byte) []string {
	return []string{}
}

func (h *PlaintextHandler) Name() string {
	return "plaintext"
}
