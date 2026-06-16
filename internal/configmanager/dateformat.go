package configmanager

import (
	"time"

	"knov/internal/logging"
)

// -----------------------------------------------------------------------------
// -------------------------------- Date Format ---------------------------------
// -----------------------------------------------------------------------------

// dateLayouts maps a user-facing date style to its Go reference-time layout.
var dateLayouts = map[string]string{
	"DD.MM.YYYY": "02.01.2006",
	"YYYY-MM-DD": "2006-01-02",
	"MM/DD/YYYY": "01/02/2006",
	"DD/MM/YYYY": "02/01/2006",
}

// GetAvailableDateFormats returns the supported date style keys.
func GetAvailableDateFormats() []string {
	return []string{"DD.MM.YYYY", "YYYY-MM-DD", "MM/DD/YYYY", "DD/MM/YYYY"}
}

// CheckDateFormat validates a date style key, falling back to the default if unknown.
func CheckDateFormat(style string) string {
	if _, ok := dateLayouts[style]; ok {
		return style
	}
	logging.LogWarning("date format '%s' not supported, falling back to 'DD.MM.YYYY'", style)
	return "DD.MM.YYYY"
}

// GetDateFormat returns the configured date display style (e.g. "DD.MM.YYYY").
func GetDateFormat() string {
	return CheckDateFormat(userSettings.DateFormat)
}

// SetDateFormat updates user settings with a new date display style.
func SetDateFormat(style string) {
	userSettings.DateFormat = CheckDateFormat(style)
	saveUserSettings()
}

// FormatDate formats t as a date only, using the configured display style.
func FormatDate(t time.Time) string {
	return t.Format(dateLayouts[GetDateFormat()])
}

// FormatDateTime formats t as date + time (HH:MM), using the configured display style for the date part.
func FormatDateTime(t time.Time) string {
	return t.Format(dateLayouts[GetDateFormat()] + " 15:04")
}
