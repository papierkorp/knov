// Package reservedtest - reserved folders suite: docs files in the top-level docs folders named
// like a path prefix (docs/docs/, docs/media/, docs/files/) are listed, opened, edited, linked,
// renamed, moved and deleted as themselves, never as the media or docs file their path would
// read as without the docs/ prefix (see docs/temp_todo.md reserved folders refactoring). Its
// sample files can't live under docs/test/ - only top-level folder names are ambiguous - so it
// wipes them itself.
package reservedtest

import (
	"knov/internal/test"
)

// Suite runs the reserved folders cases against real files, metadata and the real router.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "reserved-folders" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}
	defer wipe()

	// the cases of the steps still open in the todo are registered with their step
	cases := []func() test.CaseResult{
		caseListing,
		caseMetadata,
	}

	return test.RunCases("reserved-folders", cases), nil
}
