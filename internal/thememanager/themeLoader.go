// Package thememanager provides theme loading functionality
package thememanager

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"strings"
)

// loadThemes loads all themes from the themes directory
func loadThemes() (map[string]ITheme, map[string]ThemeInfo, error) {
	themes := make(map[string]ITheme)
	themeInfos := make(map[string]ThemeInfo)
	themesDir := "./data/themes"

	absThemesDir, err := filepath.Abs(themesDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path: %v", err)
	}
	log.Printf("Loading themes from: %s", absThemesDir)

	// First, compile all themes
	log.Println("Compiling themes...")
	err = compileThemes(absThemesDir)
	if err != nil {
		log.Printf("⚠️  Warning: Error compiling themes: %v", err)
	}

	// Then load the compiled themes
	files, err := os.ReadDir(absThemesDir)
	if err != nil {
		log.Printf("Error reading themes directory: %v", err)
		return themes, themeInfos, err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		themeName := file.Name()
		themeDir := filepath.Join(absThemesDir, themeName)
		mainPath := filepath.Join(themeDir, "main.go")
		soPath := filepath.Join(themeDir, strings.ToLower(themeName)+".so")

		log.Printf("Processing theme directory: %s", themeDir)

		// Check if this is a valid theme directory
		if mainFileInfo, err := os.Stat(mainPath); err != nil || mainFileInfo.IsDir() {
			log.Printf("Skipping directory %s - not a valid theme (no main.go found)", themeName)
			continue
		}

		// Load theme metadata
		if info, err := loadThemeMetadata(themeDir); err == nil {
			themeInfos[themeName] = info
			log.Printf("✅ Loaded metadata for theme: %s", themeName)
		} else {
			log.Printf("⚠️  Warning: Could not load metadata for theme %s: %v", themeName, err)
			// Create fallback metadata
			themeInfos[themeName] = ThemeInfo{
				Name:        themeName,
				DisplayName: themeName,
				Description: "Theme without metadata",
				Tags:        []string{"unknown"},
			}
		}

		// Load the compiled theme
		log.Printf("Loading compiled theme: %s", themeName)
		theme, err := loadThemeFromFile(soPath)
		if err != nil {
			log.Printf("⚠️  Could not load theme %s: %v", themeName, err)
			continue
		}

		themes[themeName] = theme
		log.Printf("✅ Successfully loaded theme: %s", themeName)
	}

	log.Printf("Found %d themes", len(themes))
	for name := range themes {
		log.Printf("✅ Available theme: %s", name)
	}

	return themes, themeInfos, nil
}

// compileThemes compiles all theme main.go files to .so files
func compileThemes(absDir string) error {
	files, err := os.ReadDir(absDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		themeName := file.Name()
		mainPath := filepath.Join(absDir, themeName, "main.go")
		outPath := filepath.Join(absDir, themeName, strings.ToLower(themeName)+".so")

		log.Printf("Checking theme directory: %s", themeName)

		if mainFileInfo, err := os.Stat(mainPath); err == nil && !mainFileInfo.IsDir() {
			log.Printf("Compiling theme: %s", themeName)

			// Remove existing .so file
			os.Remove(outPath)

			cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", outPath, ".")
			cmd.Dir = filepath.Dir(mainPath)

			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("⚠️  Could not compile theme %s: %v\nOutput: %s", themeName, err, string(output))
				continue
			}
			log.Printf("✅ Successfully compiled theme: %s", themeName)
		} else {
			log.Printf("No main.go found for theme %s", themeName)
		}
	}

	return nil
}

// loadThemeFromFile loads a theme from a compiled .so file
func loadThemeFromFile(path string) (ITheme, error) {
	// Check if file exists before attempting to open
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("theme file does not exist: %v", err)
	}

	plug, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open theme: %v", err)
	}

	symTheme, err := plug.Lookup("Theme")
	if err != nil {
		return nil, fmt.Errorf("could not find Theme symbol: %v", err)
	}

	var t ITheme
	switch v := symTheme.(type) {
	case ITheme:
		t = v
	case *ITheme:
		t = *v
	default:
		return nil, fmt.Errorf("invalid theme type: %T", symTheme)
	}

	return t, nil
}
