// Package settingstest - shared helpers
package settingstest

import "knov/internal/test"

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
