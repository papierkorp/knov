package configmanager

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// IsAllowedMediaType reports whether mimeType or, failing that, fileName's extension is allowed
// by the allowed media types setting.
func IsAllowedMediaType(fileName, mimeType string) bool {
	return isAllowedMimeType(mimeType) || IsAllowedMediaExtension(fileName)
}

// isAllowedMimeType reports whether mimeType (parameters like "; charset=utf-8" are ignored) matches
// the allowed media types setting, exactly or via a wildcard pattern like "image/*".
func isAllowedMimeType(mimeType string) bool {
	mimeType, _, _ = strings.Cut(mimeType, ";")
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	for _, allowedType := range GetAllowedMediaTypes() {
		allowedType = strings.ToLower(strings.TrimSpace(allowedType))
		if allowedType == mimeType {
			return true
		}
		if category, ok := strings.CutSuffix(allowedType, "/*"); ok && strings.HasPrefix(mimeType, category+"/") {
			return true
		}
	}
	return false
}

// IsAllowedMediaExtension reports whether fileName's extension (e.g. ".excalidraw") is listed in
// the allowed media types setting.
func IsAllowedMediaExtension(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return ext != "" && slices.ContainsFunc(GetAllowedMediaTypes(), func(allowedType string) bool {
		return strings.ToLower(strings.TrimSpace(allowedType)) == ext
	})
}

// validateAllowedMediaTypes rejects entries that are neither a mime type ("image/png", "image/*")
// nor a single file extension (".excalidraw").
func validateAllowedMediaTypes(entries []string) error {
	for _, e := range entries {
		isExt := len(e) > 1 && strings.LastIndex(e, ".") == 0
		kind, subtype, _ := strings.Cut(e, "/")
		isMime := kind != "" && subtype != "" && !strings.Contains(subtype, "/")
		if !isExt && !isMime {
			return fmt.Errorf("invalid entry %q, use a mime type like image/png or a file extension like .excalidraw", e)
		}
	}
	return nil
}
