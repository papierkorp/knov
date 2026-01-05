// Package types contains shared data structures used across packages
package types

// Filetype represents file types
type Filetype string

// Status represents file status
type Status string

// Priority represents file priority
type Priority string

const (
	FileTypeTodo       Filetype = "todo"
	FileTypeFleeting   Filetype = "fleeting"
	FileTypeLiterature Filetype = "literature"
	FileTypeMOC        Filetype = "moc"
	FileTypePermanent  Filetype = "permanent"
	FileTypeFilter     Filetype = "filter"
	FileTypeJournaling Filetype = "journaling"

	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"

	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// AllFiletypes returns all available file types
func AllFiletypes() []Filetype {
	return []Filetype{
		FileTypeTodo,
		FileTypeFleeting,
		FileTypeLiterature,
		FileTypeMOC,
		FileTypePermanent,
		FileTypeFilter,
		FileTypeJournaling,
	}
}

// AllPriorities returns all available priorities
func AllPriorities() []Priority {
	return []Priority{
		PriorityLow,
		PriorityMedium,
		PriorityHigh,
	}
}

// AllStatuses returns all available statuses
func AllStatuses() []Status {
	return []Status{
		StatusDraft,
		StatusPublished,
		StatusArchived,
	}
}

// IsValidFiletype checks if a filetype is valid
func IsValidFiletype(ft Filetype) bool {
	for _, valid := range AllFiletypes() {
		if ft == valid {
			return true
		}
	}
	return false
}

// IsValidPriority checks if a priority is valid
func IsValidPriority(p Priority) bool {
	for _, valid := range AllPriorities() {
		if p == valid {
			return true
		}
	}
	return false
}

// IsValidStatus checks if a status is valid
func IsValidStatus(s Status) bool {
	for _, valid := range AllStatuses() {
		if s == valid {
			return true
		}
	}
	return false
}
