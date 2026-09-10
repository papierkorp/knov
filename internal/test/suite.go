// Package test - shared types for in-app runtime test suites
package test

// CaseResult is the outcome of a single case within a Suite.
type CaseResult struct {
	Name     string
	Expected string
	Actual   string
	Error    string
	Success  bool
	// Skipped marks a case that could not run (missing opt-in env, unavailable
	// external dependency) rather than one that passed; Actual holds the reason.
	// SuiteResult.Add forces Success true for a skipped case, so Skipped always
	// wins over Success regardless of what the case set - build one with SkipCase.
	Skipped bool
	Detail  any
}

// SuiteResult aggregates the CaseResults produced by a Suite.Run().
type SuiteResult struct {
	Suite   string
	Total   int
	Passed  int
	Failed  int
	Skipped int // cases that could not run, tallied apart from Passed/Failed
	Success bool
	Cases   []CaseResult
}

// Add appends cr and bumps the matching tally: a skipped case counts only toward
// Skipped, otherwise Success decides Passed vs Failed. Every case counts toward Total.
// A skipped case is normalized to Success true here so downstream readers that key
// on Success alone (e.g. failure listings) never treat it as a failure.
func (r *SuiteResult) Add(cr CaseResult) {
	if cr.Skipped {
		cr.Success = true
	}
	r.Cases = append(r.Cases, cr)
	r.Total++
	switch {
	case cr.Skipped:
		r.Skipped++
	case cr.Success:
		r.Passed++
	default:
		r.Failed++
	}
}

// SkipCase builds a skipped CaseResult; reason explains why the case could not run
// and is surfaced as Actual. Add/RunCases count it toward Skipped only.
func SkipCase(name, reason string) CaseResult {
	return CaseResult{Name: name, Skipped: true, Success: true, Actual: reason}
}

// RunCases runs every case in order, tallies the results into a SuiteResult named
// suite, and sets Success when nothing failed. Suites with per-case setup or extra
// case arguments call Add directly instead.
func RunCases(suite string, cases []func() CaseResult) *SuiteResult {
	r := &SuiteResult{Suite: suite}
	for _, c := range cases {
		r.Add(c())
	}
	r.Success = r.Failed == 0
	return r
}

// Suite is implemented by every test group under internal/test/<group>test.
type Suite interface {
	Name() string
	Run() (*SuiteResult, error)
}

// suites holds every registered Suite. A <group>test package registers itself via
// Register() in its init(), triggered by main.go's blank imports of every <group>test
// package - avoids internal/test importing its own subpackages, which would cycle since
// those subpackages import internal/test for the shared types above.
var suites []Suite

// Register adds a suite to the registry. Called from a <group>test package's init().
func Register(s Suite) {
	suites = append(suites, s)
}

// SuiteNames returns the Name() of every registered suite, in registration order - used by
// cli's --start-tests usage text to list the suites available to run individually.
func SuiteNames() []string {
	names := make([]string, len(suites))
	for i, s := range suites {
		names[i] = s.Name()
	}
	return names
}
