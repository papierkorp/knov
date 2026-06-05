// Package render - notification rendering
package render

import (
	"fmt"
	"strings"
	"time"

	"knov/internal/notificationStorage"
)

// RenderNotificationList renders a compact notification log for the flyout panel.
// Each row has a delete button that reloads the list on success.
func RenderNotificationList(notifications []notificationStorage.Notification) string {
	if len(notifications) == 0 {
		return `<div id="notifications-list-target"><div class="fp-notify-empty">no notifications yet</div></div>`
	}

	var html strings.Builder
	html.WriteString(`<div id="notifications-list-target">`)
	fmt.Fprintf(&html, `<div class="fp-notify-header-actions">
		<button class="btn-small btn-secondary"
			hx-delete="/api/notifications"
			hx-target="#notifications-list-target"
			hx-swap="innerHTML"
			hx-confirm="%s">%s</button>
	</div>`,
		"clear all notifications?",
		"clear all",
	)
	html.WriteString(`<div class="fp-notify-list">`)
	for _, n := range notifications {
		fmt.Fprintf(&html, `<div class="fp-notify-row fp-notify-%s" id="fp-notify-%s">
			<span class="fp-notify-dot"></span>
			<div class="fp-notify-body">
				<span class="fp-notify-msg">%s</span>
				<span class="fp-notify-time">%s</span>
			</div>
			<button class="fp-notify-delete"
				hx-delete="/api/notifications/%s"
				hx-target="#notifications-list-target"
				hx-swap="innerHTML"
				title="remove">×</button>
		</div>`,
			n.Level, n.ID,
			n.Message,
			formatNotifyTime(n.CreatedAt),
			n.ID,
		)
	}
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)
	return html.String()
}

// RenderNotificationPopover renders a self-contained popover for the builtin theme
// hamburger menu — loads notifications lazily via htmx on open.
func RenderNotificationPopover() string {
	return fmt.Sprintf(`<div id="notifications-popover" popover="auto" class="notifications-popover">
	<div class="notifications-popover-header">
		<span>%s</span>
		<button popovertarget="notifications-popover" popovertargetaction="hide" class="btn-icon">×</button>
	</div>
	<div id="notifications-popover-content"
		hx-get="/api/notifications"
		hx-trigger="toggle[open] once"
		hx-target="this"
		hx-swap="innerHTML">
		<span class="notifications-loading">%s</span>
	</div>
</div>`,
		"Notifications",
		"loading...",
	)
}

func formatNotifyTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return t.Format("02 Jan 15:04")
	}
}
