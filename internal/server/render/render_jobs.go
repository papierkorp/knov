package render

import (
	"fmt"
	"html/template"

	"knov/internal/job"
	"knov/internal/jobStorage"
	"knov/internal/translation"
)

// RenderJobStatus renders the current status of an async job (started via job.StartAsync) for
// htmx polling of GET /api/jobs/{id}: a self-polling spinner while running (with a cancel button
// if cancellable - see job.IsCancellable), an empty tag once done (its caller's success feedback
// - toast/redirect - is set separately by the handler), or an inline message if it failed, was
// canceled, or was interrupted by a restart. progress is resolved by the handler (job.GetProgress)
// and passed in, so this stays a pure string builder.
func RenderJobStatus(lang, id string, rec *jobStorage.JobRecord, cancellable bool, progress job.ProgressSnapshot) string {
	safeID := template.HTMLEscapeString(id)
	switch rec.Status {
	case jobStorage.StatusRunning:
		var cancelBtn string
		if cancellable {
			cancelBtn = fmt.Sprintf(
				`<button type="button" class="job-status-cancel" hx-delete="/api/jobs/%s" hx-target="#job-status-%s" hx-swap="outerHTML">%s</button>`,
				safeID, safeID, template.HTMLEscapeString(translation.SprintfForRequest(lang, "cancel")))
		}
		label := translation.SprintfForRequest(lang, "working...")
		if progress.Reported() {
			label = translation.SprintfForRequest(lang, "working... %d/%d", progress.Done, progress.Total)
		}
		return fmt.Sprintf(
			`<span id="job-status-%s" class="job-status-pending" hx-get="/api/jobs/%s" hx-trigger="every 1s" hx-swap="outerHTML"><i class="fa fa-spinner fa-spin"></i> %s%s</span>`,
			safeID, safeID, template.HTMLEscapeString(label), cancelBtn)
	case jobStorage.StatusDone:
		return fmt.Sprintf(`<span id="job-status-%s" class="job-status-done"></span>`, safeID)
	case jobStorage.StatusCanceled:
		return fmt.Sprintf(`<span id="job-status-%s" class="job-status-canceled"><i class="fa fa-ban"></i> %s</span>`,
			safeID, template.HTMLEscapeString(translation.SprintfForRequest(lang, "canceled")))
	default:
		return fmt.Sprintf(`<span id="job-status-%s" class="job-status-failed"><i class="fa fa-triangle-exclamation"></i> %s</span>`,
			safeID, template.HTMLEscapeString(rec.Error))
	}
}

// RenderJobStatusListItem wraps RenderJobStatus in a <li>, for the one-time swap of a browse
// tree row (hx-target="closest li") into a polling status span. Every later poll response
// swaps just the inner span (RenderJobStatus), keeping this <li> wrapper - and its parent
// <ul>'s valid content model - stable.
func RenderJobStatusListItem(lang, id string, rec *jobStorage.JobRecord, cancellable bool, progress job.ProgressSnapshot) string {
	return "<li>" + RenderJobStatus(lang, id, rec, cancellable, progress) + "</li>"
}

// RenderExportDone renders the existing pdf export archive: a link to download it and a button to
// delete it. Shown on the admin page and after the export job finished.
func RenderExportDone(lang string) string {
	return fmt.Sprintf(`<span class="job-status-done"><a href="/api/exports/pdf" class="btn-secondary"><i class="fa fa-download"></i> %s</a> <button type="button" class="btn-secondary" hx-delete="/api/exports/pdf" hx-confirm="%s" hx-target="closest span" hx-swap="outerHTML" hx-status:4xx="swap:none" hx-status:5xx="swap:none"><i class="fa fa-trash"></i> %s</button></span>`,
		template.HTMLEscapeString(translation.SprintfForRequest(lang, "download export")),
		template.HTMLEscapeString(translation.SprintfForRequest(lang, "delete export?")),
		template.HTMLEscapeString(translation.SprintfForRequest(lang, "delete export")))
}
