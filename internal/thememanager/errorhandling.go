package thememanager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ThemeValidationError represents validation errors for themes
type ThemeValidationError struct {
	ThemeName string
	Errors    []string
}

func (e ThemeValidationError) Error() string {
	return fmt.Sprintf("theme '%s' validation failed: %s", e.ThemeName, strings.Join(e.Errors, ", "))
}

// NewThemeValidationError creates a new theme validation error
func NewThemeValidationError(themeName string, errors []string) error {
	if len(errors) == 0 {
		return nil
	}
	return ThemeValidationError{
		ThemeName: themeName,
		Errors:    errors,
	}
}

// ThemeNotFoundError represents a theme not found error
type ThemeNotFoundError struct {
	ThemeName string
}

func (e ThemeNotFoundError) Error() string {
	return fmt.Sprintf("theme '%s' not found", e.ThemeName)
}

// NewThemeNotFoundError creates a new theme not found error
func NewThemeNotFoundError(themeName string) error {
	return ThemeNotFoundError{ThemeName: themeName}
}

// TemplateNotFoundError represents a template not found error
type TemplateNotFoundError struct {
	TemplateName string
	ThemeName    string
}

func (e TemplateNotFoundError) Error() string {
	return fmt.Sprintf("template '%s' not found in theme '%s'", e.TemplateName, e.ThemeName)
}

// NewTemplateNotFoundError creates a new template not found error
func NewTemplateNotFoundError(templateName, themeName string) error {
	return TemplateNotFoundError{
		TemplateName: templateName,
		ThemeName:    themeName,
	}
}

// ThemeExtractionError represents an error during theme extraction
type ThemeExtractionError struct {
	ThemeName string
	Cause     error
}

func (e ThemeExtractionError) Error() string {
	return fmt.Sprintf("failed to extract theme '%s': %v", e.ThemeName, e.Cause)
}

func (e ThemeExtractionError) Unwrap() error {
	return e.Cause
}

// NewThemeExtractionError creates a new theme extraction error
func NewThemeExtractionError(themeName string, cause error) error {
	return ThemeExtractionError{
		ThemeName: themeName,
		Cause:     cause,
	}
}

// ValidateTheme validates a theme directory structure
func ValidateTheme(themeName, themePath string) error {
	var errors []string

	// Check if theme directory exists
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		errors = append(errors, "theme directory does not exist")
		return NewThemeValidationError(themeName, errors)
	}

	// Validate required templates exist
	for _, requiredTemplate := range RequiredTemplates {
		templatePath := filepath.Join(themePath, requiredTemplate)
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("missing required template: %s", requiredTemplate))
		}
	}

	// Validate theme.json if it exists
	metadataPath := filepath.Join(themePath, "theme.json")
	if _, err := os.Stat(metadataPath); err == nil {
		if err := validateThemeMetadata(metadataPath); err != nil {
			errors = append(errors, fmt.Sprintf("invalid theme.json: %v", err))
		}
	}

	// Validate style.css exists
	stylePath := filepath.Join(themePath, "style.css")
	if _, err := os.Stat(stylePath); os.IsNotExist(err) {
		errors = append(errors, "missing style.css file")
	}

	// Validate template syntax (basic check)
	for _, templateName := range RequiredTemplates {
		templatePath := filepath.Join(themePath, templateName)
		if _, err := os.Stat(templatePath); err == nil {
			if err := validateTemplateSyntax(templatePath); err != nil {
				errors = append(errors, fmt.Sprintf("template %s has syntax errors: %v", templateName, err))
			}
		}
	}

	return NewThemeValidationError(themeName, errors)
}

// validateThemeMetadata validates the theme.json file structure
func validateThemeMetadata(metadataPath string) error {
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("cannot read theme.json: %w", err)
	}

	var metadata ThemeMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("invalid json format: %w", err)
	}

	// Validate required fields
	if metadata.Name == "" {
		return fmt.Errorf("missing required field: name")
	}
	if metadata.Version == "" {
		return fmt.Errorf("missing required field: version")
	}
	if metadata.Author == "" {
		return fmt.Errorf("missing required field: author")
	}

	// Validate color schemes format
	for i, scheme := range metadata.Features.ColorSchemes {
		if scheme.Name == "" {
			return fmt.Errorf("color scheme %d missing name", i)
		}
		if scheme.Label == "" {
			return fmt.Errorf("color scheme %d missing label", i)
		}
	}

	return nil
}

// validateTemplateSyntax performs basic template syntax validation
func validateTemplateSyntax(templatePath string) error {
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("cannot read template: %w", err)
	}

	content := string(data)

	// Basic validation checks
	if !strings.Contains(content, "{{define") {
		return fmt.Errorf("template must contain at least one {{define}} block")
	}

	// Check for balanced template delimiters
	openCount := strings.Count(content, "{{")
	closeCount := strings.Count(content, "}}")
	if openCount != closeCount {
		return fmt.Errorf("unbalanced template delimiters: %d open, %d close", openCount, closeCount)
	}

	// Check for common template syntax errors
	if strings.Contains(content, "{{.") && !strings.Contains(content, "}}") {
		return fmt.Errorf("unclosed template expression")
	}

	return nil
}

// ValidateThemeCompatibility checks if a theme is compatible with current system
func ValidateThemeCompatibility(themeName string, metadata ThemeMetadata) error {
	var errors []string

	// Check minimum required version (if we have version requirements)
	// if metadata.MinSystemVersion != "" {
	//     // Version compatibility check would go here
	// }

	// Validate view names don't contain invalid characters
	allViews := [][]string{
		metadata.Views.File, metadata.Views.Home, metadata.Views.Search,
		metadata.Views.Overview, metadata.Views.Dashboard, metadata.Views.Settings,
		metadata.Views.Admin, metadata.Views.Playground, metadata.Views.History,
		metadata.Views.LatestChanges, metadata.Views.BrowseFiles,
	}

	for _, viewList := range allViews {
		for _, view := range viewList {
			if strings.ContainsAny(view, "/\\:*?\"<>|") {
				errors = append(errors, fmt.Sprintf("invalid view name '%s': contains illegal characters", view))
			}
		}
	}

	// Validate color scheme names
	for _, scheme := range metadata.Features.ColorSchemes {
		if strings.ContainsAny(scheme.Name, " /\\:*?\"<>|") {
			errors = append(errors, fmt.Sprintf("invalid color scheme name '%s': contains illegal characters", scheme.Name))
		}
	}

	return NewThemeValidationError(themeName, errors)
}

// IsThemeValidationError checks if an error is a theme validation error
func IsThemeValidationError(err error) bool {
	_, ok := err.(ThemeValidationError)
	return ok
}

// IsThemeNotFoundError checks if an error is a theme not found error
func IsThemeNotFoundError(err error) bool {
	_, ok := err.(ThemeNotFoundError)
	return ok
}

// IsTemplateNotFoundError checks if an error is a template not found error
func IsTemplateNotFoundError(err error) bool {
	_, ok := err.(TemplateNotFoundError)
	return ok
}
