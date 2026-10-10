package parser

import (
	"knov/internal/markdown"
	"knov/internal/pathutils"
)

// PathlessRender is the filePath value for Render when the markdown has no source file on
// disk (a composed book, the concatenated changelog). It makes Render emit static HTML
// only: no per-header edit/pdf buttons, no section-edit buttons, a plain <table> instead
// of the live table editor.
const PathlessRender = ""

// Parser manages all operations for a specific file type
type Parser interface {
	// CanHandle returns true if this handler supports the file
	CanHandle(filename string) bool

	// Parse converts raw content to intermediate format if needed. filePath is the source file
	// relative links are read against, PathlessRender when there is none.
	Parse(content []byte, filePath string) ([]byte, error)

	// Render converts content to HTML. filePath is the metadata path of the source
	// file; pass PathlessRender when the content has no file on disk. editableSections
	// controls whether per-heading/per-section edit affordances are emitted; pass false
	// for editors with no inline section editing (list, todo, tracker, filter, index, book).
	Render(content []byte, filePath string, editableSections bool) ([]byte, error)

	// ExtractLinks returns the target (LinkTarget) of every internal link in content, read
	// against the doc docPath
	ExtractLinks(content []byte, docPath string) []pathutils.MetaPath

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
// frontmatterYAML is nil when no front matter is present. The rule lives in
// markdown.SplitFrontMatter so the heading scanner and this share one definition.
func StripFrontMatterBytes(content []byte) (frontmatter []byte, body []byte) {
	return markdown.SplitFrontMatter(content)
}
