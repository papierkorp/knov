// Package markdowntest - Markdown suite: exercises internal/markdown's fence-aware
// scanning primitives (FenceMask, CodeBlocks, StripFencedBlocks) and parser.Headings
// on top of them. Most cases are pure string-in/value-out calls;
// caseRenderLongerRunFenceDoesNotSwallow is an end-to-end check that runs the full
// parser.MarkdownHandler render path. No file IO, no setup/teardown, no global state.
package markdowntest

import (
	"knov/internal/test"
)

// Suite runs the markdown scanner test cases against internal/markdown.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "markdown" }

func (Suite) Run() (*test.SuiteResult, error) {
	cases := []func() test.CaseResult{
		caseFenceMaskBacktick,
		caseFenceMaskTilde,
		caseFenceMaskUnterminated,
		caseFenceMaskInfoStringDoesNotClose,
		caseHeadingsSkipFenced,
		caseHeadingsDedupIDs,
		caseHeadingsUnicodeID,
		caseHeadingsLinkID,
		caseHeadingsWikiLinkAliasID,
		caseHeadingScanMatchesRenderIDs,
		caseHeadingsRequireSpaceAndLevel,
		caseHeadingsSkipFrontMatter,
		caseStripFencedBlocksBothMarkers,
		caseCodeBlocksLangAndIndent,
		caseCodeBlocksTilde,
		caseCodeBlocksLongerRunCloses,
		caseCodeBlocksShorterRunDoesNotClose,
		caseCodeBlocksUnterminated,
		caseRenderLongerRunFenceDoesNotSwallow,
	}

	result := &test.SuiteResult{Suite: "markdown"}
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
