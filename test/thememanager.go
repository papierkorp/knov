package main

// -----------------------------------------------
// ------------- Define Thememanager -------------
// -----------------------------------------------

var themeManager *ThemeManager

type ThemeManager struct {
	themes       []Theme
	currentTheme Theme
}

type Theme struct {
	Name     ThemeName
	Metadata ThemeMetadata
}

type ThemeName string

type ThemeMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
}

func InitThemeManager() {
	builtinThemeMetadata := ThemeMetadata{
		Name:        "builtin",
		Version:     "1.0.0",
		Author:      "knov team",
		Description: "default theme",
	}

	builtinTheme := Theme{
		Name:     "builtin",
		Metadata: builtinThemeMetadata,
	}

	themeManager.addTheme(builtinTheme)
	themeManager.setCurrentTheme(builtinTheme)

}

func loadAllThemes() {

}

// -----------------------------------------------
// ---------------- Getter/Setter ----------------
// -----------------------------------------------

func (tm *ThemeManager) addTheme(theme Theme) error {
	// todo validate theme before adding it
	tm.themes = append(tm.themes, theme)

	return nil
}

func (tm *ThemeManager) setCurrentTheme(theme Theme) error {
	// todo validate theme before adding it
	tm.currentTheme = theme

	return nil
}
