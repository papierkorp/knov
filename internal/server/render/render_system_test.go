package render

import "testing"

func TestReleaseRangeFilter(t *testing.T) {
	versions := []string{"1.0.0", "1.1.0", "1.2.0"}
	if releaseRangeFilter(versions, "", "1.2.0") != nil || releaseRangeFilter(versions, "0.9.0", "1.2.0") != nil {
		t.Fatal("empty or unknown range should keep everything")
	}
	keep := releaseRangeFilter(versions, "1.0.0", "1.2.0")
	for name, want := range map[string]bool{"v1.0.0.md": false, "v1.1.0.md": true, "v1.2.0.md": true, "unreleased.md": false} {
		if got := keep(name); got != want {
			t.Errorf("keep(%q) = %v, want %v", name, got, want)
		}
	}
}
