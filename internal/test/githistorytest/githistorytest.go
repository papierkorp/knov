package githistorytest

import (
	"knov/internal/test"
)

// Suite runs the git repo/file history test cases against real files, metadata and commits.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "git-history" }

func (Suite) Run() (*test.SuiteResult, error) {
	state, err := resetAndSeed()
	if err != nil {
		return nil, err
	}

	cases := []func(*sampleState) test.CaseResult{
		caseGitLatestChangesPagination,
		caseGitLatestChangesCollectionFilter,
		caseGitSearchByFilename,
		caseGitFileHistoryVersions,
		caseGitFileViewVersion,
		caseGitFileDiff,
		caseGitFileRestore,
		caseGitRemotePushPullTestAuth,
		caseGitSyncWrittenFiles,
	}

	result := &test.SuiteResult{Suite: "git-history"}
	for _, c := range cases {
		result.Add(c(state))
	}
	result.Success = result.Failed == 0
	return result, nil
}
