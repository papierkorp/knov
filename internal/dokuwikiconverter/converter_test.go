package dokuwikiconverter

import (
	"io"
	"strings"
	"testing"
)

func TestConvertExportEntry(t *testing.T) {
	convert := func(name, content string) (string, string) {
		t.Helper()
		name, r, err := ConvertExportEntry(name, strings.NewReader(content))
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(r)
		return name, string(data)
	}

	if name, data := convert("docs/page.txt", "====== Title ======"); name != "docs/page.md" || data == "====== Title ======" {
		t.Errorf("dokuwiki file = %q %q, want renamed to .md and converted", name, data)
	}
	if name, data := convert("media/c.png", "png"); name != "media/c.png" || data != "png" {
		t.Errorf("other file = %q %q, want unchanged", name, data)
	}
}
