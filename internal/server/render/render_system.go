package render

import (
	"embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/job"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/thememanager"
	"knov/internal/translation"
	"knov/internal/version"
)

var logSessionLineRe = regexp.MustCompile(`^=== session started .* ===$`)

// detectLogLevel extracts the "debug/info/warning/error" level from a raw log
// line, so the log viewer's level filter can work on raw file lines the same
// way it does for the structured "Live" table. Covers both line shapes used
// across the app's log files:
//   - app.log (KeyApp):        "<time> <level> [<caller>]: <msg>"
//   - per-key logs:            "<time> <level> [<key>] [<caller>]: <msg>"
//
// The level word always appears within the first few fields, right after the
// timestamp, so scanning a small window avoids false positives from the level
// words appearing later in a message body.
func detectLogLevel(line string) string {
	fields := strings.Fields(line)
	limit := min(len(fields), 5)
	for _, f := range fields[:limit] {
		f = strings.TrimSuffix(strings.Trim(f, "[]"), ":")
		switch f {
		case "debug", "info", "warning", "error":
			return f
		}
	}
	return ""
}

// LogFileLines groups raw log lines into collapsible sections, one per
// "=== session started ===" marker, with only the most recent session expanded.
func LogFileLines(lines []string) string {
	lastSessionIdx := -1
	for i, line := range lines {
		if logSessionLineRe.MatchString(strings.TrimSpace(line)) {
			lastSessionIdx = i
		}
	}

	var sb strings.Builder
	inSession := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if logSessionLineRe.MatchString(trimmed) {
			if inSession {
				sb.WriteString(`</div></details>`)
			}
			openAttr := ""
			if i == lastSessionIdx {
				openAttr = " open"
			}
			fmt.Fprintf(&sb, `<details class="log-session"%s><summary>%s</summary><div class="log-session-lines">`, openAttr, html.EscapeString(trimmed))
			inSession = true
			continue
		}
		class := "log-line"
		if level := detectLogLevel(line); level != "" {
			class += " log-level-" + level
		}
		fmt.Fprintf(&sb, `<div class="%s">`, class)
		sb.WriteString(html.EscapeString(line))
		sb.WriteString(`</div>`)
	}
	if inSession {
		sb.WriteString(`</div></details>`)
	}
	return sb.String()
}

// logMessageRe splits the "[<caller>]: <message>" tail common to every log
// line shape once the timestamp/level/key prefix has been stripped.
var logMessageRe = regexp.MustCompile(`^\[([^\]]*)\]:\s?(.*)$`)

// ParseLogLines parses raw lines from a single key's log file into LogEntry
// values, so files can be merged into one chronological view (the "All"
// option in the file-view dropdown). Session separators and anything whose
// leading timestamp doesn't match the currently configured date/time display
// format are skipped - lines written under a since-changed format won't parse.
func ParseLogLines(key logging.Key, lines []string) []logging.LogEntry {
	var entries []logging.LogEntry
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || logSessionLineRe.MatchString(trimmed) {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 3 {
			continue
		}
		ts := fields[0] + " " + fields[1]

		t, err := configmanager.ParseDateTimeSeconds(ts)
		if err != nil {
			continue
		}

		level := detectLogLevel(trimmed)
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, ts))
		rest = strings.TrimSpace(strings.TrimPrefix(rest, level))
		if key != logging.KeyApp {
			rest = strings.TrimPrefix(rest, "["+key.String()+"] ")
		}

		caller, message := "", rest
		if m := logMessageRe.FindStringSubmatch(rest); m != nil {
			caller, message = m[1], m[2]
		}

		entries = append(entries, logging.LogEntry{
			Time:    t,
			Level:   level,
			Key:     key,
			Caller:  caller,
			Message: message,
		})
	}
	return entries
}

// RenderLogTable renders a structured log table (Time/Level/Source/Caller/
// Message), newest first - shared by the live ring-buffer view and the
// merged "All" log-file view.
func RenderLogTable(entries []logging.LogEntry) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<table class="log-table"><thead><tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>`,
		t("Time"), t("Level"), t("Source"), t("Caller"), t("Message"))
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		fmt.Fprintf(&sb,
			`<tr class="log-level-%s log-key-%s"><td>%s</td><td>%s</td><td>%s</td><td class="log-caller">%s</td><td>%s</td></tr>`,
			html.EscapeString(e.Level),
			html.EscapeString(e.Key.String()),
			html.EscapeString(configmanager.FormatDateTimeSeconds(e.Time)),
			html.EscapeString(e.Level),
			html.EscapeString(e.Key.String()),
			html.EscapeString(e.Caller),
			html.EscapeString(e.Message),
		)
	}
	sb.WriteString(`</tbody></table>`)
	return sb.String()
}

var docsFiles embed.FS

func SetDocsFiles(fs embed.FS) {
	docsFiles = fs
}

func HandleSystemLogs(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	logFiles := logging.GetAllLogFiles()
	hasFile := len(logFiles) > 0

	var keyFilterOptions strings.Builder
	fmt.Fprintf(&keyFilterOptions, `<option value="">%s</option>`, t("all keys"))
	for _, key := range logging.AvailableKeys {
		name := key.String()
		fmt.Fprintf(&keyFilterOptions, `<option value="%s">%s</option>`, template.HTMLEscapeString(name), template.HTMLEscapeString(name))
	}

	fileSelect := ""
	downloadBtn := ""
	if hasFile {
		var sb strings.Builder
		sb.WriteString(`<select id="log-source-select" onchange="onLogSourceChange(this)">`)
		fmt.Fprintf(&sb, `<option value="live">%s</option>`, t("Live"))
		fmt.Fprintf(&sb, `<option value="all">%s</option>`, t("All (merged)"))
		for _, name := range logFiles {
			sb.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, template.HTMLEscapeString(name), template.HTMLEscapeString(name)))
		}
		sb.WriteString(`</select>`)
		fileSelect = sb.String()
		downloadBtn = fmt.Sprintf(`<a id="log-download-link" class="system-logs-download" href="/api/logs/download" style="display:none">%s</a>`, t("Download"))
	}

	content := `<style>
.system-logs { display: flex; flex-direction: column; gap: .75rem; }
.system-logs-toolbar { display: flex; align-items: center; gap: .5rem; flex-wrap: wrap; }
#log-filter { flex: 1; min-width: 160px; max-width: 280px; padding: .3rem .6rem; border: 1px solid var(--border); border-radius: 4px; font-size: .875rem; }
#log-level-filter { padding: .3rem .5rem; border: 1px solid var(--border); border-radius: 4px; font-size: .875rem; }
#log-key-filter { padding: .3rem .5rem; border: 1px solid var(--border); border-radius: 4px; font-size: .875rem; }
#log-source-select { padding: .3rem .5rem; border: 1px solid var(--border); border-radius: 4px; font-size: .875rem; }
.system-logs-download { padding: .3rem .75rem; border: 1px solid var(--border); border-radius: 4px; font-size: .875rem; text-decoration: none; color: inherit; }
.system-logs-download:hover { background: color-mix(in srgb, var(--text) 5%, transparent); }
.log-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.log-table th { text-align: left; padding: .35rem .6rem; border-bottom: 2px solid var(--border); white-space: nowrap; }
.log-table td { padding: .25rem .6rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); vertical-align: top; }
.log-table td:nth-child(1) { white-space: nowrap; }
.log-table td:nth-child(2) { white-space: nowrap; }
.log-table td:nth-child(3) { white-space: nowrap; }
.log-table td:nth-child(5) { word-break: break-word; }
.log-caller { white-space: nowrap; font-size: .75rem; color: var(--text-secondary) !important; }
.log-level-debug td { color: var(--text-secondary); }
.log-level-warning td { background: color-mix(in srgb, var(--warning) 15%, transparent); }
.log-level-warning td:nth-child(2) { color: var(--warning); font-weight: 600; }
.log-level-error td { background: color-mix(in srgb, var(--danger) 15%, transparent); }
.log-level-error td:nth-child(2) { color: var(--danger); font-weight: 600; }
.log-file-lines { font-family: monospace; font-size: .8rem; white-space: pre-wrap; word-break: break-all; display: flex; flex-direction: column; gap: 1px; }
.log-session { border: 1px solid var(--border); border-radius: 4px; margin-bottom: 4px; }
.log-session > summary { cursor: pointer; padding: .3rem .5rem; font-weight: 600; background: color-mix(in srgb, var(--text) 3%, transparent); }
.log-session-lines { display: flex; flex-direction: column; gap: 1px; }
.log-line { padding: .1rem .4rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); }
.log-line:hover { background: color-mix(in srgb, var(--text) 3%, transparent); }
#log-more-area { padding: .5rem 0; display: flex; align-items: center; gap: .75rem; }
.log-line-info { font-size: .8rem; color: var(--text-secondary); }
#log-pause-btn .log-resume-label { display: none; }
#log-pause-btn.active .log-pause-label { display: none; }
#log-pause-btn.active .log-resume-label { display: inline; }
</style>` +
		`<div class="system-logs">` +
		`<div class="system-logs-toolbar">` +
		fmt.Sprintf(`<input id="log-filter" type="search" placeholder="%s" autocomplete="off" oninput="applyLogFilters()">`, t("Filter logs…")) +
		`<select id="log-level-filter" onchange="applyLogFilters()">` +
		fmt.Sprintf(`<option value="">%s</option>`, t("all levels")) +
		fmt.Sprintf(`<option value="debug">%s</option>`, t("debug")) +
		fmt.Sprintf(`<option value="info">%s</option>`, t("info")) +
		fmt.Sprintf(`<option value="warning">%s</option>`, t("warning")) +
		fmt.Sprintf(`<option value="error">%s</option>`, t("error")) +
		`</select>` +
		`<select id="log-key-filter" onchange="applyLogFilters()">` +
		keyFilterOptions.String() +
		`</select>` +
		fmt.Sprintf(`<button class="btn-secondary" onclick="refreshLogs()">%s</button>`, t("Refresh")) +
		fmt.Sprintf(`<button id="log-pause-btn" class="btn-secondary" onclick="toggleLogPolling(this)"><span class="log-pause-label">%s</span><span class="log-resume-label">%s</span></button>`, t("Pause"), t("Resume")) +
		fileSelect +
		downloadBtn +
		`</div>` +
		`<div id="log-entries" hx-get="/api/logs" hx-trigger="load, every 5s" hx-swap="innerHTML"></div>` +
		`</div>` +
		`<script>
var _logPaused = false;
var _logFileView = false;
var _logCurrentFile = '';
var _logCurrentOffset = 0;
var _logLimit = 1000;

document.addEventListener('htmx:beforeRequest', function(e) {
	if (e.target.id === 'log-entries' && (_logPaused || _logFileView)) e.preventDefault();
});

document.addEventListener('htmx:afterSettle', function(e) {
	if (e.target.id === 'log-entries') applyLogFilters();
});

function applyLogFilters() {
	var msgQ   = ((document.getElementById('log-filter')       || {}).value || '').toLowerCase().trim();
	var level  = (document.getElementById('log-level-filter')  || {}).value || '';
	var key    = (document.getElementById('log-key-filter')    || {}).value || '';
	var container = document.getElementById('log-entries');
	if (!container) return;
	var rows = container.querySelectorAll('tbody tr');
	if (rows.length === 0) {
		container.querySelectorAll('.log-line').forEach(function(row) {
			var matchMsg   = msgQ === ''  || row.textContent.toLowerCase().includes(msgQ);
			var matchLevel = level === '' || row.classList.contains('log-level-' + level);
			row.style.display = matchMsg && matchLevel ? '' : 'none';
		});
		return;
	}
	rows.forEach(function(row) {
		var matchMsg   = msgQ === ''  || row.textContent.toLowerCase().includes(msgQ);
		var matchLevel = level === '' || row.classList.contains('log-level-' + level);
		var matchKey   = key === ''   || row.classList.contains('log-key-' + key);
		row.style.display = matchMsg && matchLevel && matchKey ? '' : 'none';
	});
}

function refreshLogs() {
	var sel = document.getElementById('log-source-select');
	var val = sel ? sel.value : 'live';
	if (val !== 'live') {
		_logCurrentOffset = 0;
		htmx.ajax('GET', '/api/logs/file?name=' + encodeURIComponent(val), {target: '#log-entries', swap: 'innerHTML'});
	} else {
		htmx.ajax('GET', '/api/logs', {target: '#log-entries', swap: 'innerHTML'});
	}
}

function toggleLogPolling(btn) {
	_logPaused = !_logPaused;
	btn.classList.toggle('active', _logPaused);
}

function onLogSourceChange(sel) {
	var val = sel.value;
	var pauseBtn = document.getElementById('log-pause-btn');
	var downloadLink = document.getElementById('log-download-link');
	if (val === 'live') {
		_logFileView = false;
		_logPaused = false;
		_logCurrentFile = '';
		_logCurrentOffset = 0;
		if (pauseBtn) pauseBtn.classList.remove('active');
		if (downloadLink) { downloadLink.style.display = 'none'; }
		htmx.ajax('GET', '/api/logs', {target: '#log-entries', swap: 'innerHTML'});
	} else {
		_logFileView = true;
		_logPaused = true;
		_logCurrentFile = val;
		_logCurrentOffset = 0;
		if (pauseBtn) pauseBtn.classList.add('active');
		if (downloadLink) {
			if (val === 'all') {
				downloadLink.style.display = 'none';
			} else {
				downloadLink.href = '/api/logs/download?name=' + encodeURIComponent(val);
				downloadLink.style.display = '';
			}
		}
		htmx.ajax('GET', '/api/logs/file?name=' + encodeURIComponent(val), {target: '#log-entries', swap: 'innerHTML'});
	}
}

function loadMoreLogLines() {
	_logCurrentOffset += _logLimit;
	var url = '/api/logs/file?chunk=true&limit=' + _logLimit + '&offset=' + _logCurrentOffset + '&name=' + encodeURIComponent(_logCurrentFile);
	fetch(url)
		.then(function(resp) {
			var hasMore = resp.headers.get('X-Log-Has-More') === 'true';
			var lineInfo = resp.headers.get('X-Log-Line-Info');
			var moreArea = document.getElementById('log-more-area');
			if (moreArea) moreArea.style.display = hasMore ? '' : 'none';
			var info = document.getElementById('log-line-info');
			if (info && lineInfo) info.textContent = lineInfo;
			return resp.text();
		})
		.then(function(html) {
			var linesEl = document.getElementById('log-file-lines');
			if (linesEl) linesEl.insertAdjacentHTML('afterbegin', html);
			applyLogFilters();
		});
}
</script>`

	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Logs", template.HTML(content)); err != nil {
		logging.LogError(logging.KeyApp, "failed to render logs page: %v", err)
	}
}

// RenderJobsTable returns an HTML table of recent job runs.
func RenderJobsTable(runs []job.JobRun) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<table class="jobs-table"><thead><tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>`,
		t("Job"), t("Started"), t("Finished"), t("Duration"), t("Status"), t("Error"))
	if len(runs) == 0 {
		fmt.Fprintf(&sb, `<tr><td colspan="6" style="text-align:center;color:var(--text-secondary);">%s</td></tr>`, t("No jobs recorded yet"))
	}
	for _, r := range runs {
		duration := ""
		finished := ""
		if r.FinishedAt != nil {
			finished = configmanager.FormatTime(*r.FinishedAt)
			duration = r.FinishedAt.Sub(r.StartedAt).Round(1e6).String()
		}
		statusClass := "job-status-" + string(r.Status)
		sb.WriteString(fmt.Sprintf(
			`<tr class="%s"><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			template.HTMLEscapeString(statusClass),
			template.HTMLEscapeString(r.Name),
			template.HTMLEscapeString(configmanager.FormatTime(r.StartedAt)),
			template.HTMLEscapeString(finished),
			template.HTMLEscapeString(duration),
			template.HTMLEscapeString(string(r.Status)),
			template.HTMLEscapeString(r.Error),
		))
	}
	sb.WriteString(`</tbody></table>`)
	return sb.String()
}

func HandleSystemJobs(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	content := `<style>
.jobs-table { width: 100%; border-collapse: collapse; font-size: .85rem; }
.jobs-table th { text-align: left; padding: .35rem .6rem; border-bottom: 2px solid var(--border); white-space: nowrap; }
.jobs-table td { padding: .28rem .6rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); vertical-align: top; white-space: nowrap; }
.jobs-table td:last-child { white-space: normal; word-break: break-word; color: var(--danger); font-size: .8rem; }
.job-status-running td:nth-child(5) { color: var(--primary); font-weight: 600; }
.job-status-ok td:nth-child(5) { color: var(--success); font-weight: 600; }
.job-status-error td:nth-child(5) { color: var(--danger); font-weight: 600; }
.job-status-error { background: color-mix(in srgb, var(--danger) 15%, transparent); }
.job-status-running { background: color-mix(in srgb, var(--primary) 15%, transparent); }
</style>` +
		fmt.Sprintf(`<div class="jobs-toolbar"><button class="btn-secondary" hx-get="/api/system/jobs" hx-target="#jobs-entries" hx-swap="innerHTML" hx-headers='{"Accept":"text/html"}'>%s</button></div>`, t("Refresh")) +
		`<div id="jobs-entries" hx-get="/api/system/jobs" hx-trigger="load, every 3s" hx-swap="innerHTML" hx-headers='{"Accept":"text/html"}'></div>`

	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Jobs", template.HTML(content)); err != nil {
		logging.LogError(logging.KeyApp, "failed to render jobs page: %v", err)
	}
}

// RenderChangelog concatenates every docs/changelogs/*.md file (newest
// first) into one rendered HTML string - shared by the full /system/changelog
// page and the rail "changelog" content snippet's fragment endpoint.
func RenderChangelog() (string, error) {
	entries, err := docsFiles.ReadDir("docs/changelogs")
	if err != nil {
		return "", err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})

	mdHandler := parser.NewMarkdownHandler()
	var combined strings.Builder

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		data, err := docsFiles.ReadFile("docs/changelogs/" + entry.Name())
		if err != nil {
			logging.LogWarning(logging.KeyApp, "failed to read changelog %s: %v", entry.Name(), err)
			continue
		}

		rendered, err := mdHandler.Render(data, "")
		if err != nil {
			logging.LogWarning(logging.KeyApp, "failed to render changelog %s: %v", entry.Name(), err)
			continue
		}

		combined.Write(rendered)
	}

	return combined.String(), nil
}

func HandleSystemChangelog(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	html, err := RenderChangelog()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(lang, "failed to read changelogs"), http.StatusInternalServerError)
		return
	}

	fileContent := &files.FileContent{
		HTML: html,
		TOC:  parser.GenerateTOC(html),
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileViewTemplateData("Changelog", "system/changelog.md", fileContent)
	data.SystemPage = true
	if err := tm.Render(w, "fileview", data); err != nil {
		logging.LogError(logging.KeyApp, "failed to render changelog page: %v", err)
	}
}

// VersionInfo is the JSON representation of the version/build-info table.
type VersionInfo struct {
	Version           string    `json:"version"`
	BuildTime         time.Time `json:"buildTime"`
	GoVersion         string    `json:"goVersion"`
	OS                string    `json:"os"`
	Arch              string    `json:"arch"`
	LastCommitMessage string    `json:"lastCommitMessage"`
}

// GetVersionInfo returns the version/build-info as a struct - the JSON
// counterpart of RenderVersionInfo.
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:           version.Version,
		BuildTime:         version.BuildTimeParsed,
		GoVersion:         runtime.Version(),
		OS:                runtime.GOOS,
		Arch:              runtime.GOARCH,
		LastCommitMessage: version.LastCommitMessage,
	}
}

// RenderVersionInfo renders the version/build-info table - shared by the
// full /system/version page and the rail "version" content snippet's
// fragment endpoint.
func RenderVersionInfo() string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	row := func(label, value string) string {
		return fmt.Sprintf(`<tr><td class="version-label">%s</td><td class="version-value">%s</td></tr>`,
			template.HTMLEscapeString(label), template.HTMLEscapeString(value))
	}

	return `<style>
.version-table { border-collapse: collapse; font-size: .9rem; min-width: 320px; }
.version-table td { padding: .45rem .75rem; border-bottom: 1px solid var(--border); vertical-align: top; }
.version-label { font-weight: 600; white-space: nowrap; width: 160px; }
.version-value { font-family: monospace; }
.version-changelog-link { display: inline-block; margin-top: 1.25rem; font-size: .875rem; }
</style>` +
		`<table class="version-table"><tbody>` +
		row(t("Version"), version.Version) +
		row(t("Build time"), configmanager.FormatDateTime(version.BuildTimeParsed)) +
		row(t("Go version"), runtime.Version()) +
		row(t("OS / Arch"), runtime.GOOS+"/"+runtime.GOARCH) +
		row(t("Last commit"), version.LastCommitMessage) +
		`</tbody></table>` +
		fmt.Sprintf(`<a class="version-changelog-link" href="/system/changelog">%s &rarr;</a>`, t("Release notes / Changelog"))
}

func HandleSystemVersion(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Version", template.HTML(RenderVersionInfo())); err != nil {
		logging.LogError(logging.KeyApp, "failed to render version page: %v", err)
	}
}
