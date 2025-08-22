// Package configmanager ..
package configmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// -----------------------------------------------------------------------------
// ------------------------------- configManager -------------------------------
// -----------------------------------------------------------------------------

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
func InitConfig() {
	jsonFile, err := os.ReadFile("config/config.json")
	if err != nil {
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

// -----------------------------------------------------------------------------
// ----------------------------- retrieve/set Data -----------------------------
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
