package configmanager

import (
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"knov/internal/configStorage"
	"knov/internal/logging"
	"knov/internal/translation"
)

// ── init/save ─────────────────────────────────────────────────────────────────

// InitSettings loads settings from storage, falling back to defaults if absent.
func InitSettings() {
	data, err := configStorage.Get("settings")
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to read user settings: %v", err)
		return
	}
	if data == nil {
		logging.LogInfo(logging.KeyApp, "no user settings found, using defaults")
		if err := SaveSettings(); err != nil {
			logging.LogError(logging.KeyApp, "failed to save default settings: %v", err)
		}
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		logging.LogError(logging.KeyApp, "failed to decode user settings: %v", err)
		return
	}

	for _, s := range allSettings {
		if v, ok := raw[s.Key()]; ok {
			var val interface{}
			if err := json.Unmarshal(v, &val); err == nil {
				s.setFromJSON(val)
			}
		}
	}

	applyLanguage(Language.Get())

	logging.LogInfo(logging.KeyApp, "user settings loaded")
}

func applyLanguage(lang string) {
	translation.SetLanguage(CheckLanguage(lang))
}

// SaveSettings persists all registry values to storage.
func SaveSettings() error {
	m := make(map[string]interface{})
	for _, s := range allSettings {
		m[s.Key()] = s.GetValue()
	}
	data, err := json.Marshal(m)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to marshal settings: %v", err)
		return err
	}
	if err := configStorage.Set("settings", data); err != nil {
		logging.LogError(logging.KeyApp, "failed to save settings: %v", err)
		return err
	}
	logging.LogInfo(logging.KeyApp, "user settings saved")
	return nil
}

// BulkSetFromForm applies every key in values whose key is a known setting,
// then calls SaveSettings exactly once. Validation errors are collected and
// returned; a save error is appended last. Unknown keys are logged and skipped.
func BulkSetFromForm(values map[string][]string) []error {
	var errs []error
	for key, vals := range values {
		s := GetSetting(key)
		if s == nil {
			logging.LogDebug(logging.KeyApp, "BulkSetFromForm: unknown setting key %q, skipping", key)
			continue
		}
		val := ""
		if len(vals) > 0 {
			val = vals[0]
		}
		if err := s.SetFromString(val); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
	}
	if err := SaveSettings(); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// ExportSettingsJSON returns the current settings as a JSON blob (for export).
// Note: customFaviconExt is intentionally excluded — the extension is meaningless
// without the favicon file itself, which is not part of the settings export.
func ExportSettingsJSON() ([]byte, error) {
	m := make(map[string]interface{})
	for _, s := range allSettings {
		m[s.Key()] = s.GetValue()
	}
	return json.MarshalIndent(m, "", "  ")
}

// ImportSettingsJSON loads settings from a JSON blob and persists them.
// Note: customFaviconExt is intentionally ignored on import for the same reason
// it is excluded from export — the favicon file must be uploaded separately.
func ImportSettingsJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, s := range allSettings {
		if v, ok := raw[s.Key()]; ok {
			var val interface{}
			if err := json.Unmarshal(v, &val); err == nil {
				s.setFromJSON(val)
			}
		}
	}
	applyLanguage(Language.Get())
	return SaveSettings()
}

// ── favicon accessors ─────────────────────────────────────────────────────────
//
// The custom favicon extension is stored separately from the settings registry.
// It is set as a side effect of a file upload and is meaningless without the
// accompanying file on disk, so it is excluded from settings export/import.
// Use the /api/config/favicon endpoints to manage the favicon file.

// GetCustomFaviconExt returns the file extension of the uploaded custom favicon, or "".
func GetCustomFaviconExt() string {
	data, err := configStorage.Get("customFaviconExt")
	if err != nil || data == nil {
		return ""
	}
	return string(data)
}

// SetCustomFaviconExt updates the custom favicon extension and persists.
func SetCustomFaviconExt(ext string) {
	if err := configStorage.Set("customFaviconExt", []byte(ext)); err != nil {
		logging.LogError(logging.KeyApp, "failed to save custom favicon ext: %v", err)
	}
}

// GetCustomFaviconPath returns the full filesystem path of the custom favicon, or "".
func GetCustomFaviconPath() string {
	ext := GetCustomFaviconExt()
	if ext == "" {
		return ""
	}
	return filepath.Join(appConfig.StoragePath, "favicon", "favicon"+ext)
}

// ── convenience helpers ───────────────────────────────────────────────────────

func GetMaxUploadSize() int64 {
	mb := MaxUploadSizeMB.Get()
	if mb <= 0 {
		mb = 10
	}
	return int64(mb) * 1024 * 1024
}

func GetSectionEditIncludeSubheaders() bool { return SectionEditIncludeSubheaders.Get() }
func GetDefaultPreviewSize() int {
	s := DefaultPreviewSize.Get()
	if s <= 0 {
		return 300
	}
	return s
}
func GetPreviewsEnabled() bool { return EnablePreviews.Get() }
func GetDisplayMode() string {
	m := DisplayMode.Get()
	if m == "" {
		return "center"
	}
	return m
}
func GetBorderStyle() string {
	s := BorderStyle.Get()
	if s == "" {
		return "simple"
	}
	return s
}
func GetShowCaption() bool          { return ShowCaption.Get() }
func GetClickToEnlarge() bool       { return ClickToEnlarge.Get() }
func GetAllowedMimeTypes() []string { return AllowedMimeTypes.Get() }

func GetTablePageSize() int {
	s := PageSize.Get()
	if s <= 0 {
		return 25
	}
	return s
}
func GetShowHiddenFiles() bool { return ShowHiddenFiles.Get() }
func GetHomeDashboard() string { return HomeDashboard.Get() }

// ── mime / extension helpers ──────────────────────────────────────────────────

func IsHiddenByMime(mimeType string) bool {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return HideImage.Get()
	case strings.HasPrefix(mimeType, "video/"):
		return HideVideo.Get()
	case mimeType == "application/pdf":
		return HidePDF.Get()
	}
	return false
}

func IsHiddenByExt(ext string) bool {
	switch ext {
	case ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".ods", ".odt", ".odp", ".odg", ".odc":
		return HideOfficeDocuments.Get()
	case ".zip", ".rar", ".7z", ".gz", ".tar", ".bz2", ".xz", ".tgz":
		return HideArchives.Get()
	case ".exe", ".jar", ".pfx", ".db", ".jd2backup",
		".ts3_plugin", ".ppk", ".chm", ".ini":
		return HideExecutables.Get()
	case ".sh", ".bat", ".cmd", ".ps1", ".py", ".rb", ".pl":
		return HideScripts.Get()
	}
	return false
}

// MimeTypeByExtension returns the clean mime type for an extension (no parameters).
func MimeTypeByExtension(ext string) string {
	mimeType := mime.TypeByExtension(ext)
	if i := strings.Index(mimeType, ";"); i >= 0 {
		mimeType = strings.TrimSpace(mimeType[:i])
	}
	return mimeType
}

// IsImageExtension returns true if the file extension maps to an allowed image/* mime type
func IsImageExtension(ext string) bool {
	mimeType := MimeTypeByExtension(ext)
	if !strings.HasPrefix(mimeType, "image/") {
		return false
	}
	for _, allowed := range GetAllowedMimeTypes() {
		if allowed == mimeType {
			return true
		}
	}
	return false
}

// IsVideoExtension returns true if the extension maps to a video/* mime type.
func IsVideoExtension(ext string) bool {
	return strings.HasPrefix(MimeTypeByExtension(ext), "video/")
}

// IsAudioExtension returns true if the extension maps to an audio/* mime type.
func IsAudioExtension(ext string) bool {
	return strings.HasPrefix(MimeTypeByExtension(ext), "audio/")
}
