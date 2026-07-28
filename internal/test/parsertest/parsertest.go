// Package parsertest - Parser suite: exercises internal/parser's exported ProcessMarkdownLinks
// directly. Every case here is a pure string-in/string-out function call with no file IO, no
// setup/teardown and no global state to restore - unlike every prior suite, there's no
// resetAndSeed step at all.
package parsertest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the parser test cases against internal/parser.ProcessMarkdownLinks.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("parser-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
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
	}

	result := &test.SuiteResult{Suite: "parser"}
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
