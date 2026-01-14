// Package server ..
package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/server/render"
	_ "knov/internal/server/swagger" // swaggo api docs
	"knov/internal/thememanager"
	"knov/internal/translation"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

var staticFiles embed.FS

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
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------ template routes ------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/", handleHome)
	r.Get("/home", handleHome)
	r.Get("/settings", handleSettings)
	r.Get("/admin", handleAdmin)
	r.Get("/playground", handlePlayground)
	r.Get("/help", handleHelp)
	r.Get("/latest-changes", handleLatestChanges)
	r.Get("/history", handleHistory)
	r.Get("/files/history/*", handleHistory)
	r.Get("/overview", handleOverview)
	r.Get("/search", handleSearchPage)
	r.Get("/files/edit/*", handleFileEdit)
	r.Get("/files/edittable/*", handleFileEditTable)

	// use filenew template
	r.Get("/files/new/todo", handleFileNewTodo)
	r.Get("/files/new/fleeting", handleFileNewFleeting)
	r.Get("/files/new/literature", handleFileNewLiterature)
	r.Get("/files/new/moc", handleFileNewMOC)
	r.Get("/files/new/permanent", handleFileNewPermanent)
	r.Get("/files/new/filter", handleFileNewFilter)
	r.Get("/files/new/journaling", handleFileNewJournaling)

	r.Get("/files/*", handleFileContent)
	r.Get("/dashboard", handleDashboardView)
	r.Get("/dashboard/{id}", handleDashboardView)
	r.Get("/dashboard/new", handleDashboardNew)
	r.Get("/dashboard/edit/{id}", handleDashboardEdit)
	r.Get("/browse", handleBrowse)
	r.Get("/browse/{metadata}", handleBrowseMetadata)
	r.Get("/browse/{metadata}/{value}", handleBrowseFiles)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------- static routes -------------------------------------
	// ----------------------------------------------------------------------------------------

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

		r.Route("/filter", func(r chi.Router) {
			r.Post("/", handleAPIFilterFiles)
			r.Get("/form", handleAPIGetFilterForm)
			r.Get("/value-input", handleAPIGetFilterValueInput)
			r.Post("/add-criteria", handleAPIAddFilterCriteria)
			r.Post("/save", handleAPIFilterSave)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- EDITOR ----------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/editor", func(r chi.Router) {
			r.Get("/", handleAPIGetEditorHandler)
			r.Get("/markdown-form", handleAPIMarkdownEditorForm)
			r.Get("/textarea", handleAPIGetTextareaEditor)
			r.Post("/indexeditor", handleAPISaveIndexEditor)
			r.Post("/indexeditor/add-entry", handleAPIAddIndexEntry)
			r.Post("/filtereditor", handleAPISaveFilterEditor)
			r.Post("/listeditor", handleAPISaveListEditor)
			r.Post("/tableeditor", handleAPITableEditorSave)
			r.Get("/tableeditor", handleAPITableEditorForm)
		})

		// ----------------------------------------------------------------------------------------
		// ------------------------------------ system routes ------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/system", func(r chi.Router) {
			r.Post("/restart", handleAPIRestartApp)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- CRONJOB ----------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Post("/cronjob", handleAPIRunCronjob)

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

			// POST
			r.Post("/language", handleAPISetLanguage)
			r.Post("/repository", handleAPISetGitRepositoryURL)
			r.Post("/datapath", handleAPISetDataPath)

			// Media settings endpoints
			r.Post("/media/upload-size", handleAPIUpdateMediaUploadSize)
			r.Post("/media/mime-types", handleAPIUpdateMediaMimeTypes)
			r.Post("/media/orphaned-behavior", handleAPIUpdateOrphanedBehavior)
			r.Post("/media/orphaned-age", handleAPIUpdateOrphanedAge)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- FILES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/files", func(r chi.Router) {
			r.Get("/list", handleAPIGetAllFiles)
			r.Get("/content/*", handleAPIGetFileContent)
			r.Post("/filter", handleAPIFilterFiles)
			r.Get("/header", handleAPIGetFileHeader)
			r.Get("/raw", handleAPIGetRawContent)
			r.Post("/save", handleAPIFileSave)
			r.Post("/save/", handleAPIFileSave)
			r.Post("/section/save", handleAPISaveSectionEditor)
			r.Get("/browse", handleAPIBrowseFiles)
			r.Get("/form", handleAPIFileForm)
			r.Get("/metadata-form", handleAPIMetadataForm)
			r.Get("/folder", handleAPIGetFolder)
			r.Get("/folder-suggestions", handleAPIGetFolderSuggestions)
			r.Get("/export/markdown", handleAPIExportToMarkdown)
			r.Post("/export/zip", handleAPIExportAllFiles)
			r.Post("/export/markdown-converted", handleAPIExportAllFilesWithMarkdownConversion)

			// file version routes
			r.Get("/versions/diff/*", handleAPIGetFileVersionDiff)
			r.Post("/versions/restore/*", handleAPIRestoreFileVersion)
			r.Get("/versions/*", handleAPIGetFileVersions)

			// file operations
			r.Post("/rename/*", handleAPIRenameFile)
			r.Delete("/delete/*", handleAPIDeleteFile)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- MEDIA -----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/media", func(r chi.Router) {
			r.Post("/upload", handleAPIMediaUpload)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- METADATA ---------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/metadata", func(r chi.Router) {
			r.Get("/", handleAPIGetMetadata)
			r.Post("/", handleAPISetMetadata)
			r.Post("/rebuild", handleAPIRebuildMetadata)
			r.Post("/export", handleAPIExportMetadata)

			r.Get("/collection", handleAPIGetMetadataCollection)
			r.Get("/filetype", handleAPIGetMetadataFileType)
			r.Get("/path", handleAPIGetMetadataPath)
			r.Get("/createdat", handleAPIGetMetadataCreatedAt)
			r.Get("/lastedited", handleAPIGetMetadataLastEdited)
			r.Get("/priority", handleAPIGetMetadataPriority)
			r.Get("/status", handleAPIGetMetadataStatus)
			r.Get("/targetdate", handleAPIGetMetadataTargetDate)

			r.Post("/collection", handleAPISetMetadataCollection)
			r.Post("/filetype", handleAPISetMetadataFileType)
			r.Post("/status", handleAPISetMetadataStatus)
			r.Post("/priority", handleAPISetMetadataPriority)
			r.Post("/path", handleAPISetMetadataPath)
			r.Post("/createdat", handleAPISetMetadataCreatedAt)
			r.Post("/lastedited", handleAPISetMetadataLastEdited)
			r.Post("/targetdate", handleAPISetMetadataTargetDate)
			r.Post("/folders", handleAPISetMetadataFolders)
			r.Post("/tags", handleAPISetMetadataTags)
			r.Post("/parents", handleAPISetMetadataParents)
			r.Post("/para/projects", handleAPISetMetadataPARAProjects)
			r.Post("/para/areas", handleAPISetMetadataPARAreas)
			r.Post("/para/resources", handleAPISetMetadataPARAResources)
			r.Post("/para/archive", handleAPISetMetadataPARAArchive)

			r.Get("/tags", handleAPIGetAllTags)
			r.Get("/collections", handleAPIGetAllCollections)
			r.Get("/folders", handleAPIGetAllFolders)
			r.Get("/priorities", handleAPIGetAllPriorities)
			r.Get("/statuses", handleAPIGetAllStatuses)
			r.Get("/filetypes", handleAPIGetAllFiletypes)
			r.Get("/tags/{fileId}", handleAPIGetFileMetadataTags)
			r.Get("/folders/{fileId}", handleAPIGetFileMetadataFolders)
			r.Get("/collection/{fileId}", handleAPIGetFileMetadataCollection)

			r.Get("/para/projects", handleAPIGetAllPARAProjects)
			r.Get("/para/areas", handleAPIGetAllPARAreas)
			r.Get("/para/resources", handleAPIGetAllPARAResources)
			r.Get("/para/archive", handleAPIGetAllPARAArchive)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- LINKS ------------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/links", func(r chi.Router) {
			r.Get("/parents", handleAPIGetParents)
			r.Get("/ancestors", handleAPIGetAncestors)
			r.Get("/kids", handleAPIGetKids)
			r.Get("/used", handleAPIGetUsedLinks)
			r.Get("/linkstohere", handleAPIGetLinksToHere)
		})

		// ----------------------------------------------------------------------------------------
		// ------------------------------------ GIT Operations ------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/git", func(r chi.Router) {
			r.Get("/latestchanges", handleAPIGetRecentlyChanged)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- DASHBOARDS -------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/dashboards", func(r chi.Router) {
			r.Get("/", handleAPIGetDashboards)
			r.Post("/", handleAPICreateDashboard)
			r.Get("/form", handleAPIDashboardForm)
			r.Post("/widget-form", handleAPIWidgetForm)
			r.Post("/widget-config", handleAPIWidgetConfig)
			r.Get("/widget-config", handleAPIWidgetConfig)
			r.Get("/{id}", handleAPIGetDashboard)
			r.Patch("/{id}", handleAPIUpdateDashboard)
			r.Delete("/{id}", handleAPIDeleteDashboard)
			r.Post("/widget/{id}", handleAPIRenderWidget)
			r.Post("/{id}/rename", handleAPIRenameDashboard)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- TESTDATA ---------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/testdata", func(r chi.Router) {
			r.Post("/setup", handleAPISetupTestData)
			r.Post("/clean", handleAPICleanTestData)
			r.Post("/filtertest", handleAPIFilterTest)
			r.Get("/filtertest/testdata", handleAPIFilterTestMetadata)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------- components routes ----------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/components", func(r chi.Router) {
			r.Get("/table", handleAPIGetTable)
		})
	})

	// ----------------------------------------------------------------------------------------
	// ----------------------------------- start chi server -----------------------------------
	// ----------------------------------------------------------------------------------------

	err := http.ListenAndServe(":"+port, r)
	if err != nil {
		fmt.Printf("error starting chi server: %v\n", err)
		return
	}
}

// ----------------------------------------------------------------------------------------
// ---------------------------------------- helper ----------------------------------------
// ----------------------------------------------------------------------------------------

func handleStatic(w http.ResponseWriter, r *http.Request) {
	var basePath, filePath, fullPath string

	if strings.HasPrefix(r.URL.Path, "/static/") {
		basePath = "static"
		filePath = strings.TrimPrefix(r.URL.Path, "/static/")
		fullPath = filepath.ToSlash(filepath.Join(basePath, filePath))
	} else if strings.HasPrefix(r.URL.Path, "/themes/") {
		basePath = "themes"
		filePath = strings.TrimPrefix(r.URL.Path, "/themes/")
		fullPath = filepath.Join(basePath, filePath)
	} else {
		http.NotFound(w, r)
		return
	}

	if basePath == "static" && strings.HasPrefix(filePath, "css/") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		cssFile := strings.TrimPrefix(filePath, "css/")

		if cssFile == "custom.css" {
			currentTheme := configmanager.GetUserSettings().Theme
			customCSS := ""
			if val := configmanager.GetThemeSetting(currentTheme, "customCSS"); val != nil {
				if css, ok := val.(string); ok {
					customCSS = css
				}
			}
			w.Write([]byte(customCSS))
			return
		}
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".woff":
		w.Header().Set("Content-Type", "font/woff")
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	case ".otf":
		w.Header().Set("Content-Type", "font/otf")
	case ".eot":
		w.Header().Set("Content-Type", "application/vnd.ms-fontobject")
	}

	if basePath == "themes" {
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			logging.LogDebug("theme file not found: %s", fullPath)
			http.NotFound(w, r)
			return
		}
		logging.LogDebug("serving theme file: %s", fullPath)
		http.ServeFile(w, r, fullPath)
	} else {
		data, err := staticFiles.ReadFile(fullPath)
		if err != nil {
			fmt.Printf("failed to read embedded file %s: %v\n", fullPath, err)
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}
}

// handleWebfontsRedirect redirects /webfonts/* requests to /static/webfonts/*
func handleWebfontsRedirect(w http.ResponseWriter, r *http.Request) {
	fontPath := strings.TrimPrefix(r.URL.Path, "/webfonts/")
	newPath := "/static/webfonts/" + fontPath

	// create new request for the static handler
	newURL := *r.URL
	newURL.Path = newPath

	newReq := r.Clone(r.Context())
	newReq.URL = &newURL

	handleStatic(w, newReq)
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ default routes ------------------------------------
// ----------------------------------------------------------------------------------------

func handleHome(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("home")

	err := tm.Render(w, "home", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewSettingsTemplateData()

	err := tm.Render(w, "settings", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	// viewName removed
	data := thememanager.NewBaseTemplateData("Admin")

	err := tm.Render(w, "admin", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleHelp(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("help")

	err := tm.Render(w, "help", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("playground")

	err := tm.Render(w, "playground", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleLatestChanges(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("latestchanges")

	err := tm.Render(w, "latestchanges", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	// check if this is a file history request
	if strings.HasPrefix(r.URL.Path, "/files/history/") {
		filePath := strings.TrimPrefix(r.URL.Path, "/files/history/")

		if filePath == "" {
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"), http.StatusBadRequest)
			return
		}

		// get full file path
		fullPath := filepath.Join(configmanager.GetAppConfig().DataPath, filePath)

		// get selected commit from query param
		selectedCommit := r.URL.Query().Get("commit")

		// get file history
		versions, err := git.GetFileHistory(fullPath)
		if err != nil {
			logging.LogError("failed to get file history for %s: %v", filePath, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file history"), http.StatusInternalServerError)
			return
		}

		currentCommit := ""
		if len(versions) > 0 {
			currentCommit = versions[0].Commit
		}

		data := thememanager.NewHistoryTemplateData(filePath, currentCommit, selectedCommit, versions, false)

		err = tm.Render(w, "history", data)
		if err != nil {
			http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
			return
		}
		return
	}

	// general history page
	data := thememanager.NewBaseTemplateData("history")

	err := tm.Render(w, "history", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleOverview(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("overview")

	err := tm.Render(w, "overview", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleSearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	tm := thememanager.GetThemeManager()
	data := thememanager.NewSearchPageData(query)

	err := tm.Render(w, "search", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleBrowseFiles(w http.ResponseWriter, r *http.Request) {
	metadataType := chi.URLParam(r, "metadata")
	value := chi.URLParam(r, "value")

	if metadataType == "" || value == "" {
		http.Error(w, "missing metadata type or value", http.StatusBadRequest)
		return
	}

	// query := fmt.Sprintf("%s:%s", metadataType, value)

	tm := thememanager.GetThemeManager()
	title := fmt.Sprintf("Browse: %s", value)
	data := thememanager.NewBrowseFilesTemplateData(metadataType, value)
	data.Title = title

	err := tm.Render(w, "browsefiles", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleBrowse(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("Browse")

	err := tm.Render(w, "browse", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleBrowseMetadata(w http.ResponseWriter, r *http.Request) {
	metadataType := chi.URLParam(r, "metadata")

	if metadataType == "" {
		http.Error(w, "missing metadata type", http.StatusBadRequest)
		return
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewBrowseMetadataTemplateData(metadataType)

	err := tm.Render(w, "browsemetadata", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleDashboardNew(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("Create New Dashboard")

	err := tm.Render(w, "dashboardnew", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleDashboardEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	dash, err := dashboard.Get(id)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "dashboard not found"), http.StatusNotFound)
		return
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewDashboardEditTemplateData(dash)

	err = tm.Render(w, "dashboardedit", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleDashboardView(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = "home"
	}

	dash, err := dashboard.Get(id)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewDashboardTemplateData(dash)

	err = tm.Render(w, "dashboardview", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/")
	fullPath := contentStorage.ToDocsPath(filePath)
	ext := strings.ToLower(filepath.Ext(fullPath))

	if ext == ".pdf" {
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, fullPath)
		return
	}

	if ext == ".filter" {
		handleFilterFileContent(w, r, filePath, fullPath)
		return
	}

	fileContent, err := files.GetFileContent(fullPath)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file content"), http.StatusInternalServerError)
		return
	}

	if r.URL.Query().Get("snippet") == "true" || r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fileContent.HTML))
		return
	}

	// For full page requests, render through template system
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileViewTemplateData(filepath.Base(filePath), filePath, fileContent)

	// Always render through base template, not individual views
	err = tm.Render(w, "fileview", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFilterFileContent(w http.ResponseWriter, r *http.Request, filePath, fullPath string) {
	// read the JSON filter configuration
	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, "failed to read filter file", http.StatusInternalServerError)
		logging.LogError("failed to read filter file %s: %v", fullPath, err)
		return
	}

	// parse the filter configuration
	var config filter.Config
	if len(content) == 0 {
		// use default configuration for empty files
		config = filter.Config{
			Criteria: []filter.Criteria{},
			Logic:    "and",
			Display:  "list",
			Limit:    50,
		}
		logging.LogInfo("using default configuration for empty filter file: %s", fullPath)
	} else {
		if err := json.Unmarshal(content, &config); err != nil {
			http.Error(w, "invalid filter configuration", http.StatusInternalServerError)
			logging.LogError("failed to parse filter config in %s: %v", fullPath, err)
			return
		}
	}

	// validate the filter configuration
	if err := filter.ValidateConfig(&config); err != nil {
		http.Error(w, "invalid filter configuration", http.StatusInternalServerError)
		logging.LogError("invalid filter config in %s: %v", fullPath, err)
		return
	}

	// execute the filter
	result, err := filter.FilterFilesWithConfig(&config)
	if err != nil {
		http.Error(w, "failed to execute filter", http.StatusInternalServerError)
		logging.LogError("failed to execute filter from %s: %v", fullPath, err)
		return
	}

	// render the filter results
	var html string
	if r.URL.Query().Get("snippet") == "true" || r.Header.Get("HX-Request") == "true" {
		// for snippet requests, just return the filter results HTML
		html = render.RenderFilterResult(result, config.Display)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	// for full page requests, create a file content structure with the results
	resultsHTML := render.RenderFilterResult(result, config.Display)
	filterTitle := fmt.Sprintf("Filter: %s", filepath.Base(filePath))

	// create a synthetic file content structure
	fileContent := &files.FileContent{
		HTML: fmt.Sprintf(`<div class="filter-file-view">
			<h2>%s</h2>
			%s
		</div>`,
			filterTitle,
			resultsHTML),
		TOC: []files.TOCItem{},
	}

	// render through template system
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileViewTemplateData(filterTitle, filePath, fileContent)

	err = tm.Render(w, "fileview", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileEdit(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/edit/")
	sectionID := r.URL.Query().Get("section")

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileEditTemplateData(filePath, sectionID)

	err := tm.Render(w, "fileedit", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

// ----------------------------------------------------------------------------------------
// -------------------------------- Filetype-specific handlers ---------------------------
// ----------------------------------------------------------------------------------------

func handleFileNewTodo(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("todo")

	err := tm.Render(w, "filenew", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileNewFleeting(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("fleeting")

	err := tm.Render(w, "filenew", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileNewLiterature(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("literature")

	err := tm.Render(w, "filenew", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileNewMOC(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("moc")

	err := tm.Render(w, "filenew", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileNewPermanent(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("permanent")

	err := tm.Render(w, "filenew", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileNewFilter(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("filter")

	err := tm.Render(w, "filenew", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileNewJournaling(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("journaling")

	err := tm.Render(w, "filenew", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileEditTable(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/edittable/")

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileEditTableTemplateData(filePath)

	err := tm.Render(w, "filedittable", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}
