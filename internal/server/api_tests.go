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
	logging.LogDebug(logging.KeyApp, "filter test request received")

	results, err := job.RunFilterTest()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError(logging.KeyApp, "failed to run filter tests: %v", err)
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
	logging.LogDebug(logging.KeyApp, "filter test metadata table request received")

	metadataList := filtertest.GetFilterTestMetadata()
	if metadataList == nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get filter test metadata"))
		writeResponse(w, r, nil, "")
		return
	}

	html := render.RenderFilterTestMetadataTable(metadataList)
	writeResponse(w, r, metadataList, html)
}

// @Summary Run editors tests
// @Description Executes the editors test suite (create/edit/save per editor type, section/table save, todo-toggle, convert-to-markdown, file rename/move, bulk ops)
// @Tags testdata
// @Produce json,html
// @Success 200 {object} test.SuiteResult "editors test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/editorstest [post]
func handleAPIEditorsTest(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug(logging.KeyApp, "editors test request received")

	results, err := job.RunEditorsTest()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError(logging.KeyApp, "failed to run editors tests: %v", err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}

	html := render.RenderSuiteResult(results)
	writeResponse(w, r, results, html)
}

// @Summary Run search tests
// @Description Executes the search suite (title-only, full-content, deleted-file search, empty query, limit truncation)
// @Tags testdata
// @Produce json,html
// @Success 200 {object} test.SuiteResult "search test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/searchtest [post]
func handleAPISearchTest(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug(logging.KeyApp, "search test request received")

	results, err := job.RunSearchTest()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError(logging.KeyApp, "failed to run search tests: %v", err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}

	html := render.RenderSuiteResult(results)
	writeResponse(w, r, results, html)
}

// @Summary Run git history tests
// @Description Executes the git repo/file history suite (latest-changes pagination + collection filter, filename history search, file version history/view/diff/restore, remote push/pull/test-auth)
// @Tags testdata
// @Produce json,html
// @Success 200 {object} test.SuiteResult "git history test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/githistorytest [post]
func handleAPIGitHistoryTest(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug(logging.KeyApp, "git history test request received")

	results, err := job.RunGitHistoryTest()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError(logging.KeyApp, "failed to run git history tests: %v", err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}

	html := render.RenderSuiteResult(results)
	writeResponse(w, r, results, html)
}

// @Summary Run chat tests
// @Description Executes the chat suite (add/delete/get message global + file-scoped, pagination, single move append/new-file, bulk move new-file, bulk delete, file rename/delete cascade)
// @Tags testdata
// @Produce json,html
// @Success 200 {object} test.SuiteResult "chat test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/chattest [post]
func handleAPIChatTest(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug(logging.KeyApp, "chat test request received")

	results, err := job.RunChatTest()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError(logging.KeyApp, "failed to run chat tests: %v", err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}

	html := render.RenderSuiteResult(results)
	writeResponse(w, r, results, html)
}

// @Summary Run dashboard tests
// @Description Executes the dashboard suite (CRUD, rename, export/import round-trip, filter/fileContent/tags/collections/folders widget data)
// @Tags testdata
// @Produce json,html
// @Success 200 {object} test.SuiteResult "dashboard test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/dashboardtest [post]
func handleAPIDashboardTest(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug(logging.KeyApp, "dashboard test request received")

	results, err := job.RunDashboardTest()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError(logging.KeyApp, "failed to run dashboard tests: %v", err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}

	html := render.RenderSuiteResult(results)
	writeResponse(w, r, results, html)
}

// @Summary Run kanban tests
// @Description Executes the kanban suite (board load, filter, search query, sorting, card move + event log, column order persistence, pure helpers)
// @Tags testdata
// @Produce json,html
// @Success 200 {object} test.SuiteResult "kanban test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/kanbantest [post]
func handleAPIKanbanTest(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug(logging.KeyApp, "kanban test request received")

	results, err := job.RunKanbanTest()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError(logging.KeyApp, "failed to run kanban tests: %v", err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}

	html := render.RenderSuiteResult(results)
	writeResponse(w, r, results, html)
}

// @Summary Run all test suites
// @Description Executes every registered in-app test suite and aggregates the results
// @Tags testdata
// @Produce json,html
// @Success 200 {object} test.SuiteResult "aggregated test results"
// @Failure 500 {object} string "Internal server error"
// @Router /api/testdata/run-all [post]
func handleAPIRunAllTests(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug(logging.KeyApp, "run all tests request received")

	results, err := job.RunAllTests()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		logging.LogError(logging.KeyApp, "failed to run all tests: %v", err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		http.Error(w, err.Error(), status)
		return
	}

	html := render.RenderSuiteResult(results)
	writeResponse(w, r, results, html)
}
