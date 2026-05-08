// Package testdata - Filter test metadata definitions
package testdata

import (
	"time"

	"knov/internal/files"
)

// getFilterTestMetadata returns the metadata definitions for all filter test files
func getFilterTestMetadata() []*files.Metadata {
	return []*files.Metadata{
		// filterTestA
		{
			Name:       "filterTestA.md",
			Path:       "docs/filter-tests/filterTestA.md",
			CreatedAt:  time.Date(2025, 10, 1, 10, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 1, 10, 0, 0, 0, time.UTC),
			Collection: "filter-testing-unique",
			Folders:    []string{"filter-tests"},
			Tags:       []string{"unique-experimental", "filter-specific", "alpha-test"},
			Boards:     []string{"filter-board", "testing-board"},
			FileType:   files.FileTypeFleeting,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
		},
		// filterTestB
		{
			Name:       "filterTestB.md",
			Path:       "docs/filter-tests/filterTestB.md",
			CreatedAt:  time.Date(2025, 10, 2, 11, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 2, 11, 0, 0, 0, time.UTC),
			Collection: "filter-testing-unique",
			Folders:    []string{"filter-tests"},
			Tags:       []string{"unique-stable", "filter-specific", "beta-test"},
			Boards:     []string{"filter-board"},
			FileType:   files.FileTypeLiterature,
			Status:     files.StatusPublished,
			Priority:   files.PriorityMedium,
		},
		// filterTestC
		{
			Name:       "filterTestC.md",
			Path:       "docs/filter-tests/filterTestC.md",
			CreatedAt:  time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 3, 12, 0, 0, 0, time.UTC),
			Collection: "filter-testing-unique",
			Folders:    []string{"filter-tests"},
			Tags:       []string{"unique-performance", "optimization-specific", "gamma-test"},
			Boards:     []string{"filter-board", "performance-board"},
			FileType:   files.FileTypePermanent,
			Status:     files.StatusPublished,
			Priority:   files.PriorityLow,
		},
		// filterTestD
		{
			Name:       "filterTestD.md",
			Path:       "docs/filter-tests/advanced/filterTestD.md",
			CreatedAt:  time.Date(2025, 10, 4, 13, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 4, 13, 0, 0, 0, time.UTC),
			Collection: "advanced-filter-testing",
			Folders:    []string{"filter-tests", "advanced"},
			Tags:       []string{"unique-advanced", "unique-experimental", "delta-test"},
			Boards:     []string{"advanced-board", "testing-board"},
			FileType:   files.FileTypeFleeting,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
		},
		// filterTestE
		{
			Name:       "filterTestE.md",
			Path:       "docs/filter-tests/advanced/filterTestE.md",
			CreatedAt:  time.Date(2025, 10, 5, 14, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 5, 14, 0, 0, 0, time.UTC),
			Collection: "advanced-filter-testing",
			Folders:    []string{"filter-tests", "advanced"},
			Tags:       []string{"unique-advanced", "unique-stable", "epsilon-test"},
			Boards:     []string{"advanced-board"},
			FileType:   files.FileTypeLiterature,
			Status:     files.StatusArchived,
			Priority:   files.PriorityMedium,
		},
		// filterTestF
		{
			Name:       "filterTestF.md",
			Path:       "docs/filter-tests/basic/filterTestF.md",
			CreatedAt:  time.Date(2025, 10, 6, 15, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 6, 15, 0, 0, 0, time.UTC),
			Collection: "basic-filter-testing",
			Folders:    []string{"filter-tests", "basic"},
			Tags:       []string{"unique-basic", "fundamental-specific", "zeta-test"},
			Boards:     []string{"basic-board", "testing-board"},
			FileType:   files.FileTypeJournaling,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
		},
		// filterTestG
		{
			Name:       "filterTestG.md",
			Path:       "docs/filter-tests/basic/filterTestG.md",
			CreatedAt:  time.Date(2025, 10, 7, 16, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 7, 16, 0, 0, 0, time.UTC),
			Collection: "basic-filter-testing",
			Folders:    []string{"filter-tests", "basic"},
			Tags:       []string{"unique-basic", "validation-specific", "eta-test"},
			Boards:     []string{"basic-board"},
			FileType:   files.FileTypeMOC,
			Status:     files.StatusPublished,
			Priority:   files.PriorityMedium,
		},
		// filterTestH
		{
			Name:       "filterTestH.md",
			Path:       "docs/filter-tests/integration/filterTestH.md",
			CreatedAt:  time.Date(2025, 10, 8, 17, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 8, 17, 0, 0, 0, time.UTC),
			Collection: "integration-filter-testing",
			Folders:    []string{"filter-tests", "integration"},
			Tags:       []string{"unique-integration", "system-specific", "theta-test"},
			Boards:     []string{"integration-board", "testing-board"},
			FileType:   files.FileTypeFleeting,
			Status:     files.StatusPublished,
			Priority:   files.PriorityLow,
		},
		// filterTestI
		{
			Name:       "filterTestI.md",
			Path:       "docs/filter-tests/integration/filterTestI.md",
			CreatedAt:  time.Date(2025, 10, 9, 18, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 9, 18, 0, 0, 0, time.UTC),
			Collection: "integration-filter-testing",
			Folders:    []string{"filter-tests", "integration"},
			Tags:       []string{"unique-integration", "complex-specific", "iota-test"},
			Boards:     []string{"integration-board"},
			FileType:   files.FileTypePermanent,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
		},
		// filterTestJ
		{
			Name:       "filterTestJ.md",
			Path:       "docs/filter-tests/performance/filterTestJ.md",
			CreatedAt:  time.Date(2025, 10, 10, 19, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 10, 19, 0, 0, 0, time.UTC),
			Collection: "performance-filter-testing",
			Folders:    []string{"filter-tests", "performance"},
			Tags:       []string{"unique-performance", "benchmark-specific", "kappa-test"},
			Boards:     []string{"performance-board", "testing-board"},
			FileType:   files.FileTypeLiterature,
			Status:     files.StatusArchived,
			Priority:   files.PriorityMedium,
		},
		// filterTestK
		{
			Name:       "filterTestK.md",
			Path:       "docs/filter-tests/performance/filterTestK.md",
			CreatedAt:  time.Date(2025, 10, 11, 20, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 11, 20, 0, 0, 0, time.UTC),
			Collection: "performance-filter-testing",
			Folders:    []string{"filter-tests", "performance"},
			Tags:       []string{"unique-performance", "scalability-specific", "lambda-test"},
			Boards:     []string{"performance-board"},
			FileType:   files.FileTypeJournaling,
			Status:     files.StatusPublished,
			Priority:   files.PriorityLow,
		},
		// filterTestL
		{
			Name:       "filterTestL.md",
			Path:       "docs/filter-tests/special/filterTestL.md",
			CreatedAt:  time.Date(2025, 10, 12, 21, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 12, 21, 0, 0, 0, time.UTC),
			Collection: "special-filter-testing",
			Folders:    []string{"filter-tests", "special"},
			Tags:       []string{"unique-special", "edge-case-specific", "mu-test"},
			Boards:     []string{"special-board", "testing-board"},
			FileType:   files.FileTypeMOC,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
		},
	}
}
