// Package types contains shared data structures used across packages
package types

import "slices"

// FieldType represents the type of a metadata field
type FieldType int

const (
	StringField FieldType = iota
	ArrayField
	DateField
	IntField
	BoolField
)

// OperatorType represents filter operators
type OperatorType string

const (
	OpEquals    OperatorType = "equals"
	OpContains  OperatorType = "contains"
	OpIn        OperatorType = "in"
	OpGreater   OperatorType = "greater"
	OpLess      OperatorType = "less"
	OpRegex     OperatorType = "regex"
	OpGreaterEq OperatorType = "gte"
	OpLessEq    OperatorType = "lte"
)

// FieldDescriptor describes a metadata field
type FieldDescriptor struct {
	Name      string
	DBColumn  string
	Type      FieldType
	Operators []OperatorType
	IsArray   bool
}

// SupportsOperator checks if this field supports the given operator
func (f FieldDescriptor) SupportsOperator(op OperatorType) bool {
	return slices.Contains(f.Operators, op)
}

// MetadataFields is the registry of all metadata fields
var MetadataFields = struct {
	Name          FieldDescriptor
	Path          FieldDescriptor
	Collection    FieldDescriptor
	Tags          FieldDescriptor
	Folders       FieldDescriptor
	Boards        FieldDescriptor
	CreatedAt     FieldDescriptor
	LastEdited    FieldDescriptor
	TargetDate    FieldDescriptor
	FileType      FieldDescriptor
	Status        FieldDescriptor
	Priority      FieldDescriptor
	PARAProjects  FieldDescriptor
	PARAreas      FieldDescriptor
	PARAResources FieldDescriptor
	PARAArchive   FieldDescriptor
}{
	Name: FieldDescriptor{
		Name:      "name",
		DBColumn:  "name",
		Type:      StringField,
		Operators: []OperatorType{OpEquals, OpContains, OpRegex},
		IsArray:   false,
	},
	Path: FieldDescriptor{
		Name:      "path",
		DBColumn:  "path",
		Type:      StringField,
		Operators: []OperatorType{OpEquals, OpContains, OpRegex},
		IsArray:   false,
	},
	Collection: FieldDescriptor{
		Name:      "collection",
		DBColumn:  "collection",
		Type:      StringField,
		Operators: []OperatorType{OpEquals, OpContains, OpIn},
		IsArray:   false,
	},
	Tags: FieldDescriptor{
		Name:      "tags",
		DBColumn:  "tags",
		Type:      ArrayField,
		Operators: []OperatorType{OpContains, OpIn},
		IsArray:   true,
	},
	Folders: FieldDescriptor{
		Name:      "folders",
		DBColumn:  "folders",
		Type:      ArrayField,
		Operators: []OperatorType{OpContains, OpIn},
		IsArray:   true,
	},
	Boards: FieldDescriptor{
		Name:      "boards",
		DBColumn:  "boards",
		Type:      ArrayField,
		Operators: []OperatorType{OpContains, OpIn},
		IsArray:   true,
	},
	CreatedAt: FieldDescriptor{
		Name:      "createdAt",
		DBColumn:  "createdAt",
		Type:      DateField,
		Operators: []OperatorType{OpEquals, OpGreater, OpLess, OpGreaterEq, OpLessEq},
		IsArray:   false,
	},
	LastEdited: FieldDescriptor{
		Name:      "lastEdited",
		DBColumn:  "lastEdited",
		Type:      DateField,
		Operators: []OperatorType{OpEquals, OpGreater, OpLess, OpGreaterEq, OpLessEq},
		IsArray:   false,
	},
	TargetDate: FieldDescriptor{
		Name:      "targetDate",
		DBColumn:  "targetDate",
		Type:      DateField,
		Operators: []OperatorType{OpEquals, OpGreater, OpLess, OpGreaterEq, OpLessEq},
		IsArray:   false,
	},
	FileType: FieldDescriptor{
		Name:      "type",
		DBColumn:  "type",
		Type:      StringField,
		Operators: []OperatorType{OpEquals, OpIn},
		IsArray:   false,
	},
	Status: FieldDescriptor{
		Name:      "status",
		DBColumn:  "status",
		Type:      StringField,
		Operators: []OperatorType{OpEquals, OpIn},
		IsArray:   false,
	},
	Priority: FieldDescriptor{
		Name:      "priority",
		DBColumn:  "priority",
		Type:      StringField,
		Operators: []OperatorType{OpEquals, OpIn},
		IsArray:   false,
	},
	PARAProjects: FieldDescriptor{
		Name:      "para_projects",
		DBColumn:  "para_projects",
		Type:      ArrayField,
		Operators: []OperatorType{OpContains, OpIn},
		IsArray:   true,
	},
	PARAreas: FieldDescriptor{
		Name:      "para_areas",
		DBColumn:  "para_areas",
		Type:      ArrayField,
		Operators: []OperatorType{OpContains, OpIn},
		IsArray:   true,
	},
	PARAResources: FieldDescriptor{
		Name:      "para_resources",
		DBColumn:  "para_resources",
		Type:      ArrayField,
		Operators: []OperatorType{OpContains, OpIn},
		IsArray:   true,
	},
	PARAArchive: FieldDescriptor{
		Name:      "para_archive",
		DBColumn:  "para_archive",
		Type:      ArrayField,
		Operators: []OperatorType{OpContains, OpIn},
		IsArray:   true,
	},
}

// fieldNameMap maps field names to descriptors for lookup
var fieldNameMap = map[string]*FieldDescriptor{
	"name":           &MetadataFields.Name,
	"path":           &MetadataFields.Path,
	"collection":     &MetadataFields.Collection,
	"tags":           &MetadataFields.Tags,
	"folders":        &MetadataFields.Folders,
	"boards":         &MetadataFields.Boards,
	"createdAt":      &MetadataFields.CreatedAt,
	"lastEdited":     &MetadataFields.LastEdited,
	"targetDate":     &MetadataFields.TargetDate,
	"type":           &MetadataFields.FileType,
	"status":         &MetadataFields.Status,
	"priority":       &MetadataFields.Priority,
	"para_projects":  &MetadataFields.PARAProjects,
	"para_areas":     &MetadataFields.PARAreas,
	"para_resources": &MetadataFields.PARAResources,
	"para_archive":   &MetadataFields.PARAArchive,
}

// GetFieldDescriptor returns the field descriptor for a given field name
func GetFieldDescriptor(fieldName string) (*FieldDescriptor, bool) {
	field, ok := fieldNameMap[fieldName]
	return field, ok
}

// AllFieldNames returns all valid field names
func AllFieldNames() []string {
	names := make([]string, 0, len(fieldNameMap))
	for name := range fieldNameMap {
		names = append(names, name)
	}
	return names
}
