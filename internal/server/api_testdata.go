package server

import (
	"errors"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/job"
	"knov/internal/logging"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/test/filtertest"
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

// @Summary Run filter tests
// @Description Executes comprehensive filter test scenarios with 12 test metadata objects
// @Tags testdata
// @Produce json,html
// @Success 200 {object} test.SuiteResult "filter test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/filtertest [post]
func handleAPIFilterTest(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug("filter test request received")

	results, err := job.RunFilterTest()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError("failed to run filter tests: %v", err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}

	html := render.RenderSuiteResult(results)
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

	metadataList := filtertest.GetFilterTestMetadata()
	if metadataList == nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get filter test metadata"))
		writeResponse(w, r, nil, "")
		return
	}

	html := render.RenderFilterTestMetadataTable(metadataList)
	writeResponse(w, r, metadataList, html)
}

// @Summary Run all test suites
// @Description Executes every registered in-app test suite and aggregates the results
// @Tags testdata
// @Produce json,html
// @Success 200 {object} test.SuiteResult "aggregated test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/run-all [post]
func handleAPIRunAllTests(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug("run all tests request received")

	results, err := job.RunAllTests()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError("failed to run all tests: %v", err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}

	html := render.RenderSuiteResult(results)
	writeResponse(w, r, results, html)
}
