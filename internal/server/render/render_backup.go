package render

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"knov/internal/backup"
	"knov/internal/configmanager"
	"knov/internal/job"
	"knov/internal/logging"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

// backupContentsLabel summarizes the set an entry refers to - "full" when every registered
// storage was included at backup time, the actual (partial) selection (e.g. "metadata") when
// not, or "-" once the set itself is gone (nothing left to summarize).
func backupContentsLabel(t func(string, ...any) string, e backup.LogEntry) string {
	if !e.Available {
		return "-"
	}
	if e.Full {
		return t("full")
	}
	manifest, err := job.BackupManifest(e.Set)
	if err != nil {
		return "-"
	}
	return strings.Join(manifest, ", ")
}

// backupSourceLabel describes what triggered a backup-kind entry: "manual", "scheduled" (via
// KNOV_BACKUP_AUTO_ENABLED), or "restore" (the pre-restore safety snapshot every restore takes
// first). Restore-kind entries get "-" instead - a restore is always manually triggered (there's
// no scheduled-restore feature), so showing a trigger there would be redundant. Also "-" for
// entries logged before this field existed.
func backupSourceLabel(t func(string, ...any) string, e backup.LogEntry) string {
	if e.Kind != backup.EventBackup || e.Source == "" {
		return "-"
	}
	return t(string(e.Source))
}

// RenderBackupLog renders the backup/restore history table, newest first - one row per backup
// created or restore applied. Restore is destructive, so its button carries a confirm prompt.
// A row's actions (restore/lock/download) only appear while the set it refers to is still
// available - once rotated away (or, for a restore row, once the restored-from set is gone),
// the row stays for the historical record but with no actions, just a note that it's gone.
// There's no manual delete action - rotation trims automatically - but a set can be locked to
// opt out of that entirely, e.g. right before a risky change.
func RenderBackupLog(entries []backup.LogEntry) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var sb strings.Builder
	if configmanager.GetBackupAutoEnabled() {
		fmt.Fprintf(&sb, `<p class="backup-auto-status">%s</p>`,
			template.HTMLEscapeString(t("Auto backup: enabled (every %s)", configmanager.GetBackupAutoInterval())))
	} else {
		fmt.Fprintf(&sb, `<p class="backup-auto-status">%s</p>`, template.HTMLEscapeString(t("Auto backup: disabled")))
	}
	fmt.Fprintf(&sb, `<table class="backup-table"><thead><tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th></th></tr></thead><tbody>`,
		t("Time"), t("Event"), t("Trigger"), t("Contents"))
	if len(entries) == 0 {
		fmt.Fprintf(&sb, `<tr><td colspan="5" style="text-align:center;color:var(--text-secondary);">%s</td></tr>`, t("No backups yet"))
	}
	for _, e := range entries {
		event := t("Backup created: %s", e.Set)
		if e.Kind == "restore" {
			event = t("Restored: %s", e.Set)
		}

		actions := fmt.Sprintf(`<span class="backup-unavailable">%s</span>`, t("no longer available"))
		if e.Available {
			lockVerb, lockLabel := "hx-post", t("Lock")
			if e.Locked {
				lockVerb, lockLabel = "hx-delete", t("Unlock")
			}
			actions = fmt.Sprintf(
				`<button class="btn-secondary" %s="/api/system/backups/%s/lock" hx-target="#backup-list" hx-swap="innerHTML">%s</button>`+
					`<a class="btn-secondary" href="/api/system/backups/%s/download" download>%s</a>`+
					`<button class="btn-secondary" hx-post="/api/system/backups/%s/restore" hx-confirm="%s" hx-target="#backup-status" hx-swap="innerHTML">%s</button>`,
				lockVerb, template.HTMLEscapeString(e.Set), lockLabel,
				template.HTMLEscapeString(e.Set), t("Download"),
				template.HTMLEscapeString(e.Set),
				template.HTMLEscapeString(t("Restore %s? Current state is snapshotted first, then the app restarts to apply it.", e.Set)),
				t("Restore"),
			)
		}

		fmt.Fprintf(&sb, `<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td class="backup-actions">%s</td></tr>`,
			template.HTMLEscapeString(configmanager.FormatDateTimeSeconds(e.Time)),
			template.HTMLEscapeString(event),
			template.HTMLEscapeString(backupSourceLabel(t, e)),
			template.HTMLEscapeString(backupContentsLabel(t, e)),
			actions,
		)
	}
	sb.WriteString(`</tbody></table>`)
	return sb.String()
}

// renderBackupStorageCheckboxes renders one checkbox per registered storage, all checked by
// default (a full backup) - unchecking some (e.g. everything but "metadata") creates a partial
// backup set instead.
func renderBackupStorageCheckboxes() string {
	var sb strings.Builder
	for _, name := range job.RegisteredStorageNames() {
		fmt.Fprintf(&sb, `<label class="backup-storage-option"><input type="checkbox" name="storages" value="%s" checked> %s</label>`,
			template.HTMLEscapeString(name), template.HTMLEscapeString(name))
	}
	return sb.String()
}

func HandleSystemBackup(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	entries, err := job.ListBackupLog()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to list backups: %v", err)
	}

	content := `<style>
.backup-table { width: 100%; border-collapse: collapse; font-size: .85rem; }
.backup-table th { text-align: left; padding: .35rem .6rem; border-bottom: 2px solid var(--border); }
.backup-table td { padding: .28rem .6rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); vertical-align: middle; }
.backup-note { color: var(--text-secondary); font-size: .8rem; margin-bottom: .75rem; }
.backup-auto-status { font-size: .8rem; margin: 0 0 .5rem; }
.backup-create-form { display: flex; flex-direction: column; gap: .5rem; margin-bottom: .75rem; }
.backup-storage-select { display: flex; flex-wrap: wrap; gap: .1rem 1rem; }
.backup-storage-option { display: flex; align-items: center; gap: .3rem; font-size: .85rem; }
.backup-toolbar { display: flex; align-items: center; gap: .75rem; }
.backup-actions { display: flex; gap: .4rem; justify-content: flex-end; }
.backup-unavailable { color: var(--text-secondary); font-style: italic; }
</style>` +
		fmt.Sprintf(`<p class="backup-note">%s</p>`, t("Each backup set snapshots StoragePath (metadata, cache, chat, kanban, notifications, config, search) - not the docs/media in DataPath, which git already covers. Storages are snapshotted one at a time, not as a single point-in-time transaction. Automatic backups can be enabled via KNOV_BACKUP_AUTO_ENABLED, and rotation tuned via KNOV_BACKUP_ROTATION_KEEP_DAYS/KNOV_BACKUP_ROTATION_KEEP_FULL (see .env.example). Lock a set to keep it regardless of rotation.")) +
		`<form class="backup-create-form" hx-post="/api/system/backups" hx-target="#backup-list" hx-swap="innerHTML" hx-indicator="#backup-status">` +
		fmt.Sprintf(`<div class="backup-storage-select">%s</div>`, renderBackupStorageCheckboxes()) +
		`<div class="backup-toolbar">` +
		fmt.Sprintf(`<button type="submit" class="btn-primary">%s</button>`, t("Create backup")) +
		`<span id="backup-status"></span>` +
		`</div>` +
		`</form>` +
		fmt.Sprintf(`<div id="backup-list">%s</div>`, RenderBackupLog(entries))

	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Backups", template.HTML(content)); err != nil {
		logging.LogError(logging.KeyApp, "failed to render backup page: %v", err)
	}
}
