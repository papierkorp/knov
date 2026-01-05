// Package storage - SQLite metadata storage implementation
package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"knov/internal/logging"
	"knov/internal/types"
)

// SQLiteMetadataStorage implements MetadataStorage using SQLite with indexed fields
type SQLiteMetadataStorage struct {
	*baseSQLiteStorage
}

// NewSQLiteMetadataStorage creates a new SQLite metadata storage
func NewSQLiteMetadataStorage(dbPath string) (*SQLiteMetadataStorage, error) {
	// define metadata table schema
	// NOTE: This schema is manually based on files.Metadata struct
	// Keep in sync when adding/removing fields from files.Metadata
	// Cannot auto-generate due to import cycle: files Ã¢â€ â€™ storage Ã¢â€ â€™ files
	tables := map[string]TableDef{
		"metadata": {
			Columns: []ColumnDef{
				// primary key
				{Name: "path", Type: "TEXT", Primary: true, NotNull: true},

				// scalar fields
				{Name: "name", Type: "TEXT", NotNull: true},
				{Name: "title", Type: "TEXT"},
				{Name: "collection", Type: "TEXT", Index: true, IndexWhere: "collection IS NOT NULL"},
				{Name: "type", Type: "TEXT", Index: true, IndexWhere: "type IS NOT NULL"},
				{Name: "status", Type: "TEXT", Index: true, IndexWhere: "status IS NOT NULL"},
				{Name: "priority", Type: "TEXT", Index: true, IndexWhere: "priority IS NOT NULL"},
				{Name: "size", Type: "INTEGER"},

				// timestamps
				{Name: "createdAt", Type: "INTEGER", Index: true},
				{Name: "lastEdited", Type: "INTEGER", Index: true},
				{Name: "targetDate", Type: "INTEGER"},

				// array fields stored as JSON TEXT
				{Name: "folders", Type: "TEXT"},     // JSON array
				{Name: "tags", Type: "TEXT"},        // JSON array
				{Name: "boards", Type: "TEXT"},      // JSON array
				{Name: "ancestor", Type: "TEXT"},    // JSON array
				{Name: "parents", Type: "TEXT"},     // JSON array
				{Name: "kids", Type: "TEXT"},        // JSON array
				{Name: "usedLinks", Type: "TEXT"},   // JSON array
				{Name: "linksToHere", Type: "TEXT"}, // JSON array

				// PARA fields stored as JSON TEXT
				{Name: "para_projects", Type: "TEXT"},  // JSON array
				{Name: "para_areas", Type: "TEXT"},     // JSON array
				{Name: "para_resources", Type: "TEXT"}, // JSON array
				{Name: "para_archive", Type: "TEXT"},   // JSON array

				// metadata
				{Name: "updated_at", Type: "INTEGER", NotNull: true, Index: true},
			},
		},
	}

	base, err := newBaseSQLiteStorage(dbPath, tables, "metadata")
	if err != nil {
		return nil, err
	}

	if err := base.Init(); err != nil {
		return nil, err
	}

	return &SQLiteMetadataStorage{
		baseSQLiteStorage: base,
	}, nil
}

// Set stores metadata with all fields as columns
func (s *SQLiteMetadataStorage) Set(key string, data []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// parse metadata to extract all fields
	var metadata map[string]any
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	// extract scalar fields
	name := getString(metadata, "name")
	title := getString(metadata, "title")
	collection := getString(metadata, "collection")
	fileType := getString(metadata, "type")
	status := getString(metadata, "status")
	priority := getString(metadata, "priority")
	size := getInt64(metadata, "size")

	// extract timestamps
	createdAt := getTimestamp(metadata, "createdAt")
	lastEdited := getTimestamp(metadata, "lastEdited")
	targetDate := getTimestamp(metadata, "targetDate")

	// extract array fields and convert to JSON
	folders := arrayToJSON(metadata, "folders")
	tags := arrayToJSON(metadata, "tags")
	boards := arrayToJSON(metadata, "boards")
	ancestor := arrayToJSON(metadata, "ancestor")
	parents := arrayToJSON(metadata, "parents")
	kids := arrayToJSON(metadata, "kids")
	usedLinks := arrayToJSON(metadata, "usedLinks")
	linksToHere := arrayToJSON(metadata, "linksToHere")

	// extract PARA fields
	paraProjects := ""
	paraAreas := ""
	paraResources := ""
	paraArchive := ""
	if para, ok := metadata["para"].(map[string]any); ok {
		paraProjects = arrayToJSON(para, "projects")
		paraAreas = arrayToJSON(para, "areas")
		paraResources = arrayToJSON(para, "resources")
		paraArchive = arrayToJSON(para, "archive")
		logging.LogDebug("extracted PARA for %s: projects=%s, areas=%s, resources=%s, archive=%s",
			key, paraProjects, paraAreas, paraResources, paraArchive)
	} else {
		logging.LogDebug("no PARA data found for %s (para field: %v)", key, metadata["para"])
	}

	// insert/update with all fields
	query := `INSERT INTO metadata (
		path, name, title, collection, type, status, priority, size,
		createdAt, lastEdited, targetDate,
		folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere,
		para_projects, para_areas, para_resources, para_archive,
		updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		name = ?, title = ?, collection = ?, type = ?, status = ?, priority = ?, size = ?,
		createdAt = ?, lastEdited = ?, targetDate = ?,
		folders = ?, tags = ?, boards = ?, ancestor = ?, parents = ?, kids = ?, usedLinks = ?, linksToHere = ?,
		para_projects = ?, para_areas = ?, para_resources = ?, para_archive = ?,
		updated_at = ?`

	now := time.Now().Unix()

	_, err := s.db.Exec(query,
		// insert values
		key, name, title, collection, fileType, status, priority, size,
		createdAt, lastEdited, targetDate,
		folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere,
		paraProjects, paraAreas, paraResources, paraArchive,
		now,
		// update values
		name, title, collection, fileType, status, priority, size,
		createdAt, lastEdited, targetDate,
		folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere,
		paraProjects, paraAreas, paraResources, paraArchive,
		now)
	if err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	logging.LogDebug("stored metadata for key: %s", key)
	return nil
}

// Get retrieves metadata by reconstructing JSON from columns
func (s *SQLiteMetadataStorage) Get(key string) ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	query := `SELECT name, title, collection, type, status, priority, size,
	          createdAt, lastEdited, targetDate,
	          folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere,
	          para_projects, para_areas, para_resources, para_archive
	          FROM metadata WHERE path = ?`

	var name, title, collection, fileType, status, priority string
	var size, createdAt, lastEdited, targetDate int64
	var folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere string
	var paraProjects, paraAreas, paraResources, paraArchive string

	err := s.db.QueryRow(query, key).Scan(
		&name, &title, &collection, &fileType, &status, &priority, &size,
		&createdAt, &lastEdited, &targetDate,
		&folders, &tags, &boards, &ancestor, &parents, &kids, &usedLinks, &linksToHere,
		&paraProjects, &paraAreas, &paraResources, &paraArchive,
	)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get metadata for %s: %w", key, err)
	}

	// reconstruct metadata JSON
	metadata := map[string]any{
		"path":     key,
		"name":     name,
		"title":    title,
		"type":     fileType,
		"status":   status,
		"priority": priority,
		"size":     size,
	}

	// add optional fields
	if collection != "" {
		metadata["collection"] = collection
	}

	// add timestamps
	if createdAt > 0 {
		metadata["createdAt"] = time.Unix(createdAt, 0).Format(time.RFC3339)
	}
	if lastEdited > 0 {
		metadata["lastEdited"] = time.Unix(lastEdited, 0).Format(time.RFC3339)
	}
	if targetDate > 0 {
		metadata["targetDate"] = time.Unix(targetDate, 0).Format(time.RFC3339)
	}

	// parse array fields
	if folders != "" {
		metadata["folders"] = jsonToArray(folders)
	} else {
		metadata["folders"] = []string{}
	}
	if tags != "" {
		metadata["tags"] = jsonToArray(tags)
	} else {
		metadata["tags"] = []string{}
	}
	if boards != "" {
		metadata["boards"] = jsonToArray(boards)
	} else {
		metadata["boards"] = []string{}
	}
	if ancestor != "" {
		metadata["ancestor"] = jsonToArray(ancestor)
	} else {
		metadata["ancestor"] = []string{}
	}
	if parents != "" {
		metadata["parents"] = jsonToArray(parents)
	} else {
		metadata["parents"] = []string{}
	}
	if kids != "" {
		metadata["kids"] = jsonToArray(kids)
	} else {
		metadata["kids"] = []string{}
	}
	if usedLinks != "" {
		metadata["usedLinks"] = jsonToArray(usedLinks)
	} else {
		metadata["usedLinks"] = []string{}
	}
	if linksToHere != "" {
		metadata["linksToHere"] = jsonToArray(linksToHere)
	} else {
		metadata["linksToHere"] = []string{}
	}

	// build PARA object
	para := make(map[string]any)
	if paraProjects != "" {
		para["projects"] = jsonToArray(paraProjects)
	}
	if paraAreas != "" {
		para["areas"] = jsonToArray(paraAreas)
	}
	if paraResources != "" {
		para["resources"] = jsonToArray(paraResources)
	}
	if paraArchive != "" {
		para["archive"] = jsonToArray(paraArchive)
	}
	if len(para) > 0 {
		metadata["para"] = para
	}

	// convert to JSON
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	logging.LogDebug("retrieved metadata for key: %s", key)
	return data, nil
}

// Query searches metadata by criteria
func (s *SQLiteMetadataStorage) Query(criteria []types.Criteria, logic string) ([][]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(criteria) == 0 {
		return nil, nil
	}

	var whereClauses []string
	var args []any

	for i, c := range criteria {
		clause, clauseArgs, err := s.buildWhereClause(c)
		if err != nil {
			return nil, fmt.Errorf("failed to convert criterion %d (%s %s %v): %w",
				i, c.Metadata, c.Operator, c.Value, err)
		}
		whereClauses = append(whereClauses, clause)
		args = append(args, clauseArgs...)
	}

	if len(whereClauses) == 0 {
		return nil, nil
	}

	// combine clauses
	logicOp := "AND"
	if strings.ToUpper(logic) == "OR" {
		logicOp = "OR"
	}
	whereClause := strings.Join(whereClauses, " "+logicOp+" ")

	query := fmt.Sprintf(`SELECT path, name, title, collection, type, status, priority, size,
	          createdAt, lastEdited, targetDate,
	          folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere,
	          para_projects, para_areas, para_resources, para_archive
	          FROM metadata WHERE %s`, whereClause)

	logging.LogDebug("sqlite query: %s with args: %v", query, args)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query metadata: %w", err)
	}
	defer rows.Close()

	var results [][]byte
	for rows.Next() {
		var path, name, title, collection, fileType, status, priority string
		var size, createdAt, lastEdited, targetDate int64
		var folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere string
		var paraProjects, paraAreas, paraResources, paraArchive string

		if err := rows.Scan(
			&path, &name, &title, &collection, &fileType, &status, &priority, &size,
			&createdAt, &lastEdited, &targetDate,
			&folders, &tags, &boards, &ancestor, &parents, &kids, &usedLinks, &linksToHere,
			&paraProjects, &paraAreas, &paraResources, &paraArchive,
		); err != nil {
			continue
		}

		// debug log PARA fields when querying
		if paraProjects != "" || paraAreas != "" {
			logging.LogDebug("query row %s: para_projects=%q, para_areas=%q", path, paraProjects, paraAreas)
		}

		// reconstruct JSON (same logic as Get)
		metadata := map[string]any{
			"path":        path,
			"name":        name,
			"title":       title,
			"type":        fileType,
			"status":      status,
			"priority":    priority,
			"size":        size,
			"folders":     jsonToArrayOrEmpty(folders),
			"tags":        jsonToArrayOrEmpty(tags),
			"boards":      jsonToArrayOrEmpty(boards),
			"ancestor":    jsonToArrayOrEmpty(ancestor),
			"parents":     jsonToArrayOrEmpty(parents),
			"kids":        jsonToArrayOrEmpty(kids),
			"usedLinks":   jsonToArrayOrEmpty(usedLinks),
			"linksToHere": jsonToArrayOrEmpty(linksToHere),
		}

		if collection != "" {
			metadata["collection"] = collection
		}
		if createdAt > 0 {
			metadata["createdAt"] = time.Unix(createdAt, 0).Format(time.RFC3339)
		}
		if lastEdited > 0 {
			metadata["lastEdited"] = time.Unix(lastEdited, 0).Format(time.RFC3339)
		}
		if targetDate > 0 {
			metadata["targetDate"] = time.Unix(targetDate, 0).Format(time.RFC3339)
		}

		// build PARA
		para := make(map[string]any)
		if paraProjects != "" {
			para["projects"] = jsonToArray(paraProjects)
		}
		if paraAreas != "" {
			para["areas"] = jsonToArray(paraAreas)
		}
		if paraResources != "" {
			para["resources"] = jsonToArray(paraResources)
		}
		if paraArchive != "" {
			para["archive"] = jsonToArray(paraArchive)
		}
		if len(para) > 0 {
			metadata["para"] = para
		}

		data, err := json.Marshal(metadata)
		if err != nil {
			continue
		}
		results = append(results, data)
	}

	logging.LogDebug("sqlite query returned %d results", len(results))

	// debug: if this was a PARA query that returned 0 results, dump all PARA data
	if len(results) == 0 && len(criteria) > 0 {
		for _, c := range criteria {
			if c.Metadata == "para_projects" || c.Metadata == "para_areas" || c.Metadata == "para_resources" {
				logging.LogDebug("PARA query returned 0 results, dumping all PARA data from database")
				dumpRows, _ := s.db.Query(`SELECT path, para_projects, para_areas FROM metadata WHERE para_projects != '' OR para_areas != '' LIMIT 5`)
				if dumpRows != nil {
					defer dumpRows.Close()
					for dumpRows.Next() {
						var p, projects, areas string
						dumpRows.Scan(&p, &projects, &areas)
						logging.LogDebug("  DB row: path=%s, para_projects=%q, para_areas=%q", p, projects, areas)
					}
				}
				break
			}
		}
	}

	return results, nil
}

func (s *SQLiteMetadataStorage) buildWhereClause(c types.Criteria) (string, []any, error) {
	// get field descriptor from registry
	field, ok := types.GetFieldDescriptor(c.Metadata)
	if !ok {
		return "", nil, fmt.Errorf("unknown field: %s", c.Metadata)
	}

	// check operator is supported
	if !field.SupportsOperator(types.OperatorType(c.Operator)) {
		return "", nil, fmt.Errorf("operator %s not supported for field %s", c.Operator, field.Name)
	}

	value := c.Value

	// handle array fields (including PARA fields)
	if field.IsArray {
		return s.buildArrayWhereClause(field, c.Operator, value)
	}

	// handle date fields
	if field.Type == types.DateField {
		return s.buildDateWhereClause(field, c.Operator, value)
	}

	// handle scalar fields (string, int, etc.)
	return s.buildScalarWhereClause(field, c.Operator, value)
}

// buildArrayWhereClause builds WHERE clause for array fields (tags, folders, boards, PARA)
func (s *SQLiteMetadataStorage) buildArrayWhereClause(field *types.FieldDescriptor, operator, value string) (string, []any, error) {
	switch operator {
	case "equals":
		// exact match: array contains this exact value
		clause := fmt.Sprintf("EXISTS (SELECT 1 FROM json_each(%s) WHERE value = ?)", field.DBColumn)
		return clause, []any{value}, nil

	case "contains":
		// substring match: array contains a value that contains this substring
		clause := fmt.Sprintf("EXISTS (SELECT 1 FROM json_each(%s) WHERE value LIKE ?)", field.DBColumn)
		return clause, []any{"%" + value + "%"}, nil

	case "in":
		// check if array contains any of the values
		values := strings.Split(value, ",")
		var conditions []string
		var args []any
		for _, val := range values {
			conditions = append(conditions, "value = ?")
			args = append(args, strings.TrimSpace(val))
		}
		clause := fmt.Sprintf("EXISTS (SELECT 1 FROM json_each(%s) WHERE %s)",
			field.DBColumn, strings.Join(conditions, " OR "))
		return clause, args, nil

	default:
		return "", nil, fmt.Errorf("unsupported operator %s for array field %s", operator, field.Name)
	}
}

// buildDateWhereClause builds WHERE clause for date fields
func (s *SQLiteMetadataStorage) buildDateWhereClause(field *types.FieldDescriptor, operator, value string) (string, []any, error) {
	// parse date string to Unix timestamp
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		// try RFC3339 format
		t, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return "", nil, fmt.Errorf("invalid date format: %v", value)
		}
	}
	unixTime := t.Unix()

	switch operator {
	case "equals":
		return fmt.Sprintf("%s = ?", field.DBColumn), []any{unixTime}, nil
	case "greater", "gt":
		return fmt.Sprintf("%s > ?", field.DBColumn), []any{unixTime}, nil
	case "less", "lt":
		return fmt.Sprintf("%s < ?", field.DBColumn), []any{unixTime}, nil
	case "gte":
		return fmt.Sprintf("%s >= ?", field.DBColumn), []any{unixTime}, nil
	case "lte":
		return fmt.Sprintf("%s <= ?", field.DBColumn), []any{unixTime}, nil
	default:
		return "", nil, fmt.Errorf("unsupported operator %s for date field %s", operator, field.Name)
	}
}

// buildScalarWhereClause builds WHERE clause for scalar fields (string, int, etc.)
func (s *SQLiteMetadataStorage) buildScalarWhereClause(field *types.FieldDescriptor, operator, value string) (string, []any, error) {
	switch operator {
	case "equals":
		return fmt.Sprintf("%s = ?", field.DBColumn), []any{value}, nil

	case "contains":
		return fmt.Sprintf("%s LIKE ?", field.DBColumn), []any{"%" + value + "%"}, nil

	case "in":
		// split comma-separated values and create SQL IN clause
		values := strings.Split(value, ",")
		placeholders := make([]string, len(values))
		args := make([]any, len(values))
		for i, val := range values {
			placeholders[i] = "?"
			args[i] = strings.TrimSpace(val)
		}
		return fmt.Sprintf("%s IN (%s)", field.DBColumn, strings.Join(placeholders, ",")), args, nil

	case "regex":
		// SQLite REGEXP requires custom function
		return "", nil, fmt.Errorf("regex operator not supported in sqlite (use in-memory filtering)")

	default:
		return "", nil, fmt.Errorf("unsupported operator %s for scalar field %s", operator, field.Name)
	}
}

// BulkSet stores multiple metadata entries
func (s *SQLiteMetadataStorage) BulkSet(items map[string][]byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO metadata (
		path, name, title, collection, type, status, priority, size,
		createdAt, lastEdited, targetDate,
		folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere,
		para_projects, para_areas, para_resources, para_archive,
		updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		name = ?, title = ?, collection = ?, type = ?, status = ?, priority = ?, size = ?,
		createdAt = ?, lastEdited = ?, targetDate = ?,
		folders = ?, tags = ?, boards = ?, ancestor = ?, parents = ?, kids = ?, usedLinks = ?, linksToHere = ?,
		para_projects = ?, para_areas = ?, para_resources = ?, para_archive = ?,
		updated_at = ?`

	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()

	for key, data := range items {
		var metadata map[string]any
		if err := json.Unmarshal(data, &metadata); err != nil {
			logging.LogWarning("failed to parse metadata for %s: %v", key, err)
			continue
		}

		// extract all fields
		name := getString(metadata, "name")
		title := getString(metadata, "title")
		collection := getString(metadata, "collection")
		fileType := getString(metadata, "type")
		status := getString(metadata, "status")
		priority := getString(metadata, "priority")
		size := getInt64(metadata, "size")
		createdAt := getTimestamp(metadata, "createdAt")
		lastEdited := getTimestamp(metadata, "lastEdited")
		targetDate := getTimestamp(metadata, "targetDate")

		folders := arrayToJSON(metadata, "folders")
		tags := arrayToJSON(metadata, "tags")
		boards := arrayToJSON(metadata, "boards")
		ancestor := arrayToJSON(metadata, "ancestor")
		parents := arrayToJSON(metadata, "parents")
		kids := arrayToJSON(metadata, "kids")
		usedLinks := arrayToJSON(metadata, "usedLinks")
		linksToHere := arrayToJSON(metadata, "linksToHere")

		paraProjects := ""
		paraAreas := ""
		paraResources := ""
		paraArchive := ""
		if para, ok := metadata["para"].(map[string]any); ok {
			paraProjects = arrayToJSON(para, "projects")
			paraAreas = arrayToJSON(para, "areas")
			paraResources = arrayToJSON(para, "resources")
			paraArchive = arrayToJSON(para, "archive")
		}

		_, err := stmt.Exec(
			// insert values
			key, name, title, collection, fileType, status, priority, size,
			createdAt, lastEdited, targetDate,
			folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere,
			paraProjects, paraAreas, paraResources, paraArchive,
			now,
			// update values
			name, title, collection, fileType, status, priority, size,
			createdAt, lastEdited, targetDate,
			folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere,
			paraProjects, paraAreas, paraResources, paraArchive,
			now)
		if err != nil {
			return fmt.Errorf("failed to set metadata for %s: %w", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logging.LogDebug("bulk set %d metadata entries", len(items))
	return nil
}

// GetAll retrieves all metadata entries
func (s *SQLiteMetadataStorage) GetAll() (map[string][]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	query := `SELECT path, name, title, collection, type, status, priority, size,
	          createdAt, lastEdited, targetDate,
	          folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere,
	          para_projects, para_areas, para_resources, para_archive
	          FROM metadata`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all metadata: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]byte)
	for rows.Next() {
		var path, name, title, collection, fileType, status, priority string
		var size, createdAt, lastEdited, targetDate int64
		var folders, tags, boards, ancestor, parents, kids, usedLinks, linksToHere string
		var paraProjects, paraAreas, paraResources, paraArchive string

		if err := rows.Scan(
			&path, &name, &title, &collection, &fileType, &status, &priority, &size,
			&createdAt, &lastEdited, &targetDate,
			&folders, &tags, &boards, &ancestor, &parents, &kids, &usedLinks, &linksToHere,
			&paraProjects, &paraAreas, &paraResources, &paraArchive,
		); err != nil {
			continue
		}

		// reconstruct JSON
		metadata := map[string]any{
			"path":        path,
			"name":        name,
			"title":       title,
			"type":        fileType,
			"status":      status,
			"priority":    priority,
			"size":        size,
			"folders":     jsonToArrayOrEmpty(folders),
			"tags":        jsonToArrayOrEmpty(tags),
			"boards":      jsonToArrayOrEmpty(boards),
			"ancestor":    jsonToArrayOrEmpty(ancestor),
			"parents":     jsonToArrayOrEmpty(parents),
			"kids":        jsonToArrayOrEmpty(kids),
			"usedLinks":   jsonToArrayOrEmpty(usedLinks),
			"linksToHere": jsonToArrayOrEmpty(linksToHere),
		}

		if collection != "" {
			metadata["collection"] = collection
		}
		if createdAt > 0 {
			metadata["createdAt"] = time.Unix(createdAt, 0).Format(time.RFC3339)
		}
		if lastEdited > 0 {
			metadata["lastEdited"] = time.Unix(lastEdited, 0).Format(time.RFC3339)
		}
		if targetDate > 0 {
			metadata["targetDate"] = time.Unix(targetDate, 0).Format(time.RFC3339)
		}

		para := make(map[string]any)
		if paraProjects != "" {
			para["projects"] = jsonToArray(paraProjects)
		}
		if paraAreas != "" {
			para["areas"] = jsonToArray(paraAreas)
		}
		if paraResources != "" {
			para["resources"] = jsonToArray(paraResources)
		}
		if paraArchive != "" {
			para["archive"] = jsonToArray(paraArchive)
		}
		if len(para) > 0 {
			metadata["para"] = para
		}

		data, err := json.Marshal(metadata)
		if err != nil {
			continue
		}
		result[path] = data
	}

	return result, nil
}

// helper functions for extracting and converting values

func getString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt64(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

func getTimestamp(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	if val, ok := m[key]; ok {
		// handle different time formats
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		case string:
			// try parsing RFC3339 format
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t.Unix()
			}
		}
	}
	return 0
}

func arrayToJSON(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key]; ok {
		// handle []any
		if arr, ok := val.([]any); ok {
			// convert to string array
			strArr := make([]string, 0, len(arr))
			for _, item := range arr {
				if str, ok := item.(string); ok {
					strArr = append(strArr, str)
				} else {
					strArr = append(strArr, fmt.Sprintf("%v", item))
				}
			}
			// marshal to JSON
			if data, err := json.Marshal(strArr); err == nil {
				return string(data)
			}
		}
		// handle []string directly
		if arr, ok := val.([]string); ok {
			if data, err := json.Marshal(arr); err == nil {
				return string(data)
			}
		}
	}
	return ""
}

func jsonToArray(jsonStr string) []string {
	if jsonStr == "" {
		return []string{}
	}
	var arr []string
	if err := json.Unmarshal([]byte(jsonStr), &arr); err != nil {
		return []string{}
	}
	return arr
}

func jsonToArrayOrEmpty(jsonStr string) []string {
	if jsonStr == "" {
		return []string{}
	}
	var arr []string
	if err := json.Unmarshal([]byte(jsonStr), &arr); err != nil {
		return []string{}
	}
	return arr
}
