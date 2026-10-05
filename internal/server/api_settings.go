package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/job"
	"knov/internal/kanban"
	"knov/internal/logging"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

type settingJSON struct {
	Key     string                        `json:"key"`
	Type    string                        `json:"type"`
	Value   interface{}                   `json:"value"`
	Label   string                        `json:"label,omitempty"`
	Desc    string                        `json:"desc,omitempty"`
	Group   string                        `json:"group,omitempty"`
	Options []configmanager.SettingOption `json:"options,omitempty"`
	DynURL  string                        `json:"dynUrl,omitempty"`
}

type sectionJSON struct {
	Key      string        `json:"key"`
	Label    string        `json:"label"`
	Settings []settingJSON `json:"settings"`
}

func toSettingJSON(s configmanager.RenderableSetting) settingJSON {
	meta := s.GetMeta()
	return settingJSON{
		Key:     s.Key(),
		Type:    s.Type(),
		Value:   s.GetValue(),
		Label:   meta.Label,
		Desc:    meta.Desc,
		Group:   meta.Group.Key,
		Options: meta.Options,
		DynURL:  meta.DynURL,
	}
}

func toSectionJSON(section configmanager.SettingSection) sectionJSON {
	items := configmanager.SettingsBySection(section)
	settings := make([]settingJSON, len(items))
	for i, s := range items {
		settings[i] = toSettingJSON(s)
	}
	return sectionJSON{Key: section.Key, Label: section.Label, Settings: settings}
}

// @Summary Get settings section
// @Description Returns settings for a single section as HTML (HTMX) or JSON
// @Tags settings
// @Param section path string true "Section key (e.g. general, appearance, editor, table, media, file-types)"
// @Produce json,html
// @Success 200 {object} sectionJSON
// @Failure 404 {string} string "unknown section"
// @Router /api/settings/{section} [get]
func handleAPIGetSettingsSection(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "section")
	for _, s := range configmanager.AllSections() {
		if s.Key == slug {
			lang := configmanager.GetLanguage()
			t := func(key string, args ...any) string {
				return translation.SprintfForRequest(lang, key, args...)
			}
			writeResponse(w, r, toSectionJSON(s), renderSettingsSection(s, t))
			return
		}
	}
	writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "unknown section"))
}

// renderSettingsSection renders a section together with its section-specific extra items.
func renderSettingsSection(s configmanager.SettingSection, t func(string, ...any) string) string {
	switch s.Key {
	case configmanager.SectionAppearance.Key:
		return render.RenderSettingsSection(s, t, render.RenderFaviconItem(t))
	case configmanager.SectionKanban.Key:
		return render.RenderSettingsSection(s, t, render.RenderKanbanConfigWarnings(kanban.ConfigWarnings(t), t), render.RenderKanbanStatusRenameItem(configmanager.GetKanbanStatuses(), t))
	default:
		return render.RenderSettingsSection(s, t)
	}
}

// @Summary Get kanban config warnings
// @Description Returns inconsistencies in the kanban settings (e.g. missing board folders, columns not in the status list)
// @Tags settings
// @Produce json,html
// @Success 200 {array} string
// @Router /api/settings/kanban/warnings [get]
func handleAPIGetKanbanConfigWarnings(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}
	warnings := kanban.ConfigWarnings(t)
	writeResponse(w, r, warnings, render.RenderKanbanConfigWarnings(warnings, t))
}

// @Summary Rename a kanban status
// @Description Renames a kanban status everywhere it's stored: the kanban settings, the status tag of every file, the status folders of foldersync boards, the stored card order and the logged events. Runs as the kanban-rename-status job.
// @Tags settings
// @Accept x-www-form-urlencoded
// @Produce json,html
// @Param from formData string true "Current status name"
// @Param to formData string true "New status name"
// @Success 200 {object} kanban.RenameResult
// @Failure 400 {string} string "invalid status"
// @Failure 409 {string} string "job already running"
// @Failure 500 {string} string "failed to rename status"
// @Router /api/settings/kanban/statuses/rename [post]
func handleAPIRenameKanbanStatus(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	oldStatus, newStatus := strings.TrimSpace(r.FormValue("from")), strings.TrimSpace(r.FormValue("to"))
	result, err := job.RunKanbanRenameStatus(oldStatus, newStatus)
	if errors.Is(err, job.ErrAlreadyRunning) {
		writeAPIError(w, r, http.StatusConflict, translation.SprintfForRequest(lang, "job already running"))
		return
	}
	if errors.Is(err, kanban.ErrInvalidRename) {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(lang, "%s", err.Error()))
		return
	}
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to rename kanban status %s to %s: %v", oldStatus, newStatus, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(lang, "failed to rename status: %s", err.Error()))
		return
	}

	// full reload so every kanban setting shows its renamed value
	if result.LinksFailed > 0 {
		notify.SetFlash(notify.LevelWarning, translation.SprintfForRequest(lang, "renamed status %s to %s (%d files, %d folders) - the links of %d moved files couldn't be updated", oldStatus, newStatus, result.Retagged, result.Folders, result.LinksFailed))
	} else {
		notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(lang, "renamed status %s to %s (%d files, %d folders)", oldStatus, newStatus, result.Retagged, result.Folders))
	}
	w.Header().Set("HX-Refresh", "true")
	writeResponse(w, r, result, "")
}

// @Summary Get all settings
// @Description Returns all settings sections as HTML (HTMX) or JSON
// @Tags settings
// @Produce json,html
// @Success 200 {array} sectionJSON
// @Router /api/settings [get]
func handleAPIGetAllSettings(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}
	var html string
	sections := configmanager.AllSections()
	jsonData := make([]sectionJSON, len(sections))
	for i, s := range sections {
		html += renderSettingsSection(s, t)
		jsonData[i] = toSectionJSON(s)
	}
	writeResponse(w, r, jsonData, html)
}

// @Summary Update multiple settings at once
// @Description Applies all recognised form fields as settings in one call, saving once at the end
// @Tags settings
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {string} string "saved"
// @Failure 400 {string} string "one or more validation errors"
// @Failure 500 {string} string "failed to save settings"
// @Router /api/settings [post]
func handleAPIBulkSetSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	errs := configmanager.BulkSetFromForm(r.Form)
	if len(errs) == 1 && errors.Is(errs[0], configmanager.ErrSaveSettings) {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"))
		return
	}
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", strings.Join(msgs, "; ")))
		return
	}
	for key := range r.Form {
		if configmanager.RefreshesFileCaches(key) {
			files.RefreshCaches()
			break
		}
	}
	writeResponse(w, r, "saved", "")
}

// @Summary Update a setting
// @Description Updates a single setting value by key and persists it
// @Tags settings
// @Accept application/x-www-form-urlencoded
// @Param key path string true "Setting key (e.g. language, logLevel, showHiddenFiles)"
// @Param key formData string true "New value for the setting"
// @Produce json,html
// @Success 200 {object} settingJSON
// @Failure 404 {string} string "unknown setting"
// @Failure 500 {string} string "failed to save setting"
// @Router /api/settings/{key} [post]
func handleAPISetSetting(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	s := configmanager.GetSetting(key)
	if s == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "unknown setting"))
		return
	}
	err := configmanager.SetSetting(s, r.FormValue(key))
	if errors.Is(err, configmanager.ErrSaveSettings) {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"))
		return
	}
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error()))
		return
	}
	if configmanager.RefreshesFileCaches(key) {
		files.RefreshCaches()
	}
	if rs, ok := s.(configmanager.RenderableSetting); ok {
		if rs.GetMeta().Refresh {
			w.Header().Set("HX-Refresh", "true")
		} else {
			// lets a section re-fetch itself after a save, e.g. to update its warnings
			w.Header().Set("HX-Trigger", "settings-"+rs.GetMeta().Section.Key+"-saved")
		}
	}
	writeResponse(w, r, map[string]interface{}{"key": key, "value": s.GetValue()}, "")
}
