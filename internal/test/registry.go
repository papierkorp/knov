package test

// RunAllTests runs every registered suite in order and aggregates the results.
func RunAllTests() (*SuiteResult, error) {
	result := &SuiteResult{Suite: "all"}

	for _, suite := range suites {
		suiteResult, err := suite.Run()
		if err != nil {
			return nil, err
		}
		result.Cases = append(result.Cases, suiteResult.Cases...)
		result.Total += suiteResult.Total
		result.Passed += suiteResult.Passed
		result.Failed += suiteResult.Failed
	}

	result.Success = result.Failed == 0
	return result, nil
}
