package job

import (
	"context"
	"fmt"

	"knov/internal/export"
	"knov/internal/jobStorage"
	"knov/internal/logging"
)

// StartExport creates the pdf export archive in the background. Returns the job id to poll for completion.
func StartExport() (string, error) {
	return StartAsync(&exportMu, &exportJob{}, "")
}

type exportJob struct {
	withProgress
	skipped int
}

func (j *exportJob) Name() string { return JobTypeExport }

func (j *exportJob) Run(ctx context.Context) error {
	skipped, err := export.Create(ctx, j.prog.Report)
	if err != nil {
		return err
	}
	j.skipped = skipped
	logging.LogInfo(logging.KeyExport, "%s", j.Message())
	return nil
}

func (j *exportJob) Message() string {
	if j.skipped > 0 {
		return fmt.Sprintf("created pdf export, %d files skipped (see logs)", j.skipped)
	}
	return "created pdf export"
}

// RunningExport returns the job id of the running pdf export, or "" if none is running.
func RunningExport() string {
	running, err := jobStorage.ListRunning()
	if err != nil {
		logging.LogError(logging.KeyExport, "failed to list running jobs: %v", err)
		return ""
	}
	for _, rec := range running {
		if rec.Type == JobTypeExport {
			return rec.ID
		}
	}
	return ""
}
