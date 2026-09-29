package types

import "testing"

func TestMediaCategory(t *testing.T) {
	cases := map[string]string{
		"media/a.png":        "image",
		"media/b.JPG":        "image",
		"media/c.mp4":        "video",
		"media/d.flac":       "audio",
		"media/e.pdf":        "document",
		"media/f.tar.gz":     "archive",
		"media/g.excalidraw": "text",
		"media/h.woff2":      "font",
		"media/i.exe":        "program",
		"media/j.unknownext": "other",
		"media/no-extension": "other",
	}
	for path, want := range cases {
		if got := MediaCategory(path); got != want {
			t.Errorf("MediaCategory(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestMimeTypeByExtension(t *testing.T) {
	cases := map[string]string{
		".mp4":        "video/mp4",
		".MP3":        "audio/mpeg",
		".vtt":        "text/vtt",
		".ico":        "image/vnd.microsoft.icon",
		".excalidraw": "",
	}
	for ext, want := range cases {
		if got := MimeTypeByExtension(ext); got != want {
			t.Errorf("MimeTypeByExtension(%q) = %q, want %q", ext, got, want)
		}
	}
}
