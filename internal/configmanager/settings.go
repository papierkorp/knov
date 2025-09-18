// Package configmanager - User settings stored in JSON
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

// UserSettings contains user-specific settings stored in JSON
type UserSettings struct {
	Theme    string `json:"theme"`
	Language string `json:"language"`
	FileView string `json:"fileView"`
}

// InitUserSettings initializes user settings from JSON file for specific user
func InitUserSettings(userID string) {
	// TODO store user settings locally?
	currentUserID = userID
	userSettings = UserSettings{
		Theme:    "builtin",
		Language: "en",
		FileView: "detailed",
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
	return filepath.Join("config", "users", userID, "settings.json")
}

func saveUserSettings() error {
	settingsPath := getUserSettingsPath(currentUserID)
	settingsDir := filepath.Dir(settingsPath)

	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create user settings directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(userSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user settings: %s", err)
	}

	if err = os.WriteFile(settingsPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write user settings to file: %s", err)
	}

	logging.LogInfo("user settings saved for user: %s", currentUserID)
	return nil
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
