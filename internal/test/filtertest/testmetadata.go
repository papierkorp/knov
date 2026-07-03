// Package filtertest - Filter test metadata definitions
package filtertest

import (
	"time"

	"knov/internal/files"
)

// getFilterTestMetadata returns the metadata definitions for all filter test files
func getFilterTestMetadata() []*files.Metadata {
	return []*files.Metadata{
		// filterTestA
		{
			Path:       "docs/test/filter-tests/filtertestfolder/filterTestA.md",
			CreatedAt:  time.Date(2025, 10, 1, 10, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 1, 10, 0, 0, 0, time.UTC),
			Tags:       []string{"filtertest-unique"},
			Editor:     files.EditorTypeToastUI,
		},
		// filterTestB
		{
			Path:       "docs/test/filter-tests/filtertestfolder/filterTestB.md",
			CreatedAt:  time.Date(2025, 10, 2, 11, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 2, 11, 0, 0, 0, time.UTC),
			Tags:       []string{"filtertest-group"},
			Editor:     files.EditorTypeToastUI,
		},
		// filterTestC
		{
			Path:       "docs/test/filter-tests/filterTestC.md",
			CreatedAt:  time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 3, 12, 0, 0, 0, time.UTC),
			Tags:       []string{"filtertest-group", "filtertest-group2"},
			Editor:     files.EditorTypeToastUI,
		},
		// filterTestD
		{
			Path:       "docs/test/filter-tests/filterTestD.md",
			CreatedAt:  time.Date(2025, 10, 4, 13, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 4, 13, 0, 0, 0, time.UTC),
			Tags:       []string{"filtertest-group2"},
			Editor:     files.EditorTypeToastUI,
		},
		// filterTestE
		{
			Path:       "docs/test/filter-tests/filterTestE.md",
			CreatedAt:  time.Date(2025, 10, 5, 14, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 5, 14, 0, 0, 0, time.UTC),
			Parents:    []string{"docs/test/filter-tests/filterTestD.md"},
			Editor:     files.EditorTypeToastUI,
			References: []files.Reference{{URL: "https://example.com", Description: "example reference for testing"}, {URL: "https://www.google.com", Description: "another reference"}},
		},
		// filterTestF
		{
			Path:       "docs/test/filter-tests/filterTestF.md",
			CreatedAt:  time.Date(2025, 10, 6, 15, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 6, 15, 0, 0, 0, time.UTC),
			Parents:    []string{"docs/test/filter-tests/filterTestE.md"},
			Editor:     files.EditorTypeToastUI,
		},
	}
}
