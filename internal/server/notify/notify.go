// Package notify provides HTMX-compatible toast notification helpers.
//
// Two tracks — handlers pick the right one based on whether the response navigates:
//
//   - SetHeader: in-page response (no navigation). Fires HX-Trigger immediately.
//     Stores notification with pending=false (already displayed).
//
//   - SetFlash: navigation response (HX-Redirect / HX-Refresh). Stores with
//     pending=true. The new page picks it up via a single DOMContentLoaded fetch
//     to GET /api/notifications/flash.
//
// Both write to notificationStorage for the persistent log. A level below
// KNOV_NOTIFY_MIN_LEVEL still persists but shows no toast (see muted).
// JS injection is handled by render.RenderNotificationJS, called by thememanager.
package notify

import (
	"encoding/json"
	"fmt"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/notificationStorage"
)

// Level represents the visual severity of a notification.
type Level string

const (
	LevelSuccess Level = "success"
	LevelError   Level = "error"
	LevelWarning Level = "warning"
	LevelInfo    Level = "info"
)

type payload struct {
	Type    Level  `json:"type"`
	Message string `json:"message"`
}

// severity ranks levels for the KNOV_NOTIFY_MIN_LEVEL gate.
var severity = map[Level]int{LevelInfo: 0, LevelSuccess: 0, LevelWarning: 1, LevelError: 2}

// muted reports whether a notification at level should skip its toast for the
// configured minimum level. It is still written to the persistent log.
func muted(level Level) bool {
	switch configmanager.GetNotifyMinLevel() {
	case "off":
		return true
	case "error":
		return severity[level] < severity[LevelError]
	case "warning":
		return severity[level] < severity[LevelWarning]
	default: // "info" (normalized at startup)
		return false
	}
}

// SetHeader fires an immediate toast via HX-Trigger and persists the notification.
// Use for in-page responses where the user stays on the same page.
func SetHeader(w http.ResponseWriter, level Level, message string) {
	if !muted(level) {
		p := payload{Type: level, Message: message}
		data, err := json.Marshal(map[string]payload{"notify": p})
		if err != nil {
			logging.LogError(logging.KeyApp, "notify: failed to marshal header payload: %v", err)
			return
		}
		w.Header().Set("HX-Trigger", string(data))
	}

	if _, err := notificationStorage.Add(string(level), message, false); err != nil {
		logging.LogError(logging.KeyApp, "notify: failed to persist notification: %v", err)
	}
}

// SetFlash persists a pending notification for display on the next page load.
// Use for navigation responses (HX-Redirect / HX-Refresh) where HX-Trigger
// would be lost before the browser renders the toast.
func SetFlash(level Level, message string) {
	// muted levels persist but are not pending, so no toast fires on the next load.
	if _, err := notificationStorage.Add(string(level), message, !muted(level)); err != nil {
		logging.LogError(logging.KeyApp, "notify: failed to store flash notification: %v", err)
	}
}

// RenderJS returns the HTML snippet (container div + script src)
// injected into every page before </body> by thememanager.
// duration is the toast display time in milliseconds (KNOV_NOTIFY_DURATION).
func RenderJS(duration int) string {
	return fmt.Sprintf(`    <div id="component-notify" data-duration="%d"></div>
    <script src="/static/notify-toast.js"></script>
`, duration)
}
