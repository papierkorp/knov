package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"knov/internal/notificationStorage"
)

// writeAPIError reaches notify.SetHeader, which persists every notification, so
// the storage backend has to exist before any test in this package runs.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "knov-server-test")
	if err != nil {
		panic(err)
	}
	if err := notificationStorage.Init(dir); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestWriteAPIErrorContentNegotiation(t *testing.T) {
	cases := []struct {
		name       string
		accept     string
		omitAccept bool
		wantCT     string
		wantBody   []string
		notWant    []string
	}{
		{name: "html Accept gets an inline status-error span", accept: "text/html", wantCT: "text/html", wantBody: []string{`class="status-error"`, "boom"}, notWant: []string{`"error":`}},
		{name: "bare */* (htmx/browser) gets html", accept: "*/*", wantCT: "text/html", wantBody: []string{`class="status-error"`, "boom"}, notWant: []string{`"error":`}},
		{name: "json Accept gets a JSON error object", accept: "application/json", wantCT: "application/json", wantBody: []string{`"error":"boom"`}, notWant: []string{"<span"}},
		{name: "explicit json wins over a trailing */*", accept: "application/json, */*", wantCT: "application/json", wantBody: []string{`"error":"boom"`}, notWant: []string{"<span"}},
		{name: "missing Accept defaults to JSON", omitAccept: true, wantCT: "application/json", wantBody: []string{`"error":"boom"`}, notWant: []string{"<span"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/x", nil)
			if !tc.omitAccept {
				req.Header.Set("Accept", tc.accept)
			}
			rec := httptest.NewRecorder()

			writeAPIError(rec, req, http.StatusConflict, "boom")

			if rec.Code != http.StatusConflict {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
			}
			if ct := rec.Header().Get("Content-Type"); ct != tc.wantCT {
				t.Errorf("content-type = %q, want %q", ct, tc.wantCT)
			}
			for _, want := range tc.wantBody {
				if !strings.Contains(rec.Body.String(), want) {
					t.Errorf("body missing %q\ngot: %s", want, rec.Body.String())
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(rec.Body.String(), notWant) {
					t.Errorf("body should not contain %q\ngot: %s", notWant, rec.Body.String())
				}
			}
		})
	}
}

// The html error body is built from error strings and user file paths, so
// writeAPIError must route it through the renderer (which escapes) rather than
// splicing the raw message into markup.
func TestWriteAPIErrorHTMLBodyIsEscaped(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/x", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	writeAPIError(rec, req, http.StatusBadRequest, `<b>&"bad"`)

	body := rec.Body.String()
	if strings.Contains(body, "<b>") {
		t.Errorf("html error body not escaped: %s", body)
	}
	if !strings.Contains(body, "&lt;b&gt;") {
		t.Errorf("html error body missing escaped markup: %s", body)
	}
}

// writeAPIError feeds raw err.Error() / file-path text into the HX-Trigger header;
// it must go through json.Marshal so quotes, backslashes and newlines are escaped
// and the header stays a single valid JSON line.
func TestWriteAPIErrorHXTriggerIsJSONEscaped(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/x", nil)
	rec := httptest.NewRecorder()
	msg := "C:\\Users\\me\\note \"x\".md\n<bad>"

	writeAPIError(rec, req, http.StatusInternalServerError, msg)

	trigger := rec.Header().Get("HX-Trigger")
	if trigger == "" {
		t.Fatal("HX-Trigger header not set")
	}
	if strings.ContainsAny(trigger, "\n\r") {
		t.Errorf("raw newline/CR in header value: %q", trigger)
	}
	var parsed map[string]struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(trigger), &parsed); err != nil {
		t.Fatalf("HX-Trigger is not valid JSON: %v (%q)", err, trigger)
	}
	if got := parsed["notify"].Message; got != msg {
		t.Errorf("round-tripped message = %q, want %q", got, msg)
	}
	if got := parsed["notify"].Type; got != "error" {
		t.Errorf("toast type = %q, want %q", got, "error")
	}
}

// writeAPIError owns the error toast + notification-log entry for a failed
// request, so one call must persist exactly one entry. This does not exercise
// the callers - a handler that also calls notify.SetHeader itself still doubles
// the log entry (that was the api_kanban sync bug); keeping call sites off
// notify.SetHeader when they use writeAPIError is a review rule, not asserted here.
func TestWriteAPIErrorPersistsOneNotification(t *testing.T) {
	msg := fmt.Sprintf("persist-probe-%d", time.Now().UnixNano())

	req := httptest.NewRequest(http.MethodPost, "/api/x", nil)
	writeAPIError(httptest.NewRecorder(), req, http.StatusConflict, msg)

	recent, err := notificationStorage.GetRecent(50)
	if err != nil {
		t.Fatalf("get recent: %v", err)
	}
	matches := 0
	for _, n := range recent {
		if n.Message != msg {
			continue
		}
		matches++
		if n.Level != "error" {
			t.Errorf("notification level = %q, want error", n.Level)
		}
	}
	if matches != 1 {
		t.Fatalf("writeAPIError persisted %d notifications for one call, want 1", matches)
	}
}
