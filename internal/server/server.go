// Package server ..
package server

import (
	"embed"
	"fmt"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/server/render"
	_ "knov/internal/server/swagger" // swaggo api docs

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

var staticFiles embed.FS
var docsFiles embed.FS

func SetDocsFiles(files embed.FS) {
	docsFiles = files
	render.SetDocsFiles(files)
}

func SetStaticFiles(files embed.FS) {
	staticFiles = files
}

// StartServerChi ...
func StartServerChi() {
	// ----------------------------------------------------------------------------------------
	// ----------------------------------- define chi server -----------------------------------
	// ----------------------------------------------------------------------------------------
	appConfig := configmanager.GetAppConfig()
	port := appConfig.ServerPort

	fmt.Printf("starting chi http server on http://localhost:%s\n", port)
	r := NewRouter()

	// ----------------------------------------------------------------------------------------
	// ----------------------------------- start chi server -----------------------------------
	// ----------------------------------------------------------------------------------------

	err := http.ListenAndServe(":"+port, r)
	if err != nil {
		fmt.Printf("error starting chi server: %v\n", err)
		return
	}
}

// NewRouter builds the chi router with all routes registered, without starting
// an HTTP listener. Used by StartServerChi and by httptest-based tests.
func NewRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------ template routes ------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/", handleHome)
	r.Get("/home", handleHome)
	r.Get("/system/changelog", render.HandleSystemChangelog)
	r.Get("/system/logs", render.HandleSystemLogs)
	r.Get("/system/version", render.HandleSystemVersion)
	r.Get("/system/jobs", render.HandleSystemJobs)
	r.Get("/settings", handleSettings)
	r.Get("/admin", handleAdmin)
	r.Get("/playground", handlePlayground)
	r.Get("/help", handleHelp)
	r.Get("/history", handleHistory)
	r.Get("/search", handleSearchPage)
	r.Get("/media", handleRedirectToBrowseMedia)
	r.Get("/media/*", handleMedia)

	r.Get("/files", handleRedirectToBrowseFiles)
	r.Get("/files/*", handleFileContent)
	r.Get("/files/edit/*", handleFileEdit)
	r.Get("/files/edittable/*", handleFileEditTable)
	r.Get("/files/history/*", handleHistory)
	r.Get("/files/new/codemirror", handleFileNewCodeMirror)
	r.Get("/files/new/overtype", handleFileNewOverType)
	r.Get("/files/new/list", handleFileNewList)
	r.Get("/files/new/todo", handleFileNewTodo)
	r.Get("/files/new/filter", handleFileNewFilter)
	r.Get("/files/new/index", handleFileNewIndex)

	r.Get("/dashboard", handleDashboardView)
	r.Get("/dashboard/{id}", handleDashboardView)
	r.Get("/dashboard/new", handleDashboardNew)
	r.Get("/dashboard/edit/{id}", handleDashboardEdit)

	r.Get("/browse", handleBrowse)
	r.Get("/browse/files", handleFileOverview)
	r.Get("/browse/media", handleBrowseMedia)
	r.Get("/browse/{metadata}", handleBrowseMetadata)
	r.Get("/browse/{metadata}/{value}", handleBrowseFiles)

	r.Get("/chat", handleChat)

	r.Get("/kanban", handleKanbanSelect)
	r.Get("/kanban/{board}", handleKanbanBoard)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------- static routes -------------------------------------
	// ----------------------------------------------------------------------------------------

	// favicon: serve custom if uploaded, otherwise fall back to embedded default
	r.Get("/favicon.ico", handleFavicon)

	r.Get("/static/*", handleStatic)
	r.Get("/themes/*", handleStatic)
	r.Get("/webfonts/*", handleWebfontsRedirect)

	// ----------------------------------------------------------------------------------------
	// -------------------------------------- api routes --------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/swagger/*", httpSwagger.Handler())
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handleAPIHealth)
		r.Get("/search", handleAPISearch)

		// ----------------------------------------------------------------------------------------
		// ----------------------------------------- FILTER ----------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/filters", func(r chi.Router) {
			r.Post("/", handleAPIFilterFiles)
			r.Get("/value-input", handleAPIGetFilterValueInput)
			r.Get("/criteria-row", handleAPIGetFilterCriteriaRow)
			r.Post("/add-criteria", handleAPIAddFilterCriteria)
			r.Post("/save", handleAPIFilterSave)
			r.Delete("/delete/*", handleAPIFilterDelete)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- EDITOR ----------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/editor", func(r chi.Router) {
			r.Get("/", handleAPIGetEditorHandler)
			r.Post("/indexeditor", handleAPISaveIndexEditor)
			r.Post("/indexeditor/add-entry", handleAPIAddIndexEntry)
			r.Post("/filtereditor", handleAPISaveFilterEditor)
			r.Post("/listeditor", handleAPISaveListEditor)
			r.Post("/todoeditor", handleAPISaveTodoEditor)
			r.Post("/tableeditor", handleAPITableEditorSave)
			r.Get("/tableeditor", handleAPITableEditorForm)
		})

		// ----------------------------------------------------------------------------------------
		// ------------------------------------ system routes ------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/system", func(r chi.Router) {
			r.Post("/restart", handleAPIRestartApp)
			r.Delete("/cache", handleAPIInvalidateCache)
			r.Get("/jobs", handleAPIGetJobs)
		})

		// ----------------------------------------------------------------------------------------
		// ----------------------------------------- LOGS -----------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/logs", func(r chi.Router) {
			r.Get("/", handleAPIGetLogs)
			r.Get("/file", handleAPIGetLogsFile)
			r.Get("/download", handleAPIDownloadLogs)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- CRONJOB ----------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Post("/cronjob", handleAPIRunCronjob)

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- SETTINGS ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Get("/settings", handleAPIGetAllSettings)
		r.Post("/settings", handleAPIBulkSetSettings)
		r.Get("/settings/{section}", handleAPIGetSettingsSection)
		r.Post("/settings/{key}", handleAPISetSetting)

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- THEMES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/themes", func(r chi.Router) {
			r.Get("/", handleAPIGetThemes)
			r.Post("/", handleAPISetTheme)

			// current theme settings routes
			r.Get("/settings", handleAPIGetThemeSettingsForm)
			r.Post("/settings", handleAPIUpdateThemeSetting)

			// RESTful theme settings routes
			r.Route("/{themeName}/settings", func(r chi.Router) {
				r.Get("/", handleAPIGetThemeSettings)
				r.Put("/{settingKey}", handleAPISetThemeSetting)
			})
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- CONFIG ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/config", func(r chi.Router) {
			// GET
			r.Get("/", handleAPIGetConfig)
			r.Get("/datapath", handleAPIGetCurrentDataPath)
			r.Get("/languages", handleAPIGetLanguages)
			r.Get("/repository", handleAPIGetGitRepositoryURL)
			r.Get("/export", handleAPIExportSettings)

			// POST
			r.Post("/import", handleAPIImportSettings)
			r.Post("/repository", handleAPISetGitRepositoryURL)
			r.Post("/datapath", handleAPISetDataPath)

			r.Post("/favicon", handleAPIUploadFavicon)
			r.Delete("/favicon", handleAPIDeleteFavicon)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- FILES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/files", func(r chi.Router) {
			r.Get("/list", handleAPIGetAllFiles)
			r.Get("/tree", handleAPIGetFileTree)
			r.Get("/overview", handleAPIGetFileOverview)
			r.Get("/content/*", handleAPIGetFileContent)
			r.Post("/filter", handleAPIFilterFiles)
			r.Get("/header", handleAPIGetFileHeader)
			r.Get("/raw", handleAPIGetRawContent)
			r.Post("/save", handleAPIFileSave)
			r.Post("/save/", handleAPIFileSave)
			r.Post("/todo-toggle", handleAPIToggleTodoState)
			r.Post("/section/save", handleAPISaveSectionEditor)
			r.Post("/convert-to-markdown", handleAPIConvertFileToMarkdown)
			r.Get("/browse", handleAPIBrowseFiles)
			r.Get("/form", handleAPIFileForm)
			r.Get("/metadata-form", handleAPIMetadataForm)
			r.Get("/folder", handleAPIGetFolder)
			r.Get("/folder-suggestions", handleAPIGetFolderSuggestions)
			r.Get("/autocomplete", handleAPIFilesAutocomplete)
			r.Get("/headers", handleAPIFilesHeaders)
			r.Get("/export/markdown", handleAPIExportToMarkdown)
			r.Get("/export/pdf", handleAPIExportToPDF)
			r.Post("/export/zip", handleAPIExportAllFiles)
			r.Post("/export/markdown-converted", handleAPIExportAllFilesWithMarkdownConversion)

			// file version routes
			r.Get("/versions/diff/*", handleAPIGetFileVersionDiff)
			r.Post("/versions/restore/*", handleAPIRestoreFileVersion)
			r.Get("/versions/*", handleAPIGetFileVersions)

			// file operations
			r.Post("/rename/*", handleAPIRenameFile)
			r.Post("/move-folder/*", handleAPIMoveFolderFile)
			r.Delete("/delete/*", handleAPIDeleteFile)
			r.Delete("/delete-folder/*", handleAPIDeleteFolder)
			r.Delete("/bulk", handleAPIDeleteFilesBulk)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- MEDIA -----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/media", func(r chi.Router) {
			r.Post("/upload", handleAPIMediaUpload)
			r.Get("/list", handleAPIGetAllMedia)
			r.Get("/autocomplete", handleAPIMediaAutocomplete)
			r.Get("/preview", handleAPIMediaPreview)
			r.Delete("/*", handleAPIDeleteMedia)
			r.Get("/stats", handleAPIMediaStats)
			r.Post("/cleanup-orphaned", handleAPICleanupOrphanedMedia)
			r.Post("/rename/*", handleAPIMediaRename)
			r.Get("/rename-form/*", handleAPIMediaRenameForm)
			r.Get("/path-display/*", handleAPIMediaPathDisplay)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- METADATA ---------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/metadata", func(r chi.Router) {
			r.Get("/", handleAPIGetMetadata)
			r.Post("/", handleAPISetMetadata)
			r.Post("/rebuild", handleAPIRebuildMetadata)
			r.Post("/rebuild/*", handleAPIRebuildFileMetadata)
			r.Post("/export", handleAPIExportMetadata)
			r.Post("/bulk-update", handleAPIBulkUpdateMetadata)
			r.Get("/broken-links", handleAPIScanBrokenLinks)
			r.Post("/broken-links/repair", handleAPIRepairBrokenLinks)

			r.Get("/collection", handleAPIGetMetadataCollection)
			r.Get("/editor", handleAPIGetMetadataEditor)
			r.Get("/path", handleAPIGetMetadataPath)
			r.Get("/createdat", handleAPIGetMetadataCreatedAt)
			r.Get("/lastedited", handleAPIGetMetadataLastEdited)
			r.Get("/references", handleAPIGetMetadataReferences)
			r.Post("/references", handleAPIAddMetadataReference)
			r.Delete("/references", handleAPIDeleteMetadataReference)

			r.Post("/collection", handleAPISetMetadataCollection)
			r.Post("/editor", handleAPISetMetadataEditor)
			r.Post("/path", handleAPISetMetadataPath)
			r.Post("/createdat", handleAPISetMetadataCreatedAt)
			r.Post("/lastedited", handleAPISetMetadataLastEdited)
			r.Post("/tags", handleAPISetMetadataTags)
			r.Post("/parents", handleAPISetMetadataParents)

			r.Get("/tags", handleAPIGetAllTags)
			r.Get("/collections", handleAPIGetAllCollections)
			r.Get("/folders", handleAPIGetAllFolders)
			r.Get("/titles", handleAPIGetAllTitles)
			r.Get("/editors", handleAPIGetAllEditors)
			r.Get("/tags/{fileId}", handleAPIGetFileMetadataTags)
			r.Get("/folders/{fileId}", handleAPIGetFileMetadataFolders)
			r.Get("/collection/{fileId}", handleAPIGetFileMetadataCollection)

			r.Get("/inline-display", handleAPIMetadataInlineDisplay)
			r.Get("/inline-edit", handleAPIMetadataInlineEdit)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- LINKS ------------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/links", func(r chi.Router) {
			r.Get("/parents", handleAPIGetParents)
			r.Get("/ancestors", handleAPIGetAncestors)
			r.Get("/ancestors-in-folder", handleAPIGetAncestorsInFolder)
			r.Get("/kids", handleAPIGetKids)
			r.Get("/grandchildren", handleAPIGetGrandchildren)
			r.Get("/used", handleAPIGetUsedLinks)
			r.Get("/linkstohere", handleAPIGetLinksToHere)
			r.Get("/media", handleAPIGetMediaLinks)
			r.Get("/related", handleAPIGetRelatedFiles)
			r.Get("/conflicts/diff", handleAPIGetConflictDiff)
			r.Get("/conflicts/banner", handleAPIGetConflictBanner)
			r.Get("/conflicts/of-banner", handleAPIGetConflictOfBanner)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- KANBAN ------------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/kanban", func(r chi.Router) {
			r.Get("/{board}", handleAPIGetKanbanBoard)
			r.Get("/{board}/archive", handleAPIGetKanbanArchive)
			r.Get("/{board}/events", handleAPIGetKanbanEvents)
			r.Get("/{board}/files", handleAPIGetKanbanFiles)
			r.Get("/{board}/tags", handleAPIGetKanbanTags)
			r.Post("/{board}/filter", handleAPIPostKanbanFilter)
			r.Post("/{board}/order", handleAPIKanbanSaveOrder)
			r.Post("/card/move", handleAPIKanbanMoveCard)
			r.Get("/excerpt", handleAPIGetKanbanExcerpt)
		})

		// ----------------------------------------------------------------------------------------
		// ------------------------------------ GIT Operations ------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/git", func(r chi.Router) {
			r.Get("/latestchanges", handleAPIGetRecentlyChanged)
			r.Post("/push", handleAPIGitPush)
			r.Post("/pull", handleAPIGitPull)
			r.Post("/test-auth", handleAPIGitTestAuth)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- DASHBOARDS -------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/dashboards", func(r chi.Router) {
			r.Get("/", handleAPIGetDashboards)
			r.Post("/", handleAPICreateDashboard)
			r.Post("/import", handleAPIImportDashboard)
			r.Get("/form", handleAPIDashboardForm)
			r.Post("/widget-form", handleAPIWidgetForm)
			r.Post("/widget-config", handleAPIWidgetConfig)
			r.Get("/widget-config", handleAPIWidgetConfig)
			r.Get("/{id}", handleAPIGetDashboard)
			r.Patch("/{id}", handleAPIUpdateDashboard)
			r.Delete("/{id}", handleAPIDeleteDashboard)
			r.Get("/{id}/export", handleAPIExportDashboard)
			r.Post("/{id}/rename", handleAPIRenameDashboard)
			r.Post("/widget/{id}", handleAPIRenderWidget)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- TESTDATA ---------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/testdata", func(r chi.Router) {
			r.Post("/setup", handleAPISetupTestData)
			r.Post("/clean", handleAPICleanTestData)
			r.Post("/filtertest", handleAPIFilterTest)
			r.Get("/filtertest/testdata", handleAPIFilterTestMetadata)
			r.Post("/editorstest", handleAPIEditorsTest)
			r.Post("/searchtest", handleAPISearchTest)
			r.Post("/githistorytest", handleAPIGitHistoryTest)
			r.Post("/chattest", handleAPIChatTest)
			r.Post("/dashboardtest", handleAPIDashboardTest)
			r.Post("/kanbantest", handleAPIKanbanTest)
			r.Post("/browsetest", handleAPIBrowseTest)
			r.Post("/metadatatest", handleAPIMetadataTest)
			r.Post("/connectionstest", handleAPIConnectionsTest)
			r.Post("/jobstest", handleAPIJobsTest)
			r.Post("/mediatest", handleAPIMediaTest)
			r.Post("/exporttest", handleAPIExportTest)
			r.Post("/notificationtest", handleAPINotificationTest)
			r.Post("/settingstest", handleAPISettingsTest)
			r.Post("/logstest", handleAPILogsTest)
			r.Post("/parsertest", handleAPIParserTest)
			r.Post("/run-all", handleAPIRunAllTests)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------- components routes ----------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/components", func(r chi.Router) {
			r.Get("/table", handleAPIGetTable)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------- chat routes ----------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/chat", func(r chi.Router) {
			r.Get("/messages", handleAPIGetChat)
			r.Post("/messages", handleAPIPostChatMessage)
			r.Delete("/messages/{id}", handleAPIDeleteChatMessage)
			r.Get("/messages/{id}", handleAPIGetChatByID)
			r.Get("/messages/{id}/move", handleAPIGetChatMoveForm)
			r.Post("/messages/{id}/move", handleAPIMoveChatMessage)
			r.Post("/messages/bulk/move", handleAPIBulkMoveChatMessages)
			r.Delete("/messages/bulk", handleAPIBulkDeleteChatMessages)
			r.Get("/bulk-form", handleAPIGetChatBulkForm)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------- notification routes ----------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/notifications", func(r chi.Router) {
			r.Get("/flash", handleAPIGetNotificationFlash)
			r.Get("/", handleAPIGetNotifications)
			r.Delete("/", handleAPIDeleteNotifications)
			r.Delete("/{id}", handleAPIDeleteNotification)
		})

	})

	return r
}
