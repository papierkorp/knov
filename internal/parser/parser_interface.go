package parser

import "bytes"

// Parser manages all operations for a specific file type
type Parser interface {
	// CanHandle returns true if this handler supports the file
	CanHandle(filename string) bool

	// Parse converts raw content to intermediate format if needed
	Parse(content []byte) ([]byte, error)

	// Render converts content to HTML
	Render(content []byte, filePath string) ([]byte, error)

	// ExtractLinks extracts internal links from content
	ExtractLinks(content []byte) []string

	// Name returns the handler identifier
	Name() string
}

// Registry manages file type handlers
type Registry struct {
	handlers []Parser
}

func NewRegistry() *Registry {
	return &Registry{handlers: make([]Parser, 0)}
}

func (r *Registry) Register(h Parser) {
	r.handlers = append(r.handlers, h)
}

func (r *Registry) GetHandler(filename string) Parser {
	for _, h := range r.handlers {
		if h.CanHandle(filename) {
			return h
		}
	}
	return nil
}

// Global registry instance
var parserRegistry *Registry

// Init initializes parsers
func Init() {
	parserRegistry = NewRegistry()
	parserRegistry.Register(NewMarkdownHandler())
	parserRegistry.Register(NewDokuwikiHandler())
	parserRegistry.Register(NewPlaintextHandler())
}

// GetParserRegistry returns the global parser registry
func GetParserRegistry() *Registry {
	return parserRegistry
}

// StripFrontMatter removes a YAML front matter block (---\n...\n---\n) from content.
// Returns the body without the front matter block.
// If no front matter is present the content is returned unchanged.
func StripFrontMatter(content []byte) []byte {
	_, body := StripFrontMatterBytes(content)
	return body
}

// StripFrontMatterBytes splits content into (frontmatterYAML, body).
// frontmatterYAML is nil when no front matter is present.
func StripFrontMatterBytes(content []byte) (frontmatter []byte, body []byte) {
	delimiter := []byte("---\n")
	closing := []byte("\n---\n")

	if !bytes.HasPrefix(content, delimiter) {
		return nil, content
	}
	rest := content[len(delimiter):]
	idx := bytes.Index(rest, closing)
	if idx < 0 {
		return nil, content // malformed — leave untouched
	}
	return rest[:idx], rest[idx+len(closing):]
}
