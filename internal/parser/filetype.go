package parser

// Handler manages all operations for a specific file type
type Handler interface {
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
	handlers []Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make([]Handler, 0)}
}

func (r *Registry) Register(h Handler) {
	r.handlers = append(r.handlers, h)
}

func (r *Registry) GetHandler(filename string) Handler {
	for _, h := range r.handlers {
		if h.CanHandle(filename) {
			return h
		}
	}
	return nil
}
