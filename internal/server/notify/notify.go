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
// Both write to notificationStorage for the persistent log.
// JS injection is handled by render.RenderNotificationJS, called by thememanager.
package notify

import (
	"encoding/json"
	"fmt"
	"net/http"

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

// SetHeader fires an immediate toast via HX-Trigger and persists the notification.
// Use for in-page responses where the user stays on the same page.
func SetHeader(w http.ResponseWriter, level Level, message string) {
	p := payload{Type: level, Message: message}
	data, err := json.Marshal(map[string]payload{"notify": p})
	if err != nil {
		logging.LogError("notify: failed to marshal header payload: %v", err)
		return
	}
	w.Header().Set("HX-Trigger", string(data))

	if _, err := notificationStorage.Add(string(level), message, false); err != nil {
		logging.LogError("notify: failed to persist notification: %v", err)
	}
}

// SetFlash persists a pending notification for display on the next page load.
// Use for navigation responses (HX-Redirect / HX-Refresh) where HX-Trigger
// would be lost before the browser renders the toast.
func SetFlash(level Level, message string) {
	if _, err := notificationStorage.Add(string(level), message, true); err != nil {
		logging.LogError("notify: failed to store flash notification: %v", err)
	}
}

// RenderJS returns the self-contained HTML snippet (container div + script)
// injected into every page before </body> by thememanager.
// duration is the toast display time in milliseconds (KNOV_NOTIFY_DURATION).
func RenderJS(duration int) string {
	return fmt.Sprintf(`    <div id="component-notify"></div>
    <script>
    (function () {
        var container = document.getElementById('component-notify');
        var DURATION = %d;

        function showToast(type, message) {
            var toast = document.createElement('div');
            toast.className = 'notify-toast notify-' + type;
            toast.textContent = message;
            toast.addEventListener('click', function () { dismiss(toast); });
            container.appendChild(toast);
            setTimeout(function () { dismiss(toast); }, DURATION);
        }

        function dismiss(toast) {
            toast.style.animation = 'notify-out 0.2s ease forwards';
            setTimeout(function () {
                if (toast.parentNode) { toast.parentNode.removeChild(toast); }
            }, 200);
        }

        // in-page toasts fired via HX-Trigger from SetHeader
        document.body.addEventListener('notify', function (e) {
            var detail = e.detail;
            if (detail && detail.type && detail.message) {
                showToast(detail.type, detail.message);
            }
        });

        // cross-navigation flash: fetch once on page load
        // no guard needed — script tags do not re-execute on htmx swaps
        fetch('/api/notifications/flash')
            .then(function (r) { return r.ok ? r.json() : null; })
            .then(function (data) {
                if (data && data.level && data.message) {
                    showToast(data.level, data.message);
                }
            })
            .catch(function () {});
    })();
    </script>
`, duration)
}
