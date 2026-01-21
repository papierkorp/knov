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

// handlers maps handler names to their implementations
var handlers map[string]ContentHandler

// Init initializes content handlers
func Init() {
	handlers = make(map[string]ContentHandler)
	handlers["markdown"] = NewMarkdownContentHandler()
	// Future: handlers["dokuwiki"] = NewDokuwikiContentHandler()
}

// GetHandler returns a content handler by name
func GetHandler(handlerType string) ContentHandler {
	return handlers[handlerType]
}

// GetAllHandlers returns all available handlers
func GetAllHandlers() map[string]ContentHandler {
	return handlers
}
