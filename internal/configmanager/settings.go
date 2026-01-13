package configmanager

import (
	"encoding/json"

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
	Theme         string           `json:"theme"`
	Language      string           `json:"language"`
	ThemeSettings AllThemeSettings `json:"themeSettings,omitempty"`
	MediaSettings MediaSettings    `json:"mediaSettings,omitempty"`
}

// MediaSettings contains media upload and management settings
type MediaSettings struct {
	MaxUploadSizeMB       int      `json:"maxUploadSizeMB"`
	AllowedMimeTypes      []string `json:"allowedMimeTypes"`
	OrphanedMediaBehavior string   `json:"orphanedMediaBehavior"` // "keep" or "manual"
	OrphanedMediaAgeDays  int      `json:"orphanedMediaAgeDays"`
}

// InitUserSettings initializes user settings from storage
func InitUserSettings() {
	userSettings = UserSettings{
		Theme:         "builtin",
		Language:      "en",
		ThemeSettings: make(AllThemeSettings),
		MediaSettings: MediaSettings{
			MaxUploadSizeMB:       10,
			AllowedMimeTypes:      []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml", "application/pdf"},
			OrphanedMediaBehavior: "manual",
			OrphanedMediaAgeDays:  7,
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
