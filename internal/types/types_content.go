package types

// TableData represents parsed table structure
type TableData struct {
	Headers []TableHeader
	Rows    [][]TableCell
	Total   int
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
