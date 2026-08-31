// Package filtertest - Filter suite: seeds real files/metadata and runs real filter configs
package filtertest

import (
	"fmt"

	"knov/internal/logging"
	"knov/internal/test"
)

// Suite runs the filter test scenarios against real filter configs.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "filter" }

// Run executes the filter test scenarios and returns the aggregated suite result.
func (Suite) Run() (*test.SuiteResult, error) {
	if err := createFilterTestMetadata(); err != nil {
		logging.LogInfo(logging.KeyFilterDebug, "failed to create filter test metadata: %v", err)
		return nil, fmt.Errorf("failed to create filter test metadata: %v", err)
	}

	result := &test.SuiteResult{Suite: "filter"}

	for _, tc := range testConfigs {
		caseResult := runCase(tc)
		result.Cases = append(result.Cases, caseResult)
		if caseResult.Success {
			result.Passed++
		} else {
			result.Failed++
			logging.LogInfo(logging.KeyFilterDebug, "test %s failed: %s", caseResult.Name, caseResult.Error)
		}
	}

	scopedCase := caseFilterScopedHidePath()
	result.Cases = append(result.Cases, scopedCase)
	if scopedCase.Success {
		result.Passed++
	} else {
		result.Failed++
	}

	result.Total = len(testConfigs) + 1
	result.Success = result.Failed == 0

	if result.Failed > 0 {
		logging.LogInfo(logging.KeyFilterDebug, "filter tests completed with failures: %d passed, %d failed", result.Passed, result.Failed)
	}

	return result, nil
}
