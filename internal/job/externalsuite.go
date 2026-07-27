package job

import (
	"fmt"
	"sync"

	"knov/internal/test"
)

// suiteRunners/suiteMus hold every registered in-app test suite's run function and its own
// dedup mutex, keyed by job name. Every <group>test suite self-registers here via
// RegisterSuiteRunner in its own init() - internal/job never imports a suite package
// directly, avoiding the import cycle that would otherwise exist for any suite that itself
// needs to import internal/job (e.g. jobstest, which exercises the job scheduler/history
// directly). All registration happens during package init (single-threaded, before any HTTP
// request is served), so the maps are safe to read concurrently afterward without a lock of
// their own.
var (
	suiteRunners = map[string]func() (*test.SuiteResult, error){}
	suiteMus     = map[string]*sync.Mutex{}
)

// RegisterSuiteRunner wires a named admin-button job to a suite's Run function. Call this
// from the suite package's init(), alongside its test.Register(Suite{}) call for run-all.
func RegisterSuiteRunner(name string, run func() (*test.SuiteResult, error)) {
	suiteRunners[name] = run
	suiteMus[name] = &sync.Mutex{}
}

// RunSuiteTest runs the named registered suite (serialized against concurrent calls for that
// same name via its own dedup mutex) and returns its results alongside any error. Every
// exported Run<X>Test function is a thin wrapper around this - see scheduler.go.
func RunSuiteTest(name string) (*test.SuiteResult, error) {
	mu, ok := suiteMus[name]
	if !ok {
		return nil, fmt.Errorf("no suite runner registered for %s", name)
	}
	j := &externalSuiteJob{name: name}
	if err := execute(mu, j); err != nil {
		return nil, err
	}
	return j.results, nil
}

type externalSuiteJob struct {
	name    string
	results *test.SuiteResult
}

func (j *externalSuiteJob) Name() string { return j.name }

func (j *externalSuiteJob) Run() error {
	run, ok := suiteRunners[j.name]
	if !ok {
		return fmt.Errorf("no suite runner registered for %s", j.name)
	}
	results, err := run()
	j.results = results
	if err != nil {
		return fmt.Errorf("%s failed: %w", j.name, err)
	}
	return nil
}

func (j *externalSuiteJob) Output() any { return j.results }

func (j *externalSuiteJob) Message() string {
	if j.results == nil {
		return ""
	}
	return fmt.Sprintf("%d passed, %d failed", j.results.Passed, j.results.Failed)
}
