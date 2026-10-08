package utils

import "testing"

func TestNormalizeLinkPathDocsRoot(t *testing.T) {
	if got := NormalizeLinkPath("/"); got != "docs/" {
		t.Errorf("NormalizeLinkPath(\"/\") = %q, want \"docs/\"", got)
	}
}
