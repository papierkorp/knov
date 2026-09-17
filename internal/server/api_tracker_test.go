package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// A counter_columns[] entry count that doesn't match counter_title[] signals a
// malformed request (e.g. a client bug dropping one row's columns) rather than
// "no preference" for the mismatched rows, so it must be rejected before ever
// reaching tracker.SetMeta.
func TestHandleAPITrackerSaveRejectsMismatchedColumnsLength(t *testing.T) {
	form := url.Values{
		"trackerid":         {"sometracker"},
		"counter_title[]":   {"first", "second"},
		"counter_id[]":      {"", ""},
		"counter_columns[]": {`{"day24h":true}`}, // only one entry for two rows
	}
	req := httptest.NewRequest(http.MethodPost, "/api/trackers/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handleAPITrackerSave(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// A malformed (non-JSON) counter_columns[] entry must also be rejected as a bad
// request rather than silently decoding to the zero value.
func TestHandleAPITrackerSaveRejectsInvalidColumnsJSON(t *testing.T) {
	form := url.Values{
		"trackerid":         {"sometracker"},
		"counter_title[]":   {"first"},
		"counter_id[]":      {""},
		"counter_columns[]": {"not-json"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/trackers/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handleAPITrackerSave(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
