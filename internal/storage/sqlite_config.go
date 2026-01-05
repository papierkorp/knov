// Package storage - SQLite config storage implementation
package storage

// SQLiteConfigStorage implements ConfigStorage using SQLite
type SQLiteConfigStorage struct {
	*baseSQLiteStorage
}

// NewSQLiteConfigStorage creates a new SQLite config storage
func NewSQLiteConfigStorage(dbPath string) (*SQLiteConfigStorage, error) {
	tables := map[string]TableDef{
		"config": {
			Columns: []ColumnDef{
				{Name: "key", Type: "TEXT", Primary: true, NotNull: true},
				{Name: "value", Type: "BLOB", NotNull: true},
				{Name: "updated_at", Type: "INTEGER", NotNull: true, Index: true},
			},
		},
	}

	base, err := newBaseSQLiteStorage(dbPath, tables, "config")
	if err != nil {
		return nil, err
	}

	if err := base.Init(); err != nil {
		return nil, err
	}

	return &SQLiteConfigStorage{
		baseSQLiteStorage: base,
	}, nil
}
