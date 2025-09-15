// Package configmanager ..
package configmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"

	"knov/internal/logging"
	"knov/internal/translation"
)

// -----------------------------------------------------------------------------
// ------------------------------- configManager -------------------------------
// -----------------------------------------------------------------------------

var configManager ConfigManager

// DataPath defaults to data or is set via env
var DataPath = "data"

// ConfigManager ..
type ConfigManager struct {
	Themes   ConfigThemes   `json:"themes"`
	General  ConfigGeneral  `json:"general"`
	Git      ConfigGit      `json:"git"`
	Metadata ConfigMetadata `json:"metadata"`
}

// ConfigGeneral ..
type ConfigGeneral struct {
	Language string `json:"language"`
	LogLevel string `json:"logLevel"` // debug, info, warning, error
}

// ConfigThemes ..
type ConfigThemes struct {
	CurrentTheme string `json:"currentTheme"`
}

// ConfigGit ..
type ConfigGit struct {
	RepositoryURL string `json:"repositoryUrl"`
}

// ConfigMetadata ..
type ConfigMetadata struct {
	StorageMethod string   `json:"storagemethod"` //"json", "sqlite", "postgres", "yaml"
	LinkRegex     []string `json:"linkRegex"`
}

// InitConfig intializing config/config.json
func InitConfig() {
	initEnv()
	jsonFile, err := os.ReadFile("config/config.json")
	if err != nil {
		translation.Sprintf("testmessage from configmanager")
		logging.LogError("unable to open config.json file: %s", err)
	}

	if len(jsonFile) == 0 {
		logging.LogError("config.json file is empty")
	}

	if !json.Valid(jsonFile) {
		logging.LogError("config.json contains invalid JSON")
	}

	decoder := json.NewDecoder(bytes.NewBuffer(jsonFile))
	if err := decoder.Decode(&configManager); err != nil {
		logging.LogError("failed to decode config.json: %s", err)
	}

	SetLanguage(GetLanguage())
	initLogLevel()

	if err := InitGitRepository(); err != nil {
		logging.LogError("failed to initialize git repository: %s", err)
	}

}

func saveConfigToFile() error {
	err := os.MkdirAll("config", 0755)
	if err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(configManager, "", " ")

	if err != nil {
		return fmt.Errorf("failed to marshal config: %s", err)
	}

	if err = os.WriteFile("config/config.json", jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write config to file: %s", err)
	}

	logging.LogDebug("config: %s", jsonData)
	logging.LogInfo("config saved successfully")
	return nil
}

// GetConfig ..
func GetConfig() ConfigManager {
	return configManager
}

// SetConfig ..
func SetConfig(newConfig ConfigManager) {
	configManager = newConfig
	saveConfigToFile()
}

// -----------------------------------------------------------------------------
// --------------------------------- log level ---------------------------------
// -----------------------------------------------------------------------------

func initLogLevel() {
	envLogLevel := os.Getenv("KNOV_LOG_LEVEL")
	if envLogLevel != "" {
		logging.LogInfo("loglevel set to: %s", envLogLevel)
		return
	}

	logLevel := GetLogLevel()
	logging.LogInfo("loglevel set to: %s", logLevel)
	os.Setenv("KNOV_LOG_LEVEL", logLevel)
}

// GetLogLevel return the current log level, defaults to "info"
func GetLogLevel() string {
	if configManager.General.LogLevel == "" {
		return "info"
	}
	return configManager.General.LogLevel
}

// SetLogLevel set log level
func SetLogLevel(level string) {
	validLevels := []string{"debug", "info", "warning", "error"}

	if !slices.Contains(validLevels, level) {
		logging.LogWarning("invalid log level '%s', falling back to 'info'", level)
		level = "info"
	}

	configManager.General.LogLevel = level
	os.Setenv("KNOV_LOG_LEVEL", level)
	saveConfigToFile()
}

// -----------------------------------------------------------------------------
// ----------------------------------- themes -----------------------------------
// -----------------------------------------------------------------------------

// GetConfigThemes ..
func GetConfigThemes() ConfigThemes {
	return configManager.Themes
}

// SetConfigThemes ..
func SetConfigThemes(newConfigThemes ConfigThemes) {
	configManager.Themes = newConfigThemes
	saveConfigToFile()
}

// -----------------------------------------------------------------------------
// ---------------------------------- general ----------------------------------
// -----------------------------------------------------------------------------

// GetGeneral ..
func GetGeneral() ConfigGeneral {
	return configManager.General
}

// SetGeneral ..
func SetGeneral(general ConfigGeneral) {
	configManager.General = general
	saveConfigToFile()
}

// -----------------------------------------------------------------------------
// ---------------------------------- language ----------------------------------
// -----------------------------------------------------------------------------

// Language lists all available Languages
type Language struct {
	Code string
	Name string
}

// GetAvailableLanguages returns all supported languages
func GetAvailableLanguages() []Language {
	return []Language{
		{Code: "en", Name: "English"},
		{Code: "de", Name: "Deutsch"},
	}
}

// CheckLanguage validates if a language code is supported
func CheckLanguage(lang string) string {
	if lang == "" {
		return "en"
	}

	availableLanguages := GetAvailableLanguages()
	for _, availableLang := range availableLanguages {
		if availableLang.Code == lang {
			return lang
		}
	}

	logging.LogWarning("language '%s' not supported, falling back to 'en'", lang)
	return "en"
}

// GetLanguage ..
func GetLanguage() string {
	return CheckLanguage(configManager.General.Language)
}

// SetLanguage ..
func SetLanguage(lang string) {
	validLang := CheckLanguage(lang)
	configManager.General.Language = validLang

	logging.LogDebug("setlanguage: %s", validLang)
	saveConfigToFile()
}

// -----------------------------------------------------------------------------
// ------------------------------------ git ------------------------------------
// -----------------------------------------------------------------------------

// GetConfigGit ..
func GetConfigGit() ConfigGit {
	return configManager.Git
}

// SetConfigGit ..
func SetConfigGit(newConfigGit ConfigGit) {
	configManager.Git = newConfigGit
	saveConfigToFile()
}

// InitGitRepository Initalize a git repository in data folder
func InitGitRepository() error {
	gitDir := DataPath + "/.git"
	if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
		return nil
	}

	if configManager.Git.RepositoryURL != "" && configManager.Git.RepositoryURL != "local" {
		// Clone existing repository
		cmd := exec.Command("git", "clone", configManager.Git.RepositoryURL, DataPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
		logging.LogInfo("git repository cloned from %s to %s", configManager.Git.RepositoryURL, DataPath)
	} else {
		// Initialize new repository
		if _, err := os.Stat(DataPath); os.IsNotExist(err) {
			if err := os.MkdirAll(DataPath, 0755); err != nil {
				return fmt.Errorf("failed to create data directory: %w", err)
			}
		}

		cmd := exec.Command("git", "init")
		cmd.Dir = DataPath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to initialize git repository: %w", err)
		}
		logging.LogInfo("git repository initialized in %s", DataPath)
	}

	return nil
}

// -----------------------------------------------------------------------------
// --------------------------------- METADATA ---------------------------------
// -----------------------------------------------------------------------------

// GetConfigMetadata ..
func GetConfigMetadata() ConfigMetadata {
	return configManager.Metadata
}

// SetConfigMetadata ..
func SetConfigMetadata(newConfigMetadata ConfigMetadata) {
	configManager.Metadata = newConfigMetadata
	saveConfigToFile()
}

// GetMetadataStorageMethod returns storage method with default
func GetMetadataStorageMethod() string {
	method := configManager.Metadata.StorageMethod
	if method == "" {
		return "json" // default
	}
	return method
}

// GetMetadataLinkRegex ..
func GetMetadataLinkRegex() []string {
	return configManager.Metadata.LinkRegex
}
