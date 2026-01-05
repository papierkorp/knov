// Package types contains shared data structures used across packages
package types

import "time"

// File represents a file in the system
type File struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

// Metadata represents file metadata
type Metadata struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"createdAt"`
	LastEdited  time.Time `json:"lastEdited"`
	TargetDate  time.Time `json:"targetDate"`
	Collection  string    `json:"collection"`
	Folders     []string  `json:"folders"`
	Tags        []string  `json:"tags"`
	Boards      []string  `json:"boards"`
	Ancestor    []string  `json:"ancestor"`
	Parents     []string  `json:"parents"`
	Kids        []string  `json:"kids"`
	UsedLinks   []string  `json:"usedLinks"`
	LinksToHere []string  `json:"linksToHere"`
	FileType    Filetype  `json:"type"`
	PARA        PARA      `json:"para"`
	Status      Status    `json:"status"`
	Priority    Priority  `json:"priority"`
	Size        int64     `json:"size"`
}

// PARA represents PARA organization
type PARA struct {
	Projects  []string `json:"projects,omitempty"`
	Areas     []string `json:"areas,omitempty"`
	Resources []string `json:"resources,omitempty"`
	Archive   []string `json:"archive,omitempty"`
}

// typed count maps for metadata aggregations
type TagCount map[string]int
type CollectionCount map[string]int
type FolderCount map[string]int
type BoardCount map[string]int
type FiletypeCount map[string]int
type PriorityCount map[string]int
type StatusCount map[string]int
type PARAProjectCount map[string]int
type PARAAreaCount map[string]int
type PARAResourceCount map[string]int
type PARAArchiveCount map[string]int
