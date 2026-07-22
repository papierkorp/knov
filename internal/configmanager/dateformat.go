package configmanager

import (
	"time"

	"knov/internal/logging"
)

// GetTimezone returns the configured timezone location, falling back to time.Local on error.
func GetTimezone() *time.Location {
	tz := Timezone.Get()
	loc, err := time.LoadLocation(tz)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "timezone '%s' not supported, falling back to local time", tz)
		return time.Local
	}
	return loc
}

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
	logging.LogWarning(logging.KeyApp, "date format '%s' not supported, falling back to 'DD.MM.YYYY'", style)
	return "DD.MM.YYYY"
}

// GetDateFormat returns the configured date display style (e.g. "DD.MM.YYYY").
func GetDateFormat() string {
	return CheckDateFormat(DateFormat.Get())
}

// SetDateFormat updates user settings with a new date display style.
func SetDateFormat(style string) {
	DateFormat.SetFromString(CheckDateFormat(style)) //nolint:errcheck // pre-validated by CheckDateFormat
	SaveSettings()                                   //nolint:errcheck
}

// FormatDate formats t as a date only, using the configured display style and timezone.
func FormatDate(t time.Time) string {
	return t.In(GetTimezone()).Format(dateLayouts[GetDateFormat()])
}

// FormatDateTime formats t as date + time (HH:MM), using the configured display style and timezone.
func FormatDateTime(t time.Time) string {
	return t.In(GetTimezone()).Format(dateLayouts[GetDateFormat()] + " 15:04")
}

// dateTimeSecondsLayout is the Go reference-time layout for a date + time
// (HH:MM:SS) value, shared by FormatDateTimeSeconds and ParseDateTimeSeconds
// so they can never drift apart.
func dateTimeSecondsLayout() string {
	return dateLayouts[GetDateFormat()] + " 15:04:05"
}

// FormatDateTimeSeconds formats t as date + time (HH:MM:SS), using the configured display style and timezone.
func FormatDateTimeSeconds(t time.Time) string {
	return t.In(GetTimezone()).Format(dateTimeSecondsLayout())
}

// ParseDateTimeSeconds parses a string produced by FormatDateTimeSeconds back
// into a time.Time, using the configured display style and timezone.
func ParseDateTimeSeconds(s string) (time.Time, error) {
	return time.ParseInLocation(dateTimeSecondsLayout(), s, GetTimezone())
}

// FormatTime formats t as time only (HH:MM:SS), using the configured timezone.
func FormatTime(t time.Time) string {
	return t.In(GetTimezone()).Format("15:04:05")
}
