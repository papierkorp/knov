package render

import (
	"embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
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

// knovRepoURL is linked from the version/build-info table.
const knovRepoURL = "https://github.com/papierkorp/knov"

// logSessionTimeRe matches both session-separator shapes and captures the
// timestamp: the per-key files' "=== session started <ts> ===" and app.log's
// banner middle line "session started <ts>". logRuleLineRe drops app.log's
// "════…" banner rule lines.
var logSessionTimeRe = regexp.MustCompile(`^(?:=== )?session started (.+?)(?: ===)?$`)
var logRuleLineRe = regexp.MustCompile(`^═+$`)

// parseLogTimestamp parses a log-line timestamp. Log files store timestamps in
// a fixed RFC3339 machine format (see logging.formatLogTime), so this never
// depends on the current date/time display settings - reformatting to the
// display style/timezone happens later, at render time.
func parseLogTimestamp(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

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

// logMessageRe splits the "[<caller>]: <message>" tail common to every log
// line shape once the timestamp/level/key prefix has been stripped.
var logMessageRe = regexp.MustCompile(`^\[([^\]]*)\]:\s?(.*)$`)

// ParseLogLines parses raw lines from a single key's log file into LogEntry
// values, so files can be merged into one chronological view (the "All"
// option in the file-view dropdown). Session separators become marker
// entries; a line with no parseable leading timestamp is folded into the
// previous entry as a message continuation.
func ParseLogLines(key logging.Key, lines []string) []logging.LogEntry {
	var entries []logging.LogEntry
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || logRuleLineRe.MatchString(trimmed) {
			continue
		}

		// session separators become full-width marker rows in the single-file
		// table (the merged view drops them - see handleAPIGetLogsFile)
		if m := logSessionTimeRe.FindStringSubmatch(trimmed); m != nil {
			if t, ok := parseLogTimestamp(m[1]); ok {
				entries = append(entries, logging.LogEntry{Time: t, Level: "session", Key: key})
			}
			continue
		}

		var (
			ts string
			t  time.Time
			ok bool
		)
		if fields := strings.Fields(trimmed); len(fields) >= 3 {
			ts = fields[0]
			t, ok = parseLogTimestamp(ts)
		}
		if !ok {
			// no parseable RFC3339 timestamp = continuation of the previous
			// entry (stack traces, pretty JSON) - fold it in rather than drop it
			if n := len(entries); n > 0 && entries[n-1].Level != "session" {
				entries[n-1].Message += "\n" + line
			}
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
		if e.Level == "session" {
			label := t("session started")
			if e.Key != logging.KeyApp {
				label = "[" + e.Key.String() + "] " + label
			}
			fmt.Fprintf(&sb, `<tr class="log-session-row" data-ts="%d"><td colspan="5">%s &middot; %s</td></tr>`,
				e.Time.Unix(), html.EscapeString(label), html.EscapeString(configmanager.FormatDateTimeSeconds(e.Time)))
			continue
		}
		fmt.Fprintf(&sb,
			`<tr class="log-level-%s log-key-%s" data-ts="%d"><td>%s</td><td>%s</td><td>%s</td><td class="log-caller">%s</td><td>%s</td></tr>`,
			html.EscapeString(e.Level),
			html.EscapeString(e.Key.String()),
			e.Time.Unix(),
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

// RawLogLines reconstructs structured entries into monospace log lines (no
// table, no session grouping) - the "raw" view. Rebuilt from parsed fields,
// so it's "raw-styled" rather than byte-for-byte verbatim; folded-in message
// continuations are kept. Each row carries the level/key classes and data-ts
// the client filter needs. Newest first.
func RawLogLines(entries []logging.LogEntry) string {
	var sb strings.Builder
	sb.WriteString(`<div class="log-file-lines">`)
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Level == "session" {
			continue
		}
		class := "log-line"
		if e.Level != "" {
			class += " log-level-" + html.EscapeString(e.Level)
		}
		if e.Key != logging.KeyApp {
			class += " log-key-" + html.EscapeString(e.Key.String())
		}
		line := configmanager.FormatDateTimeSeconds(e.Time) + " " + e.Level
		if e.Key != logging.KeyApp {
			line += " [" + e.Key.String() + "]"
		}
		line += " [" + e.Caller + "]: " + e.Message
		fmt.Fprintf(&sb, `<div class="%s" data-ts="%d">%s</div>`, class, e.Time.Unix(), html.EscapeString(line))
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// RenderLogTableSummary renders a compact time + level + message table, newest first, with a
// refresh button on top - for the rail "logs" content snippet's flyout, where the full table's
// Source/Caller columns are too much detail for that small space (mirrors RenderBackupSummary).
func RenderLogTableSummary(entries []logging.LogEntry) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var sb strings.Builder
	sb.WriteString(`<style>
.log-summary-toolbar { display: flex; justify-content: flex-end; margin-bottom: .4rem; }
.log-summary-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.log-summary-table th { text-align: left; padding: .25rem .5rem; border-bottom: 2px solid var(--border); }
.log-summary-table td { padding: .2rem .5rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); vertical-align: top; }
.log-summary-table td:nth-child(1) { white-space: nowrap; }
.log-summary-table td:nth-child(2) { white-space: nowrap; }
.log-summary-table td:nth-child(3) { word-break: break-word; }
.log-summary-link { display: inline-block; margin-top: .5rem; font-size: .875rem; }
</style>`)
	fmt.Fprintf(&sb, `<div class="log-summary-toolbar"><button class="btn-secondary" hx-get="/api/logs/file?name=all&view=summary" hx-target="closest .flyout-content" hx-swap="innerHTML"><i class="fa fa-rotate"></i> %s</button></div>`,
		t("Refresh"))
	fmt.Fprintf(&sb, `<table class="log-summary-table"><thead><tr><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>`,
		t("Time"), t("Level"), t("Message"))
	if len(entries) == 0 {
		fmt.Fprintf(&sb, `<tr><td colspan="3" style="text-align:center;color:var(--text-secondary);">%s</td></tr>`, t("No logs yet"))
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Level == "session" {
			continue // marker rows have no message - noise in the compact view
		}
		fmt.Fprintf(&sb, `<tr class="log-level-%s"><td>%s</td><td>%s</td><td>%s</td></tr>`,
			html.EscapeString(e.Level),
			html.EscapeString(configmanager.FormatDateTimeSeconds(e.Time)),
			html.EscapeString(e.Level),
			html.EscapeString(e.Message),
		)
	}
	sb.WriteString(`</tbody></table>`)
	fmt.Fprintf(&sb, `<a class="log-summary-link" href="/system/logs">%s &rarr;</a>`, t("open full logs"))
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

	// display-tz offset (seconds east of UTC) so the datetime-local pickers read
	// in that tz, not the browser's. captured once for "now" - a range across a
	// DST boundary skews by the DST delta, fine for a log viewer.
	_, tzOffset := time.Now().In(configmanager.GetTimezone()).Zone()

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
.system-logs-filters { border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent); padding-top: .6rem; }
.system-logs-toolbar input, .system-logs-toolbar select { height: 2rem; padding: 0 .55rem; border: 1px solid var(--border); border-radius: 6px; font-size: .875rem; background: var(--bg); color: var(--text); }
.system-logs-toolbar input:focus, .system-logs-toolbar select:focus { outline: none; border-color: var(--accent); box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 25%, transparent); }
#log-filter { flex: 1; min-width: 160px; max-width: 280px; }
.log-range-group { display: flex; align-items: center; gap: .4rem; height: 2rem; padding: 0 .3rem 0 .55rem; border: 1px solid var(--border); border-radius: 6px; background: color-mix(in srgb, var(--text) 3%, transparent); }
.log-range-group > i { color: var(--text-secondary); font-size: .8rem; }
.log-range { display: flex; align-items: center; gap: .35rem; font-size: .78rem; color: var(--text-secondary); }
.log-range input { height: 1.6rem; padding: 0 .35rem; border-color: transparent; }
.log-range-sep { color: var(--text-secondary); }
.log-range-clear { display: flex; align-items: center; justify-content: center; width: 1.5rem; height: 1.5rem; padding: 0; border: none; border-radius: 4px; background: transparent; color: var(--text-secondary); cursor: pointer; }
.log-range-clear:hover { background: color-mix(in srgb, var(--text) 8%, transparent); color: var(--text); }
.system-logs-download { padding: .3rem .75rem; border: 1px solid var(--border); border-radius: 4px; font-size: .875rem; text-decoration: none; color: inherit; }
.system-logs-download:hover { background: color-mix(in srgb, var(--text) 5%, transparent); }
.log-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.log-table th { text-align: left; padding: .35rem .6rem; border-bottom: 2px solid var(--border); white-space: nowrap; }
.log-table td { padding: .25rem .6rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); vertical-align: top; }
.log-table td:nth-child(1) { white-space: nowrap; }
.log-table td:nth-child(2) { white-space: nowrap; }
.log-table td:nth-child(3) { white-space: nowrap; }
.log-table td:nth-child(5) { word-break: break-word; white-space: pre-wrap; }
.log-table tr.log-session-row td { padding: .85rem .6rem .4rem; background: color-mix(in srgb, var(--text) 6%, transparent); border-top: 2px solid var(--border); border-bottom: none; text-align: center; font-weight: 600; font-size: .75rem; letter-spacing: .04em; text-transform: uppercase; color: var(--text-secondary); }
.log-caller { white-space: nowrap; font-size: .75rem; color: var(--text-secondary) !important; }
.log-level-debug td { color: var(--text-secondary); }
.log-level-warning td { background: color-mix(in srgb, var(--warning) 15%, transparent); }
.log-level-warning td:nth-child(2) { color: var(--warning); font-weight: 600; }
.log-level-error td { background: color-mix(in srgb, var(--danger) 15%, transparent); }
.log-level-error td:nth-child(2) { color: var(--danger); font-weight: 600; }
.log-file-lines { font-family: ui-monospace, monospace; font-size: .8rem; line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere; tab-size: 4; display: flex; flex-direction: column; }
.log-line { padding: .15rem .55rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 35%, transparent); }
.log-line:nth-child(even) { background: color-mix(in srgb, var(--text) 3%, transparent); }
.log-line.log-level-debug { color: var(--text-secondary); }
.log-line.log-level-warning { background: color-mix(in srgb, var(--warning) 15%, transparent); box-shadow: inset 3px 0 0 var(--warning); }
.log-line.log-level-error { background: color-mix(in srgb, var(--danger) 15%, transparent); box-shadow: inset 3px 0 0 var(--danger); }
.log-line:hover { background: color-mix(in srgb, var(--text) 7%, transparent); }
#log-view-toggle.active { background: var(--primary); color: var(--bg); }
#log-pause-btn .log-resume-label { display: none; }
#log-pause-btn.active .log-pause-label { display: none; }
#log-pause-btn.active .log-resume-label { display: inline; }
</style>` +
		`<div class="system-logs">` +
		`<div class="system-logs-toolbar">` +
		fileSelect +
		fmt.Sprintf(`<button class="btn-secondary" onclick="refreshLogs()">%s</button>`, t("Refresh")) +
		fmt.Sprintf(`<button id="log-pause-btn" class="btn-secondary" onclick="toggleLogPolling(this)"><span class="log-pause-label">%s</span><span class="log-resume-label">%s</span></button>`, t("Pause"), t("Resume")) +
		fmt.Sprintf(`<button id="log-view-toggle" class="btn-secondary" onclick="toggleLogRaw(this)" title="%s">%s</button>`, t("Toggle raw / table view"), t("Raw")) +
		downloadBtn +
		`</div>` +
		`<div class="system-logs-toolbar system-logs-filters">` +
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
		fmt.Sprintf(`<span class="log-range-group"><i class="fa fa-clock"></i><label class="log-range"><span>%s</span><input id="log-from" type="datetime-local" step="1" onchange="onLogRangeChange()"></label><span class="log-range-sep">&ndash;</span><label class="log-range"><span>%s</span><input id="log-to" type="datetime-local" step="1" onchange="onLogRangeChange()"></label><button type="button" class="log-range-clear" title="%s" onclick="clearLogRange()"><i class="fa fa-xmark"></i></button></span>`, t("from"), t("to"), t("Clear range")) +
		`</div>` +
		`<div id="log-entries" hx-get="/api/logs" hx-vals='js:{"raw": _logRaw ? "true" : ""}' hx-trigger="load, poll-tick" hx-swap="innerHTML"></div>` +
		`</div>` +
		`<script>
var _logPaused = false;
var _logFileView = false;
var _logRaw = false;
var _logCurrentFile = '';
var _logTzOffsetSec = ` + strconv.Itoa(tzOffset) + `;

// htmx 4 dropped declarative trigger filters, so a plain interval fires
// "poll-tick" on #log-entries - but only for the live view, and not while paused
function shouldPollLive() { return !_logPaused && !_logFileView; }
setInterval(function() { if (shouldPollLive()) htmx.trigger('#log-entries', 'poll-tick'); }, 5000);

// datetime-local value (wall-clock in the display tz) -> unix seconds, so
// range compares match each row's data-ts regardless of the browser tz
function pickerToUnix(v) {
	if (!v) return null;
	if (v.length === 16) v += ':00'; // datetime-local omits :00 seconds
	return Math.floor(Date.parse(v + 'Z') / 1000) - _logTzOffsetSec;
}

document.addEventListener('htmx:after:settle', function(e) {
	if (e.target.id === 'log-entries') applyLogFilters();
});

function applyLogFilters() {
	var msgQ   = ((document.getElementById('log-filter')       || {}).value || '').toLowerCase().trim();
	var level  = (document.getElementById('log-level-filter')  || {}).value || '';
	var key    = (document.getElementById('log-key-filter')    || {}).value || '';
	var fromU  = pickerToUnix((document.getElementById('log-from') || {}).value || '');
	var toU    = pickerToUnix((document.getElementById('log-to')   || {}).value || '');
	var fromTs = fromU === null ? -Infinity : fromU;
	var toTs   = toU   === null ?  Infinity : toU;
	var container = document.getElementById('log-entries');
	if (!container) return;
	var rows = container.querySelectorAll('tbody tr');
	if (rows.length === 0) {
		container.querySelectorAll('.log-line').forEach(function(row) {
			var matchMsg   = msgQ === ''  || row.textContent.toLowerCase().includes(msgQ);
			var matchLevel = level === '' || row.classList.contains('log-level-' + level);
			// app.log raw lines carry no log-key- class (they are all KeyApp),
			// so a key filter must not hide them
			var hasKey     = row.className.indexOf('log-key-') !== -1;
			var matchKey   = key === '' || !hasKey || row.classList.contains('log-key-' + key);
			var ts         = row.dataset.ts ? parseInt(row.dataset.ts, 10) : null;
			var matchTime  = ts === null || (ts >= fromTs && ts <= toTs);
			row.style.display = matchMsg && matchLevel && matchKey && matchTime ? '' : 'none';
		});
		return;
	}
	rows.forEach(function(row) {
		if (row.classList.contains('log-session-row')) {
			// marker rows carry only a timestamp - honour the range, ignore the rest
			var sts = parseInt(row.dataset.ts || '0', 10);
			row.style.display = (sts >= fromTs && sts <= toTs) ? '' : 'none';
			return;
		}
		var matchMsg   = msgQ === ''  || row.textContent.toLowerCase().includes(msgQ);
		var matchLevel = level === '' || row.classList.contains('log-level-' + level);
		var matchKey   = key === ''   || row.classList.contains('log-key-' + key);
		var ts         = parseInt(row.dataset.ts || '0', 10);
		var matchTime  = ts >= fromTs && ts <= toTs;
		row.style.display = matchMsg && matchLevel && matchKey && matchTime ? '' : 'none';
	});
}

// from/to inputs as unix-seconds query params, so a file view fetches exactly
// the wanted window - how you reach entries older than a plain load returns.
function logRangeParams() {
	var p = [];
	var f  = pickerToUnix((document.getElementById('log-from') || {}).value || '');
	var tt = pickerToUnix((document.getElementById('log-to')   || {}).value || '');
	if (f  !== null) p.push('from=' + f);
	if (tt !== null) p.push('to='   + tt);
	return p;
}

function logFileURL() {
	var p = ['name=' + encodeURIComponent(_logCurrentFile)];
	if (_logRaw) p.push('raw=true');
	return '/api/logs/file?' + p.concat(logRangeParams()).join('&');
}

function liveURL() {
	return '/api/logs' + (_logRaw ? '?raw=true' : '');
}

function reloadLogSource() {
	htmx.ajax('GET', _logFileView ? logFileURL() : liveURL(), {target: '#log-entries', swap: 'innerHTML'});
}

function onLogRangeChange() {
	applyLogFilters();
	if (_logFileView) reloadLogSource();
}

function toggleLogRaw(btn) {
	_logRaw = !_logRaw;
	btn.classList.toggle('active', _logRaw);
	reloadLogSource();
}

function clearLogRange() {
	var f = document.getElementById('log-from'); if (f) f.value = '';
	var tEl = document.getElementById('log-to'); if (tEl) tEl.value = '';
	onLogRangeChange();
}

function refreshLogs() {
	reloadLogSource();
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
		if (pauseBtn) pauseBtn.classList.remove('active');
		if (downloadLink) { downloadLink.style.display = 'none'; }
	} else {
		_logFileView = true;
		_logPaused = true;
		_logCurrentFile = val;
		if (pauseBtn) pauseBtn.classList.add('active');
		if (downloadLink) {
			if (val === 'all') {
				downloadLink.style.display = 'none';
			} else {
				downloadLink.href = '/api/logs/download?name=' + encodeURIComponent(val);
				downloadLink.style.display = '';
			}
		}
	}
	reloadLogSource();
}
</script>`

	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Logs", template.HTML(content)); err != nil {
		logging.LogError(logging.KeyApp, "failed to render logs page: %v", err)
	}
}

// RenderJobsTable returns an HTML table of recent job runs. sortKey/sortDir mark
// the active column so its header shows an arrow and clicking it toggles direction.
func RenderJobsTable(runs []job.JobRun, sortKey, sortDir string) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var sb strings.Builder
	sb.WriteString(`<table class="jobs-table"><thead><tr>`)
	for _, c := range []struct{ key, label string }{
		{"job", t("Job")}, {"started", t("Started")}, {"finished", t("Finished")},
		{"duration", t("Duration")}, {"status", t("Status")},
	} {
		nextDir, arrow := "asc", ""
		if c.key == sortKey {
			if sortDir == "asc" {
				nextDir, arrow = "desc", " ▲"
			} else {
				arrow = " ▼"
			}
		}
		fmt.Fprintf(&sb, `<th><a href="#" hx-get="/api/system/jobs?sort=%s&dir=%s" hx-target="#jobs-entries" hx-swap="innerHTML" hx-headers='{"Accept":"text/html"}'>%s%s</a></th>`,
			c.key, nextDir, template.HTMLEscapeString(c.label), arrow)
	}
	fmt.Fprintf(&sb, `<th>%s</th></tr></thead><tbody>`, t("Error"))
	if len(runs) == 0 {
		fmt.Fprintf(&sb, `<tr><td colspan="6" style="text-align:center;color:var(--text-secondary);">%s</td></tr>`, t("No jobs recorded yet"))
	}
	for _, r := range runs {
		duration := ""
		finished := ""
		if r.FinishedAt != nil {
			finished = configmanager.FormatTime(*r.FinishedAt)
			duration = job.RunDuration(r).Round(1e6).String()
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
	// carried by the auto-refresh poll's hx-include so the chosen sort survives it
	fmt.Fprintf(&sb, `<input type="hidden" name="sort" value="%s"><input type="hidden" name="dir" value="%s">`,
		template.HTMLEscapeString(sortKey), template.HTMLEscapeString(sortDir))
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
.jobs-table th a { color: inherit; text-decoration: none; cursor: pointer; }
.jobs-table th a:hover { color: var(--primary); }
.jobs-table td { padding: .28rem .6rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); vertical-align: top; white-space: nowrap; }
.jobs-table td:last-child { white-space: normal; word-break: break-word; color: var(--danger); font-size: .8rem; }
.job-status-running td:nth-child(5) { color: var(--primary); font-weight: 600; }
.job-status-ok td:nth-child(5) { color: var(--success); font-weight: 600; }
.job-status-error td:nth-child(5) { color: var(--danger); font-weight: 600; }
.job-status-canceled td:nth-child(5) { color: var(--warning); font-weight: 600; }
.job-status-interrupted td:nth-child(5) { color: var(--danger); font-weight: 600; }
.job-status-error { background: color-mix(in srgb, var(--danger) 15%, transparent); }
.job-status-canceled { background: color-mix(in srgb, var(--warning) 15%, transparent); }
.job-status-interrupted { background: color-mix(in srgb, var(--danger) 15%, transparent); }
.job-status-running { background: color-mix(in srgb, var(--primary) 15%, transparent); }
</style>` +
		fmt.Sprintf(`<div class="jobs-toolbar"><button class="btn-secondary" hx-get="/api/system/jobs" hx-target="#jobs-entries" hx-include="#jobs-entries input[type=hidden]" hx-swap="innerHTML" hx-headers='{"Accept":"text/html"}'>%s</button></div>`, t("Refresh")) +
		`<div id="jobs-entries" hx-get="/api/system/jobs" hx-trigger="load, every 3s" hx-include="#jobs-entries input[type=hidden]" hx-swap="innerHTML" hx-headers='{"Accept":"text/html"}'></div>`

	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Jobs", template.HTML(content)); err != nil {
		logging.LogError(logging.KeyApp, "failed to render jobs page: %v", err)
	}
}

// RenderChangelog concatenates the full changelog history
// (docs/changelogs/<year>.md, newest year first) into one rendered HTML string -
// shared by the /system/changelog page and the rail "changelog" fragment. It has
// no README or release-notes fallback; with no changelog files it returns a
// placeholder.
func RenderChangelog() (string, []parser.TOCItem) {
	if html, toc := renderDocsMarkdown("docs/changelogs", func(a, b string) bool { return a > b }, make(map[string]int)); html != "" {
		return html, toc
	}
	return `<p class="no-changelog">` + translation.SprintfForRequest(configmanager.GetLanguage(), "no changelog available") + `</p>`, nil
}

// RenderRelease renders the static release-information preamble (docs/release.md)
// followed by the version/build info table and the curated end-user release
// notes (docs/releases/*.md, newest first) - shared by the /system/release page
// and the rail "release" fragment. The preamble shares its heading-id dedup map
// with the release notes that follow (see renderDocsMarkdown) so a heading
// repeated between docs/release.md and a release note still gets a unique id.
//
// from and to select the "upgrade path" tool's version range: when both are
// set to known versions (see ReleaseVersions) only the release notes between
// them (inclusive) are rendered; otherwise the full history is rendered, same
// as before the tool existed.
func RenderRelease(from, to string) (string, []parser.TOCItem) {
	usedIDs := make(map[string]int)

	var out strings.Builder
	var toc []parser.TOCItem
	if data, err := docsFiles.ReadFile("docs/release.md"); err == nil {
		if rendered, headings, err := parser.NewMarkdownHandler().RenderWithUsedIDs(data, parser.PathlessRender, false, usedIDs); err == nil {
			out.Write(rendered)
			toc = parser.HeadingsToTOC(headings)
		}
	}
	out.WriteString(RenderVersionInfo(false))
	versions := ReleaseVersions()
	if len(versions) > 1 {
		from, to = normalizeReleaseRange(versions, from, to)
		out.WriteString(renderReleaseRangeForm(versions, from, to))
	}
	html, releasesTOC := renderDocsMarkdownFiltered("docs/releases", releaseBefore, releaseRangeFilter(versions, from, to), usedIDs)
	out.WriteString(html)
	toc = append(toc, releasesTOC...)
	return out.String(), toc
}

// ReleaseVersions returns every version with a curated release-notes file
// (docs/releases/vX.Y.Z.md), ascending - the options for the /system/release
// "upgrade path" tool's from/to selects. "unreleased" is excluded since it
// isn't a cut version yet.
func ReleaseVersions() []string {
	var names []string
	if entries, err := docsFiles.ReadDir("docs/releases"); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || name == "unreleased.md" || !strings.HasSuffix(name, ".md") {
				continue
			}
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(name, "v"), ".md"))
		}
	}
	sort.Slice(names, func(i, j int) bool { return semverKey("v"+names[i]+".md") < semverKey("v"+names[j]+".md") })
	return names
}

// normalizeReleaseRange reorders from/to ascending by version so the
// rendered picker always agrees with releaseRangeFilter's range, even if the
// request had them swapped. Values that aren't both known versions are
// returned unchanged (releaseRangeFilter then falls back to full history).
func normalizeReleaseRange(versions []string, from, to string) (string, string) {
	if !slices.Contains(versions, from) || !slices.Contains(versions, to) {
		return from, to
	}
	if semverKey("v"+from+".md") > semverKey("v"+to+".md") {
		return to, from
	}
	return from, to
}

// releaseRangeFilter returns a renderDocsMarkdownFiltered predicate keeping
// only release-notes files between from and to (inclusive). It keeps
// everything when from or to is empty or unknown, so an absent/bad range
// falls back to the full history instead of rendering nothing.
func releaseRangeFilter(versions []string, from, to string) func(name string) bool {
	if from == "" || to == "" || !slices.Contains(versions, from) || !slices.Contains(versions, to) {
		return nil
	}
	lo, hi := semverKey("v"+from+".md"), semverKey("v"+to+".md")
	return func(name string) bool {
		if name == "unreleased.md" {
			return false
		}
		k := semverKey(name)
		return k >= lo && k <= hi
	}
}

// renderReleaseRangeForm renders the "upgrade path" from/to version picker.
// It's a plain GET form back to /system/release (like the file
// version-compare picker in render_git.go) rather than htmx, since a full
// page navigation is simplest for a page that's also embedded in the narrow
// rail panel. Styling lives in #release-range-form (themes/builtin/css/panels.css),
// matching the #component-version-compare picker in render_git.go.
func renderReleaseRangeForm(versions []string, from, to string) string {
	lang := configmanager.GetLanguage()
	var b strings.Builder
	b.WriteString(`<form id="release-range-form" method="get" action="/system/release">`)
	fmt.Fprintf(&b, `<span>%s</span>`, translation.SprintfForRequest(lang, "show changes from"))
	b.WriteString(releaseVersionSelect("from", versions, from, versions[0]))
	fmt.Fprintf(&b, `<span>%s</span>`, translation.SprintfForRequest(lang, "to"))
	b.WriteString(releaseVersionSelect("to", versions, to, versions[len(versions)-1]))
	fmt.Fprintf(&b, `<button type="submit">%s</button></form>`, translation.SprintfForRequest(lang, "show"))
	return b.String()
}

// releaseVersionSelect renders a <select name=name> of every version,
// pre-selecting selected (falling back to fallback when selected is empty or
// unknown).
func releaseVersionSelect(name string, versions []string, selected, fallback string) string {
	if !slices.Contains(versions, selected) {
		selected = fallback
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<select name="%s">`, name)
	for _, v := range versions {
		sel := ""
		if v == selected {
			sel = " selected"
		}
		fmt.Fprintf(&b, `<option value="%s"%s>v%s</option>`, v, sel, v)
	}
	b.WriteString(`</select>`)
	return b.String()
}

// renderDocsMarkdown reads every *.md directly inside the embedded dir, orders
// the names with less, renders each as pathless markdown and concatenates the
// resulting HTML and TOC. usedIDs carries heading-id dedup across the whole
// combined page (threaded across files and, for RenderRelease, across its
// docs/release.md preamble too) so two headings sharing text like "## Added"
// don't collide into the same id; the TOC is built from the headings each
// render call actually used (see parser.HeadingsToTOC), not a second scan, so
// it can't drift from the rendered anchors.
func renderDocsMarkdown(dir string, less func(a, b string) bool, usedIDs map[string]int) (string, []parser.TOCItem) {
	return renderDocsMarkdownFiltered(dir, less, nil, usedIDs)
}

// renderDocsMarkdownFiltered is renderDocsMarkdown with an optional keep
// predicate (nil keeps every file) - used by RenderRelease to restrict the
// "upgrade path" tool's output to a version range.
func renderDocsMarkdownFiltered(dir string, less func(a, b string) bool, keep func(name string) bool, usedIDs map[string]int) (string, []parser.TOCItem) {
	var names []string
	if entries, err := docsFiles.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") && (keep == nil || keep(entry.Name())) {
				names = append(names, entry.Name())
			}
		}
	}
	sort.Slice(names, func(i, j int) bool { return less(names[i], names[j]) })

	mdHandler := parser.NewMarkdownHandler()
	var combined strings.Builder
	var toc []parser.TOCItem
	for _, name := range names {
		data, err := docsFiles.ReadFile(dir + "/" + name)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "failed to read %s/%s: %v", dir, name, err)
			continue
		}

		rendered, headings, err := mdHandler.RenderWithUsedIDs(data, parser.PathlessRender, false, usedIDs)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "failed to render %s/%s: %v", dir, name, err)
			continue
		}

		combined.Write(rendered)
		toc = append(toc, parser.HeadingsToTOC(headings)...)
	}
	return combined.String(), toc
}

// releaseBefore orders release-notes filenames for display: "unreleased.md"
// first, then version files ("v1.2.3.md") by descending semver.
func releaseBefore(a, b string) bool {
	if a == "unreleased.md" || b == "unreleased.md" {
		return a == "unreleased.md" && b != "unreleased.md"
	}
	return semverKey(a) > semverKey(b)
}

// semverKey turns "v1.2.3.md" into a comparable int (major, minor, patch each
// assumed < 1000).
func semverKey(name string) int {
	name = strings.TrimPrefix(strings.TrimSuffix(name, ".md"), "v")
	key := 0
	for _, part := range strings.SplitN(name, ".", 3) {
		n, _ := strconv.Atoi(part)
		key = key*1000 + n
	}
	return key
}

func HandleSystemChangelog(w http.ResponseWriter, r *http.Request) {
	html, toc := RenderChangelog()
	renderSystemMarkdownPage(w, "Changelog", "system/changelog.md", html, toc)
}

func HandleSystemRelease(w http.ResponseWriter, r *http.Request) {
	html, toc := RenderRelease(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	renderSystemMarkdownPage(w, "Release", "system/release.md", html, toc)
}

// renderSystemMarkdownPage wraps pre-rendered HTML in the fileview template so it
// gets a table of contents and the system-page chrome. The TOC (and its rail
// panel) is only attached when the page has more than one heading.
func renderSystemMarkdownPage(w http.ResponseWriter, title, virtualPath, html string, toc []parser.TOCItem) {
	fileContent := &files.FileContent{HTML: html}
	if len(toc) > 1 {
		fileContent.TOC = toc
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileViewTemplateData(title, virtualPath, fileContent)
	data.SystemPage = true
	if err := tm.Render(w, "fileview", data); err != nil {
		logging.LogError(logging.KeyApp, "failed to render %s page: %v", title, err)
	}
}

// VersionInfo is the JSON representation of the version/build-info table.
type VersionInfo struct {
	Version           string    `json:"version"`
	Build             string    `json:"build"`
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
		Build:             version.Build,
		BuildTime:         version.BuildTimeParsed,
		GoVersion:         runtime.Version(),
		OS:                runtime.GOOS,
		Arch:              runtime.GOARCH,
		LastCommitMessage: version.LastCommitMessage,
	}
}

// RenderVersionInfo renders the version/build-info table - shared by the
// full /system/version page and the rail "version" content snippet's
// fragment endpoint. withChangelogLink appends a link to /system/changelog
// (omitted when embedded in the /system/release page, which already lists the
// release notes).
func RenderVersionInfo(withChangelogLink bool) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	row := func(label, value string) string {
		return fmt.Sprintf(`<tr><td class="version-label">%s</td><td class="version-value">%s</td></tr>`,
			template.HTMLEscapeString(label), template.HTMLEscapeString(value))
	}

	out := `<style>
.version-table { border-collapse: collapse; font-size: .9rem; min-width: 320px; }
.version-table td { padding: .45rem .75rem; border-bottom: 1px solid var(--border); vertical-align: top; }
.version-label { font-weight: 600; white-space: nowrap; width: 160px; }
.version-value { font-family: monospace; }
.version-changelog-link { display: inline-block; margin-top: 1.25rem; font-size: .875rem; }
</style>` +
		`<table class="version-table"><tbody>` +
		row(t("Version"), version.Version) +
		row(t("Build"), version.Build) +
		row(t("Build time"), configmanager.FormatDateTime(version.BuildTimeParsed)) +
		row(t("Go version"), runtime.Version()) +
		row(t("OS / Arch"), runtime.GOOS+"/"+runtime.GOARCH) +
		row(t("Last commit"), version.LastCommitMessage) +
		fmt.Sprintf(`<tr><td class="version-label">%s</td><td class="version-value"><a href="%s">%s</a></td></tr>`,
			template.HTMLEscapeString(t("Repository")), knovRepoURL, knovRepoURL) +
		`</tbody></table>`
	if withChangelogLink {
		out += fmt.Sprintf(`<a class="version-changelog-link" href="/system/changelog">%s &rarr;</a>`, t("Changelog"))
	}
	return out
}

func HandleSystemVersion(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Version", template.HTML(RenderVersionInfo(true))); err != nil {
		logging.LogError(logging.KeyApp, "failed to render version page: %v", err)
	}
}

// sensitiveMask replaces a set sensitive value (git password/token) so it never leaves the
// server - the environment page/API only ever reveals whether one is configured.
const sensitiveMask = "••••••••"

// EnvVarInfo is one row of the environment page/API - a configmanager.EnvVarDef merged with
// its live current value.
type EnvVarInfo struct {
	Key             string   `json:"key"`
	Category        string   `json:"category"`
	Description     string   `json:"description"`
	AvailableValues []string `json:"availableValues,omitempty"`
	Default         string   `json:"default"`
	Current         string   `json:"current"`
	Sensitive       bool     `json:"sensitive"`
}

// GetEnvironmentInfo returns every documented KNOV_* env var with its live current value -
// the JSON counterpart of RenderEnvironmentTable. A Sensitive var's Current is masked when
// set, never the raw secret.
func GetEnvironmentInfo() []EnvVarInfo {
	current := configmanager.CurrentEnvValues()

	infos := make([]EnvVarInfo, 0, len(configmanager.EnvVarDefs))
	for _, def := range configmanager.EnvVarDefs {
		value := current[def.Key]
		if def.Sensitive && value != "" {
			value = sensitiveMask
		}
		infos = append(infos, EnvVarInfo{
			Key:             def.Key,
			Category:        def.Category,
			Description:     def.Description,
			AvailableValues: def.AvailableValues,
			Default:         def.Default,
			Current:         value,
			Sensitive:       def.Sensitive,
		})
	}
	return infos
}

// RenderEnvironmentTable renders every documented KNOV_* env var, grouped by category, with
// its description, default and current value - shared by the full /system/environment page
// and the admin panel's environment section, which loads this same fragment via htmx so the
// two never drift apart.
func RenderEnvironmentTable() string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	var sb strings.Builder
	sb.WriteString(`<style>
.env-table-wrap { display: flex; flex-direction: column; gap: 1.5rem; }
.env-table-wrap h3 { margin: 0 0 .35rem; font-size: .95rem; text-transform: capitalize; }
.env-category-desc { color: var(--text-secondary); font-size: .8rem; margin: 0 0 .5rem; white-space: pre-line; }
.env-table { width: 100%; border-collapse: collapse; font-size: .85rem; }
.env-table th { text-align: left; padding: .35rem .6rem; border-bottom: 2px solid var(--border); white-space: nowrap; }
.env-table td { padding: .3rem .6rem; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); vertical-align: top; }
.env-table td:first-child { font-family: monospace; white-space: nowrap; }
.env-table td:nth-child(2) { color: var(--text-secondary); white-space: pre-line; }
.env-table td:nth-child(3) { color: var(--text-secondary); font-size: .8rem; }
.env-table td:nth-child(4), .env-table td:nth-child(5) { font-family: monospace; white-space: pre-line; word-break: break-word; }
.env-current-unset { color: var(--text-secondary); font-style: italic; }
</style>`)
	sb.WriteString(`<div class="env-table-wrap">`)

	category := ""
	for _, info := range GetEnvironmentInfo() {
		if info.Category != category {
			if category != "" {
				sb.WriteString(`</tbody></table></div>`)
			}
			category = info.Category
			fmt.Fprintf(&sb, `<div><h3>%s</h3>`, template.HTMLEscapeString(t(category)))
			if desc := configmanager.EnvCategoryDescriptions[category]; desc != "" {
				fmt.Fprintf(&sb, `<p class="env-category-desc">%s</p>`, template.HTMLEscapeString(t(desc)))
			}
			fmt.Fprintf(&sb, `<table class="env-table"><thead><tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>`,
				t("Variable"), t("Description"), t("Options"), t("Current Value"), t("Default"))
		}

		current := fmt.Sprintf(`<span class="env-current-unset">%s</span>`, t("(default)"))
		if info.Current != "" {
			current = template.HTMLEscapeString(info.Current)
		}

		options := "-"
		if len(info.AvailableValues) > 0 {
			options = template.HTMLEscapeString(strings.Join(info.AvailableValues, ", "))
		}

		fmt.Fprintf(&sb, `<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			template.HTMLEscapeString(info.Key),
			template.HTMLEscapeString(t(info.Description)),
			options,
			current,
			template.HTMLEscapeString(info.Default),
		)
	}
	if category != "" {
		sb.WriteString(`</tbody></table></div>`)
	}
	sb.WriteString(`</div>`)

	return sb.String()
}

// RenderEnvironmentSummary renders a compact "KEY: value" line per documented KNOV_* env var,
// no grouping/description/options - used by the admin panel's environment section and the
// rail "environment" content snippet's flyout, both of which link to the full
// /system/environment page (RenderEnvironmentTable) for details.
func RenderEnvironmentSummary() string {
	lang := configmanager.GetLanguage()
	var sb strings.Builder
	sb.WriteString(`<style>
.env-summary { display: flex; flex-direction: column; gap: .15rem; }
.env-summary code { font-family: monospace; }
.env-summary-link { display: inline-block; margin-top: .75rem; font-size: .875rem; }
</style>`)
	sb.WriteString(`<div class="env-summary">`)
	for _, info := range GetEnvironmentInfo() {
		fmt.Fprintf(&sb, `<div class="help-text"><code>%s</code>: <code>%s</code></div>`,
			template.HTMLEscapeString(info.Key), template.HTMLEscapeString(info.Current))
	}
	sb.WriteString(`</div>`)
	fmt.Fprintf(&sb, `<a class="env-summary-link" href="/system/environment">%s &rarr;</a>`,
		translation.SprintfForRequest(lang, "full list of environment variables"))
	return sb.String()
}

func HandleSystemEnvironment(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Environment", template.HTML(RenderEnvironmentTable())); err != nil {
		logging.LogError(logging.KeyApp, "failed to render environment page: %v", err)
	}
}
