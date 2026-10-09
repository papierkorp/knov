// Package server - Tracker API handlers
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/tracker"
	"knov/internal/translation"
)

// @Summary Save tracker configuration
// @Description Save a tracker's title and reconcile its counters by id (removed counters lose their recorded counts); regenerates the paired markdown table
// @Tags tracker
// @Accept application/x-www-form-urlencoded
// @Param trackerid formData string true "Tracker identifier (name)"
// @Param title formData string false "Optional heading"
// @Param counter_id[] formData array false "Counter ids (blank for a new counter), index-aligned with counter_title[]"
// @Param counter_title[] formData array false "Counter titles, index-aligned with counter_id[]"
// @Param counter_columns[] formData array false "Per-counter ColumnSet as JSON, index-aligned with counter_id[]"
// @Produce json,html
// @Success 200 {object} map[string]string "empty body; sets HX-Redirect to the tracker's edit page"
// @Router /api/trackers/save [post]
func handleAPITrackerSave(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(lang, "failed to parse form"))
		return
	}

	trackerID := strings.TrimSpace(r.FormValue("trackerid"))
	if trackerID == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(lang, "tracker name is required"))
		return
	}

	rows, err := trackerCounterRows(r)
	if err != nil {
		logging.LogError(logging.KeyApp, "malformed tracker counter rows: %v", err)
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(lang, "failed to parse form"))
		return
	}

	if err := tracker.SetMeta(trackerID, r.FormValue("title"), rows); err != nil {
		if writeNewPathError(w, r, err) {
			return
		}
		logging.LogError(logging.KeyApp, "failed to save tracker config: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(lang, "failed to save tracker"))
		return
	}

	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(lang, "tracker saved successfully"))
	w.Header().Set("HX-Redirect", pathutils.ToFileEditURL(pathutils.ToWithPrefix(tracker.TrackerIndexPath(trackerID))))
	writeResponse(w, r, map[string]string{"tracker": trackerID}, "")
}

// trackerCounterRows zips the parallel counter_id[]/counter_title[]/counter_columns[]
// form arrays into editor rows for tracker.SetMeta. counter_columns[] must be either
// absent or exactly as long as counter_title[] - a partial submission signals a
// malformed request rather than "no preference", so it's rejected instead of
// silently defaulting some rows' columns.
func trackerCounterRows(r *http.Request) ([]tracker.CounterInput, error) {
	ids := r.Form["counter_id[]"]
	titles := r.Form["counter_title[]"]
	columns := r.Form["counter_columns[]"]
	if len(columns) != 0 && len(columns) != len(titles) {
		return nil, fmt.Errorf("counter_columns[] has %d entries, want 0 or %d", len(columns), len(titles))
	}

	rows := make([]tracker.CounterInput, 0, len(titles))
	for i, title := range titles {
		id := ""
		if i < len(ids) {
			id = ids[i]
		}
		var cols tracker.ColumnSet
		if i < len(columns) {
			if err := json.Unmarshal([]byte(columns[i]), &cols); err != nil {
				return nil, fmt.Errorf("counter_columns[%d]: %w", i, err)
			}
		}
		rows = append(rows, tracker.CounterInput{ID: id, Title: title, Columns: cols})
	}
	return rows, nil
}

// @Summary Add tracker counter row
// @Description Return HTML for a new empty tracker counter name input
// @Tags tracker
// @Produce json,html
// @Success 200 {string} string "tracker counter row html"
// @Router /api/trackers/add-counter [post]
func handleAPIAddTrackerCounter(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, r, map[string]string{}, render.RenderTrackerCounterRow("", nil))
}

// @Summary Record a tracker change
// @Description Add +1/-1 to a counter's day bucket, save, and return the updated counters-view row
// @Tags tracker
// @Accept application/x-www-form-urlencoded
// @Param trackerid formData string true "Tracker identifier (name)"
// @Param counterid formData string true "Counter id"
// @Param delta formData int true "Change: 1 or -1"
// @Produce json,html
// @Success 200 {string} string "updated tracker counter row html"
// @Router /api/trackers/tick [post]
func handleAPITrackerTick(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(lang, "failed to parse form"))
		return
	}

	trackerID := strings.TrimSpace(r.FormValue("trackerid"))
	counterID := strings.TrimSpace(r.FormValue("counterid"))
	delta, _ := strconv.Atoi(r.FormValue("delta"))

	counter, total, err := tracker.Tick(trackerID, counterID, delta)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to record tracker change: %v", err)
		status := http.StatusInternalServerError
		if errors.Is(err, tracker.ErrInvalidInput) || errors.Is(err, tracker.ErrNotFound) {
			status = http.StatusBadRequest
		}
		writeAPIError(w, r, status, translation.SprintfForRequest(lang, "failed to record change"))
		return
	}

	writeResponse(w, r, map[string]any{"tracker": trackerID, "counter": counter.ID, "delta": delta},
		render.RenderTrackerClickRow(trackerID, counter, total))
}

// @Summary Reset a tracker counter
// @Description Zero a counter's recorded days back to 0, save, and return the updated counter row
// @Tags tracker
// @Accept application/x-www-form-urlencoded
// @Param trackerid formData string true "Tracker identifier (name)"
// @Param counterid formData string true "Counter id"
// @Produce json,html
// @Success 200 {string} string "updated tracker counter row html"
// @Router /api/trackers/reset [post]
func handleAPITrackerReset(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(lang, "failed to parse form"))
		return
	}

	trackerID := strings.TrimSpace(r.FormValue("trackerid"))
	counterID := strings.TrimSpace(r.FormValue("counterid"))

	counter, err := tracker.Reset(trackerID, counterID)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to reset tracker counter: %v", err)
		status := http.StatusInternalServerError
		if errors.Is(err, tracker.ErrNotFound) {
			status = http.StatusBadRequest
		}
		writeAPIError(w, r, status, translation.SprintfForRequest(lang, "failed to reset counter"))
		return
	}

	writeResponse(w, r, map[string]any{"tracker": trackerID, "counter": counter.ID},
		render.RenderTrackerCounterRow(trackerID, counter))
}

// @Summary Delete tracker
// @Description Delete a tracker from config storage and its paired file
// @Tags tracker
// @Param id path string true "tracker id"
// @Produce json,html
// @Success 200 {object} map[string]string "empty body; sets HX-Redirect to /"
// @Router /api/trackers/delete/{id} [delete]
func handleAPITrackerDelete(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	trackerID := strings.TrimPrefix(r.URL.Path, "/api/trackers/delete/")

	if err := tracker.DeleteConfig(trackerID); err != nil {
		logging.LogError(logging.KeyApp, "failed to delete tracker %s: %v", trackerID, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(lang, "failed to delete tracker"))
		return
	}

	logging.LogInfo(logging.KeyApp, "deleted tracker: %s", trackerID)
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(lang, "tracker deleted"))
	w.Header().Set("HX-Redirect", "/")
	writeResponse(w, r, map[string]string{"deleted": trackerID}, "")
}
