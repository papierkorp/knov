// Package configmanager ..
package configmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

var configManager ConfigManager

// ConfigManager ..
type ConfigManager struct {
	Themes ConfigThemes `json:"themes"`
}

// ConfigThemes ..
type ConfigThemes struct {
	CurrentTheme string `json:"currentTheme"`
}

// InitConfig intializing config/config.json
func InitConfig() error {
	jsonFile, err := os.ReadFile("config/config.json")
	if err != nil {
		return fmt.Errorf("unable to open config.json file: %s", err)
	}

	if len(jsonFile) == 0 {
		return fmt.Errorf("config.json file is empty")
	}

	if !json.Valid(jsonFile) {
		return fmt.Errorf("config.json contains invalid JSON")
	}

	decoder := json.NewDecoder(bytes.NewBuffer(jsonFile))
	if err := decoder.Decode(&configManager); err != nil {
		return fmt.Errorf("failed to decode config.json: %s", err)
	}

	log.Printf("configManager: %+v", configManager)
	log.Printf("configManager.themes: %+v", configManager.Themes)
	log.Printf("configManager current theme: %+v", configManager.Themes.CurrentTheme)

	return nil
}

func saveConfigToFile() error {
	jsonData, err := json.MarshalIndent(configManager, "", " ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %s", err)
	}

	if err = os.WriteFile("config/config.json", jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write config to file: %s", err)
	}

	log.Printf("config saved successfully")
	return nil
}

// GetConfigThemes ..
func GetConfigThemes() ConfigThemes {
	return configManager.Themes
}

// SetConfigThemes ..
func SetConfigThemes(newConfigThemes ConfigThemes) {
	configManager.Themes = newConfigThemes
	saveConfigToFile()
}
