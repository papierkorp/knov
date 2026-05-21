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
			Path:       "docs/filter-tests/filterTestA.md",
			CreatedAt:  time.Date(2025, 10, 1, 10, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 1, 10, 0, 0, 0, time.UTC),
			Collection: "filter-testing-unique",
			Folders:    []string{"filter-tests"},
			Tags:       []string{"unique-experimental", "filter-specific", "alpha-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestB
		{
			Path:       "docs/filter-tests/filterTestB.md",
			CreatedAt:  time.Date(2025, 10, 2, 11, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 2, 11, 0, 0, 0, time.UTC),
			Collection: "filter-testing-unique",
			Folders:    []string{"filter-tests"},
			Tags:       []string{"unique-stable", "filter-specific", "beta-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestC
		{
			Path:       "docs/filter-tests/filterTestC.md",
			CreatedAt:  time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 3, 12, 0, 0, 0, time.UTC),
			Collection: "filter-testing-unique",
			Folders:    []string{"filter-tests"},
			Tags:       []string{"unique-performance", "optimization-specific", "gamma-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestD
		{
			Path:       "docs/filter-tests/advanced/filterTestD.md",
			CreatedAt:  time.Date(2025, 10, 4, 13, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 4, 13, 0, 0, 0, time.UTC),
			Collection: "advanced-filter-testing",
			Folders:    []string{"filter-tests", "advanced"},
			Tags:       []string{"unique-advanced", "unique-experimental", "delta-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestE
		{
			Path:       "docs/filter-tests/advanced/filterTestE.md",
			CreatedAt:  time.Date(2025, 10, 5, 14, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 5, 14, 0, 0, 0, time.UTC),
			Collection: "advanced-filter-testing",
			Folders:    []string{"filter-tests", "advanced"},
			Tags:       []string{"unique-advanced", "unique-stable", "epsilon-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestF
		{
			Path:       "docs/filter-tests/basic/filterTestF.md",
			CreatedAt:  time.Date(2025, 10, 6, 15, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 6, 15, 0, 0, 0, time.UTC),
			Collection: "basic-filter-testing",
			Folders:    []string{"filter-tests", "basic"},
			Tags:       []string{"unique-basic", "fundamental-specific", "zeta-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestG
		{
			Path:       "docs/filter-tests/basic/filterTestG.md",
			CreatedAt:  time.Date(2025, 10, 7, 16, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 7, 16, 0, 0, 0, time.UTC),
			Collection: "basic-filter-testing",
			Folders:    []string{"filter-tests", "basic"},
			Tags:       []string{"unique-basic", "validation-specific", "eta-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestH
		{
			Path:       "docs/filter-tests/integration/filterTestH.md",
			CreatedAt:  time.Date(2025, 10, 8, 17, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 8, 17, 0, 0, 0, time.UTC),
			Collection: "integration-filter-testing",
			Folders:    []string{"filter-tests", "integration"},
			Tags:       []string{"unique-integration", "system-specific", "theta-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestI
		{
			Path:       "docs/filter-tests/integration/filterTestI.md",
			CreatedAt:  time.Date(2025, 10, 9, 18, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 9, 18, 0, 0, 0, time.UTC),
			Collection: "integration-filter-testing",
			Folders:    []string{"filter-tests", "integration"},
			Tags:       []string{"unique-integration", "complex-specific", "iota-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestJ
		{
			Path:       "docs/filter-tests/performance/filterTestJ.md",
			CreatedAt:  time.Date(2025, 10, 10, 19, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 10, 19, 0, 0, 0, time.UTC),
			Collection: "performance-filter-testing",
			Folders:    []string{"filter-tests", "performance"},
			Tags:       []string{"unique-performance", "benchmark-specific", "kappa-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestK
		{
			Path:       "docs/filter-tests/performance/filterTestK.md",
			CreatedAt:  time.Date(2025, 10, 11, 20, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 11, 20, 0, 0, 0, time.UTC),
			Collection: "performance-filter-testing",
			Folders:    []string{"filter-tests", "performance"},
			Tags:       []string{"unique-performance", "scalability-specific", "lambda-test"},
			Editor:     files.EditorTypeMarkdown,
		},
		// filterTestL
		{
			Path:       "docs/filter-tests/special/filterTestL.md",
			CreatedAt:  time.Date(2025, 10, 12, 21, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 12, 21, 0, 0, 0, time.UTC),
			Collection: "special-filter-testing",
			Folders:    []string{"filter-tests", "special"},
			Tags:       []string{"unique-special", "edge-case-specific", "mu-test"},
			Editor:     files.EditorTypeMarkdown,
		},
	}
}
