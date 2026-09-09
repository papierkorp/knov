package job

import "sync/atomic"

// Progress is a live work counter a long-running async job updates as it runs. The jobs UI
// reads it via GetProgress (as a ProgressSnapshot) while polling GET /api/jobs/{id}, turning
// the binary running/done spinner into a "working... 340/1200" readout. A total of 0 means
// "no progress info yet".
type Progress struct {
	done, total atomic.Int64
}

// Report sets the done and total counts in one call. Its signature matches the optional
// func(done, total int) reporter the files package's bulk operations accept, so a job can pass
// j.prog.Report straight through. Safe for concurrent use.
func (p *Progress) Report(done, total int) {
	p.total.Store(int64(total))
	p.done.Store(int64(done))
}

// Load returns the current done and total counts. Report stores total before done, so as long
// as a run keeps total constant (as every current caller does) a torn read can only miss a done
// increment, never report done > total; a caller that varied total mid-run would void that.
func (p *Progress) Load() (done, total int) {
	return int(p.done.Load()), int(p.total.Load())
}

// withProgress is embedded by async job types that report live progress: it carries the
// counter and supplies the Progresser method so each job type doesn't re-declare either.
type withProgress struct{ prog Progress }

func (w *withProgress) Progress() *Progress { return &w.prog }

// Progresser is implemented by a Job that exposes a live Progress counter.
type Progresser interface {
	Progress() *Progress
}

// ProgressSnapshot is an immutable read of a job's Progress counter, passed to the render layer
// so it never has to reach back into the async-job registry itself.
type ProgressSnapshot struct{ Done, Total int }

// Reported reports whether the snapshot carries a usable count - a job that reports progress
// always has a non-zero Total. Callers key the "working... 3/10" vs plain "working..." choice
// off this rather than re-deriving Total > 0 themselves.
func (s ProgressSnapshot) Reported() bool { return s.Total > 0 }

// GetProgress returns a snapshot of the running async job id's progress counter. The zero
// ProgressSnapshot (Reported() == false) is returned when id is unknown, already finished, or
// its job type doesn't report progress.
func GetProgress(id string) ProgressSnapshot {
	asyncRunsMu.Lock()
	j := asyncRuns[id].job
	asyncRunsMu.Unlock()

	p, isP := j.(Progresser)
	if !isP {
		return ProgressSnapshot{}
	}
	done, total := p.Progress().Load()
	return ProgressSnapshot{Done: done, Total: total}
}
