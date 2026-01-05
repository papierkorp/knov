package server

import (
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/storage"
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
		errMsg := translation.SprintfForRequest(configmanager.GetLanguage(), "failed to run filter tests: %v", err)
		html := render.RenderStatusMessage(render.StatusError, errMsg)
		writeResponse(w, r, nil, html)
		return
	}

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
		errMsg := translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get filter test metadata")
		html := render.RenderStatusMessage(render.StatusError, errMsg)
		writeResponse(w, r, nil, html)
		return
	}

	html := render.RenderFilterTestMetadataTable(metadataList)
	writeResponse(w, r, metadataList, html)
}

// @Summary Download filter test log
// @Description Downloads the filter test log from cache storage
// @Tags testdata
// @Produce text/plain
// @Param key path string true "log key"
// @Success 200 {string} string "log content"
// @Failure 404 {object} string "log not found"
// @Failure 500 {object} string "internal server error"
// @Router /api/testdata/filtertest/log/{key} [get]
func handleAPIDownloadFilterTestLog(w http.ResponseWriter, r *http.Request) {
	logKey := r.URL.Query().Get("key")
	if logKey == "" {
		http.Error(w, "missing log key", http.StatusBadRequest)
		return
	}

	cacheStorage := storage.GetCacheStorage()
	logContent, err := cacheStorage.Get(logKey)
	if err != nil {
		logging.LogDebug("log not found in cache: %s", logKey)
		http.NotFound(w, r)
		return
	}

	// set headers for download
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"filter-test-log.txt\"")

	w.Write(logContent)
	logging.LogDebug("served filter test log from cache: %s", logKey)
}
