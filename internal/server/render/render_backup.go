package render

import (
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"strings"

	"knov/internal/backup"
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/job"
	"knov/internal/logging"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

// backupContentsLabel summarizes the set an entry refers to - "default" when exactly the default
// storages were included at backup time, the actual (explicit) selection (e.g. "metadata", or
// "metadata, ..., docs, media") when not given, or "-" once the set itself is gone (nothing left
// to summarize). manifest is nil for a default/unavailable entry, where it's never read - see
// rowManifest.
func backupContentsLabel(t func(string, ...any) string, e backup.LogEntry, manifest []string) string {
	if !e.Available {
		return "-"
	}
	if e.Default {
		return t("default")
	}
	if manifest == nil {
		return "-"
	}
	return strings.Join(manifest, ", ")
}

// restoreTouchesGit reports whether restoring e.Set would touch the working tree at all - true
// only when manifest includes docs/media (see files.DocsStorageName/MediaStorageName), the only
// storages that restore straight onto disk rather than into a database.
func restoreTouchesGit(manifest []string) bool {
	return slices.Contains(manifest, files.DocsStorageName) || slices.Contains(manifest, files.MediaStorageName)
}

// autoBackupStatusHTML renders the configured auto-backup profiles (KNOV_BACKUP_AUTO_PROFILES) as
// a short status block, one line per profile naming its cron schedule and storage selection
// ("default" when none is set, i.e. the default backup set at run time) - "disabled" when none are
// configured.
func autoBackupStatusHTML(t func(string, ...any) string) string {
	profiles := configmanager.GetBackupAutoProfiles()
	if len(profiles) == 0 {
		return fmt.Sprintf(`<p class="backup-auto-status">%s</p>`, template.HTMLEscapeString(t("Auto backup: disabled")))
	}
	var sb strings.Builder
	for _, p := range profiles {
		storages := strings.Join(p.Storages, ", ")
		if storages == "" {
			storages = t("default")
		}
		fmt.Fprintf(&sb, `<p class="backup-auto-status">%s</p>`,
			template.HTMLEscapeString(t("Auto backup %s: cron %s, storages: %s", p.Name, p.Cron, storages)))
	}
	return sb.String()
}

// backupStorageText is the one-line "where backup sets currently go" description shown under the
// auto-backup status: a local filesystem path, or an S3 bucket (with prefix and endpoint when
// set). job.BackupStorage returns the pieces; composing and translating them is render's job.
func backupStorageText(t func(string, ...any) string) string {
	s := job.BackupStorage()
	if s.Kind != "S3" {
		return t("local filesystem: %s", s.Path)
	}
	loc := s.Bucket
	if s.Prefix != "" {
		loc += "/" + s.Prefix
	}
	if s.Endpoint != "" {
		loc += " @ " + s.Endpoint
	}
	return t("S3: %s", loc)
}

// rowManifest returns e.Set's manifest, fetched once per row and shared between
// backupContentsLabel and restoreTouchesGit rather than each reading the archive on its own. Nil
// whenever there's nothing to read: e is a default set (its manifest is never shown), unavailable
// (the set itself is gone), or the read failed.
func rowManifest(e backup.LogEntry) []string {
	if !e.Available || e.Default {
		return nil
	}
	manifest, err := job.BackupManifest(e.Set)
	if err != nil {
		return nil
	}
	return manifest
}

// backupSourceLabel describes what triggered a backup-kind entry: "manual", "scheduled" (via
// KNOV_BACKUP_AUTO_PROFILES), or "restore" (the pre-restore safety snapshot every restore takes
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
// created or restore applied. Restore and delete are destructive, so their buttons carry a
// confirm prompt. A row's actions (restore/lock/download/delete) only appear while the set it
// refers to is still available - once rotated away (or, for a restore row, once the
// restored-from set is gone), the row stays for the historical record but with no actions, just
// a note that it's gone. A set can be locked to opt out of both automatic rotation and manual
// deletion, e.g. right before a risky change.
func RenderBackupLog(entries []backup.LogEntry) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var sb strings.Builder
	sb.WriteString(autoBackupStatusHTML(t))
	fmt.Fprintf(&sb, `<p class="backup-storage">%s</p>`,
		template.HTMLEscapeString(t("Backup storage: %s", backupStorageText(t))))
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

		manifest := rowManifest(e)
		actions := fmt.Sprintf(`<span class="backup-unavailable">%s</span>`, t("no longer available"))
		if e.Available {
			lockVerb, lockLabel := "hx-post", t("Lock")
			if e.Locked {
				lockVerb, lockLabel = "hx-delete", t("Unlock")
			}
			restoreMessage := t("Restore %s? The app restarts to apply it.", e.Set)
			restoreWarning := ""
			if git.RemoteEnabled() && restoreTouchesGit(manifest) {
				restoreWarning = t("The restored state will also be force-pushed to the configured remote, overwriting anything there - or on any other synced device - that this device hasn't seen.")
			}
			deleteBtn := ""
			if !e.Locked {
				deleteMessage := t("Delete backup %s? This cannot be undone.", e.Set)
				deleteBtn = fmt.Sprintf(
					`<button class="btn-danger-icon" hx-delete="/api/system/backups/%s" hx-confirm="%s" hx-target="#backup-list" hx-swap="innerHTML" title="%s"><i class="fa fa-trash"></i></button>`,
					template.HTMLEscapeString(e.Set), template.HTMLEscapeString(deleteMessage), t("Delete"),
				)
			}
			actions = fmt.Sprintf(
				`<button class="btn-secondary" %s="/api/system/backups/%s/lock" hx-target="#backup-list" hx-swap="innerHTML">%s</button>`+
					`<a class="btn-secondary" href="/api/system/backups/%s/download" download>%s</a>`+
					`<button type="button" class="btn-secondary backup-restore-btn" popovertarget="restore-modal" data-url="/api/system/backups/%s/restore" data-message="%s" data-warning="%s">%s</button>`+
					`%s`,
				lockVerb, template.HTMLEscapeString(e.Set), lockLabel,
				template.HTMLEscapeString(e.Set), t("Download"),
				template.HTMLEscapeString(e.Set),
				template.HTMLEscapeString(restoreMessage),
				template.HTMLEscapeString(restoreWarning),
				t("Restore"),
				deleteBtn,
			)
		}

		fmt.Fprintf(&sb, `<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td class="backup-actions">%s</td></tr>`,
			template.HTMLEscapeString(configmanager.FormatDateTimeSeconds(e.Time)),
			template.HTMLEscapeString(event),
			template.HTMLEscapeString(backupSourceLabel(t, e)),
			template.HTMLEscapeString(backupContentsLabel(t, e, manifest)),
			actions,
		)
	}
	sb.WriteString(`</tbody></table>`)
	return sb.String()
}

// RenderBackupSummary renders a compact time + event table, newest first, for the rail "backup"
// content snippet's flyout - the full table's other columns (trigger/contents) and per-row
// actions (restore/lock/download/delete) are too much detail for that small space. showActions
// is reserved for a future compact action row (e.g. restore) and currently always omitted.
func RenderBackupSummary(entries []backup.LogEntry, showActions bool) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var sb strings.Builder
	sb.WriteString(`<style>
.backup-summary-table { width: 100%; border-collapse: collapse; font-size: .85rem; }
.backup-summary-table th { text-align: left; padding: .25rem .5rem; border-bottom: 2px solid var(--border); }
.backup-summary-table td { padding: .2rem .5rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); }
.backup-summary-link { display: inline-block; margin-top: .75rem; font-size: .875rem; }
</style>`)
	sb.WriteString(autoBackupStatusHTML(t))
	fmt.Fprintf(&sb, `<table class="backup-summary-table"><thead><tr><th>%s</th><th>%s</th></tr></thead><tbody>`,
		t("Time"), t("Event"))
	if len(entries) == 0 {
		fmt.Fprintf(&sb, `<tr><td colspan="2" style="text-align:center;color:var(--text-secondary);">%s</td></tr>`, t("No backups yet"))
	}
	for _, e := range entries {
		event := t("Backup created: %s", e.Set)
		if e.Kind == backup.EventRestore {
			event = t("Restored: %s", e.Set)
		}
		fmt.Fprintf(&sb, `<tr><td>%s</td><td>%s</td></tr>`,
			template.HTMLEscapeString(configmanager.FormatDateTimeSeconds(e.Time)),
			template.HTMLEscapeString(event))
	}
	sb.WriteString(`</tbody></table>`)
	fmt.Fprintf(&sb, `<a class="backup-summary-link" href="/system/backup">%s &rarr;</a>`,
		t("full backup history"))
	return sb.String()
}

// renderBackupStorageCheckboxes renders one checkbox per registered storage - checked by default
// for every storage in the default backup set, unchecked for optional ones (e.g. docs/media),
// which stay opt-in. Changing any checkbox from that starting point creates an explicit
// (non-default) backup set instead.
func renderBackupStorageCheckboxes() string {
	defaultNames := job.DefaultStorageNames()
	var sb strings.Builder
	for _, name := range job.RegisteredStorageNames() {
		checked := ""
		if slices.Contains(defaultNames, name) {
			checked = " checked"
		}
		fmt.Fprintf(&sb, `<label class="backup-storage-option"><input type="checkbox" name="storages" value="%s"%s> %s</label>`,
			template.HTMLEscapeString(name), checked, template.HTMLEscapeString(name))
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
.backup-storage { font-size: .8rem; margin: 0 0 .5rem; color: var(--text-secondary); }
.backup-create-form { display: flex; flex-direction: column; gap: .5rem; margin-bottom: .75rem; }
.backup-storage-select { display: flex; flex-wrap: wrap; gap: .1rem 1rem; }
.backup-storage-option { display: flex; align-items: center; gap: .3rem; font-size: .85rem; }
.backup-toolbar { display: flex; align-items: center; gap: .75rem; }
.backup-actions { display: flex; gap: .4rem; justify-content: flex-end; }
.backup-unavailable { color: var(--text-secondary); font-style: italic; }
</style>` +
		fmt.Sprintf(`<p class="backup-note">%s</p>`, t("Each backup set snapshots StoragePath (metadata, chat, kanban, notifications, config, search) by default - not cache, which holds only data rebuilt from files/git on demand. DataPath's docs/media folders are optional: select them below to include them, since they're already covered by git and can make a backup much larger. Storages are snapshotted one at a time, not as a single point-in-time transaction. Automatic backup profiles are configured via KNOV_BACKUP_AUTO_PROFILES, and rotation tuned via KNOV_BACKUP_ROTATION_KEEP_DAYS/KNOV_BACKUP_ROTATION_KEEP_DEFAULT (see .env.example). Lock a set to keep it regardless of rotation.")) +
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
