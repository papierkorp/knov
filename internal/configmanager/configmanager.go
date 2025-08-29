// Package configmanager ..
package configmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"

	"knov/internal/translation"
)

// -----------------------------------------------------------------------------
// ------------------------------- configManager -------------------------------
// -----------------------------------------------------------------------------

var configManager ConfigManager

// ConfigManager ..
type ConfigManager struct {
	Themes  ConfigThemes  `json:"themes"`
	General ConfigGeneral `json:"general"`
	Git     ConfigGit     `json:"git"`
}

// ConfigGeneral ..
type ConfigGeneral struct {
	Language string `json:"language"`
}

// ConfigThemes ..
type ConfigThemes struct {
	CurrentTheme string `json:"currentTheme"`
}

// ConfigGit ..
type ConfigGit struct {
	RepositoryURL string `json:"repositoryUrl"`
	DataPath      string `json:"dataPath"`
}

// InitConfig intializing config/config.json
func InitConfig() {
	jsonFile, err := os.ReadFile("config/config.json")
	if err != nil {
		translation.Sprintf("testmessage from configmanager")
		log.Printf("unable to open config.json file: %s", err)
	}

	if len(jsonFile) == 0 {
		log.Printf("config.json file is empty")
	}

	if !json.Valid(jsonFile) {
		log.Printf("config.json contains invalid JSON")
	}

	decoder := json.NewDecoder(bytes.NewBuffer(jsonFile))
	if err := decoder.Decode(&configManager); err != nil {
		log.Printf("failed to decode config.json: %s", err)
	}

	SetLanguage(GetLanguage())

	if err := initGitRepository(); err != nil {
		log.Printf("failed to initialize git repository: %s", err)
	}

}

func saveConfigToFile() error {
	jsonData, err := json.MarshalIndent(configManager, "", " ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %s", err)
	}

	if err = os.WriteFile("config/config.json", jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write config to file: %s", err)
	}

	log.Printf("DEBUG config: %s", jsonData)
	log.Printf("config saved successfully")
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

// GetLanguage ..
func GetLanguage() string {
	if configManager.General.Language == "" {
		return "en"
	}
	return configManager.General.Language
}

// SetLanguage ..
func SetLanguage(lang string) {
	configManager.General.Language = lang
	log.Printf("DEBUG setlanguage: %s", lang)
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

func initGitRepository() error {
	dataDir := configManager.Git.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	gitDir := dataDir + "/.git"
	if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
		log.Printf("git repository already exists in %s", dataDir)
		return nil
	}

	if configManager.Git.RepositoryURL != "" && configManager.Git.RepositoryURL != "local" {
		// Clone existing repository
		cmd := exec.Command("git", "clone", configManager.Git.RepositoryURL, dataDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
		log.Printf("git repository cloned from %s to %s", configManager.Git.RepositoryURL, dataDir)
	} else {
		// Initialize new repository
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			if err := os.MkdirAll(dataDir, 0755); err != nil {
				return fmt.Errorf("failed to create data directory: %w", err)
			}
		}

		cmd := exec.Command("git", "init")
		cmd.Dir = dataDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to initialize git repository: %w", err)
		}
		log.Printf("git repository initialized in %s", dataDir)
	}

	return nil
}
