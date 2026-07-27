// Package mediatest - Media suite: exercises internal/files/media.go and the media handlers'
// inline logic (internal/server/api_media.go) directly - upload, list/partition (all/used/
// orphaned), rename, delete (including the "still referenced" guard), and storage stats (see
// docs/temp_todo.md step 6). The orphaned-media *cleanup job* itself (job.RunMediaCleanup) is
// covered by jobstest's media-cleanup case instead, since that's fundamentally a jobs-suite
// concern (trigger a job, assert resulting FS/DB state) that happens to operate on media -
// this suite only asserts orphan *detection*, not deletion.
package mediatest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the media test cases against real media files and metadata.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("media-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "media" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseUpload,
		caseListPartition,
		caseRename,
		caseDelete,
		caseDeleteBlockedWhenReferenced,
		caseStats,
	}

	result := &test.SuiteResult{Suite: "media"}
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
