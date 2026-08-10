package render

import (
	"fmt"
	"html/template"

	"knov/internal/jobStorage"
	"knov/internal/translation"
)

// RenderJobStatus renders the current status of an async job (started via job.StartAsync) for
// htmx polling of GET /api/jobs/{id}: a self-polling spinner while running, an empty tag once
// done (its caller's success feedback - toast/redirect - is set separately by the handler), or
// an inline error message if it failed or was interrupted by a restart.
func RenderJobStatus(lang, id string, rec *jobStorage.JobRecord) string {
	safeID := template.HTMLEscapeString(id)
	switch rec.Status {
	case jobStorage.StatusRunning:
		return fmt.Sprintf(
			`<span id="job-status-%s" class="job-status-pending" hx-get="/api/jobs/%s" hx-trigger="every 1s" hx-swap="outerHTML"><i class="fa fa-spinner fa-spin"></i> %s</span>`,
			safeID, safeID, template.HTMLEscapeString(translation.SprintfForRequest(lang, "working...")))
	case jobStorage.StatusDone:
		return fmt.Sprintf(`<span id="job-status-%s" class="job-status-done"></span>`, safeID)
	default:
		return fmt.Sprintf(`<span id="job-status-%s" class="job-status-failed"><i class="fa fa-triangle-exclamation"></i> %s</span>`,
			safeID, template.HTMLEscapeString(rec.Error))
	}
}

// RenderJobStatusListItem wraps RenderJobStatus in a <li>, for the one-time swap of a browse
// tree row (hx-target="closest li") into a polling status span. Every later poll response
// swaps just the inner span (RenderJobStatus), keeping this <li> wrapper - and its parent
// <ul>'s valid content model - stable.
func RenderJobStatusListItem(lang, id string, rec *jobStorage.JobRecord) string {
	return "<li>" + RenderJobStatus(lang, id, rec) + "</li>"
}
