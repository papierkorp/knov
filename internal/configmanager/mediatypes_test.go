package configmanager

import "testing"

func TestIsAllowedMediaType(t *testing.T) {
	old := AllowedMediaTypes.Default
	AllowedMediaTypes.Default = []string{"application/pdf", "image/*", "text/plain", ".Excalidraw"}
	defer func() { AllowedMediaTypes.Default = old }()

	cases := []struct {
		fileName, mimeType string
		want               bool
	}{
		{"a.pdf", "application/pdf", true},
		{"a.png", "image/png", true},
		{"a.txt", "text/plain; charset=utf-8", true},
		{"a.html", "text/html; charset=utf-8", false},
		{"a.excalidraw", "application/json", true},
		{"a.EXCALIDRAW", "", true},
		{"excalidraw", "", false},
		{"a.zip", "application/zip", false},
	}
	for _, c := range cases {
		if got := IsAllowedMediaType(c.fileName, c.mimeType); got != c.want {
			t.Errorf("IsAllowedMediaType(%q, %q) = %v, want %v", c.fileName, c.mimeType, got, c.want)
		}
	}
}

func TestValidateAllowedMediaTypes(t *testing.T) {
	if err := validateAllowedMediaTypes([]string{"image/*", "application/pdf", ".excalidraw"}); err != nil {
		t.Errorf("valid entries rejected: %v", err)
	}
	for _, bad := range []string{"excalidraw", ".tar.gz", ".", "", "/", "image/", "/png", "a/b/c"} {
		if validateAllowedMediaTypes([]string{bad}) == nil {
			t.Errorf("entry %q accepted, want error", bad)
		}
	}
}
