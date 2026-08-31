// Package asyncjobtest - sample file/probe-job helpers
package asyncjobtest

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"knov/internal/contentStorage"
	"knov/internal/jobStorage"
	"knov/internal/notificationStorage"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// testDir is the docs-relative sample folder every case seeds into, wiped and reseeded at the
// start of each run so cases never see stale state from a previous run.
const testDir = "test/asyncjob-tests"

func testPath(name string) string {
	return pathutils.ToSlash(filepath.Join(testDir, name))
}

func withPrefix(name string) string {
	return pathutils.ToWithPrefix(testPath(name))
}

func writeFile(relPath, content string) error {
	full := pathutils.ToDocsPath(relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, []byte(content), 0644)
}

func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	return os.MkdirAll(full, 0755)
}

// probeJob is a minimal job.Job used to control exact timing/outcome for the StartAsync cases -
// none of the real job types expose a way to block mid-run or panic on demand.
type probeJob struct {
	name    string
	release <-chan struct{} // if set, Run blocks until closed or ctx is canceled
	panic   string          // if set, Run panics with this value instead of blocking/returning
}

func (j *probeJob) Name() string { return j.name }

func (j *probeJob) Run(ctx context.Context) error {
	if j.panic != "" {
		panic(j.panic)
	}
	if j.release != nil {
		select {
		case <-j.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// waitForTerminal polls jobStorage for id until it leaves the running status or timeout elapses.
func waitForTerminal(id string, timeout time.Duration) (*jobStorage.JobRecord, error) {
	deadline := time.Now().Add(timeout)
	for {
		rec, err := jobStorage.Get(id)
		if err != nil {
			return nil, err
		}
		if rec != nil && rec.Status != jobStorage.StatusRunning {
			return rec, nil
		}
		if time.Now().After(deadline) {
			return rec, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// drainPending consumes and discards every currently pending notification, so a case starting
// from a known-empty pending queue doesn't observe a stale item left over from real app usage or
// a previous run. Destructive to any real pending flash message not yet displayed - accepted
// tradeoff, same as notificationstest.drainPending.
func drainPending() error {
	for {
		n, err := notificationStorage.ConsumePending()
		if err != nil {
			return err
		}
		if n == nil {
			return nil
		}
	}
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
