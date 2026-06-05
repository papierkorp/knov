// Package render - kanban board HTML rendering
package render

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/translation"
)

// KanbanCard holds the data for a single kanban card
type KanbanCard struct {
	FilePath   string
	Title      string
	Collection string
	Status     string
	Tags       []string
	CreatedAt  string
	LastEdited string
}

// RenderKanbanCard renders a single draggable card
func RenderKanbanCard(card KanbanCard) string {
	var html strings.Builder
	prefix := configmanager.GetKanbanPrefix()

	displayTitle := card.Title
	if displayTitle == "" {
		displayTitle = card.FilePath
	}

	// filter out kanban tags from visible tags
	var visibleTags []string
	for _, t := range card.Tags {
		if !configmanager.IsKanbanTag(t) {
			visibleTags = append(visibleTags, t)
		}
	}

	fmt.Fprintf(&html, `<div class="kanban-card" id="kanban-card-%s"
		draggable="true"
		data-filepath="%s"
		data-status="%s"
		data-prefix="%s"
		ondragstart="kanbanDragStart(event)">`,
		sanitizeID(card.FilePath), card.FilePath, card.Status, prefix)

	// title + tag chips on the same row
	html.WriteString(`<div class="kanban-card-header">`)
	fmt.Fprintf(&html, `<a class="kanban-card-title" href="/files/%s">%s</a>`, card.FilePath, displayTitle)
	if len(visibleTags) > 0 {
		html.WriteString(`<div class="kanban-card-tags">`)
		for _, t := range visibleTags {
			fmt.Fprintf(&html, `<span class="kanban-tag" data-tag="%s" onclick="kanbanSetTagFilter(this.dataset.tag)" title="%s">%s</span>`, t, t, t)
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
		fmt.Fprintf(&html, `<span title="%s">%s %s</span>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "created at"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "created at"),
			card.CreatedAt)
	}
	if card.LastEdited != "" {
		fmt.Fprintf(&html, `<span title="%s">%s %s</span>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "last edited"),
			translation.SprintfForRequest(configmanager.GetLanguage(), "last edited"),
			card.LastEdited)
	}
	html.WriteString(`</div>`)

	html.WriteString(`</div>`)
	return html.String()
}

// RenderKanbanColumn renders a single column with its cards
func RenderKanbanColumn(status, label string, cards []KanbanCard) string {
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
func RenderKanbanBoard(columns []struct {
	Status string
	Cards  []KanbanCard
}) string {
	var html strings.Builder
	html.WriteString(`<div class="kanban-board" id="kanban-board">`)
	for _, col := range columns {
		html.WriteString(RenderKanbanColumn(col.Status, col.Status, col.Cards))
	}
	html.WriteString(`</div>`)
	return html.String()
}

// RenderKanbanCollectionSelect renders the collection picker page body
func RenderKanbanCollectionSelect(collections []string) string {
	var html strings.Builder
	html.WriteString(`<div id="page-kanban-select">`)
	fmt.Fprintf(&html, `<h2>%s</h2>`, translation.SprintfForRequest(configmanager.GetLanguage(), "kanban"))
	fmt.Fprintf(&html, `<p>%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "select a collection to open the kanban board"))
	html.WriteString(`<div class="kanban-collection-list">`)
	for _, c := range collections {
		fmt.Fprintf(&html, `<a class="kanban-collection-item" href="/kanban/%s">%s</a>`, c, c)
	}
	if len(collections) == 0 {
		fmt.Fprintf(&html, `<p class="kanban-empty">%s</p>`, translation.SprintfForRequest(configmanager.GetLanguage(), "no collections found"))
	}
	html.WriteString(`</div></div>`)
	return html.String()
}

// sanitizeID makes a file path safe for use as an HTML id attribute
func sanitizeID(path string) string {
	r := strings.NewReplacer("/", "-", ".", "-", " ", "-")
	return r.Replace(path)
}
