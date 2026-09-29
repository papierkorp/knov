package files

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

func TestMediaCategoryStatsAdd(t *testing.T) {
	var s MediaCategoryStats
	s.add(10, false)
	s.add(5, true)
	s.add(0, true)
	if s.TotalFiles != 3 || s.TotalSize != 15 || s.UsedFiles != 1 || s.UsedSize != 10 || s.OrphanedFiles != 2 || s.OrphanedSize != 5 {
		t.Errorf("unexpected stats: %+v", s)
	}
}

func TestGetFileTypeIcon(t *testing.T) {
	cases := map[string]string{
		".png":  "fa-image",
		".PNG":  "fa-image",
		".docx": "fa-file-word",
		".odt":  "fa-file-word",
		".md":   "fa-file-alt",
		".zip":  "fa-file-archive",
		".txt":  "fa-file-alt",
		"":      "fa-file",
	}
	for ext, want := range cases {
		if got := GetFileTypeIcon(ext); got != want {
			t.Errorf("GetFileTypeIcon(%q) = %q, want %q", ext, got, want)
		}
	}
}
