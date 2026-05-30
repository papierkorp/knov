package configmanager

import (
	"encoding/json"
	"mime"
	"path/filepath"
	"strings"

	"knov/internal/configStorage"
	"knov/internal/logging"
	"knov/internal/translation"
)

// -----------------------------------------------------------------------------
// ------------------------------- User Settings ------------------------------
// -----------------------------------------------------------------------------

var userSettings UserSettings

// UserSettings contains user-specific settings stored in JSON
type UserSettings struct {
	Theme                        string           `json:"theme"`
	Language                     string           `json:"language"`
	ThemeSettings                AllThemeSettings `json:"themeSettings,omitempty"`
	MediaSettings                MediaSettings    `json:"mediaSettings,omitempty"`
	TableSettings                TableSettings    `json:"tableSettings,omitempty"`
	SectionEditIncludeSubheaders bool             `json:"sectionEditIncludeSubheaders"`
	CodeBlockWrap                bool             `json:"codeBlockWrap"`
	CustomFaviconExt             string           `json:"customFaviconExt,omitempty"` // ".ico", ".png", or ".svg"
}

// TableSettings contains display preferences for interactive tables
type TableSettings struct {
	PageSize   int  `json:"pageSize"`   // rows per page
	ShowSearch bool `json:"showSearch"` // show search input
	ShowInfo   bool `json:"showInfo"`   // show "showing X-Y of Z" line
	ShowPaging bool `json:"showPaging"` // show prev/next/first/last buttons
}

// MediaSettings contains media upload and management settings
type MediaSettings struct {
	MaxUploadSizeMB    int      `json:"maxUploadSizeMB"`
	AllowedMimeTypes   []string `json:"allowedMimeTypes"`
	DefaultPreviewSize int      `json:"defaultPreviewSize"`
	EnablePreviews     bool     `json:"enablePreviews"`
	DisplayMode        string   `json:"displayMode"`    // "left", "center", "right", "inline"
	BorderStyle        string   `json:"borderStyle"`    // "none", "simple", "rounded", "shadow"
	ShowCaption        bool     `json:"showCaption"`    // show filename as caption
	ClickToEnlarge     bool     `json:"clickToEnlarge"` // make previews clickable
}

// InitUserSettings initializes user settings from storage
func InitUserSettings() {
	userSettings = UserSettings{
		Theme:                        "builtin",
		Language:                     "en",
		ThemeSettings:                make(AllThemeSettings),
		SectionEditIncludeSubheaders: false,
		MediaSettings: MediaSettings{
			MaxUploadSizeMB: 10,
			AllowedMimeTypes: []string{
				// Images (safe to display)
				"image/jpeg",
				"image/gif",
				"image/png",
				"image/webp",
				"image/vnd.microsoft.icon",
				"image/svg+xml",
				// Audio
				"audio/mpeg",
				"audio/ogg",
				"audio/wav",
				// Video
				"video/webm",
				"video/ogg",
				"video/mp4",
				// Documents
				"application/pdf",
				// Subtitles
				"text/vtt",
				// Uncomment these for trusted users only (security risk):
				// "text/html",    // Cross-site scripting risk
				// "text/plain",   // Could be used for spam
				// "text/xml",     // Potential security risk
				// "text/csv",     // Data extraction risk
			},
			DefaultPreviewSize: 300,
			EnablePreviews:     true,
			DisplayMode:        "center",
			BorderStyle:        "simple",
			ShowCaption:        false,
			ClickToEnlarge:     true,
		},
		TableSettings: TableSettings{
			PageSize:   25,
			ShowSearch: true,
			ShowInfo:   true,
			ShowPaging: true,
		},
	}

	data, err := configStorage.Get("settings")
	if err != nil {
		logging.LogError("failed to read user settings: %v", err)
		return
	}

	if data == nil {
		logging.LogInfo("no user settings found, using defaults")
		saveUserSettings()
		return
	}

	if err := json.Unmarshal(data, &userSettings); err != nil {
		logging.LogError("failed to decode user settings: %s", err)
		return
	}

	translation.SetLanguage(userSettings.Language)
	logging.LogInfo("user settings loaded")
}

// GetUserSettings returns current user settings
func GetUserSettings() UserSettings {
	return userSettings
}

// SetUserSettings saves new user settings for current user
func SetUserSettings(settings UserSettings) {
	userSettings = settings
	saveUserSettings()
}

func saveUserSettings() error {
	data, err := json.Marshal(userSettings)
	if err != nil {
		logging.LogError("failed to marshal user settings: %v", err)
		return err
	}

	if err := configStorage.Set("settings", data); err != nil {
		logging.LogError("failed to save user settings: %v", err)
		return err
	}

	logging.LogInfo("user settings saved")
	return nil
}

// GetMaxUploadSize returns the maximum upload size in bytes
func GetMaxUploadSize() int64 {
	maxUploadSizeMB := userSettings.MediaSettings.MaxUploadSizeMB
	if maxUploadSizeMB <= 0 {
		maxUploadSizeMB = 10 // 10MB default
	}
	return int64(maxUploadSizeMB) * 1024 * 1024
}

// GetSectionEditIncludeSubheaders returns whether section editing should include subheaders
func GetSectionEditIncludeSubheaders() bool {
	return userSettings.SectionEditIncludeSubheaders
}

// GetDefaultPreviewSize returns the default preview size for media
func GetDefaultPreviewSize() int {
	if userSettings.MediaSettings.DefaultPreviewSize <= 0 {
		return 300
	}
	return userSettings.MediaSettings.DefaultPreviewSize
}

// GetPreviewsEnabled returns whether media previews are enabled
func GetPreviewsEnabled() bool {
	return userSettings.MediaSettings.EnablePreviews
}

// GetDisplayMode returns the preview display mode
func GetDisplayMode() string {
	mode := userSettings.MediaSettings.DisplayMode
	if mode == "" {
		return "center"
	}
	return mode
}

// GetBorderStyle returns the preview border style
func GetBorderStyle() string {
	style := userSettings.MediaSettings.BorderStyle
	if style == "" {
		return "simple"
	}
	return style
}

// GetShowCaption returns whether to show captions
func GetShowCaption() bool {
	return userSettings.MediaSettings.ShowCaption
}

// GetClickToEnlarge returns whether previews are clickable
func GetClickToEnlarge() bool {
	return userSettings.MediaSettings.ClickToEnlarge
}

// GetAllowedMimeTypes returns the list of allowed mime types
func GetAllowedMimeTypes() []string {
	return userSettings.MediaSettings.AllowedMimeTypes
}

// GetCustomFaviconExt returns the file extension of the uploaded custom favicon, or "".
func GetCustomFaviconExt() string {
	return userSettings.CustomFaviconExt
}

// GetCustomFaviconPath returns the full filesystem path of the custom favicon, or "".
func GetCustomFaviconPath() string {
	ext := userSettings.CustomFaviconExt
	if ext == "" {
		return ""
	}
	return filepath.Join(appConfig.StoragePath, "favicon", "favicon"+ext)
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

// GetTableSettings returns the current table display settings
func GetTableSettings() TableSettings {
	return userSettings.TableSettings
}

// GetTablePageSize returns the configured rows-per-page, falling back to 25
func GetTablePageSize() int {
	if userSettings.TableSettings.PageSize <= 0 {
		return 25
	}
	return userSettings.TableSettings.PageSize
}

// GetTableShowSearch returns whether the table search input should be shown
func GetTableShowSearch() bool {
	return userSettings.TableSettings.ShowSearch
}

// GetTableShowInfo returns whether the "showing X-Y of Z" line should be shown
func GetTableShowInfo() bool {
	return userSettings.TableSettings.ShowInfo
}

// GetTableShowPaging returns whether pagination buttons should be shown
func GetTableShowPaging() bool {
	return userSettings.TableSettings.ShowPaging
}
