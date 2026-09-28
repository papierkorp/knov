// Package crosspath handles linux and windows path syntax the same on every host OS - for paths
// typed by users or written in file content (links, settings), which may come from either OS.
// Unlike filepath, which only knows the host's own syntax. Has no knov imports, so every package
// (including configmanager, which pathutils itself depends on) can use it.
package crosspath

import "strings"

// ToSlash converts both "\" and "/" separators to "/" - filepath.ToSlash leaves "\" alone on linux.
// Only for path text (links, settings, input) - for real filesystem paths use pathutils.ToSlash.
func ToSlash(path string) string {
	return strings.ReplaceAll(path, `\`, "/")
}

// IsWindowsAbs reports whether path is an absolute windows drive path (e.g. "C:\x", "c:/x") -
// filepath.IsAbs only detects that on windows.
func IsWindowsAbs(path string) bool {
	return len(path) >= 3 && 'a' <= path[0]|0x20 && path[0]|0x20 <= 'z' && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}
