package render

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"sort"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/job"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/thememanager"
	"knov/internal/version"
)

var docsFiles embed.FS

func SetDocsFiles(fs embed.FS) {
	docsFiles = fs
}

func HandleSystemLogs(w http.ResponseWriter, r *http.Request) {
	logFiles := logging.GetAllLogFiles()
	hasFile := len(logFiles) > 0

	sourceSelect := ""
	downloadBtn := ""
	if hasFile {
		var sb strings.Builder
		sb.WriteString(`<select id="log-source-select" onchange="onLogSourceChange(this)">`)
		sb.WriteString(`<option value="live">Live</option>`)
		for _, name := range logFiles {
			sb.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, template.HTMLEscapeString(name), template.HTMLEscapeString(name)))
		}
		sb.WriteString(`</select>`)
		sourceSelect = sb.String()
		downloadBtn = `<a id="log-download-link" class="system-logs-download" href="/api/logs/download" style="display:none">Download</a>`
	}

	content := `<style>
.system-logs { display: flex; flex-direction: column; gap: .75rem; }
.system-logs-toolbar { display: flex; align-items: center; gap: .5rem; flex-wrap: wrap; }
#log-filter { flex: 1; min-width: 160px; max-width: 280px; padding: .3rem .6rem; border: 1px solid #ccc; border-radius: 4px; font-size: .875rem; }
#log-level-filter { padding: .3rem .5rem; border: 1px solid #ccc; border-radius: 4px; font-size: .875rem; }
#log-source-select { padding: .3rem .5rem; border: 1px solid #ccc; border-radius: 4px; font-size: .875rem; }
.system-logs-download { padding: .3rem .75rem; border: 1px solid #ccc; border-radius: 4px; font-size: .875rem; text-decoration: none; color: inherit; }
.system-logs-download:hover { background: rgba(0,0,0,.05); }
.log-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.log-table th { text-align: left; padding: .35rem .6rem; border-bottom: 2px solid #ccc; white-space: nowrap; }
.log-table td { padding: .25rem .6rem; border-bottom: 1px solid #eee; vertical-align: top; }
.log-table td:nth-child(1) { white-space: nowrap; }
.log-table td:nth-child(2) { white-space: nowrap; }
.log-table td:nth-child(4) { word-break: break-word; }
.log-caller { white-space: nowrap; font-size: .75rem; color: var(--text-secondary) !important; }
.log-level-debug td { color: #aaa; }
.log-level-warning td { background: #fffbe6; }
.log-level-warning td:nth-child(2) { color: #b45309; font-weight: 600; }
.log-level-error td { background: #fff1f0; }
.log-level-error td:nth-child(2) { color: #c0392b; font-weight: 600; }
.log-file-lines { font-family: monospace; font-size: .8rem; white-space: pre-wrap; word-break: break-all; display: flex; flex-direction: column; gap: 1px; }
.log-line { padding: .1rem .4rem; border-bottom: 1px solid #f0f0f0; }
.log-line:hover { background: rgba(0,0,0,.03); }
#log-more-area { padding: .5rem 0; display: flex; align-items: center; gap: .75rem; }
.log-line-info { font-size: .8rem; color: var(--text-secondary); }
</style>` +
		`<div class="system-logs">` +
		`<div class="system-logs-toolbar">` +
		`<input id="log-filter" type="search" placeholder="Filter logs…" autocomplete="off" oninput="applyLogFilters()">` +
		`<select id="log-level-filter" onchange="applyLogFilters()">` +
		`<option value="">all levels</option>` +
		`<option value="debug">debug</option>` +
		`<option value="info">info</option>` +
		`<option value="warning">warning</option>` +
		`<option value="error">error</option>` +
		`</select>` +
		`<button class="btn-secondary" onclick="refreshLogs()">Refresh</button>` +
		`<button id="log-pause-btn" class="btn-secondary" onclick="toggleLogPolling(this)">Pause</button>` +
		sourceSelect +
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
	var container = document.getElementById('log-entries');
	if (!container) return;
	var rows = container.querySelectorAll('tbody tr');
	if (rows.length === 0) {
		container.querySelectorAll('.log-line').forEach(function(row) {
			row.style.display = msgQ === '' || row.textContent.toLowerCase().includes(msgQ) ? '' : 'none';
		});
		return;
	}
	rows.forEach(function(row) {
		var matchMsg   = msgQ === ''  || row.textContent.toLowerCase().includes(msgQ);
		var matchLevel = level === '' || row.classList.contains('log-level-' + level);
		row.style.display = matchMsg && matchLevel ? '' : 'none';
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
	btn.textContent = _logPaused ? 'Resume' : 'Pause';
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
		if (pauseBtn) { pauseBtn.textContent = 'Pause'; pauseBtn.classList.remove('active'); }
		if (downloadLink) { downloadLink.style.display = 'none'; }
		htmx.ajax('GET', '/api/logs', {target: '#log-entries', swap: 'innerHTML'});
	} else {
		_logFileView = true;
		_logPaused = true;
		_logCurrentFile = val;
		_logCurrentOffset = 0;
		if (pauseBtn) { pauseBtn.textContent = 'Resume'; pauseBtn.classList.add('active'); }
		if (downloadLink) {
			downloadLink.href = '/api/logs/download?name=' + encodeURIComponent(val);
			downloadLink.style.display = '';
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
			var total = parseInt(resp.headers.get('X-Log-Total-Lines') || '0', 10);
			var moreArea = document.getElementById('log-more-area');
			if (moreArea) moreArea.style.display = hasMore ? '' : 'none';
			var info = document.getElementById('log-line-info');
			if (info && total > 0) {
				var shown = Math.min(_logCurrentOffset + _logLimit, total);
				info.textContent = 'showing last ' + shown + ' of ' + total + ' lines';
			}
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
		logging.LogError("failed to render logs page: %v", err)
	}
}

// RenderJobsTable returns an HTML table of recent job runs.
func RenderJobsTable(runs []job.JobRun) string {
	var sb strings.Builder
	sb.WriteString(`<table class="jobs-table"><thead><tr><th>Job</th><th>Started</th><th>Finished</th><th>Duration</th><th>Status</th><th>Error</th></tr></thead><tbody>`)
	if len(runs) == 0 {
		sb.WriteString(`<tr><td colspan="6" style="text-align:center;color:var(--text-secondary);">No jobs recorded yet</td></tr>`)
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
	content := `<style>
.jobs-table { width: 100%; border-collapse: collapse; font-size: .85rem; }
.jobs-table th { text-align: left; padding: .35rem .6rem; border-bottom: 2px solid #ccc; white-space: nowrap; }
.jobs-table td { padding: .28rem .6rem; border-bottom: 1px solid #eee; vertical-align: top; white-space: nowrap; }
.jobs-table td:last-child { white-space: normal; word-break: break-word; color: #c0392b; font-size: .8rem; }
.job-status-running td:nth-child(5) { color: #2563eb; font-weight: 600; }
.job-status-ok td:nth-child(5) { color: #16a34a; font-weight: 600; }
.job-status-error td:nth-child(5) { color: #c0392b; font-weight: 600; }
.job-status-error { background: #fff1f0; }
.job-status-running { background: #eff6ff; }
</style>` +
		`<div id="jobs-entries" hx-get="/api/system/jobs" hx-trigger="load, every 3s" hx-swap="innerHTML" hx-headers='{"Accept":"text/html"}'></div>`

	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Jobs", template.HTML(content)); err != nil {
		logging.LogError("failed to render jobs page: %v", err)
	}
}

func HandleSystemChangelog(w http.ResponseWriter, r *http.Request) {
	entries, err := docsFiles.ReadDir("docs/changelogs")
	if err != nil {
		http.Error(w, "failed to read changelogs", http.StatusInternalServerError)
		return
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
			logging.LogWarning("failed to read changelog %s: %v", entry.Name(), err)
			continue
		}

		rendered, err := mdHandler.Render(data, "")
		if err != nil {
			logging.LogWarning("failed to render changelog %s: %v", entry.Name(), err)
			continue
		}

		combined.Write(rendered)
	}

	html := combined.String()
	fileContent := &files.FileContent{
		HTML: html,
		TOC:  parser.GenerateTOC(html),
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileViewTemplateData("Changelog", "system/changelog.md", fileContent)
	data.SystemPage = true
	if err := tm.Render(w, "fileview", data); err != nil {
		logging.LogError("failed to render changelog page: %v", err)
	}
}

func HandleSystemVersion(w http.ResponseWriter, r *http.Request) {
	row := func(label, value string) string {
		return fmt.Sprintf(`<tr><td class="version-label">%s</td><td class="version-value">%s</td></tr>`,
			template.HTMLEscapeString(label), template.HTMLEscapeString(value))
	}

	content := `<style>
.version-table { border-collapse: collapse; font-size: .9rem; min-width: 320px; }
.version-table td { padding: .45rem .75rem; border-bottom: 1px solid var(--border-color, #e5e5e5); vertical-align: top; }
.version-label { font-weight: 600; white-space: nowrap; width: 160px; }
.version-value { font-family: monospace; }
.version-changelog-link { display: inline-block; margin-top: 1.25rem; font-size: .875rem; }
</style>` +
		`<table class="version-table"><tbody>` +
		row("Version", version.Version) +
		row("Build time", configmanager.FormatDateTime(version.BuildTimeParsed)) +
		row("Go version", runtime.Version()) +
		row("OS / Arch", runtime.GOOS+"/"+runtime.GOARCH) +
		`</tbody></table>` +
		`<a class="version-changelog-link" href="/system/changelog">Release notes / Changelog &rarr;</a>`

	tm := thememanager.GetThemeManager()
	if err := tm.RenderSystemPage(w, "Version", template.HTML(content)); err != nil {
		logging.LogError("failed to render version page: %v", err)
	}
}
