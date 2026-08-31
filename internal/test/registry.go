package test

import "fmt"

// RunAllTests runs the registered suite matching name, or every registered suite in order if
// name is empty, and aggregates the results.
func RunAllTests(name string) (*SuiteResult, error) {
	result := &SuiteResult{Suite: "all"}

	toRun := suites
	if name != "" {
		toRun = nil
		for _, suite := range suites {
			if suite.Name() == name {
				toRun = []Suite{suite}
				break
			}
		}
		if toRun == nil {
			return nil, fmt.Errorf("no test suite named %q", name)
		}
		result.Suite = name
	}

	for _, suite := range toRun {
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
