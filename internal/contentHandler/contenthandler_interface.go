package contentHandler

// ContentHandler provides advanced content manipulation capabilities for different file types
type ContentHandler interface {
	// ExtractSection extracts content of a specific section by ID
	ExtractSection(filePath, sectionID string) (string, error)

	// SaveSection saves content to a specific section by ID
	SaveSection(filePath, sectionID, content string) error

	// ExtractTable extracts table data at specific index, returns headers and rows
	ExtractTable(filePath string, tableIndex int) (headers []string, rows [][]string, err error)

	// SaveTable saves table data at specific index
	SaveTable(filePath string, tableIndex int, headers []string, rows [][]string) error

	// SupportsSection returns true if the handler supports section operations
	SupportsSection() bool

	// SupportsTable returns true if the handler supports table operations
	SupportsTable() bool

	// Name returns the handler identifier
	Name() string
}

// Registry manages content handlers by name
type Registry struct {
	handlers map[string]ContentHandler
}

// NewRegistry creates a new content handler registry
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]ContentHandler),
	}
}

// Register adds a content handler to the registry
func (r *Registry) Register(handler ContentHandler) {
	r.handlers[handler.Name()] = handler
}

// GetHandler returns a content handler by name
func (r *Registry) GetHandler(name string) ContentHandler {
	return r.handlers[name]
}

// GetAllHandlers returns all available handlers
func (r *Registry) GetAllHandlers() map[string]ContentHandler {
	result := make(map[string]ContentHandler)
	for name, handler := range r.handlers {
		result[name] = handler
	}
	return result
}

// Global registry instance
var contentHandlerRegistry *Registry

// Init initializes content handlers
func Init() {
	contentHandlerRegistry = NewRegistry()
	contentHandlerRegistry.Register(NewMarkdownContentHandler())
	// Future: contentHandlerRegistry.Register(NewDokuwikiContentHandler())
}

// GetHandler returns a content handler by name
func GetHandler(handlerType string) ContentHandler {
	return contentHandlerRegistry.GetHandler(handlerType)
}

// GetAllHandlers returns all available handlers
func GetAllHandlers() map[string]ContentHandler {
	return contentHandlerRegistry.GetAllHandlers()
}

// GetContentHandlerRegistry returns the global registry for direct access if needed
func GetContentHandlerRegistry() *Registry {
	return contentHandlerRegistry
}
