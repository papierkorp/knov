package server

import (
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/testdata"
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
	if err := testdata.SetupTestData(); err != nil {
		notify.SetFlash(notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to setup test data"))
		http.Error(w, "failed to setup test data", http.StatusInternalServerError)
		return
	}
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "test data setup completed"))
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
	if err := testdata.CleanTestData(); err != nil {
		notify.SetFlash(notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to clean test data"))
		http.Error(w, "failed to clean test data", http.StatusInternalServerError)
		return
	}
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "test data cleaned"))
	writeResponse(w, r, map[string]string{"status": "ok", "message": "test data cleaned"}, "")
}

// @Summary Run filter tests
// @Description Executes comprehensive filter test scenarios with 12 test metadata objects
// @Tags testdata
// @Produce json,html
// @Success 200 {object} testdata.FilterTestResults "filter test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/filtertest [post]
func handleAPIFilterTest(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug("filter test request received")

	results, err := testdata.RunFilterTests()
	if err != nil {
		logging.LogError("failed to run filter tests: %v", err)
		notify.SetFlash(notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to run filter tests: %v", err))
		writeResponse(w, r, nil, "")
		return
	}

	// filter test results are real content, not just a status — keep the HTML swap
	html := render.RenderFilterTestResults(results)
	writeResponse(w, r, results, html)
}

// @Summary Get filter test metadata table
// @Description Returns filter test metadata in table format showing all 12 test objects
// @Tags testdata
// @Produce json,html
// @Success 200 {object} string "filter test metadata table"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/filtertest/testdata [get]
func handleAPIFilterTestMetadata(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug("filter test metadata table request received")

	metadataList := testdata.GetFilterTestMetadata()
	if metadataList == nil {
		notify.SetFlash(notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get filter test metadata"))
		writeResponse(w, r, nil, "")
		return
	}

	html := render.RenderFilterTestMetadataTable(metadataList)
	writeResponse(w, r, metadataList, html)
}
