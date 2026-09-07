package job

import "sync/atomic"

// Progress is a live work counter a long-running async job updates as it runs. The jobs UI
// reads it via GetProgress while polling GET /api/jobs/{id}, turning the binary running/done
// spinner into a "working... 340/1200" readout. A total of 0 means "no progress info yet".
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

// Load returns the current done and total counts. total is written once at the start of a run
// and never changes after, so a torn read can only ever miss a done increment, never report
// done > total.
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

// GetProgress returns the running async job id's done/total counts. ok is false when id is
// unknown, already finished, or its job type doesn't report progress (total still 0).
func GetProgress(id string) (done, total int, ok bool) {
	asyncRunsMu.Lock()
	j := asyncRuns[id].job
	asyncRunsMu.Unlock()

	p, isP := j.(Progresser)
	if !isP {
		return 0, 0, false
	}
	done, total = p.Progress().Load()
	return done, total, total > 0
}
