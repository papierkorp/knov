package parser

import (
	"strings"

	"knov/internal/pathutils"
)

type PlaintextHandler struct{}

func NewPlaintextHandler() *PlaintextHandler {
	return &PlaintextHandler{}
}

func (h *PlaintextHandler) CanHandle(filename string) bool {
	return true
}

func (h *PlaintextHandler) Parse(content []byte, filePath string) ([]byte, error) {
	content = StripFrontMatter(content)
	s := string(content)
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return []byte(s), nil
}

func (h *PlaintextHandler) Render(content []byte, filePath string, editableSections bool) ([]byte, error) {
	html := "<pre>" + string(content) + "</pre>"
	return []byte(html), nil
}

func (h *PlaintextHandler) ExtractLinks(content []byte, docPath string) []pathutils.MetaPath {
	return nil
}

func (h *PlaintextHandler) Name() string {
	return "plaintext"
}
