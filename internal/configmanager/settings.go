package configmanager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/logging"
	"knov/internal/translation"
)

// -----------------------------------------------------------------------------
// ------------------------------- User Settings ------------------------------
// -----------------------------------------------------------------------------

var userSettings UserSettings
var configPath string

// UserSettings contains user-specific settings stored in JSON
type UserSettings struct {
	Theme         string           `json:"theme"`
	Language      string           `json:"language"`
	ThemeSettings AllThemeSettings `json:"themeSettings,omitempty"`
}

// InitUserSettings initializes user settings from direct JSON file
func InitUserSettings() {
	configPath = GetAppConfig().ConfigPath
	userSettings = UserSettings{
		Theme:         "builtin",
		Language:      "en",
		ThemeSettings: make(AllThemeSettings),
	}

	settingsPath := getUserSettingsPath()
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.LogInfo("no user settings found, using defaults")
			saveUserSettings()
			return
		}
		logging.LogError("failed to read user settings: %v", err)
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
		return fmt.Errorf("failed to marshal user settings: %s", err)
	}

	settingsPath := getUserSettingsPath()
	settingsDir := filepath.Dir(settingsPath)

	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create user settings directory: %s", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save user settings: %s", err)
	}

	logging.LogInfo("user settings saved")
	return nil
}

// getUserSettingsPath returns the file path for user settings JSON file
func getUserSettingsPath() string {
	return filepath.Join(configPath, "settings.json")
}
