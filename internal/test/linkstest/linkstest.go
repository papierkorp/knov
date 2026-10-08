// Package linkstest - Links suite: every link writer (editor autocomplete, upload, rename, media
// relocate, filter index, book editor) links the specialchars corpus, and every reader - link
// metadata (used links, linked from) and the rendered page - has to resolve each link to the same
// file (see docs/temp_todo.md link refactor). The pure codec round trips (encode -> ExtractLinks,
// ResolveWikiTarget, render, book entries, dokuwiki converter) are go tests next to their code.
package linkstest

import (
	"knov/internal/test"
)

// Suite runs the link writers against real files, metadata and the real router.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "links" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseAutocomplete,
		caseUpload,
		caseRepairOldUpload,
		caseRename,
		caseRelative,
		caseRelocate,
		caseFilterIndex,
		caseBookEditor,
	}

	return test.RunCases("links", cases), nil
}
