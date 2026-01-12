// Package testdata - Test file creation for filter testing
package testdata

import (
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/logging"
)

// createFilterTestFiles creates the physical test files on disk
func createFilterTestFiles() error {
	logging.LogInfo("creating filter test files on disk")

	docsPath := contentStorage.GetDocsPath()

	// remove existing filter-tests directory to ensure clean state
	filterTestsPath := filepath.Join(docsPath, "filter-tests")
	if _, err := os.Stat(filterTestsPath); err == nil {
		logging.LogInfo("removing existing filter-tests directory")
		if err := os.RemoveAll(filterTestsPath); err != nil {
			return fmt.Errorf("failed to remove existing filter-tests directory: %v", err)
		}
	}

	// define test files with their paths and unique content
	testFiles := []struct {
		path    string
		content string
	}{
		{
			path: "filter-tests/filterTestA.md",
			content: `# filterTestA

**Unique experimental filter test file A**

This is a test file for testing the filter system with unique experimental features.

## Features
- Unique filter testing
- Experimental functionality
- High priority testing

## Tags
- unique-experimental
- filter-specific
- alpha-test

## Collection
filter-testing-unique

## Metadata
- Type: fleeting
- Status: draft
- Priority: high
- Boards: filter-board, testing-board
- PARA Projects: unique_filter_system, filter_testing_project
- PARA Areas: filter_development, unique_quality_assurance
`,
		},
		{
			path: "filter-tests/filterTestB.md",
			content: `# filterTestB

**Unique stable filter test file B**

This is a test file for testing stable filter functionality.

## Features
- Unique stable filtering
- Literature type
- Published status

## Tags
- unique-stable
- filter-specific
- beta-test

## Collection
filter-testing-unique

## Metadata
- Type: literature
- Status: published
- Priority: medium
- Boards: filter-board
- PARA Projects: unique_filter_system
`,
		},
		{
			path: "filter-tests/filterTestC.md",
			content: `# filterTestC

**Unique performance optimization test file C**

This file tests unique performance and optimization features.

## Features
- Unique performance testing
- Optimization features
- Low priority

## Tags
- unique-performance
- optimization-specific
- gamma-test

## Collection
filter-testing-unique

## Metadata
- Type: permanent
- Status: published
- Priority: low
- Boards: filter-board, performance-board
`,
		},
		{
			path: "filter-tests/advanced/filterTestD.md",
			content: `# filterTestD

**Unique advanced experimental test file D**

This file contains unique advanced experimental features.

## Features
- Unique advanced functionality
- Experimental features
- Research content

## Tags
- unique-advanced
- unique-experimental
- delta-test

## Collection
advanced-filter-testing

## Metadata
- Type: fleeting
- Status: draft
- Priority: high
- Boards: advanced-board, testing-board
- PARA Projects: unique_advanced_features, unique_filter_system
`,
		},
		{
			path: "filter-tests/advanced/filterTestE.md",
			content: `# filterTestE

**Unique advanced stable test file E**

This file contains unique advanced stable features.

## Features
- Unique advanced functionality
- Stable implementation
- Archived content

## Tags
- unique-advanced
- unique-stable
- epsilon-test

## Collection
advanced-filter-testing

## Metadata
- Type: literature
- Status: archived
- Priority: medium
- Boards: advanced-board
`,
		},
		{
			path: "filter-tests/basic/filterTestF.md",
			content: `# filterTestF

**Unique basic fundamental test file F**

This file tests unique basic fundamental features.

## Features
- Unique basic functionality
- Fundamental concepts
- Journaling type

## Tags
- unique-basic
- fundamental-specific
- zeta-test

## Collection
basic-filter-testing

## Metadata
- Type: journaling
- Status: draft
- Priority: high
- Boards: basic-board, testing-board
`,
		},
		{
			path: "filter-tests/basic/filterTestG.md",
			content: `# filterTestG

**Unique basic validation test file G**

This file tests unique basic validation features.

## Features
- Unique basic functionality
- Validation testing
- MOC type

## Tags
- unique-basic
- validation-specific
- eta-test

## Collection
basic-filter-testing

## Metadata
- Type: moc
- Status: published
- Priority: medium
- Boards: basic-board
`,
		},
		{
			path: "filter-tests/integration/filterTestH.md",
			content: `# filterTestH

**Unique integration system test file H**

This file tests unique integration and system features.

## Features
- Unique integration testing
- System validation
- Published content

## Tags
- unique-integration
- system-specific
- theta-test

## Collection
integration-filter-testing

## Metadata
- Type: fleeting
- Status: published
- Priority: low
- Boards: integration-board, testing-board
`,
		},
		{
			path: "filter-tests/integration/filterTestI.md",
			content: `# filterTestI

**Unique integration complex test file I**

This file tests unique complex integration scenarios.

## Features
- Unique integration testing
- Complex scenarios
- High priority

## Tags
- unique-integration
- complex-specific
- iota-test

## Collection
integration-filter-testing

## Metadata
- Type: permanent
- Status: draft
- Priority: high
- Boards: integration-board
`,
		},
		{
			path: "filter-tests/performance/filterTestJ.md",
			content: `# filterTestJ

**Unique performance benchmark test file J**

This file contains unique performance benchmarks.

## Features
- Unique performance testing
- Benchmark data
- Archived content

## Tags
- unique-performance
- benchmark-specific
- kappa-test

## Collection
performance-filter-testing

## Metadata
- Type: literature
- Status: archived
- Priority: medium
- Boards: performance-board, testing-board
`,
		},
		{
			path: "filter-tests/performance/filterTestK.md",
			content: `# filterTestK

**Unique performance scalability test file K**

This file tests unique scalability features.

## Features
- Unique performance testing
- Scalability testing
- Journaling type

## Tags
- unique-performance
- scalability-specific
- lambda-test

## Collection
performance-filter-testing

## Metadata
- Type: journaling
- Status: published
- Priority: low
- Boards: performance-board
`,
		},
		{
			path: "filter-tests/special/filterTestL.md",
			content: `# filterTestL

**Unique special edge-case test file L**

This file tests unique special edge cases.

## Features
- Unique special testing
- Edge case scenarios
- MOC type

## Tags
- unique-special
- edge-case-specific
- mu-test

## Collection
special-filter-testing

## Metadata
- Type: moc
- Status: draft
- Priority: high
- Boards: special-board, testing-board
`,
		},
	}

	// create directories and files
	for _, file := range testFiles {
		fullPath := filepath.Join(docsPath, file.path)

		// create directory if it doesn't exist
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}

		// create the file
		if err := os.WriteFile(fullPath, []byte(file.content), 0644); err != nil {
			return fmt.Errorf("failed to create file %s: %v", fullPath, err)
		}
	}

	logging.LogInfo("filter test files created successfully")
	return nil
}
