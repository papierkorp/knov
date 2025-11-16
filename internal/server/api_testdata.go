package server

import (
	"net/http"

	"knov/internal/server/render"
	"knov/internal/testdata"
)

// @Summary Setup test data
// @Description Creates test files, git operations, and metadata for testing
// @Tags testdata
// @Produce json,html
// @Success 200 {object} string "{"status":"ok","message":"test data setup completed"}"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/setup [post]
func handleAPISetupTestData(w http.ResponseWriter, r *http.Request) {
	err := testdata.SetupTestData()
	if err != nil {
		http.Error(w, "failed to setup test data", http.StatusInternalServerError)
		return
	}

	data := map[string]string{"status": "ok", "message": "test data setup completed"}
	html := render.RenderStatusMessage(render.StatusOK, "test data setup completed")
	writeResponse(w, r, data, html)
}

// @Summary Clean test data
// @Description Removes all test data files and metadata
// @Tags testdata
// @Produce json,html
// @Success 200 {object} string "{"status":"ok","message":"test data cleaned"}"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/clean [post]
func handleAPICleanTestData(w http.ResponseWriter, r *http.Request) {
	err := testdata.CleanTestData()
	if err != nil {
		http.Error(w, "failed to clean test data", http.StatusInternalServerError)
		return
	}

	data := map[string]string{"status": "ok", "message": "test data cleaned"}
	html := render.RenderStatusMessage(render.StatusOK, "test data cleaned")
	writeResponse(w, r, data, html)
}

