// Package testdata - Test data setup and management
package testdata

import (
	"time"

	"knov/internal/files"
)

func getDefaultFiles() []*files.Metadata {
	return []*files.Metadata{
		{
			Path:       "docs/getting-started.md",
			CreatedAt:  time.Date(2025, 9, 8, 21, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "",
			Folders:    []string{},
			Tags:       []string{"guide", "onboarding", "getting-started"},
			Parents:    []string{"docs/project-overview.md"},
			Editor:     files.EditorTypeMarkdown,
		},
		{
			Path:       "docs/project-overview.md",
			CreatedAt:  time.Date(2025, 9, 8, 20, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "",
			Folders:    []string{},
			Tags:       []string{"project", "overview", "status"},
			Editor:     files.EditorTypeMarkdown,
		},
		{
			Path:       "docs/meeting-notes.md",
			CreatedAt:  time.Date(2025, 9, 11, 10, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "",
			Folders:    []string{},
			Tags:       []string{"meeting", "sprint", "planning"},
			Parents:    []string{"docs/project-overview.md"},
			Editor:     files.EditorTypeMarkdown,
		},
		{
			Path:       "docs/technical-documentation.md",
			CreatedAt:  time.Date(2025, 9, 8, 19, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "",
			Folders:    []string{},
			Tags:       []string{"technical", "api", "documentation"},
			Parents:    []string{"docs/project-overview.md"},
			Editor:     files.EditorTypeMarkdown,
		},
		{
			Path:       "docs/troubleshooting.md",
			CreatedAt:  time.Date(2025, 9, 7, 14, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "",
			Folders:    []string{},
			Tags:       []string{"troubleshooting", "help", "debug"},
			Editor:     files.EditorTypeMarkdown,
		},
		{
			Path:       "docs/guides/developer-setup.md",
			CreatedAt:  time.Date(2025, 9, 3, 16, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "guides",
			Folders:    []string{"guides"},
			Tags:       []string{"developer", "setup", "guide", "technical"},
			Editor:     files.EditorTypeMarkdown,
		},
		{
			Path:       "docs/guides/user-manual.md",
			CreatedAt:  time.Date(2025, 9, 4, 13, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "guides",
			Folders:    []string{"guides"},
			Tags:       []string{"user", "manual", "guide", "help"},
			Editor:     files.EditorTypeMarkdown,
		},
		{
			Path:       "docs/projects/backend-api.md",
			CreatedAt:  time.Date(2025, 9, 5, 9, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "projects",
			Folders:    []string{"projects"},
			Tags:       []string{"backend", "api", "development", "in-progress"},
			Editor:     files.EditorTypeMarkdown,
		},
		{
			Path:       "docs/projects/database-migration.md",
			CreatedAt:  time.Date(2025, 8, 15, 8, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "projects",
			Folders:    []string{"projects"},
			Tags:       []string{"database", "migration", "completed", "infrastructure"},
			Editor:     files.EditorTypeMarkdown,
		},
		{
			Path:       "docs/projects/frontend-redesign.md",
			CreatedAt:  time.Date(2025, 9, 6, 11, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 9, 12, 7, 50, 15, 0, time.UTC),
			Collection: "projects",
			Folders:    []string{"projects"},
			Tags:       []string{"frontend", "ui", "redesign", "planning"},
			Editor:     files.EditorTypeMarkdown,
		},
	}
}
