package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get current configuration
// @Tags config
// @Produce json,html
// @Router /api/config [get]
func handleAPIGetConfig(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	userSettings := configmanager.GetUserSettings()

	config := struct {
		App  configmanager.AppConfig    `json:"app"`
		User configmanager.UserSettings `json:"user"`
	}{
		App:  appConfig,
		User: userSettings,
	}

	html := render.RenderConfigDisplay(userSettings, appConfig)
	writeResponse(w, r, config, html)
}

// @Summary Get current data path as input field
// @Tags config
// @Produce html
// @Router /api/config/datapath [get]
func handleAPIGetCurrentDataPath(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	dataPath := appConfig.DataPath

	html := render.RenderInputField("text", "dataPath", "data-path", dataPath, translation.SprintfForRequest(configmanager.GetLanguage(), "/path/to/data"), true)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Set language
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/language [post]
func handleAPISetLanguage(w http.ResponseWriter, r *http.Request) {
	lang := r.FormValue("language")

	logging.LogDebug("language set to: %s", lang)

	if lang != "" {
		configmanager.SetLanguage(lang)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// @Summary Get git repository URL
// @Tags config
// @Produce json,html
// @Success 200 {object} string
// @Router /api/config/repository [get]
func handleAPIGetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	repositoryURL := appConfig.GitRepoURL

	html := render.RenderInputField("url", "repositoryURL", "git-url", repositoryURL, translation.SprintfForRequest(configmanager.GetLanguage(), "https://github.com/user/repo.git"), false)
	writeResponse(w, r, repositoryURL, html)
}

// @Summary Update git repository URL
// @Description updates git repository url in .env file (requires restart)
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param repositoryUrl formData string true "repository url"
// @Produce json,html
// @Success 200 {string} string "saved"
// @Router /api/config/repository [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	repositoryURL := r.FormValue("repositoryUrl")

	if err := configmanager.UpdateEnvFile("KNOV_GIT_REPO_URL", repositoryURL); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save"), http.StatusInternalServerError)
		return
	}

	data := "saved"
	html := render.RenderStatusMessage(render.StatusOK, translation.SprintfForRequest(configmanager.GetLanguage(), "git url saved. restart required."))
	writeResponse(w, r, data, html)
}

// @Summary Restart application
// @Description Restarts the application (requires process manager like systemd or docker)
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {string} string "restarting"
// @Router /api/system/restart [post]
func handleAPIRestartApp(w http.ResponseWriter, r *http.Request) {
	logging.LogInfo("application restart requested")

	data := "restarting"
	html := render.RenderStatusMessage(render.StatusOK, translation.SprintfForRequest(configmanager.GetLanguage(), "restarting application..."))
	writeResponse(w, r, data, html)

	// give response time to send
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

// @Summary Update data path
// @Description updates data path in .env file (requires restart)
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param dataPath formData string true "data path"
// @Produce json,html
// @Success 200 {string} string "saved"
// @Router /api/config/datapath [post]
func handleAPISetDataPath(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	dataPath := r.FormValue("dataPath")

	if dataPath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "data path cannot be empty"), http.StatusBadRequest)
		return
	}

	if err := configmanager.UpdateEnvFile("KNOV_DATA_PATH", dataPath); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save"), http.StatusInternalServerError)
		return
	}

	data := "saved"
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "data path saved. restart required."))
	writeResponse(w, r, data, html)
}

// @Summary Get available languages
// @Tags config
// @Produce json,html
// @Router /api/config/languages [get]
func handleAPIGetLanguages(w http.ResponseWriter, r *http.Request) {
	languages := configmanager.GetAvailableLanguages()
	currentLang := configmanager.GetLanguage()

	options := render.GetLanguageOptions()
	html := render.RenderSelectOptions(options, currentLang)
	writeResponse(w, r, languages, html)
}

// @Summary Update media upload size limit
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param maxUploadSizeMB formData int true "Maximum upload size in MB"
// @Produce json,html
// @Router /api/config/media/upload-size [post]
func handleAPIUpdateMediaUploadSize(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	sizeStr := r.FormValue("maxUploadSizeMB")
	if sizeStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing upload size"), http.StatusBadRequest)
		return
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 || size > 100 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid upload size"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.MaxUploadSizeMB = size

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated media upload size to %d MB", size)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "upload size updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update allowed MIME types
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param allowedMimeTypes formData string true "Comma-separated list of allowed MIME types"
// @Produce json,html
// @Router /api/config/media/mime-types [post]
func handleAPIUpdateMediaMimeTypes(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	mimeTypesStr := r.FormValue("allowedMimeTypes")
	if mimeTypesStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing mime types"), http.StatusBadRequest)
		return
	}

	// parse comma-separated list
	var mimeTypes []string
	for _, mimeType := range strings.Split(mimeTypesStr, ",") {
		trimmed := strings.TrimSpace(mimeType)
		if trimmed != "" {
			mimeTypes = append(mimeTypes, trimmed)
		}
	}

	if len(mimeTypes) == 0 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "at least one mime type required"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.AllowedMimeTypes = mimeTypes

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated allowed mime types: %v", mimeTypes)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "mime types updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update orphaned media behavior
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param orphanedMediaBehavior formData string true "Orphaned media behavior (keep or manual)"
// @Produce json,html
// @Router /api/config/media/orphaned-behavior [post]
func handleAPIUpdateOrphanedBehavior(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	behavior := r.FormValue("orphanedMediaBehavior")
	if behavior != "keep" && behavior != "manual" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid behavior option"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.OrphanedMediaBehavior = behavior

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated orphaned media behavior to: %s", behavior)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "orphaned media behavior updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update orphaned media age
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param orphanedMediaAgeDays formData int true "Number of days before orphaned files can be cleaned up"
// @Produce json,html
// @Router /api/config/media/orphaned-age [post]
func handleAPIUpdateOrphanedAge(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	ageStr := r.FormValue("orphanedMediaAgeDays")
	if ageStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing age value"), http.StatusBadRequest)
		return
	}

	age, err := strconv.Atoi(ageStr)
	if err != nil || age < 1 || age > 365 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid age value"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.OrphanedMediaAgeDays = age

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated orphaned media age to %d days", age)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "orphaned media age updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update section edit include subheaders setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param sectionEditIncludeSubheaders formData bool true "Whether section editing should include subheaders"
// @Produce json,html
// @Router /api/config/section-edit-subheaders [post]
func handleAPIUpdateSectionEditSubheaders(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	includeSubheaders := r.FormValue("sectionEditIncludeSubheaders") == "true"

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.SectionEditIncludeSubheaders = includeSubheaders

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated section edit include subheaders to: %t", includeSubheaders)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "section edit setting updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update default preview size
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param defaultPreviewSize formData int true "Default preview size in pixels"
// @Produce json,html
// @Router /api/config/media/default-preview-size [post]
func handleAPIUpdateDefaultPreviewSize(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	sizeStr := r.FormValue("defaultPreviewSize")
	if sizeStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing preview size"), http.StatusBadRequest)
		return
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 50 || size > 1000 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid preview size"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.DefaultPreviewSize = size
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated default preview size to %d pixels", size)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "preview size updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update preview display mode
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param displayMode formData string true "Display mode: left, center, right, inline"
// @Produce json,html
// @Router /api/config/media/display-mode [post]
func handleAPIUpdateDisplayMode(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	displayMode := r.FormValue("displayMode")
	if displayMode == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing display mode"), http.StatusBadRequest)
		return
	}

	validModes := []string{"left", "center", "right", "inline"}
	isValid := false
	for _, mode := range validModes {
		if displayMode == mode {
			isValid = true
			break
		}
	}
	if !isValid {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid display mode"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.DisplayMode = displayMode
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated display mode to: %s", displayMode)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "display mode updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update preview border style
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param borderStyle formData string true "Border style: none, simple, rounded, shadow"
// @Produce json,html
// @Router /api/config/media/border-style [post]
func handleAPIUpdateBorderStyle(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	borderStyle := r.FormValue("borderStyle")
	if borderStyle == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing border style"), http.StatusBadRequest)
		return
	}

	validStyles := []string{"none", "simple", "rounded", "shadow"}
	isValid := false
	for _, style := range validStyles {
		if borderStyle == style {
			isValid = true
			break
		}
	}
	if !isValid {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid border style"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.BorderStyle = borderStyle
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated border style to: %s", borderStyle)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "border style updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update show caption setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param showCaption formData bool true "Whether to show captions"
// @Produce json,html
// @Router /api/config/media/show-caption [post]
func handleAPIUpdateShowCaption(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	showCaption := r.FormValue("showCaption") == "true"

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.ShowCaption = showCaption
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated show caption to: %t", showCaption)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "caption setting updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update click to enlarge setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param clickToEnlarge formData bool true "Whether previews are clickable"
// @Produce json,html
// @Router /api/config/media/click-to-enlarge [post]
func handleAPIUpdateClickToEnlarge(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	clickToEnlarge := r.FormValue("clickToEnlarge") == "true"

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.ClickToEnlarge = clickToEnlarge
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated click to enlarge to: %t", clickToEnlarge)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "click setting updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update preview enabled setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param enablePreviews formData bool true "Whether previews are enabled"
// @Produce json,html
// @Router /api/config/media/enable-previews [post]
func handleAPIUpdateEnablePreviews(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	enablePreviews := r.FormValue("enablePreviews") == "true"

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.EnablePreviews = enablePreviews
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated enable previews to: %t", enablePreviews)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "preview setting updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide todo files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideTodo formData bool true "Whether to hide todo files"
// @Produce json,html
// @Router /api/config/file-types/hide-todo [post]
func handleAPIUpdateHideTodo(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hideTodo := r.FormValue("hideTodo") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_TODO", fmt.Sprintf("%t", hideTodo))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide todo to: %t", hideTodo)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "todo file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide fleeting files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideFleeting formData bool true "Whether to hide fleeting files"
// @Produce json,html
// @Router /api/config/file-types/hide-fleeting [post]
func handleAPIUpdateHideFleeting(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hideFleeting := r.FormValue("hideFleeting") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_FLEETING", fmt.Sprintf("%t", hideFleeting))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide fleeting to: %t", hideFleeting)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "fleeting file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide literature files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideLiterature formData bool true "Whether to hide literature files"
// @Produce json,html
// @Router /api/config/file-types/hide-literature [post]
func handleAPIUpdateHideLiterature(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hideLiterature := r.FormValue("hideLiterature") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_LITERATURE", fmt.Sprintf("%t", hideLiterature))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide literature to: %t", hideLiterature)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "literature file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide moc files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideMOC formData bool true "Whether to hide moc files"
// @Produce json,html
// @Router /api/config/file-types/hide-moc [post]
func handleAPIUpdateHideMOC(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hideMOC := r.FormValue("hideMOC") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_MOC", fmt.Sprintf("%t", hideMOC))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide moc to: %t", hideMOC)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "moc file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide permanent files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hidePermanent formData bool true "Whether to hide permanent files"
// @Produce json,html
// @Router /api/config/file-types/hide-permanent [post]
func handleAPIUpdateHidePermanent(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hidePermanent := r.FormValue("hidePermanent") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_PERMANENT", fmt.Sprintf("%t", hidePermanent))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide permanent to: %t", hidePermanent)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "permanent file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide filter files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideFilter formData bool true "Whether to hide filter files"
// @Produce json,html
// @Router /api/config/file-types/hide-filter [post]
func handleAPIUpdateHideFilter(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hideFilter := r.FormValue("hideFilter") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_FILTER", fmt.Sprintf("%t", hideFilter))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide filter to: %t", hideFilter)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "filter file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide journaling files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideJournaling formData bool true "Whether to hide journaling files"
// @Produce json,html
// @Router /api/config/file-types/hide-journaling [post]
func handleAPIUpdateHideJournaling(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hideJournaling := r.FormValue("hideJournaling") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_JOURNALING", fmt.Sprintf("%t", hideJournaling))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide journaling to: %t", hideJournaling)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "journaling file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide image files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideImage formData bool true "Whether to hide image files"
// @Produce json,html
// @Router /api/config/file-types/hide-image [post]
func handleAPIUpdateHideImage(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hideImage := r.FormValue("hideImage") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_IMAGE", fmt.Sprintf("%t", hideImage))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide image to: %t", hideImage)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "image file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide video files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideVideo formData bool true "Whether to hide video files"
// @Produce json,html
// @Router /api/config/file-types/hide-video [post]
func handleAPIUpdateHideVideo(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hideVideo := r.FormValue("hideVideo") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_VIDEO", fmt.Sprintf("%t", hideVideo))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide video to: %t", hideVideo)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "video file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide pdf files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hidePDF formData bool true "Whether to hide pdf files"
// @Produce json,html
// @Router /api/config/file-types/hide-pdf [post]
func handleAPIUpdateHidePDF(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hidePDF := r.FormValue("hidePDF") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_PDF", fmt.Sprintf("%t", hidePDF))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide pdf to: %t", hidePDF)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "pdf file visibility updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update hide text files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideText formData bool true "Whether to hide text files"
// @Produce json,html
// @Router /api/config/file-types/hide-text [post]
func handleAPIUpdateHideText(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	hideText := r.FormValue("hideText") == "true"

	err = configmanager.UpdateEnvFile("KNOV_HIDE_TEXT", fmt.Sprintf("%t", hideText))
	if err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("updated hide text to: %t", hideText)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "text file visibility updated"))
	writeResponse(w, r, "saved", html)
}
