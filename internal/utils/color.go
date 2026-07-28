package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// IsHexColor reports whether s is a 6-digit hex color in "#rrggbb" form.
func IsHexColor(s string) bool {
	return hexColorPattern.MatchString(s)
}

// ValidateHexColor returns an error if v isn't a 6-digit hex color in
// "#rrggbb" form. Matches the settings package's Validate func(string) error shape.
func ValidateHexColor(v string) error {
	if !IsHexColor(v) {
		return fmt.Errorf("invalid color %q, expected #rrggbb", v)
	}
	return nil
}

// HexToRGB parses a "#rrggbb" color into its RGB components. Invalid input returns black.
func HexToRGB(hex string) [3]int {
	hex = strings.TrimPrefix(hex, "#")
	n, err := strconv.ParseInt(hex, 16, 32)
	if len(hex) != 6 || err != nil {
		return [3]int{}
	}
	return [3]int{int(n >> 16 & 0xff), int(n >> 8 & 0xff), int(n & 0xff)}
}
