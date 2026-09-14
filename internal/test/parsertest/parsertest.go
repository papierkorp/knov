// Package parsertest - Parser suite: exercises internal/parser's exported link helpers
// (ProcessMarkdownLinks, ResolveWikiLinks, ResolveWikiTarget) directly. Every case here is a
// pure string-in/string-out function call with no file IO, no
// setup/teardown and no global state to restore - unlike every prior suite, there's no
// resetAndSeed step at all.
package parsertest

import (
	"knov/internal/test"
)

// Suite runs the parser test cases against internal/parser.ProcessMarkdownLinks.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "parser" }

func (Suite) Run() (*test.SuiteResult, error) {
	cases := []func() test.CaseResult{
		caseFallbackLabelPlainPath,
		caseFallbackLabelPathPlusAnchor,
		caseFallbackLabelPureAnchor,
		casePercentEncodedPathDecodedBeforeLabel,
		casePercentEncodedAnchorDecodedBeforeLabel,
		caseUnicodeHeaderSlugCapitalization,
		caseExternalLinkUntouched,
		caseImageEmbedUntouched,
		caseWikiTargetExtraction,
		caseWikiLinkPureAnchor,
		caseTodoDateNoDuplication,
	}

	return test.RunCases("parser", cases), nil
}
