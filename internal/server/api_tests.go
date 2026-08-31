package server

import (
	"errors"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/job"
	"knov/internal/server/notify"
	"knov/internal/translation"
)

// @Summary Setup test data
// @Description Creates test files, git operations, and metadata for testing
// @Tags testdata
// @Produce json,html
// @Success 200 {object} string "{"status":"ok","message":"test data setup completed"}"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/setup [post]
func handleAPISetupTestData(w http.ResponseWriter, r *http.Request) {
	if err := job.RunTestdataSetup(); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "test data setup completed"))
	writeResponse(w, r, map[string]string{"status": "ok", "message": "test data setup completed"}, "")
}

// @Summary Clean test data
// @Description Removes all test data files and metadata
// @Tags testdata
// @Produce json,html
// @Success 200 {object} string "{"status":"ok","message":"test data cleaned"}"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/clean [post]
func handleAPICleanTestData(w http.ResponseWriter, r *http.Request) {
	if err := job.RunTestdataClean(); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "test data cleaned"))
	writeResponse(w, r, map[string]string{"status": "ok", "message": "test data cleaned"}, "")
}
