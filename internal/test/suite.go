// Package test - shared types for in-app runtime test suites
package test

// CaseResult is the outcome of a single case within a Suite.
type CaseResult struct {
	Name     string
	Expected string
	Actual   string
	Error    string
	Success  bool
	Detail   any
}

// SuiteResult aggregates the CaseResults produced by a Suite.Run().
type SuiteResult struct {
	Suite   string
	Total   int
	Passed  int
	Failed  int
	Success bool
	Cases   []CaseResult
}

// Suite is implemented by every test group under internal/test/<group>test.
type Suite interface {
	Name() string
	Run() (*SuiteResult, error)
}

// suites holds every registered Suite. A <group>test package registers itself via
// Register() in its init(), which runs when anything imports that package (e.g. its
// job wrapper) - avoids internal/test importing its own subpackages, which would cycle
// since those subpackages import internal/test for the shared types above.
var suites []Suite

// Register adds a suite to the registry. Called from a <group>test package's init().
func Register(s Suite) {
	suites = append(suites, s)
}
