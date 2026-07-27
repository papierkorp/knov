// Package connectionstest - Connections suite: exercises the info-slideout's relationship
// data - parents/ancestors/kids/grandchildren/used-links/links-to-here/related and the
// conflict banner state (see docs/temp_todo.md step 5). Every handler in internal/server/
// api_links.go is a thin field read off *files.Metadata (or, for grandchildren, an inline
// loop with no exported equivalent - replicated directly here, same as editorstest/chattest
// replicate other unexported handler logic). Parents/Kids/Ancestor are seeded via real
// MetaDataSave calls so the actual parent-child cascade (updateParentChildRelationships/
// updateAncestors in internal/files/metadata_links.go) computes them; UsedLinks/LinksToHere
// are seeded via a real files.UpdateLinksForSingleFile call on the linking file (which updates
// both its own UsedLinks and the linked target's LinksToHere) - none of these are faked via
// a raw shortcut. Related has no per-file computation path (only the full-vault
// MetaDataLinksRebuild computes it, which would rebuild every real file in the vault, not just
// this suite's sample folder) so it's seeded directly via MetaDataSaveRaw instead, mirroring
// kanbantest's CreatedAt pin.
package connectionstest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the connections test cases against real files and metadata.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("connections-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "connections" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseParentsAncestors,
		caseKidsGrandchildren,
		caseUsedLinks,
		caseLinksToHere,
		caseRelatedFiles,
		caseAncestorsInFolder,
		caseConflictBanner,
		caseConflictOfBanner,
	}

	result := &test.SuiteResult{Suite: "connections"}
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
