package types

// TableData represents parsed table structure
type TableData struct {
	Headers []TableHeader
	Rows    [][]TableCell
	Total   int
}

// SimpleTableData represents basic table structure for content operations
type SimpleTableData struct {
	Headers    []string   `json:"headers"`
	Rows       [][]string `json:"rows"`
	Total      int        `json:"total"`
	TableIndex int        `json:"tableIndex"` // for UI operations
}

// TableHeader represents a column header with metadata
type TableHeader struct {
	Content   string
	DataType  string
	Align     string
	Sortable  bool
	ColumnIdx int
}

// TableCell represents a single table cell with metadata
type TableCell struct {
	Content  string
	DataType string
	Align    string
	RawValue string
}
