package server

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"knov/internal/configmanager"
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
// @Param section path string true "Section key (e.g. general, editor, table, media, file-types)"
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
			var html string
			if s.Key == configmanager.SectionGeneral.Key {
				html = render.RenderSettingsSection(s, t, render.RenderFaviconItem(t))
			} else {
				html = render.RenderSettingsSection(s, t)
			}
			writeResponse(w, r, toSectionJSON(s), html)
			return
		}
	}
	http.Error(w, "unknown section", http.StatusNotFound)
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
		html += render.RenderSettingsSection(s, t)
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
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	errs := configmanager.BulkSetFromForm(r.Form)
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		http.Error(w, strings.Join(msgs, "; "), http.StatusBadRequest)
		return
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
		http.Error(w, "unknown setting", http.StatusNotFound)
		return
	}
	if err := s.SetFromString(r.FormValue(key)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := configmanager.SaveSettings(); err != nil {
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}
	if rs, ok := s.(configmanager.RenderableSetting); ok && rs.GetMeta().Refresh {
		w.Header().Set("HX-Refresh", "true")
	}
	writeResponse(w, r, map[string]interface{}{"key": key, "value": s.GetValue()}, "")
}
