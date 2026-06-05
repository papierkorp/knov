// Package notify provides HTMX-compatible toast notification helpers.
// All notifications go through SetFlash → cache storage → /api/notifications/flash poll.
// SetHeader is kept for cases where an HX-Trigger is needed alongside a DOM swap
// (e.g. kanban board refresh), not for plain toasts.
package notify

import (
	"encoding/json"
	"fmt"
	"net/http"

	"knov/internal/cacheStorage"
	"knov/internal/logging"
)

// Level represents the visual severity of a notification.
type Level string

const (
	LevelSuccess Level = "success"
	LevelError   Level = "error"
	LevelWarning Level = "warning"
	LevelInfo    Level = "info"
)

// flashKey is the cache storage key for flash messages.
const flashKey = "flash:notification"

type payload struct {
	Type    Level  `json:"type"`
	Message string `json:"message"`
}

// SetFlash persists a notification in cache storage.
// It is consumed by GET /api/notifications/flash which htmx polls after every
// request and on page load, firing the toast via HX-Trigger.
// Use this for all toast notifications — it works for both in-page responses
// and cross-navigation responses (HX-Redirect / HX-Refresh).
func SetFlash(level Level, message string) {
	p := payload{Type: level, Message: message}
	data, err := json.Marshal(p)
	if err != nil {
		logging.LogError("notify: failed to marshal flash payload: %v", err)
		return
	}
	if err := cacheStorage.Set(flashKey, data); err != nil {
		logging.LogError("notify: failed to store flash notification: %v", err)
	}
}

// ConsumeFlash reads and deletes the pending flash notification from cache.
// Returns nil if no flash is stored.
func ConsumeFlash() *payload {
	data, err := cacheStorage.Get(flashKey)
	if err != nil || len(data) == 0 {
		return nil
	}
	if err := cacheStorage.Delete(flashKey); err != nil {
		logging.LogWarning("notify: failed to delete flash notification: %v", err)
	}
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		logging.LogError("notify: failed to unmarshal flash payload: %v", err)
		return nil
	}
	return &p
}

// SetHeader sets an HX-Trigger header directly on the response.
// Only use this when you need to fire a non-toast htmx event alongside a DOM swap
// in the same response (e.g. triggering a board refresh). For plain toast
// notifications, use SetFlash instead.
func SetHeader(w http.ResponseWriter, level Level, message string) {
	p := payload{Type: level, Message: message}
	data, err := json.Marshal(map[string]payload{"notify": p})
	if err != nil {
		return
	}
	w.Header().Set("HX-Trigger", string(data))
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

        // show any toast fired via HX-Trigger (e.g. direct SetHeader calls)
        document.body.addEventListener('notify', function (e) {
            var detail = e.detail;
            if (detail && detail.type && detail.message) {
                showToast(detail.type, detail.message);
            }
        });

        // poll flash after every htmx request and on initial page load
        function pollFlash() {
            console.trace('pollFlash called from:');
            htmx.ajax('GET', '/api/notifications/flash', {swap: 'none'});
        }

        document.addEventListener('htmx:afterSettle', function(evt) {
            // Only poll after non-flash requests
            if (evt.detail && evt.detail.pathInfo &&
                evt.detail.pathInfo.requestPath !== '/api/notifications/flash') {
                pollFlash();
            }
        });

    })();
    </script>
`, duration)
}
