package thememanager

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/logging"
)

var builtinThemeFS embed.FS

// SetBuiltinThemeFiles sets the embedded builtin theme files
func SetBuiltinThemeFiles(fs embed.FS) {
	builtinThemeFS = fs
}

// InitBuiltinTheme extracts and sets up the builtin theme
func initBuiltinTheme(themesPath string) error {
	builtinPath := filepath.Join(themesPath, "builtin")

	// Check if builtin theme already exists
	if _, err := os.Stat(builtinPath); err == nil {
		return nil // already exists
	}

	logging.LogInfo("extracting builtin theme to %s", builtinPath)

	// Create builtin theme directory
	if err := os.MkdirAll(builtinPath, 0755); err != nil {
		return fmt.Errorf("failed to create builtin theme directory: %w", err)
	}

	// Extract all files from embedded builtin theme
	return extractBuiltinFiles("themes/builtin", builtinPath, builtinThemeFS)
}

func extractBuiltinFiles(embedPath, destPath string, builtinFS embed.FS) error {
	entries, err := builtinFS.ReadDir(embedPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(embedPath, entry.Name())
		destFilePath := filepath.Join(destPath, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(destFilePath, 0755); err != nil {
				return err
			}
			if err := extractBuiltinFiles(srcPath, destFilePath, builtinFS); err != nil {
				return err
			}
		} else {
			data, err := builtinFS.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(destFilePath, data, 0644); err != nil {
				return err
			}
			logging.LogInfo("extracted %s", destFilePath)
		}
	}
	return nil
}

// LoadBuiltinTemplateContent loads content from builtin theme for fallback
func LoadBuiltinTemplateContent(templateName string, builtinFS embed.FS) ([]byte, error) {
	builtinPath := filepath.Join("themes/builtin", templateName)
	return builtinFS.ReadFile(builtinPath)
}
