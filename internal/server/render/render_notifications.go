// Package render - notification rendering
package render

import (
	"fmt"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/notificationStorage"
	"knov/internal/translation"
)

// RenderNotificationList renders a compact notification log for the flyout panel.
// Each row has a delete button that reloads the list on success.
func RenderNotificationList(notifications []notificationStorage.Notification) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}

	if len(notifications) == 0 {
		return fmt.Sprintf(`<div id="notifications-list-target"><div class="fp-notify-empty">%s</div></div>`, t("no notifications yet"))
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
		t("clear all notifications?"),
		t("clear all"),
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
				title="%s">×</button>
		</div>`,
			n.Level, n.ID,
			n.Message,
			formatNotifyTime(n.CreatedAt, t),
			n.ID,
			t("remove"),
		)
	}
	html.WriteString(`</div>`)
	html.WriteString(`</div>`)
	return html.String()
}

// RenderNotificationPopover renders a self-contained popover for the builtin theme
// hamburger menu — loads notifications lazily via htmx on open.
func RenderNotificationPopover() string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}
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
		t("Notifications"),
		t("loading..."),
	)
}

func formatNotifyTime(tm time.Time, t func(string, ...any) string) string {
	now := time.Now()
	diff := now.Sub(tm)
	switch {
	case diff < time.Minute:
		return t("just now")
	case diff < time.Hour:
		return t("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return t("%dh ago", int(diff.Hours()))
	default:
		return tm.In(configmanager.GetTimezone()).Format("02 Jan 15:04")
	}
}
