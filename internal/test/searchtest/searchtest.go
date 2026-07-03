package searchtest

import "knov/internal/test"

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
		caseSearchEmptyQuery,
		caseSearchLimit,
		caseSearchDeletedFileByTitle,
		caseSearchDeletedFileByContent,
	}

	result := &test.SuiteResult{Suite: "search"}
	for _, c := range cases {
		cr := c()
		result.Cases = append(result.Cases, cr)
		if cr.Success {
			result.Passed++
		} else {
			result.Failed++
		}
	}
	result.Total = len(cases)
	result.Success = result.Failed == 0
	return result, nil
}
