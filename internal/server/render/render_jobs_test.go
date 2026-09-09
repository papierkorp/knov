package render

import (
	"strings"
	"testing"

	"knov/internal/job"
	"knov/internal/jobStorage"
)

func TestRenderJobStatus(t *testing.T) {
	cases := []struct {
		name        string
		rec         *jobStorage.JobRecord
		cancellable bool
		progress    job.ProgressSnapshot
		want        []string
		notWant     []string
	}{
		{
			name:    "running without progress shows a plain spinner",
			rec:     &jobStorage.JobRecord{ID: "abc", Status: jobStorage.StatusRunning},
			want:    []string{`id="job-status-abc"`, `hx-get="/api/jobs/abc"`, "working..."},
			notWant: []string{"job-status-cancel", "working... 0"},
		},
		{
			name:        "running and cancellable shows a cancel button",
			rec:         &jobStorage.JobRecord{ID: "abc", Status: jobStorage.StatusRunning},
			cancellable: true,
			want:        []string{`class="job-status-cancel"`, `hx-delete="/api/jobs/abc"`},
		},
		{
			name:     "reported progress renders done/total in order",
			rec:      &jobStorage.JobRecord{ID: "abc", Status: jobStorage.StatusRunning},
			progress: job.ProgressSnapshot{Done: 3, Total: 10},
			want:     []string{"working... 3/10"},
			notWant:  []string{"working... 10/3"},
		},
		{
			name:     "zero total is treated as no progress",
			rec:      &jobStorage.JobRecord{ID: "abc", Status: jobStorage.StatusRunning},
			progress: job.ProgressSnapshot{Done: 5, Total: 0},
			want:     []string{"working..."},
			notWant:  []string{"working... 5", "/0"},
		},
		{
			name:    "done renders the empty done span",
			rec:     &jobStorage.JobRecord{ID: "abc", Status: jobStorage.StatusDone},
			want:    []string{`class="job-status-done"`},
			notWant: []string{"hx-get", "working"},
		},
		{
			name: "canceled renders the canceled state",
			rec:  &jobStorage.JobRecord{ID: "abc", Status: jobStorage.StatusCanceled},
			want: []string{`class="job-status-canceled"`},
		},
		{
			name: "error status renders the failure message",
			rec:  &jobStorage.JobRecord{ID: "abc", Status: jobStorage.StatusError, Error: "disk full"},
			want: []string{`class="job-status-failed"`, "disk full"},
		},
		{
			name:    "id is html-escaped everywhere it is interpolated",
			rec:     &jobStorage.JobRecord{ID: `a"<b`, Status: jobStorage.StatusRunning},
			want:    []string{"job-status-a&#34;&lt;b"},
			notWant: []string{`a"<b`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderJobStatus("en", tc.rec.ID, tc.rec, tc.cancellable, tc.progress)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("output missing %q\ngot: %s", w, got)
				}
			}
			for _, nw := range tc.notWant {
				if strings.Contains(got, nw) {
					t.Errorf("output should not contain %q\ngot: %s", nw, got)
				}
			}
		})
	}
}

func TestRenderJobStatusListItemWraps(t *testing.T) {
	rec := &jobStorage.JobRecord{ID: "abc", Status: jobStorage.StatusRunning}
	got := RenderJobStatusListItem("en", rec.ID, rec, false, job.ProgressSnapshot{})
	if !strings.HasPrefix(got, "<li>") || !strings.HasSuffix(got, "</li>") {
		t.Errorf("expected <li>-wrapped output, got: %s", got)
	}
	if !strings.Contains(got, RenderJobStatus("en", rec.ID, rec, false, job.ProgressSnapshot{})) {
		t.Errorf("list item should embed the bare status span, got: %s", got)
	}
}
