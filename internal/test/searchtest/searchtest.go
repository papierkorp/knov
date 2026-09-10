package searchtest

import (
	"knov/internal/test"
)

// Suite runs the search test cases against real files, metadata and search indexing.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "search" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseSearchTitleOnly,
		caseSearchFullContent,
		caseSearchMultiWordPartial,
		caseSearchLoneCharNoFlood,
		caseSearchEmptyQuery,
		caseSearchLimit,
		caseSearchDeletedFileByTitle,
		caseSearchDeletedFileByContent,
		caseSearchScopedHidePath,
		caseSearchCommitReindexNoDuplicate,
	}

	return test.RunCases("search", cases), nil
}
