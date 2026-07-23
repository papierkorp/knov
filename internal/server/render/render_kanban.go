// Package render - kanban board HTML rendering
package render

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/kanban"
	"knov/internal/kanbanStorage"
	"knov/internal/translation"
)

// RenderKanbanCard renders a single draggable card
func RenderKanbanCard(card kanban.Card) string {
	var html strings.Builder
	prefix := configmanager.GetKanbanPrefix()

	displayTitle := card.Title
	if displayTitle == "" {
		displayTitle = card.FilePath
	}

	// remove collection prefix from title (e.g. "mycollection/My Title" -> "My Title")
	if card.Collection != "" {
		displayTitle = strings.TrimPrefix(displayTitle, card.Collection+"/")
	}

	// filter out kanban tags from visible tags
	var visibleTags []string
	for _, t := range card.Tags {
		if !configmanager.IsKanbanTag(t) {
			visibleTags = append(visibleTags, t)
		}
	}

	cardClass := "kanban-card"
	if style := configmanager.GetKanbanCardStyles()[card.Status]; style != "" && style != "normal" {
		cardClass += " kanban-card--" + style
	}

	fmt.Fprintf(&html, `<div class="%s" id="kanban-card-%s"
		draggable="true"
		data-filepath="%s"
		data-status="%s"
		data-prefix="%s"
		ondragstart="kanbanDragStart(event)">`,
		cardClass, sanitizeID(card.FilePath), card.FilePath, card.Status, prefix)

	// title + tag chips on the same row
	html.WriteString(`<div class="kanban-card-header">`)
	fmt.Fprintf(&html, `<a class="kanban-card-title" href="/files/%s" title="%s">%s</a>`, card.FilePath, displayTitle, displayTitle)
	if len(visibleTags) > 0 {
		tagColors := configmanager.GetKanbanTagColors()
		html.WriteString(`<div class="kanban-card-tags">`)
		for _, t := range visibleTags {
			style := ""
			if color, ok := tagColors[t]; ok {
				style = fmt.Sprintf(` style="background-color:%s;border-color:%s;"`, color, color)
			}
			fmt.Fprintf(&html, `<span class="kanban-tag"%s data-tag="%s" onclick="kanbanSetTagFilter(this.dataset.tag)" title="%s">%s</span>`, style, t, t, t)
		}
		html.WriteString(`</div>`)
	}
	html.WriteString(`</div>`)

	// excerpt — loaded lazily
	fmt.Fprintf(&html, `<div hx-get="/api/kanban/excerpt?filepath=%s&chars=30" hx-trigger="load" hx-swap="outerHTML"></div>`,
		card.FilePath)

	// dates
	html.WriteString(`<div class="kanban-card-meta">`)
	if card.CreatedAt != "" {
		fmt.Fprintf(&html, `<span title="%s">%s: %s</span>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "created at"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "created at"),
			formatCardDate(card.CreatedAt))
	}

	fmt.Fprintf(&html, ` | `)

	if card.LastEdited != "" {
		fmt.Fprintf(&html, `<span title="%s">%s: %s</span>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "last edited"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "last edited"),
			formatCardDate(card.LastEdited))
	}
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	return html.String()
}

// RenderKanbanColumn renders a single column with its cards
func RenderKanbanColumn(status, label string, cards []kanban.Card) string {
	var html strings.Builder

	fmt.Fprintf(&html, `<div class="kanban-column" id="kanban-col-%s"
		data-status="%s"
		ondragover="kanbanDragOver(event)"
		ondragleave="kanbanDragLeave(event)"
		ondrop="kanbanDrop(event)">`, status, status)

	fmt.Fprintf(&html, `<div class="kanban-column-header"><span class="kanban-column-label">%s</span><span class="kanban-column-count">%d</span></div>`,
		label, len(cards))

	html.WriteString(`<div class="kanban-cards">`)
	for _, card := range cards {
		html.WriteString(RenderKanbanCard(card))
	}
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)
	return html.String()
}

// RenderKanbanBoard renders the full board (all columns)
func RenderKanbanBoard(columns []kanban.Column) string {
	var html strings.Builder
	html.WriteString(`<div class="kanban-board" id="kanban-board">`)
	for _, col := range columns {
		html.WriteString(RenderKanbanColumn(col.Status, col.Status, col.Cards))
	}
	html.WriteString(`</div>`)
	return html.String()
}

// RenderKanbanArchive renders the archive view: a search box + tag filter followed by a
// sortable table, mirroring RenderKanbanEvents. Search, the tag filter, and column sorting
// all operate client-side over the already-rendered rows (applyKanbanArchiveFilters /
// sortKanbanArchive in kanban.js), so no extra fetch/JSON round trip is needed.
func RenderKanbanArchive(cards []kanban.Card) string {
	tagSet := make(map[string]struct{})
	for _, c := range cards {
		for _, t := range c.Tags {
			if !configmanager.IsKanbanTag(t) {
				tagSet[t] = struct{}{}
			}
		}
	}
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	slices.Sort(tags)

	lang := configmanager.GetLanguage()

	var html strings.Builder
	html.WriteString(`<div class="kanban-archive-view">`)

	html.WriteString(`<div class="kanban-archive-controls">`)
	fmt.Fprintf(&html, `<input type="search" id="kanban-archive-search" class="kanban-archive-search" placeholder="%s" oninput="applyKanbanArchiveFilters()">`,
		translation.SprintfForRequest(lang, "search archived files..."))
	fmt.Fprintf(&html, `<select id="kanban-archive-tag-filter" class="kanban-archive-tag-filter" onchange="applyKanbanArchiveFilters()"><option value="">%s</option>`,
		translation.SprintfForRequest(lang, "all tags"))
	for _, t := range tags {
		fmt.Fprintf(&html, `<option value="%s">%s</option>`, t, t)
	}
	html.WriteString(`</select>`)
	html.WriteString(`</div>`)

	if len(cards) == 0 {
		fmt.Fprintf(&html, `<p class="kanban-empty">%s</p>`, translation.SprintfForRequest(lang, "no archived files"))
		html.WriteString(`</div>`)
		return html.String()
	}

	titleLabel := translation.SprintfForRequest(lang, "title")
	tagsLabel := translation.SprintfForRequest(lang, "tags")
	createdLabel := translation.SprintfForRequest(lang, "created at")
	editedLabel := translation.SprintfForRequest(lang, "last edited")

	html.WriteString(`<table class="kanban-archive-table">`)
	html.WriteString(`<thead id="kanban-archive-thead"><tr>`)
	fmt.Fprintf(&html, `<th class="kanban-archive-sortable" data-column="title" onclick="sortKanbanArchive('title')">%s</th>`, titleLabel)
	fmt.Fprintf(&html, `<th>%s</th>`, tagsLabel)
	fmt.Fprintf(&html, `<th class="kanban-archive-sortable" data-column="createdat" onclick="sortKanbanArchive('createdat')">%s</th>`, createdLabel)
	fmt.Fprintf(&html, `<th class="kanban-archive-sortable" data-column="lastedited" onclick="sortKanbanArchive('lastedited')">%s</th>`, editedLabel)
	html.WriteString(`</tr></thead>`)

	html.WriteString(`<tbody id="kanban-archive-rows">`)
	for _, card := range cards {
		html.WriteString(renderKanbanArchiveRow(card))
	}
	html.WriteString(`</tbody>`)
	html.WriteString(`</table>`)

	html.WriteString(`</div>`)
	return html.String()
}

// renderKanbanArchiveRow renders a single table row for the archive view. data-search and
// data-tags carry the values applyKanbanArchiveFilters matches against; data-title,
// data-createdat and data-lastedited back sortKanbanArchive — filtering and sorting never
// need to re-fetch or re-render anything.
func renderKanbanArchiveRow(card kanban.Card) string {
	var html strings.Builder

	displayTitle := card.Title
	if displayTitle == "" {
		displayTitle = card.FilePath
	}
	if card.Collection != "" {
		displayTitle = strings.TrimPrefix(displayTitle, card.Collection+"/")
	}

	var visibleTags []string
	for _, t := range card.Tags {
		if !configmanager.IsKanbanTag(t) {
			visibleTags = append(visibleTags, t)
		}
	}

	searchBlob := strings.ToLower(displayTitle + " " + card.FilePath + " " + strings.Join(visibleTags, " "))

	fmt.Fprintf(&html, `<tr data-search="%s" data-tags="|%s|" data-title="%s" data-createdat="%s" data-lastedited="%s">`,
		searchBlob, strings.Join(visibleTags, "|"), strings.ToLower(displayTitle), card.CreatedAt, card.LastEdited)

	fmt.Fprintf(&html, `<td><a class="kanban-archive-title" href="/files/%s" title="%s">%s</a></td>`, card.FilePath, displayTitle, displayTitle)

	html.WriteString(`<td>`)
	if len(visibleTags) > 0 {
		tagColors := configmanager.GetKanbanTagColors()
		html.WriteString(`<div class="kanban-archive-tags">`)
		for _, t := range visibleTags {
			style := ""
			if color, ok := tagColors[t]; ok {
				style = fmt.Sprintf(` style="background-color:%s;border-color:%s;"`, color, color)
			}
			fmt.Fprintf(&html, `<span class="kanban-tag"%s>%s</span>`, style, t)
		}
		html.WriteString(`</div>`)
	}
	html.WriteString(`</td>`)

	fmt.Fprintf(&html, `<td>%s</td>`, formatCardDate(card.CreatedAt))
	fmt.Fprintf(&html, `<td>%s</td>`, formatCardDate(card.LastEdited))

	html.WriteString(`</tr>`)
	return html.String()
}

// RenderKanbanEvents renders the event log view: search + file/from/to filters followed
// by a sortable table. Search and the from/to dropdowns filter the already-rendered rows
// client-side (applyKanbanEventsFilters in kanban.js). The file filter and date range
// trigger a real reload (reloadKanbanEvents) since they change which rows are pulled from
// the log — selecting a specific file also lifts the default recent-events cap so its full
// history shows. fileFilter/dateFrom/dateTo are the currently-applied query values, echoed
// back into the controls so they stay populated across a reload.
func RenderKanbanEvents(events []kanbanStorage.Event, filePaths []string, board, fileFilter, dateFrom, dateTo string) string {
	fromSet := make(map[string]struct{})
	toSet := make(map[string]struct{})
	for _, e := range events {
		if e.FromStatus != "" {
			fromSet[e.FromStatus] = struct{}{}
		}
		toSet[e.ToStatus] = struct{}{}
	}
	fromStatuses := make([]string, 0, len(fromSet))
	for s := range fromSet {
		fromStatuses = append(fromStatuses, s)
	}
	slices.Sort(fromStatuses)
	toStatuses := make([]string, 0, len(toSet))
	for s := range toSet {
		toStatuses = append(toStatuses, s)
	}
	slices.Sort(toStatuses)

	lang := configmanager.GetLanguage()
	all := translation.SprintfForRequest(lang, "all")

	var html strings.Builder
	html.WriteString(`<div class="kanban-events-view">`)

	html.WriteString(`<div class="kanban-events-controls">`)
	fmt.Fprintf(&html, `<input type="search" id="kanban-events-search" class="kanban-events-search" placeholder="%s" oninput="applyKanbanEventsFilters()">`,
		translation.SprintfForRequest(lang, "filter events..."))

	fileLabel := translation.SprintfForRequest(lang, "file")
	fmt.Fprintf(&html, `<select id="kanban-events-file-filter" class="kanban-events-status-filter" title="%s" onchange="reloadKanbanEvents('%s')"><option value="">%s: %s</option>`,
		fileLabel, board, fileLabel, all)
	for _, p := range filePaths {
		selected := ""
		if p == fileFilter {
			selected = ` selected`
		}
		fmt.Fprintf(&html, `<option value="%s" title="%s"%s>%s</option>`, p, p, selected, filepath.Base(p))
	}
	html.WriteString(`</select>`)

	fromLabel := translation.SprintfForRequest(lang, "from")
	fmt.Fprintf(&html, `<select id="kanban-events-from-filter" class="kanban-events-status-filter" title="%s" onchange="applyKanbanEventsFilters()"><option value="">%s: %s</option>`,
		fromLabel, fromLabel, all)
	for _, s := range fromStatuses {
		fmt.Fprintf(&html, `<option value="%s">%s</option>`, s, s)
	}
	html.WriteString(`</select>`)

	toLabel := translation.SprintfForRequest(lang, "to")
	fmt.Fprintf(&html, `<select id="kanban-events-to-filter" class="kanban-events-status-filter" title="%s" onchange="applyKanbanEventsFilters()"><option value="">%s: %s</option>`,
		toLabel, toLabel, all)
	for _, s := range toStatuses {
		fmt.Fprintf(&html, `<option value="%s">%s</option>`, s, s)
	}
	html.WriteString(`</select>`)

	fmt.Fprintf(&html, `<input type="date" id="kanban-events-date-from" class="kanban-events-date" title="%s" value="%s" onchange="reloadKanbanEvents('%s')">`,
		translation.SprintfForRequest(lang, "from date"), dateFrom, board)
	fmt.Fprintf(&html, `<input type="date" id="kanban-events-date-to" class="kanban-events-date" title="%s" value="%s" onchange="reloadKanbanEvents('%s')">`,
		translation.SprintfForRequest(lang, "to date"), dateTo, board)
	html.WriteString(`</div>`)

	if len(events) == 0 {
		fmt.Fprintf(&html, `<p class="kanban-empty">%s</p>`, translation.SprintfForRequest(lang, "no events yet"))
		html.WriteString(`</div>`)
		return html.String()
	}

	html.WriteString(`<table class="kanban-events-table">`)
	html.WriteString(`<thead id="kanban-events-thead"><tr>`)
	fmt.Fprintf(&html, `<th class="kanban-events-sortable" data-column="time" onclick="sortKanbanEvents('time')">%s</th>`, translation.SprintfForRequest(lang, "time"))
	fmt.Fprintf(&html, `<th class="kanban-events-sortable" data-column="file" onclick="sortKanbanEvents('file')">%s</th>`, fileLabel)
	fmt.Fprintf(&html, `<th class="kanban-events-sortable" data-column="from" onclick="sortKanbanEvents('from')">%s</th>`, fromLabel)
	fmt.Fprintf(&html, `<th class="kanban-events-sortable" data-column="to" onclick="sortKanbanEvents('to')">%s</th>`, toLabel)
	html.WriteString(`</tr></thead>`)

	html.WriteString(`<tbody id="kanban-events-rows">`)
	for _, e := range events {
		from := e.FromStatus
		if from == "" {
			from = "—"
		}
		searchBlob := strings.ToLower(e.FilePath + " " + e.FromStatus + " " + e.ToStatus)
		fmt.Fprintf(&html, `<tr data-search="%s" data-time="%s" data-file="%s" data-from="%s" data-to="%s">`,
			searchBlob, e.Timestamp.UTC().Format(time.RFC3339), strings.ToLower(e.FilePath), e.FromStatus, e.ToStatus)
		fmt.Fprintf(&html, `<td>%s</td>`, configmanager.FormatDateTime(e.Timestamp))
		fmt.Fprintf(&html, `<td title="%s">%s</td>`, e.FilePath, filepath.Base(e.FilePath))
		fmt.Fprintf(&html, `<td>%s</td>`, from)
		fmt.Fprintf(&html, `<td>%s</td>`, e.ToStatus)
		html.WriteString(`</tr>`)
	}
	html.WriteString(`</tbody>`)
	html.WriteString(`</table>`)

	html.WriteString(`</div>`)
	return html.String()
}

// RenderKanbanFilterPanel renders the advanced filter form for the kanban toolbar panel
func RenderKanbanFilterPanel(board string) string {
	return RenderFilterForm(FilterFormOpts{
		Context:     FilterFormContextKanban,
		KanbanBoard: board,
	})
}

func sanitizeID(path string) string {
	r := strings.NewReplacer("/", "-", ".", "-", " ", "-")
	return r.Replace(path)
}

// formatCardDate reformats a card's stored ISO date (YYYY-MM-DD) for display using the
// configured date style. Falls back to the raw value if it can't be parsed.
func formatCardDate(isoDate string) string {
	t, err := time.Parse("2006-01-02", isoDate)
	if err != nil {
		return isoDate
	}
	return configmanager.FormatDate(t)
}
