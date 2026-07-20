// Package render - kanban board HTML rendering
package render

import (
	"fmt"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/kanban"
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
