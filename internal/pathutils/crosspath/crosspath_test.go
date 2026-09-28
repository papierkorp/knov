package crosspath

import "testing"

func TestToSlash(t *testing.T) {
	if got := ToSlash(`a\b/c`); got != "a/b/c" {
		t.Errorf("ToSlash = %q", got)
	}
}

func TestIsWindowsAbs(t *testing.T) {
	for _, p := range []string{`C:\x`, "c:/x", `Z:\`} {
		if !IsWindowsAbs(p) {
			t.Errorf("%q not detected", p)
		}
	}
	for _, p := range []string{"C:", "C:x", "CC:/x", "1:/x", "/x", "x/y", "http://x", ""} {
		if IsWindowsAbs(p) {
			t.Errorf("%q detected", p)
		}
	}
}
