// Package server ..
package server

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/pathutils"
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
	r.Get("/search", handleSearchPage)
	r.Get("/media", handleRedirectToBrowseMedia)
	r.Get("/media/*", handleMedia)

	r.Get("/files", handleRedirectToBrowseFiles)
	r.Get("/files/*", handleFileContent)
	r.Get("/files/edit/*", handleFileEdit)
	r.Get("/files/edittable/*", handleFileEditTable)
	r.Get("/files/history/*", handleHistory)
	r.Get("/files/new/markdown", handleFileNewMarkdown)
	r.Get("/files/new/text", handleFileNewText)
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
	r.Get("/kanban/{collection}", handleKanbanBoard)

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
		r.Get("/notifications/flash", handleAPIGetNotificationFlash)

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
			r.Get("/markdown-form", handleAPIMarkdownEditorForm)
			r.Get("/textarea", handleAPIGetTextareaEditor)
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
			r.Post("/media/default-preview-size", handleAPIUpdateDefaultPreviewSize)
			r.Post("/media/enable-previews", handleAPIUpdateEnablePreviews)
			r.Post("/media/display-mode", handleAPIUpdateDisplayMode)
			r.Post("/media/border-style", handleAPIUpdateBorderStyle)
			r.Post("/media/show-caption", handleAPIUpdateShowCaption)
			r.Post("/media/click-to-enlarge", handleAPIUpdateClickToEnlarge)

			// Editor settings endpoints
			r.Post("/section-edit-subheaders", handleAPIUpdateSectionEditSubheaders)
			r.Post("/code-block-wrap", handleAPIUpdateCodeBlockWrap)

			// Table display settings endpoints
			r.Post("/table/page-size", handleAPIUpdateTablePageSize)
			r.Post("/table/show-search", handleAPIUpdateTableShowSearch)
			r.Post("/table/show-info", handleAPIUpdateTableShowInfo)
			r.Post("/table/show-paging", handleAPIUpdateTableShowPaging)

			r.Post("/favicon", handleAPIUploadFavicon)
			r.Delete("/favicon", handleAPIDeleteFavicon)

			// File type visibility endpoints
			r.Post("/file-types/hide-markdown", handleAPIUpdateHideMarkdown)
			r.Post("/file-types/hide-text", handleAPIUpdateHideText)
			r.Post("/file-types/hide-list", handleAPIUpdateHideList)
			r.Post("/file-types/hide-todo", handleAPIUpdateHideTodo)
			r.Post("/file-types/hide-filter", handleAPIUpdateHideFilter)
			r.Post("/file-types/hide-index", handleAPIUpdateHideIndex)
			r.Post("/file-types/hide-image", handleAPIUpdateHideImage)
			r.Post("/file-types/hide-video", handleAPIUpdateHideVideo)
			r.Post("/file-types/hide-pdf", handleAPIUpdateHidePDF)
			r.Post("/file-types/hide-office-documents", handleAPIUpdateHideOfficeDocuments)
			r.Post("/file-types/hide-archives", handleAPIUpdateHideArchives)
			r.Post("/file-types/hide-executables", handleAPIUpdateHideExecutables)
			r.Post("/file-types/hide-scripts", handleAPIUpdateHideScripts)
			r.Post("/file-types/show-hidden", handleAPIUpdateShowHiddenFiles)
			r.Post("/home-dashboard", handleAPIUpdateHomeDashboard)
			r.Post("/extensions/todo", handleAPIUpdateUseExtensionTodo)
			r.Post("/extensions/list", handleAPIUpdateUseExtensionList)
			r.Post("/extensions/index", handleAPIUpdateUseExtensionIndex)
			r.Post("/log-level", handleAPIUpdateLogLevel)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- FILES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/files", func(r chi.Router) {
			r.Get("/list", handleAPIGetAllFiles)
			r.Get("/tree", handleAPIGetFileTree)
			r.Get("/content/*", handleAPIGetFileContent)
			r.Post("/filter", handleAPIFilterFiles)
			r.Get("/header", handleAPIGetFileHeader)
			r.Get("/raw", handleAPIGetRawContent)
			r.Post("/save", handleAPIFileSave)
			r.Post("/save/", handleAPIFileSave)
			r.Post("/section/save", handleAPISaveSectionEditor)
			r.Post("/convert-to-markdown", handleAPIConvertFileToMarkdown)
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
			r.Get("/list", handleAPIGetAllMedia)
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
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- LINKS ------------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/links", func(r chi.Router) {
			r.Get("/parents", handleAPIGetParents)
			r.Get("/ancestors", handleAPIGetAncestors)
			r.Get("/ancestors-in-collection", handleAPIGetAncestorsInCollection)
			r.Get("/kids", handleAPIGetKids)
			r.Get("/grandchildren", handleAPIGetGrandchildren)
			r.Get("/used", handleAPIGetUsedLinks)
			r.Get("/linkstohere", handleAPIGetLinksToHere)
			r.Get("/media", handleAPIGetMediaLinks)
			r.Get("/related", handleAPIGetRelatedFiles)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- KANBAN ------------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/kanban", func(r chi.Router) {
			r.Get("/{collection}", handleAPIGetKanbanBoard)
			r.Get("/{collection}/tags", handleAPIGetKanbanTags)
			r.Post("/card/move", handleAPIKanbanMoveCard)
			r.Get("/excerpt", handleAPIGetKanbanExcerpt)
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

// handleFavicon serves the custom favicon if one has been uploaded,
// otherwise falls back to the embedded static/favicon.ico.
func handleFavicon(w http.ResponseWriter, r *http.Request) {
	customPath := configmanager.GetCustomFaviconPath()
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			ext := strings.ToLower(filepath.Ext(customPath))
			switch ext {
			case ".svg":
				w.Header().Set("Content-Type", "image/svg+xml")
			case ".png":
				w.Header().Set("Content-Type", "image/png")
			default:
				w.Header().Set("Content-Type", "image/x-icon")
			}
			http.ServeFile(w, r, customPath)
			return
		}
	}
	// fall back to embedded default favicon
	r.URL.Path = "/static/favicon.ico"
	handleStatic(w, r)
}

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

	// set content type headers before serving files
	if ct := configmanager.MimeTypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	if basePath == "themes" {
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			logging.LogDebug("theme file not found: %s", fullPath)
			http.NotFound(w, r)
			return
		}

		// for CSS files, read and serve manually to ensure correct MIME type
		if ext == ".css" {
			cssData, err := os.ReadFile(fullPath)
			if err != nil {
				logging.LogError("failed to read theme CSS file %s: %v", fullPath, err)
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Write(cssData)
			logging.LogDebug("serving theme CSS file: %s", fullPath)
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
	if id := configmanager.GetAppConfig().HomeDashboard; id != "" {
		dash, err := dashboard.Get(id)
		if err != nil {
			logging.LogWarning("home dashboard %q not found, falling back to home page: %v", id, err)
		} else {
			tm := thememanager.GetThemeManager()
			data := thememanager.NewDashboardTemplateData(dash)
			if err := tm.Render(w, "dashboardview", data); err != nil {
				http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
			}
			return
		}
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("home")
	if err := tm.Render(w, "home", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
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

	if strings.HasPrefix(r.URL.Path, "/files/history/") {
		filePath := strings.TrimPrefix(r.URL.Path, "/files/history/")

		if filePath == "" {
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file path"), http.StatusBadRequest)
			return
		}

		fullPath := pathutils.ToFullPath(filePath)
		selectedCommit := r.URL.Query().Get("commit")

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

	data := thememanager.NewBaseTemplateData("history")

	err := tm.Render(w, "history", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileOverview(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("Files Overview")

	err := tm.Render(w, "filesoverview", data)
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

func handleBrowseMedia(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewMediaOverviewTemplateData()

	err := tm.Render(w, "mediaoverview", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleRedirectToBrowseMedia(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/browse/media", http.StatusPermanentRedirect)
}

func handleRedirectToBrowseFiles(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/browse/files", http.StatusPermanentRedirect)
}

func handleMedia(w http.ResponseWriter, r *http.Request) {
	mediaPath := chi.URLParam(r, "*")
	if mediaPath == "" {
		http.NotFound(w, r)
		return
	}

	if strings.HasPrefix(mediaPath, "http://") || strings.HasPrefix(mediaPath, "https://") {
		http.Redirect(w, r, mediaPath, http.StatusPermanentRedirect)
		return
	}

	fullPath := pathutils.ToMediaPath(mediaPath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		logging.LogWarning("media file not found: %s", fullPath)
		http.NotFound(w, r)
		return
	}

	if r.URL.Query().Get("mode") == "detail" {
		tm := thememanager.GetThemeManager()
		data := thememanager.NewMediaViewTemplateData(mediaPath)

		err := tm.Render(w, "mediaview", data)
		if err != nil {
			http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
			return
		}
		return
	}

	ext := strings.ToLower(filepath.Ext(mediaPath))
	if ct := configmanager.MimeTypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000")

	logging.LogDebug("serving media file: %s", fullPath)
	http.ServeFile(w, r, fullPath)
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
	fullPath := pathutils.ToDocsPath(filePath)
	ext := strings.ToLower(filepath.Ext(fullPath))

	if ext == ".pdf" {
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, fullPath)
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

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileViewTemplateData(filepath.Base(filePath), filePath, fileContent)

	err = tm.Render(w, "fileview", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileEdit(w http.ResponseWriter, r *http.Request) {
	filePath := pathutils.ToRelative(strings.TrimPrefix(r.URL.Path, "/files/edit/"))
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

func handleFileNewMarkdown(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("markdown-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleFileNewText(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("textarea-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleFileNewList(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("list-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleFileNewTodo(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("todo-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleFileNewFilter(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("filter-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleFileNewIndex(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileNewTemplateData("index-editor")
	if err := tm.Render(w, "filenew", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleFileEditTable(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/edittable/")

	tableIndex := 0
	if idxStr := r.URL.Query().Get("tableindex"); idxStr != "" {
		if idx, err := strconv.Atoi(idxStr); err == nil && idx >= 0 {
			tableIndex = idx
		}
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewFileEditTableTemplateData(filePath, tableIndex)

	err := tm.Render(w, "filedittable", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("chat")
	if err := tm.Render(w, "chat", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleKanbanSelect(w http.ResponseWriter, r *http.Request) {
	// try cache first, fall back to live data
	collections, err := files.GetAllCollectionsFromSystemData()
	if err != nil || len(collections) == 0 {
		allCols, liveErr := files.GetAllCollections()
		if liveErr == nil {
			collections = make([]string, 0, len(allCols))
			for c := range allCols {
				collections = append(collections, c)
			}
			slices.Sort(collections)
		}
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewKanbanSelectTemplateData(collections)
	if err := tm.Render(w, "kanban", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func handleKanbanBoard(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	tm := thememanager.GetThemeManager()
	data := thememanager.NewKanbanTemplateData(collection, nil)
	if err := tm.Render(w, "kanban", data); err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
	}
}
