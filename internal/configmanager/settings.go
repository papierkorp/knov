// Package configmanager - User settings stored in JSON
package configmanager

import (
	"encoding/json"
	"fmt"
	"os"

	"knov/internal/logging"
	"knov/internal/storage"
	"knov/internal/translation"
)

// -----------------------------------------------------------------------------
// ------------------------------- User Settings ------------------------------
// -----------------------------------------------------------------------------

var userSettings UserSettings
var currentUserID string

// UserSettings contains user-specific settings stored in JSON
type UserSettings struct {
	Theme       string `json:"theme"`
	Language    string `json:"language"`
	FileView    string `json:"fileView"`
	DarkMode    bool   `json:"darkMode"`
	ColorScheme string `json:"colorScheme"`
	CustomCSS   string `json:"customCSS"`
}

// InitUserSettings initializes user settings from JSON file for specific user
func InitUserSettings(userID string) {
	currentUserID = userID
	userSettings = UserSettings{
		Theme:       "builtin",
		Language:    "en",
		FileView:    "detailed",
		DarkMode:    false,
		ColorScheme: "default",
	}

	settingsPath := getUserSettingsPath(userID)
	jsonFile, err := os.ReadFile(settingsPath)
	if err != nil {
		logging.LogInfo("no user settings found for user %s, using defaults", userID)
		saveUserSettings()
		return
	}

	if err := json.Unmarshal(jsonFile, &userSettings); err != nil {
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

func getUserSettingsPath(userID string) string {
	return fmt.Sprintf("user/%s/settings", userID)
}

func saveUserSettings() error {
	data, err := json.Marshal(userSettings)
	if err != nil {
		return err
	}

	key := getUserSettingsPath(currentUserID)
	return storage.GetStorage().Set(key, data)
}

// GetFileView returns current file view from user settings
func GetFileView() string {
	if userSettings.FileView == "" {
		return "detailed"
	}
	return userSettings.FileView
}

// SetFileView updates user settings with new file view
func SetFileView(view string) {
	userSettings.FileView = view
	saveUserSettings()
}

// GetDarkMode returns current dark mode setting
func GetDarkMode() bool {
	return userSettings.DarkMode
}

// SetDarkMode updates dark mode setting
func SetDarkMode(enabled bool) {
	userSettings.DarkMode = enabled
	saveUserSettings()
}

// GetColorScheme returns current color scheme
func GetColorScheme() string {
	if userSettings.ColorScheme == "" {
		return "default"
	}
	return userSettings.ColorScheme
}

// SetColorScheme updates color scheme setting
func SetColorScheme(scheme string) {
	userSettings.ColorScheme = scheme
	saveUserSettings()
}

// GetCustomCSS returns current custom CSS
func GetCustomCSS() string {
	return userSettings.CustomCSS
}

// SetCustomCSS updates custom CSS setting
func SetCustomCSS(css string) {
	userSettings.CustomCSS = css
	saveUserSettings()
}
