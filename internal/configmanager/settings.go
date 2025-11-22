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
var currentUserID string
var configPath string

// UserSettings contains user-specific settings stored in JSON
type UserSettings struct {
	Theme         string           `json:"theme"`
	Language      string           `json:"language"`
	ThemeSettings AllThemeSettings `json:"themeSettings,omitempty"`
}

// InitUserSettings initializes user settings from direct JSON file for specific user
func InitUserSettings(userID string) {
	currentUserID = userID
	configPath = GetAppConfig().ConfigPath
	userSettings = UserSettings{
		Theme:         "builtin",
		Language:      "en",
		ThemeSettings: make(AllThemeSettings),
	}

	settingsPath := getUserSettingsPath(userID)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.LogInfo("no user settings found for user %s, using defaults", userID)
			saveUserSettings()
			return
		}
		logging.LogError("failed to read user settings for user %s: %v", userID, err)
		return
	}

	if err := json.Unmarshal(data, &userSettings); err != nil {
		logging.LogError("failed to decode user settings for user %s: %s", userID, err)
		return
	}

	translation.SetLanguage(userSettings.Language)
	logging.LogInfo("user settings loaded for user: %s", userID)
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

// SwitchUser changes the active user and loads their settings
func SwitchUser(userID string) {
	InitUserSettings(userID)
}

func saveUserSettings() error {
	data, err := json.Marshal(userSettings)
	if err != nil {
		return fmt.Errorf("failed to marshal user settings: %s", err)
	}

	settingsPath := getUserSettingsPath(currentUserID)
	settingsDir := filepath.Dir(settingsPath)

	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create user settings directory: %s", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save user settings: %s", err)
	}

	logging.LogInfo("user settings saved for user: %s", currentUserID)
	return nil
}

// getUserSettingsPath returns the file path for user settings JSON file
func getUserSettingsPath(userID string) string {
	return filepath.Join(configPath, "user", userID, "settings.json")
}
