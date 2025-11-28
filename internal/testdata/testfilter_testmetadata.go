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
			Path:       "filter-tests/filterTestA.md",
			CreatedAt:  time.Date(2025, 10, 1, 10, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 1, 10, 0, 0, 0, time.UTC),
			Collection: "filter-testing-unique",
			Folders:    []string{"filter-tests"},
			Tags:       []string{"unique-experimental", "filter-specific", "alpha-test"},
			Boards:     []string{"filter-board", "testing-board"},
			FileType:   files.FileTypeFleeting,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
			PARA: files.PARA{
				Projects:  []string{"unique_filter_system", "filter_testing_project"},
				Areas:     []string{"filter_development", "unique_quality_assurance"},
				Resources: []string{"filter_test_data", "unique_documentation"},
				Archive:   []string{},
			},
		},
		// filterTestB
		{
			Name:       "filterTestB.md",
			Path:       "filter-tests/filterTestB.md",
			CreatedAt:  time.Date(2025, 10, 2, 11, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 2, 11, 0, 0, 0, time.UTC),
			Collection: "filter-testing-unique",
			Folders:    []string{"filter-tests"},
			Tags:       []string{"unique-stable", "filter-specific", "beta-test"},
			Boards:     []string{"filter-board"},
			FileType:   files.FileTypeLiterature,
			Status:     files.StatusPublished,
			Priority:   files.PriorityMedium,
			PARA: files.PARA{
				Projects:  []string{"unique_filter_system"},
				Areas:     []string{"filter_development"},
				Resources: []string{"filter_test_data"},
				Archive:   []string{},
			},
		},
		// filterTestC
		{
			Name:       "filterTestC.md",
			Path:       "filter-tests/filterTestC.md",
			CreatedAt:  time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 3, 12, 0, 0, 0, time.UTC),
			Collection: "filter-testing-unique",
			Folders:    []string{"filter-tests"},
			Tags:       []string{"unique-performance", "optimization-specific", "gamma-test"},
			Boards:     []string{"filter-board", "performance-board"},
			FileType:   files.FileTypePermanent,
			Status:     files.StatusPublished,
			Priority:   files.PriorityLow,
			PARA: files.PARA{
				Projects:  []string{"unique_performance_testing"},
				Areas:     []string{"unique_optimization", "unique_quality_assurance"},
				Resources: []string{"unique_benchmarks", "unique_metrics"},
				Archive:   []string{},
			},
		},
		// filterTestD
		{
			Name:       "filterTestD.md",
			Path:       "filter-tests/advanced/filterTestD.md",
			CreatedAt:  time.Date(2025, 10, 4, 13, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 4, 13, 0, 0, 0, time.UTC),
			Collection: "advanced-filter-testing",
			Folders:    []string{"filter-tests", "advanced"},
			Tags:       []string{"unique-advanced", "unique-experimental", "delta-test"},
			Boards:     []string{"advanced-board", "testing-board"},
			FileType:   files.FileTypeFleeting,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
			PARA: files.PARA{
				Projects:  []string{"unique_advanced_features", "unique_filter_system"},
				Areas:     []string{"unique_research", "filter_development"},
				Resources: []string{"unique_advanced_docs", "unique_prototypes"},
				Archive:   []string{},
			},
		},
		// filterTestE
		{
			Name:       "filterTestE.md",
			Path:       "filter-tests/advanced/filterTestE.md",
			CreatedAt:  time.Date(2025, 10, 5, 14, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 5, 14, 0, 0, 0, time.UTC),
			Collection: "advanced-filter-testing",
			Folders:    []string{"filter-tests", "advanced"},
			Tags:       []string{"unique-advanced", "unique-stable", "epsilon-test"},
			Boards:     []string{"advanced-board"},
			FileType:   files.FileTypeLiterature,
			Status:     files.StatusArchived,
			Priority:   files.PriorityMedium,
			PARA: files.PARA{
				Projects:  []string{"unique_advanced_features"},
				Areas:     []string{"unique_research"},
				Resources: []string{"unique_advanced_docs"},
				Archive:   []string{"unique_old_prototypes", "unique_deprecated_features"},
			},
		},
		// filterTestF
		{
			Name:       "filterTestF.md",
			Path:       "filter-tests/basic/filterTestF.md",
			CreatedAt:  time.Date(2025, 10, 6, 15, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 6, 15, 0, 0, 0, time.UTC),
			Collection: "basic-filter-testing",
			Folders:    []string{"filter-tests", "basic"},
			Tags:       []string{"unique-basic", "fundamental-specific", "zeta-test"},
			Boards:     []string{"basic-board", "testing-board"},
			FileType:   files.FileTypeJournaling,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
			PARA: files.PARA{
				Projects:  []string{"unique_basic_functionality", "filter_testing_project"},
				Areas:     []string{"unique_fundamentals", "unique_quality_assurance"},
				Resources: []string{"unique_basic_docs", "unique_tutorials"},
				Archive:   []string{},
			},
		},
		// filterTestG
		{
			Name:       "filterTestG.md",
			Path:       "filter-tests/basic/filterTestG.md",
			CreatedAt:  time.Date(2025, 10, 7, 16, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 7, 16, 0, 0, 0, time.UTC),
			Collection: "basic-filter-testing",
			Folders:    []string{"filter-tests", "basic"},
			Tags:       []string{"unique-basic", "validation-specific", "eta-test"},
			Boards:     []string{"basic-board"},
			FileType:   files.FileTypeMOC,
			Status:     files.StatusPublished,
			Priority:   files.PriorityMedium,
			PARA: files.PARA{
				Projects:  []string{"unique_basic_functionality"},
				Areas:     []string{"unique_fundamentals"},
				Resources: []string{"unique_basic_docs"},
				Archive:   []string{},
			},
		},
		// filterTestH
		{
			Name:       "filterTestH.md",
			Path:       "filter-tests/integration/filterTestH.md",
			CreatedAt:  time.Date(2025, 10, 8, 17, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 8, 17, 0, 0, 0, time.UTC),
			Collection: "integration-filter-testing",
			Folders:    []string{"filter-tests", "integration"},
			Tags:       []string{"unique-integration", "system-specific", "theta-test"},
			Boards:     []string{"integration-board", "testing-board"},
			FileType:   files.FileTypeFleeting,
			Status:     files.StatusPublished,
			Priority:   files.PriorityLow,
			PARA: files.PARA{
				Projects:  []string{"unique_integration_testing", "unique_system_validation"},
				Areas:     []string{"unique_integration", "unique_testing"},
				Resources: []string{"unique_integration_docs", "unique_test_suites"},
				Archive:   []string{},
			},
		},
		// filterTestI
		{
			Name:       "filterTestI.md",
			Path:       "filter-tests/integration/filterTestI.md",
			CreatedAt:  time.Date(2025, 10, 9, 18, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 9, 18, 0, 0, 0, time.UTC),
			Collection: "integration-filter-testing",
			Folders:    []string{"filter-tests", "integration"},
			Tags:       []string{"unique-integration", "complex-specific", "iota-test"},
			Boards:     []string{"integration-board"},
			FileType:   files.FileTypePermanent,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
			PARA: files.PARA{
				Projects:  []string{"unique_integration_testing", "unique_complex_scenarios"},
				Areas:     []string{"unique_integration", "unique_complexity_management"},
				Resources: []string{"unique_integration_docs", "unique_complex_test_cases"},
				Archive:   []string{},
			},
		},
		// filterTestJ
		{
			Name:       "filterTestJ.md",
			Path:       "filter-tests/performance/filterTestJ.md",
			CreatedAt:  time.Date(2025, 10, 10, 19, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 10, 19, 0, 0, 0, time.UTC),
			Collection: "performance-filter-testing",
			Folders:    []string{"filter-tests", "performance"},
			Tags:       []string{"unique-performance", "benchmark-specific", "kappa-test"},
			Boards:     []string{"performance-board", "testing-board"},
			FileType:   files.FileTypeLiterature,
			Status:     files.StatusArchived,
			Priority:   files.PriorityMedium,
			PARA: files.PARA{
				Projects:  []string{"unique_performance_testing", "unique_benchmark_suite"},
				Areas:     []string{"unique_performance", "unique_optimization"},
				Resources: []string{"unique_benchmarks", "unique_performance_docs"},
				Archive:   []string{"unique_old_benchmarks"},
			},
		},
		// filterTestK
		{
			Name:       "filterTestK.md",
			Path:       "filter-tests/performance/filterTestK.md",
			CreatedAt:  time.Date(2025, 10, 11, 20, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 11, 20, 0, 0, 0, time.UTC),
			Collection: "performance-filter-testing",
			Folders:    []string{"filter-tests", "performance"},
			Tags:       []string{"unique-performance", "scalability-specific", "lambda-test"},
			Boards:     []string{"performance-board"},
			FileType:   files.FileTypeJournaling,
			Status:     files.StatusPublished,
			Priority:   files.PriorityLow,
			PARA: files.PARA{
				Projects:  []string{"unique_scalability_testing"},
				Areas:     []string{"unique_performance", "unique_scalability"},
				Resources: []string{"unique_scalability_docs", "unique_load_tests"},
				Archive:   []string{},
			},
		},
		// filterTestL
		{
			Name:       "filterTestL.md",
			Path:       "filter-tests/special/filterTestL.md",
			CreatedAt:  time.Date(2025, 10, 12, 21, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 12, 21, 0, 0, 0, time.UTC),
			Collection: "special-filter-testing",
			Folders:    []string{"filter-tests", "special"},
			Tags:       []string{"unique-special", "edge-case-specific", "mu-test"},
			Boards:     []string{"special-board", "testing-board"},
			FileType:   files.FileTypeMOC,
			Status:     files.StatusDraft,
			Priority:   files.PriorityHigh,
			PARA: files.PARA{
				Projects:  []string{"unique_edge_case_testing", "unique_special_scenarios"},
				Areas:     []string{"unique_edge_cases", "unique_special_testing"},
				Resources: []string{"unique_edge_case_docs", "unique_special_test_data"},
				Archive:   []string{},
			},
		},
	}
}
